package stats_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/deck"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

const luffy = "OP13-001"

// deckGame is a game of Luffy, played i hours after the first, with the given list, won or lost.
// The player went second, so their turns are the even ones and their DON!! goes 2, 4, 6 and on.
func deckGame(i int, won bool, list map[string]int) stats.DeckGame {
	res := stats.Loss
	if won {
		res = stats.Win
	}
	don := []int{0}
	for t := 1; t <= 12; t++ {
		don = append(don, min(t, 10))
	}
	return stats.DeckGame{
		Match: stats.Match{
			Game:           store.Game{Fingerprint: fmt.Sprint(i), PlayedAt: now.Add(time.Duration(i) * time.Hour)},
			Me:             store.GamePlayer{ID: me, Leader: luffy},
			Result:         res,
			TurnOrderKnown: true,
		},
		Facts: deck.Facts{Player: me, List: list, Don: don},
	}
}

// fifty fills a list up to 50 cards with copies of filler cards.
func fifty(list map[string]int) map[string]int {
	out := make(map[string]int)
	n := 0
	for c, k := range list {
		out[c] = k
		n += k
	}
	for i := 0; n < stats.DeckSize; i++ {
		k := min(4, stats.DeckSize-n)
		out[fmt.Sprintf("F-%03d", i)] = k
		n += k
	}
	return out
}

func library(cs ...cards.Card) func(string) cards.Card {
	byCode := make(map[string]cards.Card)
	for _, c := range cs {
		byCode[c.Code] = c
	}
	return func(code string) cards.Card { return byCode[code] }
}

func record(w, l int) stats.Record { return stats.Record{Wins: w, Losses: l} }

var _ = Describe("Deck games", func() {
	It("completes a list the log named only in part from the nearest full list that holds it", func() {
		old := fifty(map[string]int{"A-1": 4, "B-1": 4})
		newer := fifty(map[string]int{"A-1": 4, "C-1": 4})
		part := map[string]int{"A-1": 4, "C-1": 3}
		ms := []stats.Match{deckGame(0, true, nil).Match, deckGame(1, true, nil).Match, deckGame(2, true, nil).Match, deckGame(3, true, nil).Match}
		facts := map[string]deck.Facts{
			"0": {Player: me, List: old},
			"1": {Player: me, List: part},
			"2": {Player: me, List: newer},
		}
		gs := stats.DeckGames(ms, facts)
		Expect(gs).To(HaveLen(3))
		Expect(gs[1].Facts.List).To(Equal(newer))
	})
})

var _ = Describe("Assign", func() {
	wano := store.Build{ID: 1, Leader: luffy, Core: []string{"ST30-004"}}
	miracle := store.Build{ID: 2, Leader: luffy, Core: []string{"OP01-055", "ST30-004"}}
	nova := store.Build{ID: 3, Leader: luffy, Core: []string{"ST24-002"}}

	It("puts a game in the build with the bigger core when several fit", func() {
		g := deckGame(0, true, map[string]int{"ST30-004": 4, "OP01-055": 3})
		a := stats.Assign([]stats.DeckGame{g}, []store.Build{wano, miracle})
		Expect(a.Builds[2]).To(HaveLen(1))
		Expect(a.Builds[1]).To(BeEmpty())
	})

	It("reports a game that fits two cores of the same size", func() {
		g := deckGame(0, true, map[string]int{"ST30-004": 4, "ST24-002": 4})
		a := stats.Assign([]stats.DeckGame{g}, []store.Build{wano, nova})
		Expect(a.Builds).To(BeEmpty())
		Expect(a.Conflicts).To(HaveLen(1))
	})

	It("leaves out games that miss a core card or are another leader's", func() {
		missing := deckGame(0, true, map[string]int{"OP01-055": 3})
		other := deckGame(1, true, map[string]int{"ST30-004": 4})
		other.Me.Leader = "EB05-010"
		a := stats.Assign([]stats.DeckGame{missing, other}, []store.Build{miracle, wano})
		Expect(a.Builds).To(BeEmpty())
		Expect(a.Conflicts).To(BeEmpty())
	})
})

var _ = Describe("Suggest", func() {
	It("calls a card in 95% of the games a staple", func() {
		var gs []stats.DeckGame
		for i := range 20 {
			list := map[string]int{"S-1": 4, "N-1": 4}
			if i == 0 {
				delete(list, "S-1")
			}
			if i < 2 {
				delete(list, "N-1")
			}
			gs = append(gs, deckGame(i, true, list))
		}
		Expect(stats.Staples(gs)).To(Equal([]string{"S-1"}))
	})

	It("counts a card as a searcher when it searched in a quarter of the games it was in", func() {
		var gs []stats.DeckGame
		for i := range 8 {
			g := deckGame(i, true, map[string]int{"GARP": 4, "HIYO": 4})
			g.Facts.Searches = []deck.Search{{Source: "GARP", Taken: "X-1"}}
			if i == 0 {
				g.Facts.Searches = append(g.Facts.Searches, deck.Search{Source: "HIYO", Taken: "X-1"})
			}
			gs = append(gs, g)
		}
		Expect(stats.Searchers(gs)).To(Equal([]string{"GARP"}))
	})

	It("groups games by the searchers their lists run, the biggest group first", func() {
		searched := func(i int, list map[string]int, by ...string) stats.DeckGame {
			g := deckGame(i, true, list)
			for _, s := range by {
				g.Facts.Searches = append(g.Facts.Searches, deck.Search{Source: s, Taken: "X-1"})
			}
			return g
		}
		gs := []stats.DeckGame{
			searched(0, map[string]int{"GARP": 4, "OTAMA": 4}, "GARP", "OTAMA"),
			searched(1, map[string]int{"GARP": 4, "KID": 4}, "GARP", "KID"),
			searched(2, map[string]int{"GARP": 4, "OTAMA": 4, "W-1": 2}, "OTAMA"),
			searched(3, map[string]int{"PLAIN": 4}),
		}
		sugs := stats.Suggest(gs, nil)
		Expect(sugs).To(HaveLen(2))
		Expect(sugs[0].Core).To(Equal([]string{"GARP", "OTAMA"}))
		Expect(sugs[0].Games).To(HaveLen(2))
		Expect(sugs[1].Core).To(Equal([]string{"GARP", "KID"}))
	})

	It("suggests one build when every list runs the same searchers", func() {
		var gs []stats.DeckGame
		for i := range 3 {
			g := deckGame(i, true, map[string]int{"ATLAS": 4, "LILITH": 4, fmt.Sprintf("T-%d", i): 1})
			g.Facts.Searches = []deck.Search{{Source: "ATLAS", Taken: "X-1"}, {Source: "LILITH", Taken: "X-1"}}
			gs = append(gs, g)
		}
		sugs := stats.Suggest(gs, nil)
		Expect(sugs).To(HaveLen(1))
		Expect(sugs[0].Games).To(HaveLen(3))
	})

	It("leaves out a suggestion the user dismissed", func() {
		g := deckGame(0, true, map[string]int{"KID": 4})
		g.Facts.Searches = []deck.Search{{Source: "KID", Taken: "X-1"}}
		Expect(stats.Suggest([]stats.DeckGame{g}, [][]string{{"KID"}})).To(BeEmpty())
	})
})

var _ = Describe("Cards", func() {
	lib := library(
		cards.Card{Code: "BON", Type: "Character", Cost: 5},
		cards.Card{Code: "SAM", Type: "Event", Cost: 1},
		cards.Card{Code: "SHA", Type: "Character", Cost: 10},
	)

	It("counts the games a card was in the kept hand", func() {
		g := deckGame(0, true, map[string]int{"BON": 4})
		g.Me.HandBefore = []string{"BON"}
		lost := deckGame(1, false, map[string]int{"BON": 4})
		cs := stats.Cards([]stats.DeckGame{g, lost}, lib)
		Expect(cs[0].Card).To(Equal("BON"))
		Expect(cs[0].Games).To(Equal(2))
		Expect(cs[0].Opening).To(Equal(record(1, 0)))
	})

	It("tells a card in hand by the turn its cost could be paid from one drawn after", func() {
		onTime := deckGame(0, true, map[string]int{"BON": 4})
		onTime.Facts.Events = []deck.Event{{Card: "BON", Turn: 6, Kind: deck.Drew}}
		late := deckGame(1, false, map[string]int{"BON": 4})
		late.Facts.Events = []deck.Event{{Card: "BON", Turn: 8, Kind: deck.Drew}}
		cs := stats.Cards([]stats.DeckGame{onTime, late}, lib)
		Expect(cs[0].OnCurve).To(Equal(record(1, 0)))
		Expect(cs[0].OffCurve).To(Equal(record(0, 1)))
	})

	It("keeps apart the games that ended before the card could be paid for", func() {
		short := deckGame(0, true, map[string]int{"SHA": 4})
		short.Facts.Don = short.Facts.Don[:8]
		cs := stats.Cards([]stats.DeckGame{short}, lib)
		Expect(cs[0].EndedFirst).To(Equal(1))
		Expect(cs[0].OnCurve.Games() + cs[0].OffCurve.Games()).To(BeZero())
	})

	It("splits plays into on curve, later, by an effect and never", func() {
		list := map[string]int{"BON": 4}
		onCurve := deckGame(0, true, list)
		onCurve.Facts.Events = []deck.Event{{Card: "BON", Turn: 6, Kind: deck.PaidPlay}}
		later := deckGame(1, false, list)
		later.Facts.Events = []deck.Event{{Card: "BON", Turn: 10, Kind: deck.PaidPlay}, {Card: "BON", Turn: 4, Kind: deck.EffectPlay}}
		effect := deckGame(2, true, list)
		effect.Facts.Events = []deck.Event{{Card: "BON", Turn: 4, Kind: deck.EffectPlay}}
		never := deckGame(3, false, list)
		never.Facts.Events = []deck.Event{{Card: "BON", Turn: 5, Kind: deck.Counter}}
		cs := stats.Cards([]stats.DeckGame{onCurve, later, effect, never}, lib)
		Expect(cs[0].PlayedOnCurve).To(Equal(record(1, 0)))
		Expect(cs[0].PlayedLater).To(Equal(record(0, 1)))
		Expect(cs[0].ByEffect).To(Equal(record(1, 0)))
		Expect(cs[0].NeverPlayed).To(Equal(record(0, 1)))
	})

	It("counts an event that left the hand next to its own lines as played", func() {
		g := deckGame(0, true, map[string]int{"SAM": 3})
		g.Facts.Events = []deck.Event{{Card: "SAM", Turn: 2, Kind: deck.OwnLines}}
		cs := stats.Cards([]stats.DeckGame{g}, lib)
		Expect(cs[0].PlayedOnCurve).To(Equal(record(1, 0)))
	})

	It("counts a character trashed next to its own lines as not played", func() {
		g := deckGame(0, true, map[string]int{"BON": 4})
		g.Facts.Events = []deck.Event{{Card: "BON", Turn: 6, Kind: deck.OwnLines}}
		cs := stats.Cards([]stats.DeckGame{g}, lib)
		Expect(cs[0].NeverPlayed).To(Equal(record(1, 0)))
	})

	It("counts a search as showing a card only for a searcher that has taken it", func() {
		list := map[string]int{"GARP": 4, "BON": 4, "SAM": 3}
		a := deckGame(0, true, list)
		a.Facts.Searches = []deck.Search{{Source: "GARP", Taken: "BON", Passed: []string{"SAM", "SAM"}}}
		b := deckGame(1, true, list)
		b.Facts.Searches = []deck.Search{{Source: "GARP", Passed: []string{"BON", "SAM"}}}
		cs := stats.Cards([]stats.DeckGame{a, b}, lib)
		byCard := make(map[string]stats.CardStats)
		for _, c := range cs {
			byCard[c.Card] = c
		}
		Expect(byCard["BON"].Shown).To(Equal(2))
		Expect(byCard["BON"].Taken).To(Equal(1))
		Expect(byCard["SAM"].Shown).To(BeZero())
	})

	It("lists the latest list's cards first, characters by cost, events last", func() {
		old := deckGame(0, true, map[string]int{"OLD": 4})
		latest := deckGame(1, true, map[string]int{"SAM": 3, "SHA": 4, "BON": 4})
		cs := stats.Cards([]stats.DeckGame{old, latest}, lib)
		var order []string
		for _, c := range cs {
			order = append(order, c.Card)
		}
		Expect(order).To(Equal([]string{"BON", "SHA", "SAM", "OLD"}))
	})
})

var _ = Describe("List", func() {
	It("counts the curve, the characters' counters, types, colors and traits in copies", func() {
		lib := library(
			cards.Card{Code: "OTA", Type: "Character", Cost: 1, Counter: 2000, Colors: []cards.Color{cards.Green}, Traits: []string{"Land of Wano"}},
			cards.Card{Code: "SHA", Type: "Character", Cost: 10, Colors: []cards.Color{cards.Green}, Traits: []string{"FILM", "Land of Wano"}},
			cards.Card{Code: "SAM", Type: "Event", Cost: 1, Colors: []cards.Color{cards.Green}, Traits: []string{"Land of Wano"}},
		)
		st := stats.List(map[string]int{"OTA": 4, "SHA": 4, "SAM": 3}, lib)
		Expect(st.Curve[1]).To(Equal(7))
		Expect(st.Curve[10]).To(Equal(4))
		Expect(st.Counters).To(Equal(map[int]int{2000: 4, 0: 4}))
		Expect(st.Types).To(Equal(map[string]int{"Character": 8, "Event": 3}))
		Expect(st.Colors[cards.Green]).To(Equal(11))
		Expect(st.Traits).To(Equal([]stats.TraitCount{{Trait: "Land of Wano", Copies: 11}, {Trait: "FILM", Copies: 4}}))
	})
})

var _ = Describe("Changes", func() {
	It("lists each switch with the copies swapped and marks a return to an earlier list", func() {
		a := map[string]int{"BRO": 2, "JIN": 2}
		b := map[string]int{"JIN": 4}
		gs := []stats.DeckGame{deckGame(0, true, a), deckGame(1, true, a), deckGame(2, false, b), deckGame(3, true, a)}
		ch := stats.Changes(gs)
		Expect(ch).To(HaveLen(3))
		Expect(ch[0].Swaps).To(BeEmpty())
		Expect(ch[0].Games).To(HaveLen(2))
		Expect(ch[1].Swaps).To(Equal(map[string]int{"BRO": -2, "JIN": 2}))
		Expect(ch[1].Back.IsZero()).To(BeTrue())
		Expect(ch[2].Back).To(Equal(gs[1].PlayedAt))
	})
})
