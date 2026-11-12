package cards

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
)

func fixture(name string) []byte {
	GinkgoHelper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())
	return data
}

var _ = Describe("Bandai's ban list", func() {
	It("reads the active restrictions and not the announcement above them", func() {
		r, err := ParseRestrictions(strings.NewReader(string(fixture("bandai-restriction.html"))))
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Banned).To(Equal([]string{"OP06-047", "OP03-040", "OP06-086", "ST10-001", "OP06-116", "OP14-020"}))
		Expect(r.Restricted).To(BeEmpty())
		Expect(r.Pairs).To(Equal([][]string{{"OP07-115", "EB04-058"}, {"OP11-040", "OP11-067"}, {"OP11-040", "OP08-069"}}))
	})
})

// fakeSites serves the fixtures as Limitless and Bandai, and counts the pages fetched.
type fakeSites struct {
	mu      sync.Mutex
	fetches map[string]int
	rules   []byte
}

func (f *fakeSites) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetches[r.URL.Path]++
	switch {
	case r.URL.Path == "/cards":
		_, _ = w.Write(fixture("limitless-index.html"))
	case r.URL.Path == "/cards/op14-the-azure-seas-seven":
		Expect(r.URL.Query().Get("display")).To(Equal("full"))
		_, _ = w.Write(fixture("limitless-op14.html"))
	case r.URL.Path == "/cards/st14-3D2Y":
		_, _ = w.Write(fixture("limitless-op18.html"))
	case strings.HasPrefix(r.URL.Path, "/cards/"):
		_, _ = w.Write([]byte("<html><body></body></html>"))
	case r.URL.Path == "/rules":
		_, _ = w.Write(f.rules)
	default:
		http.NotFound(w, r)
	}
}

// pagePrint is a printing as a set page shows it, with only what the parser reads.
type pagePrint struct {
	code, key, set, setCode, rarity, kind, block string
}

func setPage(ps ...pagePrint) []byte {
	var b strings.Builder
	b.WriteString("<html><body>")
	for _, p := range ps {
		fmt.Fprintf(&b, `<div class="card-page-main"><img class="card" src="https://cdn/one-piece/%s/%s_EN.webp">`, p.setCode, p.key)
		fmt.Fprintf(&b, `<span class="card-text-id">%s</span>`, p.code)
		if p.block != "" {
			fmt.Fprintf(&b, `<div class="regulation-mark">Block %s</div>`, p.block)
		}
		fmt.Fprintf(&b, `<div class="prints-current-details"><span>%s (%s)</span><span>%s</span></div>`, p.set, p.setCode, p.rarity)
		fmt.Fprintf(&b, `<table><tr class="current"><td><span class="prints-table-card-number">%s</span></td></tr></table></div>`, p.kind)
	}
	b.WriteString("</body></html>")
	return []byte(b.String())
}

// refreshPages refreshes a new library from a Limitless that lists the pages in order.
func refreshPages(pages ...[]byte) *Library {
	GinkgoHelper()
	var index strings.Builder
	for i := range pages {
		fmt.Fprintf(&index, `<a href="/cards/set%d-page">x</a>`, i)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cards" {
			_, _ = w.Write([]byte(index.String()))
			return
		}
		var i int
		if _, err := fmt.Sscanf(r.URL.Path, "/cards/set%d-page", &i); err != nil || i >= len(pages) {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(pages[i])
	}))
	DeferCleanup(srv.Close)
	l := newLibrary()
	_, err := l.Refresh(context.Background(), Source{Client: srv.Client(), Limitless: srv.URL}, time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC))
	Expect(err).NotTo(HaveOccurred())
	return l
}

var _ = Describe("Library", func() {
	var (
		sites *fakeSites
		srv   *httptest.Server
		src   Source
		now   = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	)
	BeforeEach(func() {
		sites = &fakeSites{fetches: make(map[string]int), rules: fixture("bandai-restriction.html")}
		srv = httptest.NewServer(sites)
		DeferCleanup(srv.Close)
		src = Source{Client: srv.Client(), Limitless: srv.URL, Rules: srv.URL + "/rules"}
	})

	It("reads every printing on a set page with the card's data", func() {
		l := newLibrary()
		_, err := l.Refresh(context.Background(), src, now)
		Expect(err).NotTo(HaveOccurred())
		law := l.Cards["OP14-001"]
		Expect(law).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Code": Equal("OP14-001"), "Name": Equal("Trafalgar Law"), "Type": Equal("Leader"),
			"Colors": Equal([]Color{Red}), "Cost": Equal(-1), "Life": Equal(5), "Power": Equal(5000),
			"Attributes": Equal([]string{"Slash"}), "Block": Equal("4"), "Detailed": BeTrue(), "Legality": Equal(Legal),
			"Traits": Equal([]string{"The Seven Warlords of the Sea", "Supernovas", "Heart Pirates"}),
		}))
		Expect(law.Effect).To(HavePrefix("[Activate: Main] [Once Per Turn] Select 2"))
		Expect(law.Prints[0]).To(Equal(Print{Key: "OP14-001", Block: "4", Rarity: "Leader", Artist: "Kaito Tomioka", Set: "The Azure Sea's Seven"}))
		Expect(l.Cards["OP14-002"].Counter).To(Equal(1000))
		Expect(l.Cards["OP14-002"].Cost).To(Equal(3))
		Expect(l.Cards["OP14-020"].Legality).To(Equal(Banned))
		Expect(l.Cards["OP14-020"].Effect).To(ContainSubstring("has the <Slash> attribute"))
		Expect(l.Cards["OP14-018"].Effect).To(ContainSubstring("\n[Trigger]"))
		Expect(l.Cards["OP14-018"].Type).To(Equal("Event"))
	})

	It("fetches every set once, then only the updated ones a day later", func() {
		l := newLibrary()
		n, err := l.Refresh(context.Background(), src, now)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(5))
		n, err = l.Refresh(context.Background(), src, now.Add(time.Hour))
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(0))
		n, err = l.Refresh(context.Background(), src, now.Add(24*time.Hour))
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(4))
		Expect(sites.fetches["/cards/op14-the-azure-seas-seven"]).To(Equal(1))
	})

	It("reports each set it reads", func() {
		var got []SetProgress
		src.Progress = func(p SetProgress) { got = append(got, p) }
		_, err := newLibrary().Refresh(context.Background(), src, now)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(HaveLen(5))
		Expect(got[4].Done).To(Equal(5))
		Expect(got[4].Total).To(Equal(5))
		Expect(got).To(ContainElement(SetProgress{Done: 3, Total: 5, Set: "OP14 The Azure Sea's Seven", Cards: 5}))
	})

	It("tells a set that isn't out yet from a rotated one, and fetches it again daily", func() {
		l := newLibrary()
		_, err := l.Refresh(context.Background(), src, now)
		Expect(err).NotTo(HaveOccurred())
		Expect(l.Cards["OP18-001"].Legality).To(Equal(Unreleased))
		Expect(l.Unreleased).To(HaveKeyWithValue("st14-3D2Y", true))
		Expect(l.Unreleased).To(HaveKeyWithValue("op14-the-azure-seas-seven", false))
		Expect(l.Unreleased).To(HaveKeyWithValue("promos", true), "a page with no cards yet")
		_, err = l.Refresh(context.Background(), src, now.Add(24*time.Hour))
		Expect(err).NotTo(HaveOccurred())
		Expect(sites.fetches["/cards/st14-3D2Y"]).To(Equal(2))
		Expect(sites.fetches["/cards/promos"]).To(Equal(2))
		Expect(sites.fetches["/cards/op14-the-azure-seas-seven"]).To(Equal(1))
	})

	It("keeps a card's printings together, the first one leading", func() {
		l := newLibrary()
		_, err := l.Refresh(context.Background(), src, now)
		Expect(err).NotTo(HaveOccurred())
		law := l.Cards["OP14-001"]
		Expect(law.Prints).To(HaveLen(2))
		Expect(law.Prints[0].Key).To(Equal("OP14-001"))
		Expect(law.Prints[1].Kind).To(Equal("aa"))
		Expect(law.Rarity).To(Equal("Leader"))
		Expect(l.Cards).To(HaveKey("EB04-012"))
	})

	It("takes a card's data from its own set even when a reprint is read first", func() {
		reprint := setPage(pagePrint{code: "OP01-024", key: "OP01-024", set: "One Piece Card The Best", setCode: "PRB02", rarity: "Rare"})
		original := setPage(pagePrint{code: "OP01-024", key: "OP01-024", set: "Romance Dawn", setCode: "OP01", rarity: "Super Rare"})
		c := refreshPages(reprint, original, reprint).Cards["OP01-024"]
		Expect(c.Set).To(Equal("Romance Dawn"))
		Expect(c.Rarity).To(Equal("Super Rare"))
		Expect(c.Prints).To(Equal([]Print{{Key: "OP01-024", Rarity: "Super Rare", Set: "Romance Dawn"}}))
	})

	It("shows a card's newest block among its printings, with X above all", func() {
		base := pagePrint{code: "OP01-120", key: "OP01-120", set: "Romance Dawn", setCode: "OP01", block: "1"}
		manga := pagePrint{code: "OP01-120", key: "OP01-120_p2", set: "Romance Dawn", setCode: "OP01", kind: "manga", block: "X"}
		reprint := setPage(pagePrint{code: "OP01-120", key: "OP01-120_p3", set: "One Piece Card The Best", setCode: "PRB02", block: "4"})
		Expect(refreshPages(setPage(base), reprint).Cards["OP01-120"].Block).To(Equal("4"))
		Expect(refreshPages(setPage(base, manga), reprint).Cards["OP01-120"].Block).To(Equal("X"))
	})

	It("fills in a printing's block from a later page, whatever order the pages come in", func() {
		reprinted := setPage(pagePrint{code: "OP01-120", key: "OP01-120_p2", set: "Premium", setCode: "PRB01", kind: "manga"})
		own := setPage(pagePrint{code: "OP01-120", key: "OP01-120_p2", set: "Romance Dawn", setCode: "OP01", kind: "manga", block: "X"})
		Expect(refreshPages(reprinted, own, reprinted).Cards["OP01-120"].Block).To(Equal("X"))
	})

	It("takes bans and pairs from Bandai and rotation from Limitless", func() {
		l := newLibrary()
		_, err := l.Refresh(context.Background(), src, now)
		Expect(err).NotTo(HaveOccurred())
		l.Cards["OP11-040"] = Card{Code: "OP11-040", Detailed: true, Legality: Legal}
		l.Cards["OP01-024"] = Card{Code: "OP01-024", Detailed: true, Legality: Rotated}
		db := NewDB()
		l.Apply(db)
		Expect(db.Card("OP14-020").Legality).To(Equal(Banned))
		Expect(db.Card("OP14-001").Legality).To(Equal(Legal))
		Expect(db.Card("OP01-024").Legality).To(Equal(Rotated))
		Expect(db.Card("OP11-040").BannedWith).To(Equal([]string{"OP11-067", "OP08-069"}))
	})

	It("lifts a ban once Bandai's list drops it", func() {
		l := newLibrary()
		_, err := l.Refresh(context.Background(), src, now)
		Expect(err).NotTo(HaveOccurred())
		sites.rules = bytes.ReplaceAll(sites.rules, []byte("OP14-020"), []byte("OP14-999"))
		_, err = l.Refresh(context.Background(), src, now.Add(time.Hour))
		Expect(err).NotTo(HaveOccurred())
		db := NewDB()
		l.Apply(db)
		Expect(db.Card("OP14-020").Legality).To(Equal(Legal))
	})

	It("keeps the last ban list when the page comes back without one", func() {
		l := newLibrary()
		_, err := l.Refresh(context.Background(), src, now)
		Expect(err).NotTo(HaveOccurred())
		sites.rules = []byte("<html><body><h3>Cards with Active Restrictions</h3></body></html>")
		_, err = l.Refresh(context.Background(), src, now.Add(time.Hour))
		Expect(err).To(HaveOccurred())
		Expect(l.Rules.Banned).To(ContainElement("OP14-020"))
	})

	It("saves and loads", func() {
		l := newLibrary()
		_, err := l.Refresh(context.Background(), src, now)
		Expect(err).NotTo(HaveOccurred())
		path := filepath.Join(GinkgoT().TempDir(), "library.json")
		Expect(l.Save(path)).To(Succeed())
		back, err := LoadLibrary(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(back.Cards["OP14-001"]).To(Equal(l.Cards["OP14-001"]))
		Expect(back.Rules).To(Equal(l.Rules))
	})
})
