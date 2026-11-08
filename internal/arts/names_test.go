package arts

import (
	"github.com/VTGare/logpose/internal/cards"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("names", func() {
	It("name SP cards and new art apart from reprints of the same art", func() {
		base := cards.Print{Key: "OP07-019", Artist: "Ryuda"}
		card := cards.Card{Code: "OP07-019", Prints: []cards.Print{base}}
		Expect(limitlessName(card, base)).To(Equal("Base"))
		Expect(limitlessName(card, cards.Print{Key: "OP07-019_p2", Rarity: "Special Card", Artist: "Sunohara"})).To(Equal("SP"))
		Expect(limitlessName(card, cards.Print{Key: "OP07-019_p3", Rarity: "Leader", Artist: "DAI-XT."})).To(Equal("Alt art"))
		Expect(limitlessName(card, cards.Print{Key: "OP07-019_p4", Set: "Anime 25th Collection"})).To(Equal("Reprint"))
		Expect(limitlessName(card, cards.Print{Key: "OP07-019_p5", Artist: "Ryuda"})).To(Equal("Reprint"))
		Expect(limitlessName(card, cards.Print{Key: "OP07-019_p6", Kind: "aa", Rarity: "Treasure Rare"})).To(Equal("Treasure"))
	})

	It("name Bandai's printings by their product", func() {
		for product, want := range map[string]string{
			"Offline Regional Finalist Card Set 25-26 Season 1": "Regional",
			"Championship 25-26 Offline Regionals Season 2":     "Regional",
			"Treasure Cup November 2025~":                       "Treasure Cup",
			"Premium Card Collection -Best Selection Vol.4-":    "Premium",
			"English Version 1st Anniversary Set":               "Anniversary",
			"Championship 25-26 Finals Season 1":                "Championship",
			"CS 25-26 Event Pack":                               "Promo",
			"STARTER DECK -RED Monkey.D.Luffy- [ST-31]":         "Promo",
		} {
			Expect(bandaiName("R", product)).To(Equal(want), product)
		}
		Expect(bandaiName("SP CARD", "Treasure Cup November 2025~")).To(Equal("SP"))
	})

	It("say which Regional prize a version is", func() {
		Expect(shortName("Regional", "Offline Regional Participation Pack 2025 Vol.1")).To(Equal("Offline participant"))
		Expect(shortName("Regional", "Online Regional Champion Card Set 25-26 Season 1")).To(Equal("Online champion"))
		Expect(shortName("Regional", "Championship 25-26 Offline Regionals Season 2")).To(Equal("Regional"))
		Expect(shortName("Regional", "Regional 2024 wave1")).To(Equal("Regional"))
		Expect(shortName("Treasure Cup", "Treasure Cup November 2025~")).To(Equal("Treasure Cup"))
	})

	It("match a Limitless set with Bandai's product line for it", func() {
		for set, product := range map[string]string{
			"500 Years in the Future":          "-500 YEARS IN THE FUTURE- [OP-07]",
			"The World's Strongest Warriors":   "BOOSTER PACK -THE WORLD’S STRONGEST WARRIORS- [OP-17]",
			"One Piece Heroines Edition vol.2": "EXTRA BOOSTER -ONE PIECE HEROINES EDITION vol.2- [EB-05]",
			"Ultra Deck: The Three Brothers":   "-The Three Brothers-[ST13]",
			"EX Gear 5":                        "-GEAR5- [ST-21]",
			"GREEN Roronoa Zoro":               "STARTER DECK -GREEN Roronoa Zoro- [ST-32]",
		} {
			Expect(sameProduct(set, product)).To(BeTrue(), set)
		}
		for set, product := range map[string]string{
			"One Piece Card The Best":       "-ONE PIECE CARD THE BEST vol.2- [PRB-02]",
			"One Piece Card The Best vol.2": "Premium Card Collection -Best Selection Vol.3-",
			"Royal Blood":                   "Offline Regional Participation Pack 2025 Vol.1",
			"":                              "",
		} {
			Expect(sameProduct(set, product)).To(BeFalse(), set)
		}
	})
})
