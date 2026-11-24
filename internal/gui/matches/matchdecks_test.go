package matches

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/deck"
	"github.com/VTGare/logpose/internal/gui/guitest"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/stats"
)

var _ = Describe("match decks", func() {
	var fa *fakeActions

	open := func(s *page.Snapshot) (*view, *ui.Tester) {
		v, tt := tester(s)
		fa = &fakeActions{}
		v.setActions(fa)
		return v, tt
	}

	// fillDeck adds cards summing to total copies into db and returns the list.
	fillDeck := func(db *cards.DB, prefix, name, trait string, color cards.Color, total int) map[string]int {
		lib := make(map[string]cards.Card)
		list := make(map[string]int)
		rem := total
		for i := 1; rem > 0; i++ {
			n := min(rem, 4)
			code := fmt.Sprintf("%s-%02d", prefix, i)
			ty, cost, counter := cards.Character, (i%5)+1, 1000
			if i == 2 {
				ty, cost, counter = cards.Event, 1, 0
			}
			lib[code] = cards.Card{
				Code:    code,
				Name:    fmt.Sprintf("%s %02d", name, i),
				Type:    ty,
				Cost:    cost,
				Counter: counter,
				Colors:  []cards.Color{color},
				Traits:  []string{trait},
			}
			list[code] = n
			rem -= n
		}
		(&cards.Library{Cards: lib}).Apply(db)
		return list
	}

	It("shows the opponent's 50-card list by default and copies it for the sim", func() {
		s := guitest.Snapshot(1)
		oppList := fillDeck(s.Cards, "OPP", "King", "Animal Kingdom Pirates", cards.Purple, 50)
		myList := fillDeck(s.Cards, "ME", "Zoro", "Straw Hat Crew", cards.Red, 50)

		v, tt := open(s)
		v.OpenMatch(s.Matches[0])
		v.matches.SetTimeline(MatchTimeline{Fingerprint: "0", OppList: oppList, MyList: myList})
		tt.Frame()

		for _, text := range []string{
			"Decks", "Opp · Kaido", "You · Monkey D. Luffy", "Copy for sim",
			"×4", "×2", "Cost curve", "Counters", "Types", "Traits", "Animal Kingdom Pirates",
		} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(guitest.Labelled(tt, "King 01")).To(BeTrue())
		Expect(guitest.Labelled(tt, "Zoro 01")).To(BeFalse())

		Expect(tt.Click("Collapse Decks")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Opp: 50 cards · You: 50 cards")).To(BeTrue())
		Expect(tt.Click("Expand Decks")).To(Succeed())
		tt.Frame()

		Expect(tt.Click("Copy for sim")).To(Succeed())
		tt.Frame()
		Expect(fa.Copied).To(HaveLen(1))
		want := deck.FormatSim("OP17-058", page.DeckListOrder(&v.Env, oppList), oppList)
		Expect(fa.Copied[0].Text).To(Equal(want))
		Expect(fa.Copied[0].Text).To(HavePrefix("1xOP17-058\r\n"))
		Expect(fa.Copied[0].Toast.Title).To(Equal("Copied decklist for the sim."))
		Expect(fa.Copied[0].Toast.Description).To(Equal("50 cards · Opp · Kaido"))
	})

	It("switches to your list and copies your leader and cards", func() {
		s := guitest.Snapshot(1)
		oppList := fillDeck(s.Cards, "OPP", "King", "Animal Kingdom Pirates", cards.Purple, 50)
		myList := fillDeck(s.Cards, "ME", "Zoro", "Straw Hat Crew", cards.Red, 50)
		s.Decks = map[string]stats.DeckGame{
			"0": {Match: s.Matches[0], Facts: deck.Facts{Player: guitest.Me, List: myList}},
		}

		v, tt := open(s)
		v.OpenMatch(s.Matches[0])
		v.matches.SetTimeline(MatchTimeline{Fingerprint: "0", OppList: oppList})
		tt.Frame()

		Expect(tt.Click("You · Monkey D. Luffy")).To(Succeed())
		tt.Frame()
		Expect(guitest.Labelled(tt, "Zoro 01")).To(BeTrue())
		Expect(guitest.Labelled(tt, "King 01")).To(BeFalse())
		Expect(tt.HasText("Straw Hat Crew")).To(BeTrue())

		Expect(tt.Click("Copy for sim")).To(Succeed())
		tt.Frame()
		Expect(fa.Copied).To(HaveLen(1))
		want := deck.FormatSim("OP13-001", page.DeckListOrder(&v.Env, myList), myList)
		Expect(fa.Copied[0].Text).To(Equal(want))
		Expect(fa.Copied[0].Text).To(HavePrefix("1xOP13-001\r\n"))
		Expect(fa.Copied[0].Toast.Title).To(Equal("Copied decklist for the sim."))
		Expect(fa.Copied[0].Toast.Description).To(Equal("50 cards · Monkey D. Luffy"))
	})

	It("notes a partial 49-card opponent list and copies it with the partial toast", func() {
		s := guitest.Snapshot(1)
		oppList := fillDeck(s.Cards, "OPP", "King", "Animal Kingdom Pirates", cards.Purple, 49)
		myList := fillDeck(s.Cards, "ME", "Zoro", "Straw Hat Crew", cards.Red, 50)

		v, tt := open(s)
		v.OpenMatch(s.Matches[0])
		v.matches.SetTimeline(MatchTimeline{Fingerprint: "0", OppList: oppList, MyList: myList})
		tt.Frame()

		Expect(tt.HasText("49 of 50 cards · 1 card wasn't named in the log.")).To(BeTrue())

		Expect(tt.Click("Collapse Decks")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Opp: 49 of 50 cards · You: 50 cards")).To(BeTrue())
		Expect(tt.Click("Expand Decks")).To(Succeed())
		tt.Frame()

		Expect(tt.Click("Copy for sim")).To(Succeed())
		tt.Frame()
		Expect(fa.Copied).To(HaveLen(1))
		Expect(fa.Copied[0].Toast.Title).To(Equal("Copied 49 of 50 cards for the sim."))
		Expect(fa.Copied[0].Toast.Description).To(Equal("1 card wasn't named in the log."))
	})

	It("says when a log has no decklists", func() {
		s := guitest.Snapshot(1)
		v, tt := open(s)
		v.OpenMatch(s.Matches[0])
		v.matches.SetTimeline(MatchTimeline{Fingerprint: "0"})
		tt.Frame()

		Expect(tt.HasText("The log doesn't include decklists for this game.")).To(BeTrue())
		Expect(tt.HasText("Copy for sim")).To(BeFalse())
	})

	It("handles a single missing side gracefully", func() {
		s := guitest.Snapshot(1)
		myList := fillDeck(s.Cards, "ME", "Zoro", "Straw Hat Crew", cards.Red, 50)
		v, tt := open(s)
		v.OpenMatch(s.Matches[0])
		v.matches.SetTimeline(MatchTimeline{Fingerprint: "0", MyList: myList})
		tt.Frame()

		Expect(tt.HasText("The log doesn't include this decklist.")).To(BeTrue())
		Expect(tt.Click("Copy for sim")).To(Succeed())
		Expect(fa.Copied).To(BeEmpty())

		Expect(tt.Click("Collapse Decks")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("You: 50 cards")).To(BeTrue())
		Expect(tt.Click("Expand Decks")).To(Succeed())
		tt.Frame()

		Expect(tt.Click("You · Monkey D. Luffy")).To(Succeed())
		tt.Frame()
		Expect(guitest.Labelled(tt, "Zoro 01")).To(BeTrue())
		Expect(tt.Click("Copy for sim")).To(Succeed())
		Expect(fa.Copied).To(HaveLen(1))
	})
})
