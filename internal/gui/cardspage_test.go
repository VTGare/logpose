package gui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/VTGare/logpose/internal/gui/cardspage"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// testCards is a small pool: two red Supernovas characters, a green event and a banned leader.
func testCards() map[string]cards.Card {
	return map[string]cards.Card{
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
	}
}

// cardsPage opens the Cards page on a pool. The page lists only cards with a name, a code and
// Detailed set.
func cardsWith(pool map[string]cards.Card) (*view, *ui.Tester, *fakeActions) {
	db := cards.NewDB()
	(&cards.Library{Cards: pool}).Apply(db)
	s := testSnapshot(4)
	s.Cards = db
	v, tt := tester(s)
	fa := &fakeActions{}
	v.setActions(fa)
	v.Page = page.Cards
	// More filters reaches below a window of the usual height.
	tt.SetSize(1180, 1000)
	// The split and the wall size themselves from the frame before.
	tt.Frame()
	tt.Frame()
	return v, tt, fa
}

func cardsTester() (*view, *ui.Tester, *fakeActions) { return cardsWith(testCards()) }

// shownCards is the codes of the pool's cards the page shows, in reading order.
func shownCards(tt *ui.Tester, pool map[string]cards.Card) []string {
	GinkgoHelper()
	tt.Frame()
	type tile struct {
		code string
		at   ui.Rect
	}
	var tiles []tile
	for code, card := range pool {
		if r, ok := tt.Find(card.Name + " " + code); ok {
			tiles = append(tiles, tile{code, r})
		}
	}
	slices.SortFunc(tiles, func(a, b tile) int {
		return cmp.Or(cmp.Compare(a.at.Y, b.at.Y), cmp.Compare(a.at.X, b.at.X))
	})
	out := []string{}
	for _, t := range tiles {
		out = append(out, t.code)
	}
	return out
}

func click(tt *ui.Tester, label string) {
	GinkgoHelper()
	Expect(tt.Click(label)).To(Succeed())
	tt.Frame()
}

func searchCards(tt *ui.Tester, text string) {
	GinkgoHelper()
	click(tt, "Search cards")
	tt.Type(text)
	tt.Frame()
}

func labelled(tt *ui.Tester, label string) bool {
	tt.Frame()
	_, ok := tt.Find(label)
	return ok
}

var _ = Describe("printedPower", func() {
	It("shows a character's 0 power and no power for events", func() {
		Expect(cardspage.PrintedPower(cards.Card{Type: "Character"})).To(Equal("0"))
		Expect(cardspage.PrintedPower(cards.Card{Type: "Leader", Power: 5000})).To(Equal("5000"))
		Expect(cardspage.PrintedPower(cards.Card{Type: "Event"})).To(Equal("–"))
		Expect(cardspage.PrintedPower(cards.Card{})).To(Equal("–"))
	})
})

var _ = Describe("powerRange", func() {
	It("names a range as card text does", func() {
		all := cardspage.PowerRange{Lo: 0, Hi: 13000}
		for r, want := range map[cardspage.PowerRange]string{
			all: "Any", {Lo: 6000, Hi: 6000}: "6000", {Lo: 0, Hi: 5000}: "5000 or less", {Lo: 6000, Hi: 13000}: "6000 or more",
			{Lo: 6000, Hi: 8000}: "6000–8000",
		} {
			Expect(r.Label(all)).To(Equal(want))
		}
	})
})

var _ = Describe("Cards page", func() {
	var (
		tt *ui.Tester
		fa *fakeActions
		v  *view
	)
	pool := testCards()
	shown := func() []string { return shownCards(tt, pool) }
	// Tiles name a card's legality as More filters does, so specs that click a legality use the list.
	more := func() {
		GinkgoHelper()
		if !labelled(tt, "Legal") {
			click(tt, "More filters")
		}
	}

	BeforeEach(func() { v, tt, fa = cardsTester() })
	// calls is what the page asked the app for, of the kinds with the prefix.
	calls := func(prefix string) []string {
		var out []string
		for _, c := range fa.calls {
			if strings.HasPrefix(c, prefix) {
				out = append(out, c)
			}
		}
		return out
	}
	artTab := func() {
		GinkgoHelper()
		tt.Frame()
		i := slices.IndexFunc(tt.Texts(), func(s string) bool { return strings.HasPrefix(s, "Art · ") })
		Expect(i).To(BeNumerically(">=", 0))
		click(tt, tt.Texts()[i])
	}

	It("lists the newest sets first", func() {
		Expect(shown()).To(Equal([]string{"OP14-002", "OP14-018", "OP14-020", "OP01-024"}))
	})

	It("narrows by color, type and cost together", func() {
		click(tt, "Red")
		Expect(shown()).To(Equal([]string{"OP14-002", "OP01-024"}))
		click(tt, "Cost 2, 1 card")
		Expect(shown()).To(Equal([]string{"OP01-024"}))
		click(tt, "Cost 2, 1 card")
		click(tt, "Red")
		click(tt, "Event")
		Expect(shown()).To(Equal([]string{"OP14-018"}))
	})

	It("counts the cost curve of the cards that match the other filters", func() {
		click(tt, "Red")
		click(tt, "Cost 2, 1 card")
		Expect(labelled(tt, "Cost 2, 1 card")).To(BeTrue())
		Expect(labelled(tt, "Cost 3, 1 card")).To(BeTrue())
		Expect(labelled(tt, "Cost 1, 0 cards")).To(BeTrue())
	})

	It("leaves leaders out once a cost is picked", func() {
		click(tt, "Green")
		Expect(shown()).To(Equal([]string{"OP14-018", "OP14-020"}))
		click(tt, "Cost 1, 1 card")
		Expect(shown()).To(Equal([]string{"OP14-018"}))
	})

	It("searches names, traits and effects", func() {
		searchCards(tt, "draw")
		Expect(shown()).To(Equal([]string{"OP14-002", "OP14-018"}))
		click(tt, "Clear the filters")
		searchCards(tt, "warlords")
		Expect(shown()).To(Equal([]string{"OP14-020"}))
	})

	It("turns a picked suggestion into a chip", func() {
		searchCards(tt, "Supernovas · Trait")
		Expect(labelled(tt, "Remove Supernovas")).To(BeTrue())
		Expect(tt.HasText("Supernovas · Trait")).To(BeFalse())
		Expect(shown()).To(Equal([]string{"OP14-002", "OP01-024"}))
		click(tt, "Clear the filters")
		searchCards(tt, "Super Rare · Rarity")
		Expect(shown()).To(Equal([]string{"OP01-024"}))
		click(tt, "Clear the filters")
		searchCards(tt, "Strike · Attribute")
		Expect(shown()).To(Equal([]string{"OP14-002"}))
	})

	It("takes either value of one field and every field", func() {
		click(tt, "List")
		more()
		click(tt, "Banned")
		more()
		click(tt, "Rotated")
		Expect(shown()).To(Equal([]string{"OP14-020", "OP01-024"}))
		searchCards(tt, "Supernovas · Trait")
		Expect(shown()).To(Equal([]string{"OP01-024"}))
	})

	It("hides a value on the second click and clears it on the third", func() {
		click(tt, "List")
		more()
		click(tt, "Rotated")
		Expect(shown()).To(Equal([]string{"OP01-024"}))
		more()
		click(tt, "Rotated, shown only. Click to hide")
		Expect(shown()).To(Equal([]string{"OP14-002", "OP14-018", "OP14-020"}))
		searchCards(tt, "Supernovas · Trait")
		Expect(shown()).To(Equal([]string{"OP14-002"}))
		more()
		click(tt, "Rotated, hidden. Click to clear")
		Expect(labelled(tt, "Remove Rotated")).To(BeFalse())
		Expect(labelled(tt, "Remove Supernovas")).To(BeTrue())
		Expect(shown()).To(Equal([]string{"OP14-002", "OP01-024"}))
	})

	It("finds keywords, with every DON!! count as one", func() {
		// A bracketed word is offered as a keyword once enough cards have it.
		many := map[string]cards.Card{"OP15-100": {Code: "OP15-100", Name: "Plain", Type: "Character", Effect: "[On Play] Draw 1 card.", Detailed: true}}
		for i := range 16 {
			code := fmt.Sprintf("OP15-%03d", i)
			many[code] = cards.Card{Code: code, Name: "Card " + code, Type: "Character", Effect: fmt.Sprintf("[DON!! x%d] Rush.", i%2+1), Detailed: true}
		}
		_, tt, _ = cardsWith(many)
		Expect(tt.HasText("17 cards")).To(BeTrue())
		more()
		click(tt, "DON!!")
		Expect(tt.HasText("16 cards")).To(BeTrue())
	})

	It("shows the count and the first card's details, and selects a card on click", func() {
		for _, text := range []string{"4 cards", "Urouge", "Legal in Standard", "Supernovas"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("Life")).To(BeFalse())
		click(tt, "Dracule Mihawk OP14-020")
		Expect(tt.HasText("Legal in Standard")).To(BeFalse())
		Expect(tt.HasText("Life")).To(BeTrue())
	})

	It("shows cards with either trait, or with both once the joiner says and", func() {
		searchCards(tt, "Supernovas · Trait")
		searchCards(tt, "Straw Hat Crew · Trait")
		Expect(shown()).To(Equal([]string{"OP14-002", "OP01-024"}))
		click(tt, "Show cards with all of these traits")
		Expect(shown()).To(Equal([]string{"OP01-024"}))
		Expect(labelled(tt, "Show cards with any of these traits")).To(BeTrue())
		click(tt, "Show cards with any of these traits")
		Expect(shown()).To(Equal([]string{"OP14-002", "OP01-024"}))
	})

	It("starts a field from either-or again once its chips are gone", func() {
		searchCards(tt, "Supernovas · Trait")
		searchCards(tt, "Straw Hat Crew · Trait")
		click(tt, "Show cards with all of these traits")
		click(tt, "Remove Straw Hat Crew")
		click(tt, "Remove Supernovas")
		searchCards(tt, "Supernovas · Trait")
		searchCards(tt, "Fallen Monk Pirates · Trait")
		Expect(labelled(tt, "Show cards with all of these traits")).To(BeTrue())
		Expect(shown()).To(Equal([]string{"OP14-002", "OP01-024"}))
	})

	It("offers no joiner for a field a card has one value of", func() {
		searchCards(tt, "Super Rare · Rarity")
		searchCards(tt, "Leader · Rarity")
		Expect(shown()).To(Equal([]string{"OP14-020", "OP01-024"}))
		tt.Frame()
		Expect(slices.ContainsFunc(tt.Texts(), func(t string) bool {
			return strings.HasPrefix(t, "Show cards with")
		})).To(BeFalse())
	})

	It("filters by color from the quick row and clears", func() {
		click(tt, "Green")
		Expect(tt.HasText("2 cards")).To(BeTrue())
		click(tt, "Clear the filters")
		Expect(tt.HasText("4 cards")).To(BeTrue())
		Expect(labelled(tt, "Clear the filters")).To(BeFalse())
	})

	It("works the results out again when any filter changes, and clears every filter", func() {
		set := map[string]func(){
			"search": func() { searchCards(tt, "luffy") },
			"color":  func() { click(tt, "Green") },
			"type":   func() { click(tt, "Leader") },
			"cost":   func() { click(tt, "Cost 3, 1 card") },
			"power": func() {
				click(tt, "Power: Any")
				click(tt, "maximum")
				tt.Key(0, ui.KeyLeft)
				tt.Frame()
			},
			"chip": func() { searchCards(tt, "Supernovas · Trait") },
		}
		for name, f := range set {
			f()
			Expect(tt.HasText("4 cards")).To(BeFalse(), name)
			click(tt, "Clear the filters")
			Expect(tt.HasText("4 cards")).To(BeTrue(), name)
			Expect(labelled(tt, "Clear the filters")).To(BeFalse(), name)
		}
	})

	It("finds cards by every legality and counter the More filters offer", func() {
		// The event's panel shows no counter that a click could hit instead.
		click(tt, "List")
		click(tt, "Time for the Counterattack OP14-018")
		for label, n := range map[string]int{
			"Legal": 2, "Rotated": 1, "Banned": 1, "Restricted": 0, "Upcoming": 0,
			"No counter": 2, "+1000": 2, "+2000": 0,
		} {
			more()
			click(tt, label)
			Expect(shown()).To(HaveLen(n), label)
			click(tt, "Clear the filters")
		}
	})

	It("writes counters short in the list", func() {
		click(tt, "List")
		Expect(tt.HasText("+1k")).To(BeTrue())
	})

	It("says what matched nothing", func() {
		searchCards(tt, "zzz")
		Expect(tt.HasText("No cards match")).To(BeTrue())
	})

	It("offers to download the cards before there are any", func() {
		_, tt, _ = cardsWith(nil)
		Expect(tt.HasText("Cards aren't downloaded yet")).To(BeTrue())
	})

	It("ticks off each set while the first download runs, and says so in the count after", func() {
		v, tt, _ = cardsWith(nil)
		v.addCardSync(cards.SetProgress{Done: 1, Total: 3, Set: "OP18 The Dominance of God", Cards: 123})
		tt.Frame()
		for _, text := range []string{"Reading every card from Limitless", "1 of 3 sets", "OP18 The Dominance of God · 123 cards"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		db := cards.NewDB()
		(&cards.Library{Cards: pool}).Apply(db)
		v.Snap.Cards = db
		tt.Frame()
		Expect(tt.HasText("Updating · 1 of 3 sets")).To(BeTrue())
	})

	It("keeps the wall's width when the page is opened again", func() {
		tile := func() ui.Rect {
			tt.Frame()
			r, ok := tt.Find("Dracule Mihawk OP14-020")
			Expect(ok).To(BeTrue())
			return r
		}
		before := tile()
		click(tt, "Formats")
		click(tt, "Cards")
		Expect(tile()).To(Equal(before))
	})

	It("picks a cost from the curve", func() {
		click(tt, "Cost 2, 1 card")
		Expect(shown()).To(Equal([]string{"OP01-024"}))
	})

	It("offers every cost up to the highest a card has", func() {
		Expect(labelled(tt, "Cost 0, 0 cards")).To(BeTrue())
		Expect(labelled(tt, "Cost 3, 1 card")).To(BeTrue())
		Expect(labelled(tt, "Cost 4, 0 cards")).To(BeFalse())
	})

	// lowerPower opens the power range and moves its top down by n steps.
	lowerPower := func(n int) {
		GinkgoHelper()
		click(tt, "Power: Any")
		click(tt, "maximum")
		for range n {
			tt.Key(0, ui.KeyLeft)
			tt.Frame()
		}
	}

	It("narrows by a power range and offers the thresholds card text uses", func() {
		lowerPower(2)
		Expect(labelled(tt, "Power: 3000 or less")).To(BeTrue())
		Expect(shown()).To(Equal([]string{"OP14-002", "OP01-024"}))
		click(tt, "Exactly 3000")
		Expect(shown()).To(Equal([]string{"OP01-024"}))
		click(tt, "3000 or more")
		Expect(shown()).To(Equal([]string{"OP14-020", "OP01-024"}))
		click(tt, "Clear the power range")
		Expect(labelled(tt, "Power: Any")).To(BeTrue())
		Expect(shown()).To(HaveLen(4))
	})

	It("drops the power range for a type without power and keeps it closed", func() {
		lowerPower(1)
		click(tt, "Event")
		Expect(shown()).To(Equal([]string{"OP14-018"}))
		Expect(labelled(tt, "Power: Any")).To(BeTrue())
		click(tt, "Power: Any")
		Expect(labelled(tt, "Drag either end to narrow the power.")).To(BeFalse())
	})

	It("disables the shortcuts that repeat one or mean any power at the lowest power", func() {
		lowerPower(3)
		Expect(labelled(tt, "Power: 2000")).To(BeTrue())
		for _, name := range []string{"2000 or less", "2000 or more"} {
			Expect(labelled(tt, name)).To(BeTrue(), name)
			_ = tt.Click(name)
			tt.Frame()
			Expect(labelled(tt, "Power: 2000")).To(BeTrue(), name)
		}
		Expect(shown()).To(Equal([]string{"OP14-002"}))
	})

	It("keeps the power slider in place while its range changes", func() {
		wide := testCards()
		wide["OP15-001"] = cards.Card{
			Code: "OP15-001", Name: "Kaido", Type: "Character", Colors: []cards.Color{cards.Purple}, Cost: 10,
			Power: 13000, Detailed: true, Legality: cards.Legal,
		}
		wide["OP15-002"] = cards.Card{
			Code: "OP15-002", Name: "Pincers", Type: "Character", Colors: []cards.Color{cards.Red}, Cost: 1,
			Detailed: true, Legality: cards.Legal,
		}
		_, tt, _ = cardsWith(wide)
		// The default window, where the popover meets the window's edge.
		tt.SetSize(1180, 780)
		tt.Frame()
		click(tt, "Power: Any")
		click(tt, "maximum")
		knob := func() float32 {
			r, ok := tt.Find("minimum")
			Expect(ok).To(BeTrue())
			return r.X
		}
		at := knob()
		for range 13 {
			tt.Key(0, ui.KeyLeft)
			tt.Frame()
			Expect(knob()).To(Equal(at))
		}
	})

	It("stretches an open end of the range to new cards", func() {
		lowerPower(0)
		click(tt, "minimum")
		tt.Key(0, ui.KeyRight)
		tt.Frame()
		Expect(labelled(tt, "Power: 3000 or more")).To(BeTrue())
		more := testCards()
		more["OP15-001"] = cards.Card{
			Code: "OP15-001", Name: "Kaido", Type: "Character", Colors: []cards.Color{cards.Purple}, Cost: 10,
			Power: 12000, Detailed: true, Legality: cards.Legal, Rarity: "Super Rare",
		}
		db := cards.NewDB()
		(&cards.Library{Cards: more}).Apply(db)
		v.Snap.Cards = db
		tt.Frame()
		Expect(labelled(tt, "Power: 3000 or more")).To(BeTrue())
		Expect(shownCards(tt, more)).To(ContainElement("OP15-001"))
	})

	It("resizes the card panel by its edge, folds it, and opens it again", func() {
		// The panel's edge is its margin and padding left of the card art.
		edge := func() float32 {
			GinkgoHelper()
			tt.Frame()
			r, ok := tt.Find("View the art full size")
			Expect(ok).To(BeTrue())
			return r.X - 33.5
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
		start := edge()
		drag(-60)
		Expect(edge()).To(BeNumerically("~", start-60, 3))
		Expect(tt.HasText("Show card details")).To(BeFalse())
		drag(400)
		Expect(labelled(tt, "Show card details")).To(BeTrue())
		Expect(labelled(tt, "View the art full size")).To(BeFalse())
		click(tt, "Show card details")
		Expect(edge()).To(BeNumerically("~", start, 1))
	})

	Describe("the sim's art", func() {
		luffy := func(info cardspage.SimArtInfo) {
			GinkgoHelper()
			click(tt, "Monkey D. Luffy OP01-024")
			v.cards.SetSimArt("OP01-024", info)
			tt.Frame()
		}

		It("puts a printing in the sim from the dot under it, and offers to undo", func() {
			luffy(cardspage.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
			click(tt, "Art · 3")
			for _, text := range []string{"Base", "Alt art", "Sim's own", "In sim"} {
				Expect(tt.HasText(text)).To(BeTrue(), text)
			}
			click(tt, "Use Alt art in the sim")
			Expect(fa.calls).To(ContainElement("use art OP01-024 OP01-024_p1"))
			click(tt, "Use Base in the sim")
			Expect(fa.calls).NotTo(ContainElement("use art OP01-024 OP01-024"))

			alt := sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024_p1"}
			v.cards.SimArtDone(&v.Env, v.Snap.Card("OP01-024"), alt, cardspage.SimArtInfo{Cur: alt}, sim.ArtChoice{Kind: sim.ArtOwn}, nil)
			tt.Frame()
			Expect(tt.HasText("The sim now shows Monkey D. Luffy's Alt art.")).To(BeTrue())
			click(tt, "Undo")
			Expect(fa.calls).To(ContainElement("use art OP01-024 "))
		})

		It("closes the message after a change without undoing it", func() {
			click(tt, "Monkey D. Luffy OP01-024")
			alt := sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024_p1"}
			v.cards.SimArtDone(&v.Env, v.Snap.Card("OP01-024"), alt, cardspage.SimArtInfo{Cur: alt}, sim.ArtChoice{Kind: sim.ArtOwn}, nil)
			tt.Frame()
			Expect(tt.HasText("The sim now shows Monkey D. Luffy's Alt art.")).To(BeTrue())
			click(tt, "Close")
			Expect(tt.HasText("The sim now shows Monkey D. Luffy's Alt art.")).To(BeFalse())
			Expect(fa.calls).NotTo(ContainElement("use art OP01-024 "))
		})

		It("shows the picture a click picks without changing the sim", func() {
			luffy(cardspage.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
			click(tt, "Art · 3")
			click(tt, "Show Alt art")
			click(tt, "Save image…")
			click(tt, "Show Sim's own")
			click(tt, "Save image…")
			Expect(calls("save art")).To(Equal([]string{"save art OP01-024_p1", "save art sim:OP01-024"}))
		})

		It("keeps a file of the user's as a choice and says what happens to it", func() {
			luffy(cardspage.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtFile}, OwnFile: `C:\sim\OP01-024_OVERRIDE.png`})
			artTab()
			Expect(tt.HasText("Your file")).To(BeTrue())
			Expect(tt.HasText("The sim shows a file you added yourself.")).To(BeTrue())
		})

		It("keeps printings out of the sim while art downloads are off", func() {
			v.setPrefs(page.PrefsFrom(map[string]string{page.KeyArtOff: "1"}))
			luffy(cardspage.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024"}})
			click(tt, "Art · 3")
			Expect(tt.HasText("Card art downloads are off in Settings, so only the sim's own art and your own file can be used.")).
				To(BeTrue())
			click(tt, "Use Alt art in the sim")
			Expect(calls("use art")).To(BeEmpty())
			click(tt, "Use Sim's own in the sim")
			Expect(calls("use art")).To(Equal([]string{"use art OP01-024 "}))
		})

		It("says why the dots are off without a sim", func() {
			luffy(cardspage.SimArtInfo{NoSim: true})
			artTab()
			Expect(tt.HasText("logpose can't find the sim, so it can't change the sim's art.")).To(BeTrue())
			click(tt, "Choose the sim's folder")
			Expect(fa.calls).To(ContainElement("choose sim"))
			Expect(fa.calls).NotTo(ContainElement(HavePrefix("use art")))
		})

		It("keeps the effect on the Card tab and names the sim's art above the tabs", func() {
			luffy(cardspage.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024_p1"}})
			Expect(tt.HasText("This Character cannot be K.O.'d")).To(BeTrue())
			Expect(tt.HasText("Sim's own")).To(BeFalse())
			click(tt, "In the sim: Alt art")
			Expect(tt.HasText("This Character cannot be K.O.'d")).To(BeFalse())
			Expect(tt.HasText("Sim's own")).To(BeTrue())
		})
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
		// lastGen is the grouping generation of the last LoadArts the page asked for.
		lastGen := func() int {
			GinkgoHelper()
			Expect(fa.artLoads).NotTo(BeEmpty())
			var code string
			var gen int
			_, err := fmt.Sscan(fa.artLoads[len(fa.artLoads)-1], &code, &gen)
			Expect(err).NotTo(HaveOccurred())
			return gen
		}
		open := func(cur sim.ArtChoice, ca arts.CardArts) {
			GinkgoHelper()
			v.cards.SetSimArt("OP14-002", cardspage.SimArtInfo{Cur: cur})
			v.cards.ResetArts()
			tt.Frame()
			v.cards.SetArts(lastGen(), "OP14-002", ca)
			tt.Frame()
			artTab()
		}

		It("shows once with a count, and lists each version with its product and its own dot", func() {
			open(sim.ArtChoice{Kind: sim.ArtOwn}, regional(0))
			Expect(tt.HasText("Regional")).To(BeTrue())
			Expect(tt.HasText("6")).To(BeTrue())
			Expect(tt.HasText("Offline Regional Finalist Card Set 25-26 Season 1")).To(BeFalse())

			click(tt, "Show Regional, 6 versions")
			for _, text := range []string{"Regional · 6 versions", "Offline finalist", "Offline Regional Finalist Card Set 25-26 Season 1"} {
				Expect(tt.HasText(text)).To(BeTrue(), text)
			}
			click(tt, "Use Offline finalist, Offline Regional Finalist Card Set 25-26 Season 1 in the sim")
			Expect(fa.calls).To(ContainElement("use art OP14-002 OP14-002_p3_bd"))
		})

		It("shows a version large when its row is clicked", func() {
			open(sim.ArtChoice{Kind: sim.ArtOwn}, regional(0))
			click(tt, "Show Regional, 6 versions")
			click(tt, "Save image…")
			click(tt, "Show Regional, 6 versions")
			click(tt, "Show Online champion, Online Regional Champion Card Set 25-26 Season 1")
			click(tt, "Save image…")
			Expect(calls("save art")).To(Equal([]string{"save art OP14-002_p1_bd", "save art OP14-002_p7_bd"}))
		})

		It("puts the version shown large in the sim from its art's dot, as the viewer does", func() {
			open(sim.ArtChoice{Kind: sim.ArtOwn}, regional(0))
			click(tt, "Show Regional, 6 versions")
			click(tt, "Show Online champion, Online Regional Champion Card Set 25-26 Season 1")
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			click(tt, "Use Regional in the sim")
			Expect(calls("use art")).To(Equal([]string{"use art OP14-002 OP14-002_p7_bd"}))
		})

		It("is in the sim when any version is, and names it above the tabs", func() {
			open(sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP14-002_p5_bd"}, regional(0))
			Expect(tt.HasText("In the sim: Regional")).To(BeTrue())
			Expect(tt.HasText("In sim")).To(BeTrue())
		})

		It("counts Bandai's copy of a printing as that printing", func() {
			open(sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP14-002_bd"}, regional(0))
			Expect(tt.HasText("In the sim: the base card")).To(BeTrue())
		})

		It("offers each version in the right-click menu", func() {
			open(sim.ArtChoice{Kind: sim.ArtOwn}, regional(0))
			Expect(tt.RightClick("Urouge OP14-002")).To(Succeed())
			tt.Frame()
			Expect(choose(tt, "Art in the sim", "Regional · 6 versions", "Online finalist")).NotTo(Succeed())
			tt.Key(0, ui.KeyEscape)
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			Expect(tt.RightClick("Urouge OP14-002")).To(Succeed())
			tt.Frame()
			Expect(choose(tt, "Art in the sim", "Regional · 6 versions", "Online Regional Finalist Card Set 25-26 Season 1")).
				To(Succeed())
			Expect(fa.calls).To(ContainElement("use art OP14-002 OP14-002_p6_bd"))
		})

		It("names a single art in the menu with its product, but the base card", func() {
			sp := arts.Art{Versions: []arts.Version{{
				Key: arts.Key{Source: arts.Limitless, Print: "OP14-002_p2"}, Name: "SP", Product: "Premium Booster",
			}}}
			ca := regional(0)
			ca.Arts = []arts.Art{ca.Arts[0], sp}
			open(sim.ArtChoice{Kind: sim.ArtOwn}, ca)
			Expect(tt.RightClick("Urouge OP14-002")).To(Succeed())
			Expect(choose(tt, "Art in the sim")).To(Succeed())
			Expect(tt.HasText("SP, Premium Booster")).To(BeTrue())
			Expect(tt.HasText("Base")).To(BeTrue())
			Expect(tt.HasText("Base, The Azure Sea's Seven")).To(BeFalse())
		})

		It("says how many printings are still being compared", func() {
			open(sim.ArtChoice{Kind: sim.ArtOwn}, regional(3))
			Expect(tt.HasText("Checking 3 more")).To(BeTrue())
		})

		It("asks for a card's arts once, again after a reset, and drops a step from before it", func() {
			v.cards.SetSimArt("OP14-002", cardspage.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
			tt.Frame()
			Expect(fa.artLoads).To(HaveLen(1))
			first := lastGen()

			v.cards.ResetArts()
			tt.Frame()
			Expect(fa.artLoads).To(HaveLen(2))
			second := lastGen()
			Expect(second).NotTo(Equal(first))

			v.cards.ResetArts()
			v.cards.SetArts(second, "OP14-002", regional(0))
			tt.Frame()
			Expect(fa.artLoads).To(HaveLen(3))
			Expect(tt.HasText("Art · 2")).To(BeTrue())
			v.cards.SetArts(lastGen(), "OP14-002", regional(0))
			tt.Frame()
			Expect(tt.HasText("Art · 3")).To(BeTrue())
		})
	})

	Describe("the art viewer", func() {
		luffyViewer := func() {
			GinkgoHelper()
			click(tt, "Monkey D. Luffy OP01-024")
			v.cards.SetSimArt("OP01-024", cardspage.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
			click(tt, "View full size")
		}

		It("lists the card's pictures and names the one in the sim", func() {
			luffyViewer()
			for _, text := range []string{"Monkey D. Luffy", "Base", "Alt art", "Sim's own", "In the sim"} {
				Expect(tt.HasText(text)).To(BeTrue(), text)
			}
		})

		It("walks the pictures with the up and down keys, around the ends", func() {
			luffyViewer()
			tt.Key(0, ui.KeyDown)
			click(tt, "Use this art in the sim")
			Expect(calls("use art")).To(Equal([]string{"use art OP01-024 OP01-024_p1"}))
			alt := sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024_p1"}
			v.cards.SimArtDone(&v.Env, v.Snap.Card("OP01-024"), alt, cardspage.SimArtInfo{Cur: alt}, sim.ArtChoice{Kind: sim.ArtOwn}, nil)
			tt.Key(0, ui.KeyUp)
			tt.Key(0, ui.KeyUp)
			click(tt, "Use this art in the sim")
			Expect(calls("use art")).To(Equal([]string{"use art OP01-024 OP01-024_p1", "use art OP01-024 "}))
		})

		It("puts the picture a click picks in the sim", func() {
			luffyViewer()
			click(tt, "View Alt art")
			Expect(calls("use art")).To(BeEmpty())
			click(tt, "Use this art in the sim")
			Expect(calls("use art")).To(Equal([]string{"use art OP01-024 OP01-024_p1"}))
		})

		It("closes on Escape and on its close button", func() {
			luffyViewer()
			Expect(labelled(tt, "Close the viewer")).To(BeTrue())
			tt.Key(0, ui.KeyEscape)
			Expect(labelled(tt, "Close the viewer")).To(BeFalse())
			click(tt, "View the art full size")
			Expect(labelled(tt, "Close the viewer")).To(BeTrue())
			click(tt, "Close the viewer")
			Expect(labelled(tt, "Close the viewer")).To(BeFalse())
		})

		It("opens on the card whose menu asked for it", func() {
			click(tt, "Monkey D. Luffy OP01-024")
			Expect(tt.RightClick("Urouge OP14-002")).To(Succeed())
			Expect(choose(tt, "View art")).To(Succeed())
			Expect(labelled(tt, "Close the viewer")).To(BeTrue())
			Expect(tt.Texts()).To(ContainElement("OP14-002 · Red Character"))
			Expect(tt.Texts()).NotTo(ContainElement("OP01-024 · Red Character"))
		})
	})
})

var _ = Describe("Card sort", func() {
	var (
		tt *ui.Tester
		fa *fakeActions
		v  *view
	)
	pool := testCards()
	BeforeEach(func() { v, tt, fa = cardsTester() })
	order := func(saved string) []string {
		GinkgoHelper()
		v.setPrefs(page.PrefsFrom(map[string]string{page.KeyCardSort: saved}))
		return shownCards(tt, pool)
	}

	It("puts leaders last by cost and events last by power, either way", func() {
		Expect(order("cost")).To(Equal([]string{"OP14-018", "OP01-024", "OP14-002", "OP14-020"}))
		Expect(order("cost reversed")).To(Equal([]string{"OP14-002", "OP01-024", "OP14-018", "OP14-020"}))
		Expect(order("power")).To(Equal([]string{"OP14-020", "OP01-024", "OP14-002", "OP14-018"}))
		Expect(order("power reversed")).To(Equal([]string{"OP14-002", "OP01-024", "OP14-020", "OP14-018"}))
	})

	It("sorts by name, counter, rarity, color and set, with ties in set order", func() {
		Expect(order("name")).To(Equal([]string{"OP14-020", "OP01-024", "OP14-018", "OP14-002"}))
		Expect(order("counter")).To(Equal([]string{"OP14-002", "OP01-024", "OP14-018", "OP14-020"}))
		Expect(order("rarity")).To(Equal([]string{"OP01-024", "OP14-002", "OP14-018", "OP14-020"}))
		Expect(order("rarity reversed")).To(Equal([]string{"OP14-018", "OP14-002", "OP01-024", "OP14-020"}))
		Expect(order("color")).To(Equal([]string{"OP14-002", "OP01-024", "OP14-018", "OP14-020"}))
		Expect(order("color reversed")).To(Equal([]string{"OP14-018", "OP14-020", "OP14-002", "OP01-024"}))
		Expect(order("set reversed")).To(Equal([]string{"OP01-024", "OP14-020", "OP14-018", "OP14-002"}))
	})

	It("sorts a character at 0 power as 0, ahead of events", func() {
		small := map[string]cards.Card{
			"OP14-003": {Code: "OP14-003", Name: "Event", Type: "Event", Detailed: true},
			"OP14-013": {Code: "OP14-013", Name: "Zero", Type: "Character", Detailed: true},
			"OP14-005": {Code: "OP14-005", Name: "Thousand", Type: "Character", Power: 1000, Detailed: true},
		}
		v, tt, _ = cardsWith(small)
		v.setPrefs(page.PrefsFrom(map[string]string{page.KeyCardSort: "power reversed"}))
		Expect(shownCards(tt, small)).To(Equal([]string{"OP14-013", "OP14-005", "OP14-003"}))
	})

	It("puts multicolor cards after the single color they start with", func() {
		small := map[string]cards.Card{
			"OP01-001": {Code: "OP01-001", Name: "Green card", Colors: []cards.Color{cards.Green}, Detailed: true},
			"OP01-002": {Code: "OP01-002", Name: "Red and green card", Colors: []cards.Color{cards.Red, cards.Green}, Detailed: true},
			"OP01-003": {Code: "OP01-003", Name: "Red card", Colors: []cards.Color{cards.Red}, Detailed: true},
		}
		v, tt, _ = cardsWith(small)
		v.setPrefs(page.PrefsFrom(map[string]string{page.KeyCardSort: "color"}))
		Expect(shownCards(tt, small)).To(Equal([]string{"OP01-003", "OP01-002", "OP01-001"}))
	})

	It("reads the saved sort and list switch, and names the sort in words", func() {
		Expect(tt.HasText("Set · by number")).To(BeTrue())
		for saved, label := range map[string]string{
			"set reversed": "Set · reversed", "cost": "Cost · lowest first", "counter": "Counter · highest first",
			"cost reversed": "Cost · highest first", "nonsense": "Set · by number",
		} {
			v.setPrefs(page.PrefsFrom(map[string]string{page.KeyCardSort: saved}))
			tt.Frame()
			Expect(tt.HasText(label)).To(BeTrue(), saved)
		}
		Expect(tt.HasText("Effect")).To(BeFalse())
		v.setPrefs(page.PrefsFrom(map[string]string{page.KeyCardList: "1"}))
		tt.Frame()
		Expect(tt.HasText("Effect")).To(BeTrue())
	})

	It("sorts from the pill's menu and saves the sort", func() {
		click(tt, "Sort: Set · by number")
		click(tt, "Rarity")
		Expect(tt.HasText("Rarity · rarest first")).To(BeTrue())
		Expect(shownCards(tt, pool)).To(Equal([]string{"OP01-024", "OP14-002", "OP14-018", "OP14-020"}))
		click(tt, "Sort: Rarity · rarest first")
		click(tt, "Commonest first")
		Expect(tt.HasText("Rarity · commonest first")).To(BeTrue())
		Expect(fa.calls).To(ContainElements("card_sort=rarity", "card_sort=rarity reversed"))
	})

	It("sorts the list on a header click and turns it around on the next", func() {
		click(tt, "List")
		Expect(tt.HasText("Effect")).To(BeTrue())
		click(tt, "Sort by Power")
		Expect(shownCards(tt, pool)).To(Equal([]string{"OP14-020", "OP01-024", "OP14-002", "OP14-018"}))
		click(tt, "Sort by Power")
		Expect(shownCards(tt, pool)).To(Equal([]string{"OP14-002", "OP01-024", "OP14-020", "OP14-018"}))
		click(tt, "Sort by Name")
		Expect(shownCards(tt, pool)).To(Equal([]string{"OP14-020", "OP01-024", "OP14-018", "OP14-002"}))
		Expect(fa.calls).To(ContainElements("card_list=1", "card_sort=power", "card_sort=power reversed", "card_sort=name"))
	})
})

var _ = Describe("effect text", func() {
	It("draws keywords as on the card and traits in gold", func() {
		spans := cardspage.EffectSpans("[On Play] Give {Supernovas} +1000.\n[Trigger] Draw 1.", kit.Graphite())
		Expect(spans[0].Text).To(Equal("On Play"))
		Expect(spans[0].Background).To(Equal(cardspage.KwBlue))
		Expect(spans[2].Text).To(Equal("{Supernovas}"))
		Expect(spans[2].Color).To(Equal(kit.CardGold))
		Expect(spans[5].Text).To(Equal("Trigger"))
		Expect(spans[5].Background).To(Equal(cardspage.KwYellow))
	})
})
