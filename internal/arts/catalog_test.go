package arts

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/cards"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var day0 = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

// pageFixture is a trimmed Bandai page from the cards package's specs.
func pageFixture(name string) []byte {
	GinkgoHelper()
	data, err := os.ReadFile(filepath.Join("..", "cards", "testdata", name))
	Expect(err).NotTo(HaveOccurred())
	return data
}

func parsePage(data []byte) cards.BandaiPage {
	GinkgoHelper()
	page, err := cards.ParseBandaiPage(bytes.NewReader(data))
	Expect(err).NotTo(HaveOccurred())
	return page
}

// pageHTML writes a card list page in Bandai's shape.
func pageHTML(list []cards.BandaiSeries, shown string, prints ...cards.BandaiPrint) []byte {
	var b strings.Builder
	b.WriteString(`<html><body><select name="series" id="series"><option value>ALL</option>`)
	for _, s := range list {
		sel := ""
		if s.ID == shown {
			sel = " selected"
		}
		fmt.Fprintf(&b, `<option value="%s"%s>%s</option>`, s.ID, sel, html.EscapeString(s.Name))
	}
	b.WriteString(`</select>`)
	for _, p := range prints {
		fmt.Fprintf(&b, `<dl class="modalCol" id="%s"><dt><div class="infoCol"><span>%s</span> | <span>%s</span> | <span>CHARACTER</span></div></dt>`+
			`<dd><div class="getInfo"><h3>Card Set(s)</h3>%s</div></dd></dl>`, p.Key, p.Code, p.Rarity, html.EscapeString(p.Product))
	}
	b.WriteString(`</body></html>`)
	return []byte(b.String())
}

// seriesPage is a page for a series with one printing of its own.
func seriesPage(list []cards.BandaiSeries, id string) []byte {
	code := "T" + id + "-001"
	return pageHTML(list, id, cards.BandaiPrint{Key: code, Code: code, Rarity: "C", Product: "Series " + id})
}

func ids(list []cards.BandaiSeries) []string {
	var out []string
	for _, s := range list {
		out = append(out, s.ID)
	}
	return out
}

func keysOf(ps []printing) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Key)
	}
	return out
}

var _ = Describe("catalog", func() {
	var list []cards.BandaiSeries
	BeforeEach(func() { list = parsePage(pageFixture("bandai-cardlist.html")).Series })

	// readAll stores the list's page, then a page for every series it says is due.
	readAll := func(c *catalog, now time.Time) {
		GinkgoHelper()
		_, err := c.store(parsePage(pageFixture("bandai-cardlist.html")), now)
		Expect(err).NotTo(HaveOccurred())
		for _, id := range c.due(now) {
			_, err := c.store(parsePage(seriesPage(list, id)), now)
			Expect(err).NotTo(HaveOccurred())
		}
	}

	It("reads every series once, then the promotions, other products and newest of each family daily", func() {
		c := newCatalog()
		_, err := c.store(parsePage(pageFixture("bandai-cardlist.html")), day0)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.due(day0)).To(HaveLen(60))
		Expect(c.due(day0)).NotTo(ContainElement("569117"))

		readAll(c, day0)
		Expect(c.due(day0.Add(time.Hour))).To(BeEmpty())
		Expect(c.due(day0.Add(21 * time.Hour))).To(Equal([]string{"569901", "569801", "569302", "569205", "569117", "569036"}))
		Expect(c.due(day0.Add(61 * 24 * time.Hour))).To(Equal([]string{"569901", "569801"}))
	})

	It("reads a series that shows up later daily for 60 days", func() {
		c := newCatalog()
		readAll(c, day0)
		day10 := day0.Add(10 * 24 * time.Hour)
		grown := append([]cards.BandaiSeries{{ID: "569118", Name: "BOOSTER PACK -NEW- [OP-18]"}}, list...)
		_, err := c.store(parsePage(seriesPage(grown, promotionID)), day10)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.due(day10)).To(ContainElement("569118"))
		_, err = c.store(parsePage(seriesPage(grown, "569118")), day10)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.due(day10.Add(21 * time.Hour))).To(ContainElement("569118"))
		Expect(c.due(day10.Add(59 * 24 * time.Hour))).To(ContainElement("569118"))
		Expect(c.due(day10.Add(61 * 24 * time.Hour))).NotTo(ContainElement("569118"))
	})

	It("refuses a page with no series or no printings, and keeps what it had", func() {
		c := newCatalog()
		_, err := c.store(parsePage(pageFixture("bandai-569901.html")), day0)
		Expect(err).NotTo(HaveOccurred())
		_, err = c.store(parsePage(pageHTML(list, promotionID)), day0)
		Expect(err).To(MatchError(errBadPage))
		_, err = c.store(parsePage([]byte("<html><body>Maintenance</body></html>")), day0)
		Expect(err).To(MatchError(errBadPage))
		Expect(c.forCard("OP07-021")).To(HaveLen(6))
	})

	It("keeps each card's printings from every page by key, without reprints", func() {
		c := newCatalog()
		for _, name := range []string{"bandai-569901.html", "bandai-569107.html", "bandai-569032.html"} {
			_, err := c.store(parsePage(pageFixture(name)), day0)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(keysOf(c.forCard("OP07-021"))).To(Equal([]string{
			"OP07-021", "OP07-021_p1", "OP07-021_p3", "OP07-021_p4", "OP07-021_p5", "OP07-021_p6", "OP07-021_p7",
		}))
		Expect(c.forCard("OP07-021")[1]).To(Equal(printing{
			Key: "OP07-021_p1", Code: "OP07-021", Rarity: "R", Product: "Offline Regional Participation Pack 2025 Vol.1",
		}))
		Expect(keysOf(c.forCard("OP12-031"))).To(Equal([]string{"OP12-031_p2"}))
	})

	It("replaces exactly what a page said when it is read again", func() {
		c := newCatalog()
		changed, err := c.store(parsePage(pageFixture("bandai-569901.html")), day0)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
		changed, err = c.store(parsePage(pageFixture("bandai-569901.html")), day0.Add(time.Hour))
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
		changed, err = c.store(parsePage(pageHTML(list, promotionID, cards.BandaiPrint{
			Key: "OP07-021_p1", Code: "OP07-021", Rarity: "R", Product: "Offline Regional Participation Pack 2025 Vol.1",
		})), day0.Add(2*time.Hour))
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
		Expect(keysOf(c.forCard("OP07-021"))).To(Equal([]string{"OP07-021_p1"}))
		Expect(c.forCard("OP12-031")).To(BeEmpty())
	})

	It("saves and loads, and treats a missing or unreadable file as empty", func() {
		path := filepath.Join(GinkgoT().TempDir(), "bandai.json")
		c, err := loadCatalog(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Series).To(BeEmpty())
		_, err = c.store(parsePage(pageFixture("bandai-569901.html")), day0)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.save(path)).To(Succeed())

		back, err := loadCatalog(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(back.forCard("OP07-021")).To(Equal(c.forCard("OP07-021")))
		Expect(back.Series[promotionID].Fetched).To(BeTemporally("==", day0))

		Expect(os.WriteFile(path, []byte("{"), 0o600)).To(Succeed())
		back, err = loadCatalog(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(back.Series).To(BeEmpty())
	})
})
