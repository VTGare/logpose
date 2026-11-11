package gui

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
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

var _ = Describe("printedPower", func() {
	It("shows a character's 0 power and no power for events", func() {
		Expect(printedPower(cards.Card{Type: "Character"})).To(Equal("0"))
		Expect(printedPower(cards.Card{Type: "Leader", Power: 5000})).To(Equal("5000"))
		Expect(printedPower(cards.Card{Type: "Event"})).To(Equal("–"))
		Expect(printedPower(cards.Card{})).To(Equal("–"))
	})
})

var _ = Describe("turns", func() {
	It("says one turn in the singular", func() {
		Expect(turns(1)).To(Equal("1 turn"))
		Expect(turns(12)).To(Equal("12 turns"))
	})
})

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

	It("keeps the wall's width when the page is opened again", func() {
		v, tt := cardsTester()
		wall, split := v.cardsUI.wallBox, v.cardsUI.splitBox
		Expect(wall.W).To(BeNumerically(">", 0))
		v.page = pageFormats
		tt.Frame()
		v.page = pageCards
		tt.Frame()
		Expect(v.cardsUI.wallBox).To(Equal(wall))
		Expect(v.cardsUI.splitBox).To(Equal(split))
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

	It("closes the message after a change without undoing it", func() {
		v, tt := cardsTester()
		fa := &fakeActions{}
		v.act = fa
		v.cardsUI.selected = "OP01-024"
		luffy := v.snap.card("OP01-024")
		alt := sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024_p1"}
		prev := sim.ArtChoice{Kind: sim.ArtOwn}
		v.simArtDone(luffy, alt, simArtInfo{cur: alt}, &prev, nil)
		tt.Frame()
		Expect(tt.HasText("The sim now shows Monkey D. Luffy's Alt art.")).To(BeTrue())
		Expect(tt.Click("Close")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("The sim now shows Monkey D. Luffy's Alt art.")).To(BeFalse())
		Expect(fa.calls).NotTo(ContainElement("use art OP01-024 "))
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
		luffy := v.snap.card("OP01-024")
		Expect(v.viewedKey(luffy, simArtInfo{}, arts.LimitlessArts(luffy))).To(Equal("OP01-024_p1"))
		Expect(tt.Click("Show Sim's own")).To(Succeed())
		tt.Frame()
		Expect(v.viewedKey(luffy, simArtInfo{}, arts.LimitlessArts(luffy))).To(Equal("sim:OP01-024"))
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

	It("names each art in the menu with its product, but the base card", func() {
		card := cards.Card{Code: "OP07-019"}
		base := arts.Version{Key: arts.Key{Source: arts.Limitless, Print: "OP07-019"}, Name: "Base", Product: "500 Years in the Future"}
		sp := arts.Version{Key: arts.Key{Source: arts.Limitless, Print: "OP07-019_p2"}, Name: "SP", Product: "The Azure Sea's Seven"}
		cup := arts.Version{Key: arts.Key{Source: arts.Bandai, Print: "OP07-019_p3"}, Name: "Treasure Cup", Product: "Treasure Cup November 2025~"}
		Expect(menuName(card, base)).To(Equal("Base"))
		Expect(menuName(card, sp)).To(Equal("SP, The Azure Sea's Seven"))
		Expect(menuName(card, cup)).To(Equal("Treasure Cup, Treasure Cup November 2025~"))
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

	Describe("an art printed with several stamps", func() {
		// regional is Urouge's base card and one Regional art stamped six ways, keyed as the
		// test pool's Urouge.
		regional := func(pending int) arts.CardArts {
			stamp := func(key, short, product string) arts.Version {
				return arts.Version{Key: arts.Key{Source: arts.Bandai, Print: key}, Name: "Regional", Short: short, Product: product}
			}
			return arts.CardArts{Pending: pending, Arts: []arts.Art{
				{
					Versions: []arts.Version{{Key: arts.Key{Source: arts.Limitless, Print: "OP14-002"}, Name: "Base", Short: "Base", Product: "The Azure Sea's Seven"}},
					Same:     []arts.Key{{Source: arts.Bandai, Print: "OP14-002"}},
				},
				{Versions: []arts.Version{
					stamp("OP14-002_p1", "Offline participant", "Offline Regional Participation Pack 2025 Vol.1"),
					stamp("OP14-002_p3", "Offline finalist", "Offline Regional Finalist Card Set 25-26 Season 1"),
					stamp("OP14-002_p4", "Offline champion", "Offline Regional Champion Card Set 25-26 Season 1"),
					stamp("OP14-002_p5", "Online participant", "Online Regional Participation Pack 25-26 Season 1"),
					stamp("OP14-002_p6", "Online finalist", "Online Regional Finalist Card Set 25-26 Season 1"),
					stamp("OP14-002_p7", "Online champion", "Online Regional Champion Card Set 25-26 Season 1"),
				}},
			}}
		}
		open := func(cur sim.ArtChoice, ca arts.CardArts) (*view, *ui.Tester, *fakeActions) {
			v, tt := cardsTester()
			fa := &fakeActions{}
			v.act = fa
			v.setSimArt("OP14-002", simArtInfo{cur: cur})
			v.setArts(v.arts.gen, "OP14-002", ca)
			v.cardsUI.tab = tabArt
			tt.Frame()
			return v, tt, fa
		}

		It("shows once with a count, and lists each version with its product and its own dot", func() {
			_, tt, fa := open(sim.ArtChoice{Kind: sim.ArtOwn}, regional(0))
			Expect(tt.HasText("Art · 3")).To(BeTrue())
			Expect(tt.HasText("Regional")).To(BeTrue())
			Expect(tt.HasText("6")).To(BeTrue())
			Expect(tt.HasText("Offline Regional Finalist Card Set 25-26 Season 1")).To(BeFalse())

			Expect(tt.Click("Show Regional, 6 versions")).To(Succeed())
			tt.Frame()
			for _, text := range []string{"Regional · 6 versions", "Offline finalist", "Offline Regional Finalist Card Set 25-26 Season 1"} {
				Expect(tt.HasText(text)).To(BeTrue(), text)
			}
			Expect(tt.Click("Use Offline finalist, Offline Regional Finalist Card Set 25-26 Season 1 in the sim")).To(Succeed())
			tt.Frame()
			Expect(fa.calls).To(ContainElement("use art OP14-002 OP14-002_p3_bd"))
		})

		It("shows a version large when its row is clicked", func() {
			v, tt, _ := open(sim.ArtChoice{Kind: sim.ArtOwn}, regional(0))
			Expect(tt.Click("Show Regional, 6 versions")).To(Succeed())
			tt.Frame()
			Expect(v.viewedKey(v.snap.card("OP14-002"), simArtInfo{}, regional(0))).To(Equal("OP14-002_p1_bd"))
			Expect(tt.Click("Show Online champion, Online Regional Champion Card Set 25-26 Season 1")).To(Succeed())
			tt.Frame()
			Expect(v.viewedKey(v.snap.card("OP14-002"), simArtInfo{}, regional(0))).To(Equal("OP14-002_p7_bd"))
		})

		It("is in the sim when any version is, and names it above the tabs", func() {
			_, tt, _ := open(sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP14-002_p5_bd"}, regional(0))
			Expect(tt.HasText("In the sim: Regional")).To(BeTrue())
			Expect(tt.HasText("In sim")).To(BeTrue())
		})

		It("counts Bandai's copy of a printing as that printing", func() {
			_, tt, _ := open(sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP14-002_bd"}, regional(0))
			Expect(tt.HasText("In the sim: the base card")).To(BeTrue())
		})

		It("offers each version in the right-click menu", func() {
			_, tt, fa := open(sim.ArtChoice{Kind: sim.ArtOwn}, regional(0))
			Expect(tt.RightClick("Urouge OP14-002")).To(Succeed())
			tt.Frame()
			Expect(tt.ChooseMenuItem("Art in the sim", "Regional · 6 versions", "Online finalist")).NotTo(Succeed())
			Expect(tt.RightClick("Urouge OP14-002")).To(Succeed())
			tt.Frame()
			Expect(tt.ChooseMenuItem("Art in the sim", "Regional · 6 versions", "Online Regional Finalist Card Set 25-26 Season 1")).
				To(Succeed())
			tt.Frame()
			Expect(fa.calls).To(ContainElement("use art OP14-002 OP14-002_p6_bd"))
		})

		It("says how many printings are still being compared", func() {
			_, tt, _ := open(sim.ArtChoice{Kind: sim.ArtOwn}, regional(3))
			Expect(tt.HasText("Checking 3 more")).To(BeTrue())
		})

		It("asks for a card's arts once, again after a reset, and drops a step from before it", func() {
			v, tt := cardsTester()
			fa := &fakeActions{}
			v.act = fa
			v.setSimArt("OP14-002", simArtInfo{cur: sim.ArtChoice{Kind: sim.ArtOwn}})
			v.resetArts()
			tt.Frame()
			tt.Frame()
			Expect(fa.artLoads).To(Equal([]string{"OP14-002 1"}))

			v.resetArts()
			v.setArts(1, "OP14-002", regional(0))
			tt.Frame()
			Expect(fa.artLoads).To(Equal([]string{"OP14-002 1", "OP14-002 2"}))
			Expect(tt.HasText("Art · 2")).To(BeTrue())
			v.setArts(2, "OP14-002", regional(0))
			tt.Frame()
			Expect(tt.HasText("Art · 3")).To(BeTrue())
		})
	})

	Describe("the art viewer", func() {
		luffyViewer := func() (*view, *ui.Tester, *fakeActions) {
			v, tt := cardsTester()
			fa := &fakeActions{}
			v.act = fa
			v.cardsUI.selected = "OP01-024"
			v.setSimArt("OP01-024", simArtInfo{cur: sim.ArtChoice{Kind: sim.ArtOwn}})
			tt.Frame()
			Expect(tt.Click("View full size")).To(Succeed())
			tt.Frame()
			return v, tt, fa
		}
		viewed := func(v *view) string {
			luffy := v.snap.card("OP01-024")
			info, _ := v.simArtFor(luffy)
			return v.viewedKey(luffy, info, arts.LimitlessArts(luffy))
		}

		It("lists the card's pictures and names the one in the sim", func() {
			_, tt, _ := luffyViewer()
			for _, text := range []string{"Monkey D. Luffy", "Base", "Alt art", "Sim's own", "In the sim"} {
				Expect(tt.HasText(text)).To(BeTrue(), text)
			}
		})

		It("walks the pictures with the up and down keys, around the ends", func() {
			v, tt, _ := luffyViewer()
			tt.Key(0, ui.KeyDown)
			tt.Frame()
			Expect(viewed(v)).To(Equal("OP01-024_p1"))
			tt.Key(0, ui.KeyUp)
			tt.Frame()
			tt.Key(0, ui.KeyUp)
			tt.Frame()
			Expect(viewed(v)).To(Equal("sim:OP01-024"))
		})

		It("puts the picture a click picks in the sim", func() {
			v, tt, fa := luffyViewer()
			Expect(tt.Click("View Alt art")).To(Succeed())
			tt.Frame()
			Expect(viewed(v)).To(Equal("OP01-024_p1"))
			Expect(fa.calls).NotTo(ContainElement(HavePrefix("use art")))
			Expect(tt.Click("Use this art in the sim")).To(Succeed())
			tt.Frame()
			Expect(fa.calls).To(ContainElement("use art OP01-024 OP01-024_p1"))
		})

		It("closes on Escape and on its close button", func() {
			v, tt, _ := luffyViewer()
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			Expect(v.cardsUI.viewer).To(BeFalse())
			Expect(tt.Click("View the art full size")).To(Succeed())
			tt.Frame()
			Expect(v.cardsUI.viewer).To(BeTrue())
			Expect(tt.Click("Close the viewer")).To(Succeed())
			tt.Frame()
			Expect(v.cardsUI.viewer).To(BeFalse())
		})

		It("opens on the card whose menu asked for it", func() {
			v, tt := cardsTester()
			v.cardsUI.selected = "OP01-024"
			tt.Frame()
			Expect(tt.RightClick("Urouge OP14-002")).To(Succeed())
			Expect(tt.ChooseMenuItem("View art")).To(Succeed())
			tt.Frame()
			Expect(v.cardsUI.viewer).To(BeTrue())
			Expect(v.cardsUI.selected).To(Equal("OP14-002"))
		})
	})
})
