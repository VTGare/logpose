package cards

import (
	"context"
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

var _ = Describe("Limitless", func() {
	It("reads the set index and the sets it names as updated", func() {
		idx, err := ParseSetIndex(strings.NewReader(string(fixture("limitless-index.html"))))
		Expect(err).NotTo(HaveOccurred())
		Expect(idx.Latest).To(Equal([]string{"eb05-one-piece-heroines-edition-vol-2", "op18-the-dominance-of-god"}))
		Expect(idx.Sets).To(Equal([]string{
			"eb05-one-piece-heroines-edition-vol-2", "op18-the-dominance-of-god", "op14-the-azure-seas-seven", "st14-3D2Y", "promos",
		}))
	})

	It("reads every printing on a set page with the card's data", func() {
		prints, err := parseSetPage(strings.NewReader(string(fixture("limitless-op14.html"))))
		Expect(err).NotTo(HaveOccurred())
		Expect(prints).To(HaveLen(8))
		law := prints[0]
		Expect(law.card).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Code": Equal("OP14-001"), "Name": Equal("Trafalgar Law"), "Type": Equal("Leader"),
			"Colors": Equal([]Color{Red}), "Cost": Equal(-1), "Life": Equal(5), "Power": Equal(5000),
			"Attributes": Equal([]string{"Slash"}), "Block": Equal("4"), "Detailed": BeTrue(),
			"Traits": Equal([]string{"The Seven Warlords of the Sea", "Supernovas", "Heart Pirates"}),
		}))
		Expect(law.card.Effect).To(HavePrefix("[Activate: Main] [Once Per Turn] Select 2"))
		Expect(law.print).To(Equal(Print{Key: "OP14-001", Block: "4", Rarity: "Leader", Artist: "Kaito Tomioka", Set: "The Azure Sea's Seven"}))
		Expect(law.standard).To(Equal("legal"))
	})

	It("reads alternate printings, counters, banned cards and triggers", func() {
		prints, err := parseSetPage(strings.NewReader(string(fixture("limitless-op14.html"))))
		Expect(err).NotTo(HaveOccurred())
		byKey := make(map[string]limitlessPrint)
		for _, p := range prints {
			byKey[p.print.Key] = p
		}
		Expect(byKey["OP14-001_p1"].print.Kind).To(Equal("aa"))
		Expect(byKey["OP14-002"].card.Counter).To(Equal(1000))
		Expect(byKey["OP14-002"].card.Cost).To(Equal(3))
		Expect(byKey["OP14-020"].standard).To(Equal("banned"))
		Expect(byKey["OP14-020"].card.Effect).To(ContainSubstring("has the <Slash> attribute"))
		Expect(byKey["OP14-018"].card.Effect).To(ContainSubstring("\n[Trigger]"))
		Expect(byKey["OP14-018"].card.Type).To(Equal("Event"))
	})
})

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
		l := newLibrary()
		reprint := limitlessPrint{
			card: Card{Code: "OP01-024", Rarity: "Rare", Set: "One Piece Card The Best"}, setCode: "PRB02",
			print: Print{Key: "OP01-024", Rarity: "Rare", Set: "One Piece Card The Best"},
		}
		original := limitlessPrint{
			card: Card{Code: "OP01-024", Rarity: "Super Rare", Set: "Romance Dawn"}, setCode: "OP01",
			print: Print{Key: "OP01-024", Rarity: "Super Rare", Set: "Romance Dawn"},
		}
		l.merge([]limitlessPrint{reprint})
		l.merge([]limitlessPrint{original})
		l.merge([]limitlessPrint{reprint})
		c := l.Cards["OP01-024"]
		Expect(c.Set).To(Equal("Romance Dawn"))
		Expect(c.Rarity).To(Equal("Super Rare"))
		Expect(c.Prints).To(Equal([]Print{original.print}))
	})

	It("shows a card's newest block among its printings, with X above all", func() {
		l := newLibrary()
		base := limitlessPrint{card: Card{Code: "OP01-120", Block: "1"}, setCode: "OP01", print: Print{Key: "OP01-120", Block: "1"}}
		reprint := limitlessPrint{card: Card{Code: "OP01-120", Block: "4"}, setCode: "PRB02", print: Print{Key: "OP01-120_p3", Block: "4"}}
		manga := limitlessPrint{card: Card{Code: "OP01-120", Block: "X"}, setCode: "OP01", print: Print{Key: "OP01-120_p2", Kind: "manga", Block: "X"}}
		l.merge([]limitlessPrint{base, reprint})
		Expect(l.Cards["OP01-120"].Block).To(Equal("4"))
		l.merge([]limitlessPrint{manga})
		Expect(l.Cards["OP01-120"].Block).To(Equal("X"))
	})

	It("fills in a printing's block from a later page, whatever order the pages come in", func() {
		l := newLibrary()
		reprinted := limitlessPrint{card: Card{Code: "OP01-120"}, setCode: "PRB01", print: Print{Key: "OP01-120_p2", Kind: "manga"}}
		own := limitlessPrint{card: Card{Code: "OP01-120", Block: "X"}, setCode: "OP01", print: Print{Key: "OP01-120_p2", Kind: "manga", Block: "X"}}
		l.merge([]limitlessPrint{reprinted})
		l.merge([]limitlessPrint{own})
		l.merge([]limitlessPrint{reprinted})
		Expect(l.Cards["OP01-120"].Block).To(Equal("X"))
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
		l.Rules.Banned = []string{"OP06-047"}
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
