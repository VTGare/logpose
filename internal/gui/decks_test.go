package gui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

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
	It("shows the leader's builds with their record, and the suggestions only as a link", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		for _, text := range []string{"Wano/Miracle", "5 games · 5–0 · 100%", "5 games in no build"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("Builds to make")).To(BeFalse())
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
		for _, text := range []string{"Results", "List", "Cards", "Turn order"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("keeps Cards shut without a build", func() {
		v, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		v.decks.tab = tabCards
		tt.Frame()
		Expect(v.decks.tab).To(Equal(tabResults))
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
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()
		for _, text := range []string{"Latest list", "Cost curve", "Counters", "Types", "Traits", "×4"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("shows each card of the build with its numbers", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}
		v, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		v.decks.tab = tabCards
		tt.Frame()
		Expect(v.decks.tab).To(Equal(tabCards))
		for _, text := range []string{"In opening hand", "In hand by its turn", "Not by its turn", "same as opening hand", "Played", "Searcher took it", "Otama", "×4", "The tick is the build's 100%", "the difference likely isn't luck"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("names the turn a card can first be paid for", func() {
		Expect(curveText(10)).To(Equal("the turn with 10 DON!!, your 6th turn going first and 5th going second."))
		Expect(curveText(6)).To(Equal("the turn with 6 DON!!, your 4th turn going first and 3rd going second."))
		Expect(curveText(5)).To(Equal("the turn with 5 DON!!, your 3rd turn either way."))
	})

	It("shows a build's lists in a rail, the latest first, and any earlier one with its changes", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}
		s := deckSnapshot(wano)
		s.Cards.Name("TEC", "Tech Card")
		g := s.Decks["4"]
		g.Facts.List = map[string]int{"GARP": 4, "OTA": 4, "W-1": 2, "TEC": 2}
		s.Decks["4"] = g
		v, tt := tester(s)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()
		for _, text := range []string{"Latest list", "Latest", "First list", "Show changes"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("Core")).To(BeFalse())
		v.decks.version = g.PlayedAt
		tt.Frame()
		Expect(tt.HasText("List from")).To(BeTrue())
		named := func() int {
			n := 0
			for _, text := range tt.Texts() {
				if text == "Tech Card" {
					n++
				}
			}
			return n
		}
		before := named()
		Expect(v.prefs.deckChanges).To(BeFalse())
		v.prefs.deckChanges = true
		tt.Frame()
		Expect(named()).To(Equal(before+1), "the rail names the card added")
		Expect(tt.HasText("×2")).To(BeTrue())
	})

	It("counts every game for Overall, one build's for a build, and the rest for No build", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		v, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		d := v.leaderStats.decks
		count := func(sc deckScope) (games, matches int) {
			return len(d.scopedGames(sc)), len(d.scopedMatches(sc, v.leaderStats.games))
		}
		games, matches := count(deckScope{})
		Expect([]int{games, matches}).To(Equal([]int{10, 10}))
		games, matches = count(buildScope(1))
		Expect([]int{games, matches}).To(Equal([]int{5, 5}))
		for _, g := range d.scopedGames(buildScope(1)) {
			Expect(g.Result).To(Equal(stats.Win))
		}
		games, matches = count(deckScope{kind: scopeNoBuild})
		Expect([]int{games, matches}).To(Equal([]int{5, 5}))
		Expect(d.unbuilt).To(Equal(5))
	})

	It("counts a game that fits two builds only for Overall", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano", Core: []string{"W-1"}}
		otama := store.Build{ID: 2, Leader: "OP13-001", Name: "Otama", Core: []string{"OTA"}}
		v, tt := tester(deckSnapshot(wano, otama))
		Expect(tt.Click("Decks")).To(Succeed())
		d := v.leaderStats.decks
		Expect(d.scopedGames(deckScope{})).To(HaveLen(10))
		Expect(d.scopedGames(buildScope(1))).To(BeEmpty())
		Expect(d.scopedGames(buildScope(2))).To(BeEmpty())
		Expect(d.scopedGames(deckScope{kind: scopeNoBuild})).To(HaveLen(5))
	})

	It("counts a game without deck facts in No build", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		s := deckSnapshot(wano)
		delete(s.Decks, "1")
		v, tt := tester(s)
		Expect(tt.Click("Decks")).To(Succeed())
		d := v.leaderStats.decks
		Expect(d.scopedGames(deckScope{kind: scopeNoBuild})).To(HaveLen(4))
		Expect(d.scopedMatches(deckScope{kind: scopeNoBuild}, v.leaderStats.games)).To(HaveLen(5))
	})

	It("keeps the tab when another build is chosen", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}
		nova := store.Build{ID: 2, Leader: "OP13-001", Name: "Supernovas", Core: []string{"KID"}}
		v, tt := tester(deckSnapshot(wano, nova))
		Expect(tt.Click("Decks")).To(Succeed())
		v.decks.tab = tabCards
		tt.Frame()
		Expect(tt.Click("Supernovas")).To(Succeed())
		tt.Frame()
		Expect(v.decks.scope).To(Equal(buildScope(2)))
		Expect(v.decks.tab).To(Equal(tabCards))
	})

	It("makes a new build from the cards clicked and the name typed", func() {
		v, tt := tester(deckSnapshot())
		f := &fakeActions{}
		v.act = f
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("New build")).To(Succeed())
		tt.Frame()
		Expect(v.decks.edit).NotTo(BeNil())
		Expect(tt.HasText("New Monkey D. Luffy build")).To(BeTrue())
		v.decks.edit.toggle("OTA")
		v.decks.edit.name = "Wano"
		tt.Frame()
		Expect(tt.HasText("5 games · 5–0")).To(BeTrue())
		Expect(tt.HasText("Results")).To(BeFalse())
		Expect(tt.Click("Save")).To(Succeed())
		Expect(f.builds).To(Equal([]store.Build{{Leader: "OP13-001", Name: "Wano", Core: []string{"OTA"}}}))
		Expect(v.decks.edit).To(BeNil())
	})

	It("won't save a build without a name or a core card", func() {
		v, tt := tester(deckSnapshot())
		f := &fakeActions{}
		v.act = f
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("New build")).To(Succeed())
		tt.Frame()
		v.decks.edit.name = "Wano"
		tt.Frame()
		Expect(tt.Click("Save")).To(Succeed())
		Expect(f.builds).To(BeEmpty())
	})

	It("edits, renames and deletes a build from its menu, with Undo after a delete", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}
		v, tt := tester(deckSnapshot(wano))
		f := &fakeActions{}
		v.act = f
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.RightClick("Wano/Miracle")).To(Succeed())
		Expect(tt.Click("Edit core")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Edit Wano/Miracle")).To(BeTrue())
		Expect(v.decks.edit.core).To(Equal([]string{"OTA"}))
		Expect(tt.HasText("5 games · 5–0")).To(BeTrue())
		tt.Key(0, ui.KeyEscape)
		tt.Frame()
		Expect(v.decks.edit).To(BeNil())

		Expect(tt.Click("Wano/Miracle actions")).To(Succeed())
		Expect(tt.Click("Rename")).To(Succeed())
		tt.Frame()
		v.decks.rename = "Miracle"
		Expect(tt.Click("Save")).To(Succeed())
		Expect(f.builds).To(Equal([]store.Build{{ID: 1, Leader: "OP13-001", Name: "Miracle", Core: []string{"OTA"}}}))

		Expect(tt.RightClick("Wano/Miracle")).To(Succeed())
		Expect(tt.Click("Delete build")).To(Succeed())
		tt.Frame()
		Expect(f.deleted).To(Equal([]int64{1}))
		Expect(tt.HasText("Deleted Wano/Miracle.")).To(BeTrue())
		Expect(tt.Click("Undo")).To(Succeed())
		Expect(f.builds).To(ContainElement(store.Build{Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}))
	})

	It("opens the suggestions from the build row, with a way back", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}
		v, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("1 suggested")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Back to Wano/Miracle")).To(BeTrue())
		Expect(tt.Click("Back to Wano/Miracle")).To(Succeed())
		tt.Frame()
		Expect(v.decks.suggesting).To(BeFalse())
	})
})
