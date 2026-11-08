package arts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VTGare/logpose/internal/cards"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The cards below are as Limitless and Bandai's card list had them on 2026-11-08.

func urouge() (cards.Card, []printing) {
	card := cards.Card{Code: "OP07-021", Name: "Urouge", Prints: []cards.Print{
		{Key: "OP07-021", Rarity: "Rare", Artist: "tasaka", Set: "500 Years in the Future"},
		{Key: "OP07-021_p1", Rarity: "Special Card", Artist: "Bashikou", Set: "Royal Blood"},
		{Key: "OP07-021_p3", Kind: "aa", Rarity: "Alternate Art", Artist: "touge369", Set: "One Piece Card The Best vol.2"},
	}}
	return card, []printing{
		{"OP07-021", "OP07-021", "R", "-500 YEARS IN THE FUTURE- [OP-07]"},
		{"OP07-021_p1", "OP07-021", "R", "Offline Regional Participation Pack 2025 Vol.1"},
		{"OP07-021_p2", "OP07-021", "SP CARD", "-ROYAL BLOOD- [OP-10]"},
		{"OP07-021_p3", "OP07-021", "R", "Offline Regional Finalist Card Set 25-26 Season 1"},
		{"OP07-021_p4", "OP07-021", "R", "Offline Regional Champion Card Set 25-26 Season 1"},
		{"OP07-021_p5", "OP07-021", "R", "Online Regional Participation Pack 25-26 Season 1"},
		{"OP07-021_p6", "OP07-021", "R", "Online Regional Finalist Card Set 25-26 Season 1"},
		{"OP07-021_p7", "OP07-021", "R", "Online Regional Champion Card Set 25-26 Season 1"},
		{"OP07-021_p8", "OP07-021", "R", "-ONE PIECE CARD THE BEST vol.2- [PRB-02]"},
	}
}

func nami() (cards.Card, []printing) {
	card := cards.Card{Code: "OP01-016", Name: "Nami", Prints: []cards.Print{
		{Key: "OP01-016", Rarity: "Rare", Set: "Romance Dawn"},
		{Key: "OP01-016_p1", Kind: "aa", Rarity: "Alternate Art", Artist: "Sunohara", Set: "Romance Dawn"},
		{Key: "OP01-016_p3", Artist: "Sunohara", Set: "Ultra Deck: The Three Captains"},
		{Key: "OP01-016_p4", Rarity: "Special Card", Artist: "S-KINOKO", Set: "Awakening of the New Era"},
		{Key: "OP01-016_p7", Kind: "manga", Rarity: "Manga Art", Set: "One Piece Card The Best"},
		{Key: "OP01-016_p8", Rarity: "Special Card", Artist: "Takashi Kojima", Set: "One Piece Heroines Edition vol.2"},
	}}
	return card, []printing{
		{"OP01-016", "OP01-016", "R", "-ROMANCE DAWN- [OP01]"},
		{"OP01-016_p1", "OP01-016", "R", "-ROMANCE DAWN- [OP01]"},
		{"OP01-016_p2", "OP01-016", "R", "Premium Card Collection -25th Edition-"},
		{"OP01-016_p3", "OP01-016", "R", "-The Three Captains-[ST-10]"},
		{"OP01-016_p4", "OP01-016", "SP CARD", "-AWAKENING OF THE NEW ERA-[OP05]"},
		{"OP01-016_p5", "OP01-016", "R", "GIFT COLLECTION 2023 [GC-01]"},
		{"OP01-016_p7", "OP01-016", "R", "English Version 1st Anniversary Set"},
		{"OP01-016_p8", "OP01-016", "R", "-ONE PIECE CARD THE BEST- [PRB-01]"},
		{"OP01-016_p9", "OP01-016", "R", "STARTER DECK -RED Monkey.D.Luffy- [ST-31]"},
		{"OP01-016_p10", "OP01-016", "SP CARD", "EXTRA BOOSTER -ONE PIECE HEROINES EDITION vol.2- [EB-05]"},
	}
}

func tashigi() (cards.Card, []printing) {
	card := cards.Card{Code: "OP12-031", Name: "Tashigi", Prints: []cards.Print{
		{Key: "OP12-031", Rarity: "Rare", Artist: "phima", Set: "Legacy of the Master"},
		{Key: "OP12-031_p1", Kind: "aa", Rarity: "Alternate Art", Artist: "Peach Momoko", Set: "Legacy of the Master"},
	}}
	return card, []printing{
		{"OP12-031", "OP12-031", "R", "-LEGACY OF THE MASTER- [OP-12]"},
		{"OP12-031_p1", "OP12-031", "R", "-LEGACY OF THE MASTER- [OP-12]"},
		{"OP12-031_p2", "OP12-031", "R", "Treasure Cup November 2025~"},
	}
}

// realGrids are the grids made from the real images in testdata/grids, less the keys left out.
func realGrids(without ...string) func(Key) (grid, bool) {
	return func(k Key) (grid, bool) {
		for _, w := range without {
			if k.String() == w {
				return grid{}, false
			}
		}
		data, err := os.ReadFile(filepath.Join("testdata", "grids", k.String()+".grid"))
		var g grid
		if err != nil || len(data) != len(g) {
			return grid{}, false
		}
		copy(g[:], data)
		return g, true
	}
}

// layout writes each art as "Name: version keys | same keys".
func layout(ca CardArts) []string {
	var out []string
	for _, a := range ca.Arts {
		var vs, same []string
		for _, v := range a.Versions {
			vs = append(vs, v.Key.String())
		}
		for _, k := range a.Same {
			same = append(same, k.String())
		}
		out = append(out, fmt.Sprintf("%s: %s | %s", a.Name(), strings.Join(vs, " "), strings.Join(same, " ")))
	}
	return out
}

var _ = Describe("grouping", func() {
	It("makes Urouge's six Regional prizes one art, and shows each printing on both sites once", func() {
		card, bandai := urouge()
		ca := group(card, bandai, realGrids(), true)
		Expect(layout(ca)).To(Equal([]string{
			"Base: OP07-021 | OP07-021_bd",
			"SP: OP07-021_p1 | OP07-021_p2_bd",
			"Alt art: OP07-021_p3 | OP07-021_p8_bd",
			"Regional: OP07-021_p1_bd OP07-021_p3_bd OP07-021_p4_bd OP07-021_p5_bd OP07-021_p6_bd OP07-021_p7_bd | ",
		}))
		Expect(ca.Pending).To(BeZero())
		regional := ca.Arts[3]
		Expect(regional.Head().Product).To(Equal("Offline Regional Participation Pack 2025 Vol.1"))
		Expect(regional.Versions[1]).To(Equal(Version{
			Key: Key{Bandai, "OP07-021_p3"}, Name: "Regional", Short: "Offline finalist",
			Product: "Offline Regional Finalist Card Set 25-26 Season 1",
		}))
	})

	It("counts Bandai's copy of a printing in the same product without its picture", func() {
		card, bandai := urouge()
		ca := group(card, bandai, realGrids("OP07-021_bd"), true)
		Expect(ca.Arts[0].Same).To(Equal([]Key{{Bandai, "OP07-021"}}))
	})

	It("shows Nami's SP once though the two sites number it apart", func() {
		card, bandai := nami()
		ca := group(card, bandai, realGrids(), true)
		sp, _, ok := ca.Find("OP01-016_p8")
		Expect(ok).To(BeTrue())
		Expect(sp.Same).To(Equal([]Key{{Bandai, "OP01-016_p10"}}))
		Expect(sp.Has("OP01-016_p8_bd")).To(BeFalse())
		Expect(layout(ca)).To(Equal([]string{
			"Base: OP01-016 | OP01-016_bd",
			"Alt art: OP01-016_p1 | OP01-016_p1_bd",
			"Alt art: OP01-016_p3 | OP01-016_p3_bd",
			"SP: OP01-016_p4 | OP01-016_p4_bd",
			"Manga: OP01-016_p7 | OP01-016_p8_bd",
			"SP: OP01-016_p8 | OP01-016_p10_bd",
			"Premium: OP01-016_p2_bd | ",
			"Promo: OP01-016_p5_bd | ",
			"Anniversary: OP01-016_p7_bd | ",
			"Promo: OP01-016_p9_bd | ",
		}))
	})

	It("never puts two Limitless printings in one art", func() {
		card := cards.Card{Code: "OP01-001", Prints: []cards.Print{{Key: "OP01-001"}, {Key: "OP01-001_p1", Kind: "aa"}}}
		same := func(Key) (grid, bool) { return grid{}, true }
		Expect(layout(group(card, nil, same, true))).To(Equal([]string{"Base: OP01-001 | ", "Alt art: OP01-001_p1 | "}))
	})

	Describe("a Limitless printing's twin", func() {
		card := cards.Card{Code: "OP01-001", Prints: []cards.Print{{Key: "OP01-001", Set: "Romance Dawn"}}}
		// The head's grid is all zeros. Setting 33 channels to v puts a grid 33·v²/1,056 away.
		near := func(v uint8) grid {
			var g grid
			for i := range 33 {
				g[i] = v
			}
			return g
		}
		at := map[string]grid{"OP01-001_p1_bd": near(98), "OP01-001_p2_bd": near(120)}
		known := func(k Key) (grid, bool) { return at[k.String()], true }

		It("is the closest of the close Bandai printings, and the others are versions", func() {
			Expect(distance(grid{}, at["OP01-001_p1_bd"])).To(BeNumerically("~", 300, 2))
			Expect(distance(grid{}, at["OP01-001_p2_bd"])).To(BeNumerically("~", 450, 2))
			bandai := []printing{
				{"OP01-001_p1", "OP01-001", "R", "Tournament Pack Vol.1"},
				{"OP01-001_p2", "OP01-001", "R", "Tournament Pack Vol.2"},
			}
			Expect(layout(group(card, bandai, known, true))).To(Equal([]string{"Base: OP01-001 OP01-001_p2_bd | OP01-001_p1_bd"}))
		})

		It("is one from the same product before a closer one", func() {
			bandai := []printing{
				{"OP01-001_p1", "OP01-001", "R", "Tournament Pack Vol.1"},
				{"OP01-001_p2", "OP01-001", "R", "-ROMANCE DAWN- [OP-01]"},
			}
			Expect(layout(group(card, bandai, known, true))).To(Equal([]string{"Base: OP01-001 OP01-001_p1_bd | OP01-001_p2_bd"}))
		})
	})

	It("counts printings without pictures as pending, and leaves them out at the end", func() {
		card, bandai := urouge()
		missing := realGrids("OP07-021_p1_bd", "OP07-021_p3_bd", "OP07-021_p4_bd", "OP07-021_p5_bd", "OP07-021_p6_bd", "OP07-021_p7_bd")
		ca := group(card, bandai, missing, false)
		Expect(ca.Pending).To(Equal(6))
		Expect(ca.Arts).To(HaveLen(3))
		ca = group(card, bandai, missing, true)
		Expect(ca.Pending).To(BeZero())
		Expect(ca.Arts).To(HaveLen(3))
	})

	It("starts no new art while a Limitless picture it may repeat isn't compared", func() {
		card, bandai := urouge()
		ca := group(card, bandai, realGrids("OP07-021_p3"), false)
		Expect(layout(ca)).To(Equal([]string{
			"Base: OP07-021 | OP07-021_bd", "SP: OP07-021_p1 | OP07-021_p2_bd", "Alt art: OP07-021_p3 | ",
		}))
		Expect(ca.Pending).To(Equal(7))
		ca = group(card, bandai, realGrids("OP07-021_p3"), true)
		Expect(layout(ca)).To(ContainElement("Promo: OP07-021_p8_bd | "))
	})

	It("shows one art per Limitless printing before anything is compared", func() {
		card, _ := urouge()
		Expect(layout(LimitlessArts(card))).To(Equal([]string{
			"Base: OP07-021 | ", "SP: OP07-021_p1 | ", "Alt art: OP07-021_p3 | ",
		}))
		Expect(layout(LimitlessArts(cards.Card{Code: "OP18-001"}))).To(Equal([]string{"Base: OP18-001 | "}))
	})

	It("finds the art a saved key names, a twin finding its head", func() {
		card, bandai := urouge()
		ca := group(card, bandai, realGrids(), true)
		art, v, ok := ca.Find("OP07-021_p8_bd")
		Expect(ok).To(BeTrue())
		Expect(art.Name()).To(Equal("Alt art"))
		Expect(v.Key).To(Equal(Key{Limitless, "OP07-021_p3"}))
		_, v, ok = ca.Find("OP07-021_p5_bd")
		Expect(ok).To(BeTrue())
		Expect(v.Short).To(Equal("Online participant"))
		_, _, ok = ca.Find("OP07-021_p2")
		Expect(ok).To(BeFalse())
	})

	Describe("match", func() {
		It("picks the closest stamp, not the first one close enough", func() {
			card, bandai := urouge()
			ca := group(card, bandai, realGrids(), true)
			Expect(distance(fixtureGrid("OP07-021_p4_bd"), fixtureGrid("OP07-021_p1_bd"))).To(BeNumerically("<=", matchMax))
			k, ok := match(ca, fixtureGrid("OP07-021_p4_bd"), realGrids())
			Expect(ok).To(BeTrue())
			Expect(k.String()).To(Equal("OP07-021_p4_bd"))
		})

		It("answers a twin's picture as the Limitless printing", func() {
			card, bandai := urouge()
			ca := group(card, bandai, realGrids(), true)
			k, ok := match(ca, fixtureGrid("OP07-021_p8_bd"), realGrids())
			Expect(ok).To(BeTrue())
			Expect(k.String()).To(Equal("OP07-021_p3"))
		})

		It("finds Tashigi's Treasure Cup printing on Bandai's list", func() {
			card, bandai := tashigi()
			ca := group(card, bandai, realGrids(), true)
			Expect(layout(ca)).To(Equal([]string{
				"Base: OP12-031 | OP12-031_bd", "Alt art: OP12-031_p1 | OP12-031_p1_bd", "Treasure Cup: OP12-031_p2_bd | ",
			}))
			k, ok := match(ca, fixtureGrid("OP12-031_p2_bd"), realGrids())
			Expect(ok).To(BeTrue())
			Expect(k.String()).To(Equal("OP12-031_p2_bd"))
		})

		It("finds nothing for another card's picture", func() {
			card, bandai := tashigi()
			ca := group(card, bandai, realGrids(), true)
			_, ok := match(ca, fixtureGrid("OP07-021_p1_bd"), realGrids())
			Expect(ok).To(BeFalse())
		})
	})
})
