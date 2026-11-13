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
	It("shows Overall, the builds and No build with their records, and the suggestions only as a link", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		for _, text := range []string{"Overall", "10 games · 5–5 · 50%", "Wano/Miracle", "5 games · 5–0 · 100%", "No build", "5 games · 0–5 · 0%"} {
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
		s := deckSnapshot()
		for i := range s.Matches {
			s.Matches[i].Ranked = i%2 == 0
		}
		_, tt := tester(s)
		Expect(tt.Click("All games")).To(Succeed())
		Expect(choose(tt, "Ranked")).To(Succeed())
		Expect(tt.Click("Decks")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("5 games"))
		Expect(tt.HasText("2 suggested")).To(BeTrue())
	})

	It("opens on the Results tab with the leader's own stats", func() {
		_, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		for _, text := range []string{"Results", "List", "Cards", "Turn order"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("shows every game's cards on Cards for a leader without builds", func() {
		v, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		openDecks(v, tt, "cards")
		Expect(tt.HasText("Kid & Killer")).To(BeTrue())
		Expect(tt.HasText("Otama")).To(BeTrue())
	})

	It("counts the chosen scope's games on Cards", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		v, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		openDecks(v, tt, "cards")
		Expect(tt.HasText("Kid & Killer")).To(BeTrue())
		Expect(tt.HasText("Otama")).To(BeTrue())

		Expect(tt.Click("No build")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("In starting hand")).To(BeTrue())
		Expect(tt.HasText("Kid & Killer")).To(BeTrue())
		Expect(tt.HasText("Otama")).To(BeFalse())
	})

	It("shows every list under Overall, and the suggestions under No build", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		v, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		openDecks(v, tt, "list")
		Expect(tt.HasText("Latest list")).To(BeTrue())
		Expect(tt.HasText("Show changes")).To(BeTrue())

		Expect(tt.Click("No build")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Latest list")).To(BeFalse())
		Expect(tt.HasText("Suggested builds")).To(BeTrue())
		Expect(tt.HasText("Back to")).To(BeFalse())
	})

	It("goes back to Overall from the suggestions", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("1 suggested")).To(Succeed())
		tt.Frame()
		Expect(tt.Click("Back to Overall")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Suggested builds")).To(BeFalse())
		Expect(tt.HasText("Latest list")).To(BeTrue())
	})

	It("makes a suggestion a build with the name given, its searchers as the core", func() {
		v, tt := tester(deckSnapshot())
		f := &fakeActions{}
		v.setActions(f)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("2 suggested")).To(Succeed())
		tt.Frame()
		Expect(tt.Click("Make this a build")).To(Succeed())
		tt.Frame()
		retype(tt, "Build name", "Supernovas")
		Expect(tt.Click("Save")).To(Succeed())
		Expect(f.builds).To(Equal([]store.Build{{Leader: "OP13-001", Name: "Supernovas", Core: []string{"GARP", "KID", "LUF"}}}))
	})

	It("dismisses a suggestion", func() {
		v, tt := tester(deckSnapshot())
		f := &fakeActions{}
		v.setActions(f)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()
		Expect(tt.Click("Not a build")).To(Succeed())
		Expect(f.dismissals).To(HaveLen(1))
		Expect(f.dismissals[0].Leader).To(Equal("OP13-001"))
	})

	It("opens on Overall, with a build's list a click away", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("· Overall"))
		Expect(tt.Click("Wano/Miracle")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Latest list")).To(BeTrue())
	})

	It("shows a build's latest list with its stats", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("Wano/Miracle")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()
		for _, text := range []string{"Latest list", "Cost curve", "Counters", "+1000", "Types", "Traits", "×4"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("shows each card of the build with its numbers", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}
		v, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("Wano/Miracle")).To(Succeed())
		openDecks(v, tt, "cards")
		for _, text := range []string{"In starting hand", "In hand by its turn", "Not by its turn", "as starting hand", "Played", "Searcher took it", "Otama", "×4", "The tick is the build's 100%", "the difference likely isn't luck"} {
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
		_, tt := tester(s)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("Wano/Miracle")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()
		for _, text := range []string{"Latest list", "Latest", "First list", "Show changes"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("Core")).To(BeFalse())
		Expect(tt.Click("Nov 6 15:00")).To(Succeed())
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
		Expect(tt.Click("Show changes")).To(Succeed())
		tt.Frame()
		Expect(named()).To(Equal(before+1), "the rail names the card added")
		Expect(tt.HasText("×2")).To(BeTrue())
	})

	It("counts a game that fits two builds only for Overall", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano", Core: []string{"W-1"}}
		otama := store.Build{ID: 2, Leader: "OP13-001", Name: "Otama", Core: []string{"OTA"}}
		_, tt := tester(deckSnapshot(wano, otama))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Texts()).To(ContainElements("10 games · 5–5 · 50%", "5 games · 0–5 · 0%"))
		empty := 0
		for _, text := range tt.Texts() {
			if text == "0 games · 0–0 · too few" {
				empty++
			}
		}
		Expect(empty).To(Equal(2), "neither build counts the games both fit")
	})

	It("chooses No build from its chip, and drops Overall and No build without builds", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("No build")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElements("Monkey D. Luffy ›", "· No build"))

		_, tt = tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Texts()).NotTo(ContainElement("Overall"))
		Expect(tt.Texts()).NotTo(ContainElement("No build"))
	})

	It("counts only the chosen scope's games in the banner and the Results panels", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Texts()).To(ContainElements("5–5 · 10 games", "· Overall"))

		Expect(tt.Click("No build")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElements("0–5 · 5 games", "Monkey D. Luffy ›", "No build", "· No build"))

		Expect(tt.Click("Wano/Miracle")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElements("5–0 · 5 games", "· Wano/Miracle"))
	})

	It("keeps no-build games out of a build's turn order when a turn order is filtered", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("Wano/Miracle")).To(Succeed())
		tt.Frame()
		Expect(tt.Click("Show games going first")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("Going first"), "the turn order filter is on")
		Expect(tt.Texts()).To(ContainElements("1st", "5–0", "2nd", "0–0"))
		Expect(tt.Texts()).NotTo(ContainElement("5–5"))
	})

	It("compares the builds and No build under Overall, and a row click chooses one", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.HasText("Builds of Monkey D. Luffy")).To(BeTrue())
		build, ok := tt.Find("Show Wano/Miracle")
		Expect(ok).To(BeTrue())
		none, ok := tt.Find("Show No build")
		Expect(ok).To(BeTrue())
		Expect(build.Y).To(BeNumerically("<", none.Y))

		Expect(tt.Click("Show No build")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("· No build"))
		Expect(tt.HasText("Builds of Monkey D. Luffy")).To(BeFalse())
	})

	It("leaves the builds table out for a leader without builds", func() {
		_, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.HasText("Builds of")).To(BeFalse())
	})

	It("counts a game without deck facts in No build", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		s := deckSnapshot(wano)
		delete(s.Decks, "1")
		_, tt := tester(s)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("5 games · 0–5 · 0%"))
	})

	It("keeps the tab when another build is chosen", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}
		nova := store.Build{ID: 2, Leader: "OP13-001", Name: "Supernovas", Core: []string{"KID"}}
		v, tt := tester(deckSnapshot(wano, nova))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("Wano/Miracle")).To(Succeed())
		openDecks(v, tt, "cards")
		Expect(tt.HasText("Otama")).To(BeTrue())
		Expect(tt.Click("Supernovas")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("In starting hand")).To(BeTrue())
		Expect(tt.Texts()).To(ContainElement("KID · ×4"))
		Expect(tt.Texts()).NotTo(ContainElement("OTA · ×4"))
	})

	It("makes a new build from the cards clicked and the name typed", func() {
		v, tt := tester(deckSnapshot())
		f := &fakeActions{}
		v.setActions(f)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("New build")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("New Monkey D. Luffy build")).To(BeTrue())
		Expect(tt.Click("Otama")).To(Succeed())
		retype(tt, "Build name", "Wano")
		Expect(tt.HasText("5 games · 5–0")).To(BeTrue())
		Expect(tt.HasText("Results")).To(BeFalse())
		Expect(tt.Click("Save")).To(Succeed())
		tt.Frame()
		Expect(f.builds).To(Equal([]store.Build{{Leader: "OP13-001", Name: "Wano", Core: []string{"OTA"}}}))
		Expect(tt.HasText("New Monkey D. Luffy build")).To(BeFalse())
	})

	It("chooses a just-saved build by its ID when another build has its name", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano", Core: []string{"OTA"}}
		v, tt := tester(deckSnapshot(wano))
		f := &fakeActions{}
		v.setActions(f)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("New build")).To(Succeed())
		tt.Frame()
		Expect(tt.Click("Kid & Killer")).To(Succeed())
		retype(tt, "Build name", "Wano")
		Expect(tt.Click("Save")).To(Succeed())
		v.setSnapshot(deckSnapshot(store.Build{ID: 100, Leader: "OP13-001", Name: "Wano", Core: []string{"KID"}}, wano))
		tt.Frame()
		Expect(tt.HasText("Latest list")).To(BeTrue())
		Expect(tt.Texts()).To(ContainElement("0–5 · 5 games"), "the new build runs Kid & Killer, which only lost")
	})

	It("won't save a build without a name or a core card", func() {
		v, tt := tester(deckSnapshot())
		f := &fakeActions{}
		v.setActions(f)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("New build")).To(Succeed())
		tt.Frame()
		tt.Type("Wano")
		tt.Frame()
		Expect(tt.Click("Save")).To(Succeed())
		Expect(f.builds).To(BeEmpty())

		retype(tt, "Build name", "")
		Expect(tt.Click("Otama")).To(Succeed())
		tt.Frame()
		Expect(tt.Click("Save")).To(Succeed())
		Expect(f.builds).To(BeEmpty())
	})

	It("edits, renames and deletes a build from its menu, with Undo after a delete", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"OTA"}}
		v, tt := tester(deckSnapshot(wano))
		f := &fakeActions{}
		v.setActions(f)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.RightClick("Wano/Miracle")).To(Succeed())
		Expect(tt.Click("Edit core")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Edit Wano/Miracle")).To(BeTrue())
		Expect(tt.Texts()).To(ContainElement("Core"))
		Expect(tt.HasText("5 games · 5–0")).To(BeTrue())
		tt.Key(0, ui.KeyEscape)
		tt.Frame()
		Expect(tt.HasText("Edit Wano/Miracle")).To(BeFalse())

		Expect(tt.Click("Wano/Miracle actions")).To(Succeed())
		Expect(tt.Click("Rename")).To(Succeed())
		tt.Frame()
		retype(tt, "Build name", "Miracle")
		Expect(tt.Click("Save")).To(Succeed())
		Expect(f.builds).To(Equal([]store.Build{{ID: 1, Leader: "OP13-001", Name: "Miracle", Core: []string{"OTA"}}}))

		Expect(tt.Click("Overall")).To(Succeed())
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
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("Wano/Miracle")).To(Succeed())
		Expect(tt.Click("1 suggested")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Suggested builds")).To(BeTrue())
		Expect(tt.Click("Back to Wano/Miracle")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Suggested builds")).To(BeFalse())
	})
})

// openDecks opens a Decks tab the way -page does, since the sidebar's Cards hides the tab's.
func openDecks(v *view, tt *ui.Tester, tab string) {
	v.open("decks/" + tab)
	tt.Frame()
}

func retype(tt *ui.Tester, label, text string) {
	GinkgoHelper()
	Expect(tt.Click(label)).To(Succeed())
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	tt.Type(text)
	tt.Frame()
}
