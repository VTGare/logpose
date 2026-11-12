package cards

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeTCGCSV serves tcgcsv's shape for two sets and counts product fetches per set. The set
// in down fails.
type fakeTCGCSV struct {
	mu       sync.Mutex
	modified map[int]string
	fetches  map[int]int
	down     int
}

func (f *fakeTCGCSV) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == fmt.Sprintf("/%d/products", f.down) {
		http.Error(w, "down", http.StatusInternalServerError)
		return
	}
	switch r.URL.Path {
	case "/groups":
		_, _ = fmt.Fprintf(w, `{"results":[{"groupId":1,"modifiedOn":%q},{"groupId":2,"modifiedOn":%q}]}`, f.modified[1], f.modified[2])
	case "/1/products":
		f.fetches[1]++
		_, _ = fmt.Fprint(w, `{"results":[
			{"name":"Nico Robin (010)","extendedData":[{"name":"Number","value":"EB05-010"},{"name":"CardType","value":"Leader"},{"name":"Color","value":"Green;Yellow"}]},
			{"name":"Nico Robin (010) (Alternate Art)","extendedData":[{"name":"Number","value":"EB05-010"},{"name":"CardType","value":"Leader"},{"name":"Color","value":"Green;Yellow"}]},
			{"name":"Booster Box","extendedData":[]}]}`)
	case "/2/products":
		f.fetches[2]++
		_, _ = fmt.Fprint(w, `{"results":[{"name":"Spandam (002)","extendedData":[{"name":"Number","value":"OP18-002"},{"name":"CardType","value":"Leader"},{"name":"Color","value":"Black"},{"name":"Power","value":"5000"}]}]}`)
	default:
		http.NotFound(w, r)
	}
}

var _ = Describe("Catalog", func() {
	var (
		ctx  context.Context
		fake *fakeTCGCSV
		srv  *httptest.Server
		path string
		now  = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	)

	BeforeEach(func() {
		ctx = context.Background()
		fake = &fakeTCGCSV{modified: map[int]string{1: "a", 2: "a"}, fetches: make(map[int]int)}
		srv = httptest.NewServer(fake)
		DeferCleanup(srv.Close)
		path = filepath.Join(GinkgoT().TempDir(), "cards.json")
	})

	It("fetches every set the first time", func() {
		c, err := LoadCatalog(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Refresh(ctx, srv.Client(), srv.URL, now)).To(Equal(2))

		db := NewDB()
		c.Apply(db)
		Expect(db.Card("EB05-010")).To(Equal(Card{Code: "EB05-010", Name: "Nico Robin", Type: "Leader", Colors: []Color{Green, Yellow}}))
		Expect(db.Card("OP18-002").Colors).To(Equal([]Color{Black}))
		Expect(db.Card("OP18-002").Power).To(Equal(5000))
		Expect(db.Card("EB05-010").Power).To(BeZero())
	})

	It("only fetches sets that changed since the cached copy", func() {
		c, err := LoadCatalog(path)
		Expect(err).NotTo(HaveOccurred())
		_, err = c.Refresh(ctx, srv.Client(), srv.URL, now)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Save(path)).To(Succeed())

		fake.modified[2] = "b"
		again, err := LoadCatalog(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(again.Refresh(ctx, srv.Client(), srv.URL, now)).To(Equal(1))
		Expect(fake.fetches).To(Equal(map[int]int{1: 1, 2: 2}))
		db := NewDB()
		again.Apply(db)
		Expect(db.Card("EB05-010").Name).To(Equal("Nico Robin"))
	})

	It("goes stale a day after a refresh", func() {
		c, err := LoadCatalog(path)
		Expect(err).NotTo(HaveOccurred())
		_, err = c.Refresh(ctx, srv.Client(), srv.URL, now)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Stale(now.Add(23 * time.Hour))).To(BeFalse())
		Expect(c.Stale(now.Add(25 * time.Hour))).To(BeTrue())
	})

	It("fetches every set again when the catalog was saved by an older version, and fills in power", func() {
		old := &Catalog{Fetched: now, Groups: map[string]string{"1": "x"}, Cards: map[string]catalogCard{"OP18-002": {Name: "Spandam"}}}
		Expect(old.Save(path)).To(Succeed())
		c, err := LoadCatalog(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Stale(now)).To(BeTrue())
		Expect(c.Refresh(ctx, srv.Client(), srv.URL, now)).To(Equal(2))
		db := NewDB()
		c.Apply(db)
		Expect(db.Card("OP18-002").Power).To(Equal(5000))
	})

	It("keeps the power a card has when the catalog has none", func() {
		db := NewDB()
		withPower := &Catalog{Cards: map[string]catalogCard{"OP17-028": {Name: "Bonk Punch & Monster", Type: "Character", Power: 3000}}}
		withPower.Apply(db)
		c := &Catalog{Cards: map[string]catalogCard{"OP17-028": {Name: "Bonk Punch & Monster", Type: "Character"}}}
		c.Apply(db)
		Expect(db.Card("OP17-028").Power).To(Equal(3000))
	})

	It("saves the sets it fetched when a later one fails", func() {
		fake.down = 2
		r, err := RefreshCatalog(ctx, srv.Client(), srv.URL, path, now, false)
		Expect(err).To(HaveOccurred())
		Expect(r.Saved).To(BeTrue())

		fake.down = 0
		r, err = RefreshCatalog(ctx, srv.Client(), srv.URL, path, now, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(r).To(Equal(Refreshed{Saved: true, Sets: 1, Cards: 2}))
		Expect(fake.fetches).To(Equal(map[int]int{1: 1, 2: 1}))
	})

	It("leaves a fresh catalog alone unless forced", func() {
		_, err := RefreshCatalog(ctx, srv.Client(), srv.URL, path, now, false)
		Expect(err).NotTo(HaveOccurred())
		fake.modified[2] = "b"
		Expect(RefreshCatalog(ctx, srv.Client(), srv.URL, path, now.Add(time.Hour), false)).To(Equal(Refreshed{}))
		Expect(RefreshCatalog(ctx, srv.Client(), srv.URL, path, now.Add(time.Hour), true)).To(Equal(Refreshed{Saved: true, Sets: 1, Cards: 2}))
	})

	It("starts over from a file that doesn't parse", func() {
		Expect(os.WriteFile(path, []byte("{"), 0o600)).To(Succeed())
		r, err := RefreshCatalog(ctx, srv.Client(), srv.URL, path, now, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Sets).To(Equal(2))
	})

	It("keeps names from the logs over the catalog's", func() {
		c, _ := LoadCatalog(path)
		_, err := c.Refresh(ctx, srv.Client(), srv.URL, now)
		Expect(err).NotTo(HaveOccurred())
		db := NewDB()
		c.Apply(db)
		db.Name("EB05-010", "Nico Robin (log)")
		Expect(db.Card("EB05-010").Colors).To(Equal([]Color{Green, Yellow}))
		Expect(db.Card("EB05-010").Name).To(Equal("Nico Robin (log)"))
	})
})
