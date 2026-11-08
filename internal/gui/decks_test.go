package gui

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/deck"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

// deckSnapshot is testSnapshot's games with two lists. The even games, all wins, run W-1 and
// search with Garp and Otama. The odd ones, all losses, search with Garp, Kid & Killer and Luffy.
func deckSnapshot(builds ...store.Build) *snapshot {
	s := testSnapshot(10)
	for code, name := range map[string]string{"GARP": "Garp", "OTA": "Otama", "KID": "Kid & Killer", "LUF": "Luffy"} {
		s.Cards.Name(code, name)
	}
	facts := make(map[string]deck.Facts)
	for i := range 10 {
		list := map[string]int{"GARP": 4, "OTA": 4, "W-1": 4}
		searchers := []string{"GARP", "OTA"}
		if i%2 == 1 {
			list = map[string]int{"GARP": 4, "KID": 4, "LUF": 4}
			searchers = []string{"GARP", "KID", "LUF"}
		}
		var searches []deck.Search
		for _, by := range searchers {
			searches = append(searches, deck.Search{Source: by, Taken: "X-1"})
		}
		facts[fmt.Sprint(i)] = deck.Facts{Player: me, List: list, Searches: searches}
	}
	s.Decks = make(map[string]stats.DeckGame)
	for _, g := range stats.DeckGames(s.Matches, facts) {
		s.Decks[g.Fingerprint] = g
	}
	s.Builds = builds
	return s
}

var _ = Describe("Decks page", func() {
	It("shows the leader's builds with their record, without suggestions", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		for _, text := range []string{"Wano/Miracle", "5 games · 5–0 · 100%", "5 games in no build"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("suggested")).To(BeFalse())
	})

	It("offers the suggestions in one chip when the leader has no builds", func() {
		_, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.HasText("No builds yet · 10 games")).To(BeTrue())
		Expect(tt.Click("2 suggested")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Suggested builds · 2")).To(BeTrue())
	})

	It("folds suggestions of fewer than 3 games away", func() {
		s := deckSnapshot()
		s.Cards.Name("SUN", "Thousand Sunny")
		g := s.Decks["0"]
		g.Facts.List = map[string]int{"GARP": 4, "SUN": 1}
		g.Facts.Searches = []deck.Search{{Source: "GARP", Taken: "X-1"}, {Source: "SUN", Taken: "X-1"}}
		s.Decks["0"] = g
		_, tt := tester(s)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Show 1 small group")).To(BeTrue())
		Expect(tt.HasText("Thousand Sunny")).To(BeFalse())
	})

	It("names suggestions by the searchers that tell them apart", func() {
		_, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Kid & Killer + Luffy")).To(BeTrue())
		Expect(tt.HasText("Garp + Otama")).To(BeFalse())
	})

	It("names a suggestion by what its searchers look for", func() {
		s := deckSnapshot()
		lib := &cards.Library{Cards: map[string]cards.Card{
			"OTA": {Code: "OTA", Name: "Otama", Effect: "[On Play] Look at 5 cards from the top of your deck; reveal up to 1 green {Land of Wano} type card other than [Otama] and add it to your hand."},
			"KID": {Code: "KID", Name: "Kid & Killer", Effect: "[On Play] Look at 5 cards from the top of your deck; reveal up to 1 {Supernovas} type card and add it to your hand."},
			"LUF": {Code: "LUF", Name: "Luffy", Effect: "[On Play] Look at 5 cards from the top of your deck; reveal up to 1 {Supernovas} type card other than [Monkey.D.Luffy] and add it to your hand."},
		}}
		lib.Apply(s.Cards)
		_, tt := tester(s)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()
		_, wano := tt.Find("Land of Wano")
		_, novas := tt.Find("Supernovas")
		Expect(wano).To(BeTrue())
		Expect(novas).To(BeTrue())
	})

	It("suggests builds from every game, whatever the filters", func() {
		v, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		v.filter.gameType = int(stats.RankedGames)
		tt.Frame()
		Expect(tt.HasText("2 suggested")).To(BeTrue())
	})

	It("opens on the Results tab with the leader's own stats", func() {
		_, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		for _, text := range []string{"Results", "List", "Cards", "Changes", "Turn order"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("keeps Cards and Changes shut", func() {
		v, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("Cards")).To(Succeed())
		tt.Frame()
		Expect(v.decks.tab).NotTo(Equal(tabCards))
	})

	It("makes a suggestion a build with the name given, its searchers as the core", func() {
		v, tt := tester(deckSnapshot())
		f := &fakeActions{}
		v.act = f
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("2 suggested")).To(Succeed())
		tt.Frame()
		Expect(tt.Click("Make this a build")).To(Succeed())
		tt.Frame()
		v.decks.name = "Supernovas"
		Expect(tt.Click("Save")).To(Succeed())
		Expect(f.builds).To(Equal([]store.Build{{Leader: "OP13-001", Name: "Supernovas", Core: []string{"GARP", "KID", "LUF"}}}))
	})

	It("dismisses a suggestion", func() {
		v, tt := tester(deckSnapshot())
		f := &fakeActions{}
		v.act = f
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()
		Expect(tt.Click("Not a build")).To(Succeed())
		Expect(f.dismissals).To(HaveLen(1))
		Expect(f.dismissals[0].Leader).To(Equal("OP13-001"))
	})

	It("opens List on the first build", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Latest list")).To(BeTrue())
	})

	It("shows a build's latest list with its stats", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("Wano/Miracle")).To(Succeed())
		tt.Frame()
		for _, text := range []string{"Core", "Latest list", "Cost curve", "Counters", "Types", "Traits", "×4"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})
})
