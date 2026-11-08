package cards

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bandai's card list", func() {
	read := func(name string) BandaiPage {
		GinkgoHelper()
		page, err := ParseBandaiPage(bytes.NewReader(fixture(name)))
		Expect(err).NotTo(HaveOccurred())
		return page
	}

	It("reads every series the select names, without the markup in their labels", func() {
		page := read("bandai-569901.html")
		Expect(page.Series).To(HaveLen(61))
		Expect(page.Series[0]).To(Equal(BandaiSeries{ID: "569302", Name: "PREMIUM BOOSTER -ONE PIECE CARD THE BEST vol.2- [PRB-02]"}))
		Expect(page.Series).To(ContainElements(
			BandaiSeries{ID: "569115", Name: "BOOSTER PACK -ADVENTURE ON KAMI’S ISLAND- [OP15-EB04]"},
			BandaiSeries{ID: "569036", Name: `STARTER DECK -YELLOW Eustass"Captain"Kid- [ST-36]`},
			BandaiSeries{ID: "569801", Name: "Other Product Card"},
		))
		Expect(page.Series[len(page.Series)-2]).To(Equal(BandaiSeries{ID: "569901", Name: "Promotion card"}))
		Expect(page.Shown).To(Equal("569901"))
	})

	It("reads each printing's key, code, rarity and product", func() {
		page := read("bandai-569901.html")
		Expect(page.Prints).To(HaveLen(8))
		Expect(page.Prints[0]).To(Equal(BandaiPrint{
			Key: "OP07-021_p1", Code: "OP07-021", Rarity: "R", Product: "Offline Regional Participation Pack 2025 Vol.1",
		}))
		Expect(page.Prints[1].Product).To(Equal("Offline Regional Finalist Card Set 25-26 Season 1"))
		Expect(page.Prints[6]).To(Equal(BandaiPrint{
			Key: "OP12-031_p2", Code: "OP12-031", Rarity: "R", Product: "Treasure Cup November 2025~",
		}))
		Expect(page.Prints[7].Key).To(Equal("P-001"))
	})

	It("reads a booster's own printings and reprints elsewhere", func() {
		op07 := read("bandai-569107.html")
		Expect(op07.Shown).To(Equal("569107"))
		Expect(op07.Prints[0]).To(Equal(BandaiPrint{
			Key: "OP07-021", Code: "OP07-021", Rarity: "R", Product: "-500 YEARS IN THE FUTURE- [OP-07]",
		}))
		st32 := read("bandai-569032.html")
		Expect(st32.Prints[1]).To(Equal(BandaiPrint{
			Key: "OP12-031_r1", Code: "OP12-031", Rarity: "R", Product: "STARTER DECK -GREEN Roronoa Zoro- [ST-32]",
		}))
	})

	It("shows the newest booster on the list's own page", func() {
		page := read("bandai-cardlist.html")
		Expect(page.Shown).To(Equal("569117"))
		Expect(page.Series).To(HaveLen(61))
		Expect(page.Prints[1].Key).To(Equal("OP17-001_p1"))
	})

	It("reads nothing from a page of another shape", func() {
		page, err := ParseBandaiPage(bytes.NewReader([]byte("<html><body><p>Maintenance</p></body></html>")))
		Expect(err).NotTo(HaveOccurred())
		Expect(page.Series).To(BeEmpty())
		Expect(page.Prints).To(BeEmpty())
	})
})
