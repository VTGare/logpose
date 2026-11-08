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
	It("shows the leader's builds with their record and suggests the rest", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		for _, text := range []string{"Wano/Miracle", "5 games · 5–0 · 100%", "Garp + Kid & Killer + Luffy?", "Suggested · 5 games", "5 games in no build"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("counts suggestions of fewer than 3 games instead of showing each", func() {
		s := deckSnapshot()
		s.Cards.Name("SUN", "Thousand Sunny")
		g := s.Decks["0"]
		g.Facts.List = map[string]int{"GARP": 4, "SUN": 1}
		g.Facts.Searches = []deck.Search{{Source: "GARP", Taken: "X-1"}, {Source: "SUN", Taken: "X-1"}}
		s.Decks["0"] = g
		_, tt := tester(s)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.HasText("+1 small group")).To(BeTrue())
		Expect(tt.HasText("Thousand Sunny?")).To(BeFalse())
	})

	It("names suggestions by the searchers that tell them apart", func() {
		_, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.HasText("Otama?")).To(BeTrue())
		Expect(tt.HasText("Kid & Killer + Luffy?")).To(BeTrue())
		Expect(tt.HasText("Garp + Otama?")).To(BeFalse())
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
		Expect(tt.HasText("Land of Wano?")).To(BeTrue())
		Expect(tt.HasText("Supernovas?")).To(BeTrue())
	})

	It("suggests builds from every game, whatever the filters", func() {
		v, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		v.filter.gameType = int(stats.RankedGames)
		tt.Frame()
		Expect(tt.HasText("Suggested · 5 games")).To(BeTrue())
	})

	It("suggests builds when the leader has none yet", func() {
		_, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.HasText("No builds yet · 10 games")).To(BeTrue())
		Expect(tt.HasText("Suggested · 5 games")).To(BeTrue())
	})

	It("opens on the Results tab with the leader's own stats", func() {
		_, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		for _, text := range []string{"Results", "List", "Cards", "Changes", "Turn order"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Turn order")).To(BeTrue())
	})
})
