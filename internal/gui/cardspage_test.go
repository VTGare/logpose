package gui

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// testCards is a small pool: two red Supernovas characters, a green event and a banned leader.
func testCards() *cards.DB {
	lib := &cards.Library{Cards: map[string]cards.Card{
		"OP14-002": {
			Code: "OP14-002", Name: "Urouge", Type: "Character", Colors: []cards.Color{cards.Red}, Cost: 3, Power: 2000,
			Counter: 1000, Traits: []string{"Supernovas", "Fallen Monk Pirates"}, Attributes: []string{"Strike"},
			Effect: "[When Attacking] If this Character has 5000 power or more, draw 1 card.", Detailed: true,
			Legality: cards.Legal, Rarity: "Uncommon", Prints: []cards.Print{{Key: "OP14-002", Set: "The Azure Sea's Seven"}},
		},
		"OP01-024": {
			Code: "OP01-024", Name: "Monkey D. Luffy", Type: "Character", Colors: []cards.Color{cards.Red}, Cost: 2, Power: 3000,
			Counter: 1000, Traits: []string{"Supernovas", "Straw Hat Crew"}, Detailed: true, Legality: cards.Rotated,
			Effect: "[DON!! x2] This Character cannot be K.O.'d in battle by <Strike> attribute Characters.",
			Rarity: "Super Rare", Prints: []cards.Print{{Key: "OP01-024"}, {Key: "OP01-024_p1", Kind: "aa"}},
		},
		"OP14-018": {
			Code: "OP14-018", Name: "Time for the Counterattack", Type: "Event", Colors: []cards.Color{cards.Green}, Cost: 1,
			Effect: "[Counter] Up to 1 of your Leader gains +3000 power.\n[Trigger] Draw 1 card.", Detailed: true,
			Legality: cards.Legal, Rarity: "Common",
		},
		"OP14-020": {
			Code: "OP14-020", Name: "Dracule Mihawk", Type: "Leader", Colors: []cards.Color{cards.Green}, Cost: -1, Life: 5,
			Power: 5000, Traits: []string{"The Seven Warlords of the Sea"}, Detailed: true, Legality: cards.Banned, Rarity: "Leader",
		},
	}}
	db := cards.NewDB()
	lib.Apply(db)
	return db
}

func cardsTester() (*view, *ui.Tester) {
	s := testSnapshot(4)
	s.Cards = testCards()
	v, tt := tester(s)
	v.page = pageCards
	// The split and the wall size themselves from the frame before.
	tt.Frame()
	tt.Frame()
	return v, tt
}

var _ = Describe("Cards page", func() {
	codes := func(cs []cards.Card) []string {
		var out []string
		for _, c := range cs {
			out = append(out, c.Code)
		}
		return out
	}

	It("lists the newest sets first", func() {
		v, _ := cardsTester()
		Expect(codes(v.cardsUI.results(v.snap.Cards))).To(Equal([]string{"OP14-002", "OP14-018", "OP14-020", "OP01-024"}))
	})

	It("narrows by color, type and cost together", func() {
		v, _ := cardsTester()
		s := &v.cardsUI
		s.toggleColor(cards.Red)
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP14-002", "OP01-024"}))
		s.toggleCost(2)
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP01-024"}))
		s.toggleCost(2)
		s.toggleColor(cards.Red)
		s.kind = 3
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP14-018"}))
	})

	It("counts the cost curve of the cards that match the other filters", func() {
		v, _ := cardsTester()
		s := &v.cardsUI
		s.toggleColor(cards.Red)
		s.toggleCost(2)
		s.results(v.snap.Cards)
		Expect(s.curve[2]).To(Equal(1))
		Expect(s.curve[3]).To(Equal(1))
		Expect(s.curve[1]).To(Equal(0))
	})

	It("leaves leaders out once a cost is picked", func() {
		v, _ := cardsTester()
		s := &v.cardsUI
		s.toggleColor(cards.Green)
		s.toggleCost(1)
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP14-018"}))
	})

	It("searches names, traits and effects", func() {
		v, _ := cardsTester()
		s := &v.cardsUI
		s.query = "draw"
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP14-002", "OP14-018"}))
		s.query = "warlords"
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP14-020"}))
	})

	It("turns a picked suggestion into a chip", func() {
		v, _ := cardsTester()
		s := &v.cardsUI
		Expect(s.pool.suggestions).To(ContainElements("Supernovas · Trait", "Super Rare · Rarity", "Strike · Attribute"))
		s.query = "Supernovas · Trait"
		Expect(s.takeSuggestion()).To(BeTrue())
		Expect(s.query).To(BeEmpty())
		Expect(s.chips).To(Equal([]cardChip{{field: chipTrait, value: "Supernovas"}}))
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP14-002", "OP01-024"}))
	})

	It("takes either value of one field and every field", func() {
		v, _ := cardsTester()
		s := &v.cardsUI
		s.toggleChip(cardChip{field: chipLegality, value: "Banned"})
		s.toggleChip(cardChip{field: chipLegality, value: "Rotated"})
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP14-020", "OP01-024"}))
		s.toggleChip(cardChip{field: chipTrait, value: "Supernovas"})
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP01-024"}))
	})

	It("hides a value on the second click and clears it on the third", func() {
		v, _ := cardsTester()
		s := &v.cardsUI
		s.cycleChip(chipLegality, "Rotated")
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP01-024"}))
		s.cycleChip(chipLegality, "Rotated")
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP14-002", "OP14-018", "OP14-020"}))
		s.cycleChip(chipTrait, "Supernovas")
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP14-002"}))
		s.cycleChip(chipLegality, "Rotated")
		Expect(s.chips).To(Equal([]cardChip{{field: chipTrait, value: "Supernovas"}}))
	})

	It("finds keywords, with every DON!! count as one", func() {
		v, _ := cardsTester()
		s := &v.cardsUI
		s.toggleChip(cardChip{field: chipKeyword, value: "DON!!"})
		Expect(codes(s.results(v.snap.Cards))).To(Equal([]string{"OP01-024"}))
	})

	It("shows the count and the first card's details, and selects a card on click", func() {
		v, tt := cardsTester()
		for _, text := range []string{"4 cards", "Urouge", "Legal in Standard", "Supernovas"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.Click("Dracule Mihawk OP14-020")).To(Succeed())
		tt.Frame()
		Expect(v.cardsUI.selected).To(Equal("OP14-020"))
		Expect(tt.HasText("Banned")).To(BeTrue())
		Expect(tt.HasText("Life")).To(BeTrue())
	})

	It("filters by color from the quick row and clears", func() {
		v, tt := cardsTester()
		Expect(tt.Click("Green")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("2 cards")).To(BeTrue())
		Expect(tt.Click("Clear the filters")).To(Succeed())
		tt.Frame()
		Expect(v.cardsUI.filtered()).To(BeFalse())
	})

	It("says what matched nothing", func() {
		v, tt := cardsTester()
		v.cardsUI.query = "zzz"
		tt.Frame()
		Expect(tt.HasText("No cards match")).To(BeTrue())
	})

	It("offers to download the cards before there are any", func() {
		s := testSnapshot(4)
		s.Cards = cards.NewDB()
		v, tt := tester(s)
		v.page = pageCards
		tt.Frame()
		Expect(tt.HasText("Cards aren't downloaded yet")).To(BeTrue())
	})

	It("ticks off each set while the first download runs, and says so in the count after", func() {
		s := testSnapshot(4)
		s.Cards = cards.NewDB()
		v, tt := tester(s)
		v.page = pageCards
		v.cardSync.add(cards.SetProgress{Done: 1, Total: 3, Set: "OP18 The Dominance of God", Cards: 123})
		tt.Frame()
		for _, text := range []string{"Reading every card from Limitless", "1 of 3 sets", "OP18 The Dominance of God · 123 cards"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		v.snap.Cards = testCards()
		tt.Frame()
		Expect(tt.HasText("Updating · 1 of 3 sets")).To(BeTrue())
	})

	It("picks a cost from the curve", func() {
		v, tt := cardsTester()
		Expect(tt.Click("Cost 2, 1 card")).To(Succeed())
		tt.Frame()
		Expect(v.cardsUI.costs).To(Equal([]int{2}))
	})

	It("resizes the card panel by its edge, folds it, and opens it again", func() {
		v, tt := cardsTester()
		tt.Frame()
		edge := func() float32 {
			b := v.cardsUI.splitBox
			return b.X + b.W - v.prefs.panelWidth - 0.5
		}
		drag := func(by float32) {
			GinkgoHelper()
			from := edge()
			tt.Press(from, 500)
			tt.Frame()
			step := float32(3)
			if by < 0 {
				step = -3
			}
			for d := step; (by > 0 && d <= by) || (by < 0 && d >= by); d += step {
				tt.Move(from+d, 500)
				tt.Frame()
			}
			tt.Release(from+by, 500)
			tt.Frame()
		}
		drag(-60)
		Expect(v.prefs.panelWidth).To(BeNumerically("~", panelDefault+60, 3))
		drag(400)
		Expect(v.prefs.panelWidth).To(BeEquivalentTo(panelStrip))
		Expect(tt.Click("Show card details")).To(Succeed())
		tt.Frame()
		Expect(v.prefs.panelWidth).To(BeEquivalentTo(panelDefault))
	})

	It("puts a printing in the sim from the dot under it, and offers to undo", func() {
		v, tt := cardsTester()
		fa := &fakeActions{}
		v.act = fa
		v.cardsUI.selected = "OP01-024"
		v.setSimArt("OP01-024", simArtInfo{cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		tt.Frame()
		Expect(tt.Click("Art · 3")).To(Succeed())
		tt.Frame()
		Expect(v.cardsUI.tab).To(Equal(tabArt))
		for _, text := range []string{"Base", "Alt art", "Sim's own", "In sim"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.Click("Use Alt art in the sim")).To(Succeed())
		tt.Frame()
		Expect(fa.calls).To(ContainElement("use art OP01-024 OP01-024_p1"))
		Expect(v.simArt.cards["OP01-024"].busy).To(BeTrue())

		luffy := v.snap.card("OP01-024")
		alt := sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024_p1"}
		prev := sim.ArtChoice{Kind: sim.ArtOwn}
		v.simArtDone(luffy, alt, simArtInfo{cur: alt}, &prev, nil)
		tt.Frame()
		Expect(tt.HasText("The sim now shows Monkey D. Luffy's Alt art.")).To(BeTrue())
		Expect(v.simArt.changed["OP01-024"]).To(BeTrue())
		Expect(tt.Click("Undo")).To(Succeed())
		tt.Frame()
		Expect(fa.calls).To(ContainElement("use art OP01-024 "))
	})

	It("shows the picture a click picks without changing the sim", func() {
		v, tt := cardsTester()
		fa := &fakeActions{}
		v.act = fa
		v.cardsUI.selected = "OP01-024"
		v.setSimArt("OP01-024", simArtInfo{cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		v.cardsUI.tab = tabArt
		tt.Frame()
		Expect(tt.Click("Show Alt art")).To(Succeed())
		tt.Frame()
		Expect(v.cardsUI.print).To(Equal(1))
		Expect(tt.Click("Show Sim's own")).To(Succeed())
		tt.Frame()
		Expect(v.cardsUI.print).To(Equal(viewSimOwn))
		Expect(v.viewedKey(v.snap.card("OP01-024"), simArtInfo{})).To(Equal("sim:OP01-024"))
		Expect(fa.calls).NotTo(ContainElement(HavePrefix("use art")))
	})

	It("keeps a file of the user's as a choice and says what happens to it", func() {
		v, tt := cardsTester()
		v.cardsUI.selected = "OP01-024"
		v.setSimArt("OP01-024", simArtInfo{cur: sim.ArtChoice{Kind: sim.ArtFile}, ownFile: `C:\sim\OP01-024_OVERRIDE.png`})
		v.cardsUI.tab = tabArt
		tt.Frame()
		Expect(tt.HasText("Your file")).To(BeTrue())
		Expect(tt.HasText("The sim shows a file you added yourself.")).To(BeTrue())
	})

	It("keeps printings out of the sim while art downloads are off", func() {
		v, tt := cardsTester()
		fa := &fakeActions{}
		v.act = fa
		v.prefs.artOff = true
		v.cardsUI.selected = "OP01-024"
		v.setSimArt("OP01-024", simArtInfo{cur: sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024"}})
		v.cardsUI.tab = tabArt
		tt.Frame()
		Expect(tt.HasText("Card art downloads are off in Settings, so only the sim's own art and your own file can be used.")).
			To(BeTrue())
		Expect(tt.Click("Use Alt art in the sim")).To(Succeed())
		tt.Frame()
		Expect(fa.calls).To(BeEmpty())
		Expect(tt.Click("Use Sim's own in the sim")).To(Succeed())
		tt.Frame()
		Expect(fa.calls).To(Equal([]string{"use art OP01-024 "}))
	})

	It("says why the dots are off without a sim", func() {
		v, tt := cardsTester()
		fa := &fakeActions{}
		v.act = fa
		v.cardsUI.selected = "OP01-024"
		v.setSimArt("OP01-024", simArtInfo{noSim: true})
		v.cardsUI.tab = tabArt
		tt.Frame()
		Expect(tt.HasText("logpose can't find the sim, so it can't change the sim's art.")).To(BeTrue())
		Expect(tt.Click("Choose the sim's folder")).To(Succeed())
		Expect(fa.calls).To(ContainElement("choose sim"))
		Expect(fa.calls).NotTo(ContainElement(HavePrefix("use art")))
	})

	It("keeps the effect on the Card tab and names the sim's art above the tabs", func() {
		v, tt := cardsTester()
		v.cardsUI.selected = "OP01-024"
		v.setSimArt("OP01-024", simArtInfo{cur: sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024_p1"}})
		tt.Frame()
		Expect(tt.HasText("This Character cannot be K.O.'d")).To(BeTrue())
		Expect(tt.HasText("Sim's own")).To(BeFalse())
		Expect(tt.Click("In the sim: Alt art")).To(Succeed())
		tt.Frame()
		Expect(v.cardsUI.tab).To(Equal(tabArt))
		Expect(tt.HasText("This Character cannot be K.O.'d")).To(BeFalse())
		Expect(tt.HasText("Sim's own")).To(BeTrue())
	})

	It("names SP cards and new art apart from reprints of the same art", func() {
		base := cards.Print{Key: "OP07-019", Artist: "Ryuda"}
		card := cards.Card{Code: "OP07-019", Prints: []cards.Print{base}}
		Expect(printName(card, base)).To(Equal("Base"))
		Expect(printName(card, cards.Print{Key: "OP07-019_p2", Rarity: "Special Card", Artist: "Sunohara"})).To(Equal("SP"))
		Expect(printName(card, cards.Print{Key: "OP07-019_p3", Rarity: "Leader", Artist: "DAI-XT."})).To(Equal("Alt art"))
		Expect(printName(card, cards.Print{Key: "OP07-019_p4", Set: "Anime 25th Collection"})).To(Equal("Reprint"))
		Expect(printName(card, cards.Print{Key: "OP07-019_p5", Artist: "Ryuda"})).To(Equal("Reprint"))
		Expect(printName(card, cards.Print{Key: "OP07-019_p6", Kind: "aa", Rarity: "Treasure Rare"})).To(Equal("Treasure"))
		sp := cards.Print{Key: "OP07-019_p2", Rarity: "Special Card", Set: "The Azure Sea's Seven"}
		Expect(menuPrintName(card, sp)).To(Equal("SP, The Azure Sea's Seven"))
		Expect(menuPrintName(card, base)).To(Equal("Base"))
	})

	It("draws keywords as on the card and traits in gold", func() {
		spans := effectSpans("[On Play] Give {Supernovas} +1000.\n[Trigger] Draw 1.", graphite())
		Expect(spans[0].Text).To(Equal("On Play"))
		Expect(spans[0].Background).To(Equal(kwBlue))
		Expect(spans[2].Text).To(Equal("{Supernovas}"))
		Expect(spans[2].Color).To(Equal(cardGold))
		Expect(spans[5].Text).To(Equal("Trigger"))
		Expect(spans[5].Background).To(Equal(kwYellow))
	})
})
