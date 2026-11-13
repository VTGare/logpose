package gui

import (
	"fmt"
	"time"

	"github.com/VTGare/logpose/internal/gui/matches"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// hover rests the pointer on the element showing text until its tooltip shows.
func hover(tt *ui.Tester, text string) {
	r, ok := tt.Find(text)
	Expect(ok).To(BeTrue(), text)
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(time.Second)
	tt.Frame()
}

var _ = Describe("match detail", func() {
	var fa *fakeActions

	open := func(s *page.Snapshot) (*view, *ui.Tester) {
		v, tt := tester(s)
		fa = &fakeActions{}
		v.setActions(fa)
		return v, tt
	}

	It("opens a match from the Matches page and goes back to the list", func() {
		s := testSnapshot(10)
		s.Matches[0].Me.HandBefore = []string{"OP13-001", "OP17-058"}
		s.Matches[0].Me.Mulliganed = true
		s.Matches[0].Me.HandAfter = []string{"OP17-058"}
		s.Matches[0].Opp.HandBefore = []string{"OP17-058"}
		_, tt := open(s)
		Expect(tt.Click("Matches")).To(Succeed())
		Expect(tt.Click("19:00")).To(Succeed())
		Expect(fa.loaded).To(Equal([]string{"0"}))
		for _, text := range []string{"Win", "12 turns · Concession", "Opening hands", "You mulliganed", "Sent back", "Opp kept", "went first", "went second", "Reading the log"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("All games")).To(BeFalse())

		Expect(tt.Click("Back to matches")).To(Succeed())
		Expect(tt.HasText("Opening hands")).To(BeFalse())
		Expect(tt.HasText("10 games · 5–5")).To(BeTrue())
	})

	It("opens a match from its tile on the overview and goes back there", func() {
		_, tt := open(testSnapshot(1))
		Expect(tt.Click("Open the match against Opp")).To(Succeed())
		Expect(tt.HasText("Opening hands")).To(BeTrue())
		Expect(tt.Click("Back to overview")).To(Succeed())
		Expect(tt.HasText("Opening hands")).To(BeFalse())
		Expect(tt.HasText("Open the match against Opp")).To(BeTrue())
	})

	It("closes the match when its page is picked in the sidebar", func() {
		_, tt := open(testSnapshot(1))
		Expect(tt.Click("Open the match against Opp")).To(Succeed())
		Expect(tt.Click("Overview")).To(Succeed())
		Expect(tt.HasText("Opening hands")).To(BeFalse())
		Expect(tt.HasText("Open the match against Opp")).To(BeTrue())
	})

	It("closes the match when another page is picked", func() {
		_, tt := open(testSnapshot(1))
		Expect(tt.Click("Open the match against Opp")).To(Succeed())
		Expect(tt.Click("Matches")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Opening hands")).To(BeFalse())
		Expect(tt.HasText("Back to overview")).To(BeFalse())
	})

	It("shows life, hand and board by turn once the log is read", func() {
		v, tt := open(testSnapshot(3))
		v.OpenMatch(v.Snap.Matches[1])
		v.matches.SetTimeline(matches.MatchTimeline{Fingerprint: "0", Me: []int{9}})
		tt.Frame()
		Expect(tt.HasText("Reading the log")).To(BeTrue())

		v.matches.SetTimeline(matches.MatchTimeline{
			Fingerprint: "1",
			Me:          []int{5, 5, 4}, Opp: []int{4, 3, 3},
			MeHand: []int{5, 6, 4}, OppHand: []int{5, 7, 6},
			MeBoard: []int{0, 2, 1}, OppBoard: []int{0, 1, 3},
		})
		tt.Frame()
		chart, ok := tt.Find("Life by turn")
		Expect(ok).To(BeTrue())
		tt.Move(chart.X+chart.W/2, chart.Y+chart.H/2)
		tt.Frame()
		for _, text := range []string{
			"Life", "Hand", "Board", "End of turn 1", "You: life 5, hand 6, board 2", "Opp: life 3, hand 7, board 1",
			"Taken that turn",
		} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.Click("Board")).To(Succeed())
		_, board := tt.Find("Board by turn")
		Expect(board).To(BeTrue())
	})

	It("bookmarks the match from its page", func() {
		s := testSnapshot(3)
		v, tt := open(s)
		v.OpenMatch(s.Matches[0])
		tt.Frame()
		Expect(tt.Click("Bookmark")).To(Succeed())
		Expect(fa.bookmarks).To(Equal([]string{"0=true"}))

		s.Matches[0].Bookmarked = true
		tt.Frame()
		_, unset := tt.Find("Bookmark")
		Expect(unset).To(BeFalse())
		Expect(tt.Click("Bookmarked")).To(Succeed())
		Expect(fa.bookmarks).To(Equal([]string{"0=true", "0=false"}))
	})

	It("shows only bookmarked matches when asked, with a star by the date", func() {
		s := testSnapshot(3)
		s.Matches[1].Bookmarked = true
		_, tt := open(s)
		Expect(tt.Click("Matches")).To(Succeed())
		_, starred := tt.Find("Bookmarked")
		Expect(starred).To(BeTrue())
		Expect(tt.HasText("3 games · 2–1")).To(BeTrue())
		Expect(tt.Click("Bookmarked · 1")).To(Succeed())
		Expect(tt.HasText("1 game · 0–1")).To(BeTrue(), fmt.Sprint(tt.Texts()))
	})

	It("says when the log doesn't track life", func() {
		v, tt := open(testSnapshot(3))
		v.OpenMatch(v.Snap.Matches[0])
		v.matches.SetTimeline(matches.MatchTimeline{Fingerprint: "0"})
		tt.Frame()
		Expect(tt.HasText("The log doesn't track life for this game.")).To(BeTrue())
	})

	It("sets the game type of a game in a room lobby from the top bar", func() {
		s := testSnapshot(3)
		v, tt := open(s)
		v.OpenMatch(s.Matches[0])
		tt.Frame()
		Expect(tt.HasText("Auto: Ranked")).To(BeTrue())
		Expect(tt.Click("Auto: Ranked")).To(Succeed())
		Expect(tt.Click("Friendly")).To(Succeed())
		Expect(fa.types).To(Equal([]string{"0=2"}))
		hover(tt, "Friendly")
		Expect(tt.HasText("Counted as a friendly, as you set it.")).To(BeTrue(), fmt.Sprint(tt.Texts()))
	})

	It("says why the guess counts a game as it does, from every stored game in its lobby", func() {
		s := testSnapshot(3)
		g := s.Matches[0].Game
		g.Fingerprint = "other"
		ms := stats.Matches([]store.Game{s.Matches[0].Game, g}, me)
		s.Matches[0] = ms[0]
		v, tt := open(s)
		v.OpenMatch(s.Matches[0])
		tt.Frame()
		hover(tt, "Auto: Friendly")
		Expect(tt.HasText("Counted as a friendly, since its lobby has 2 games.")).To(BeTrue(), fmt.Sprint(tt.Texts()))
	})

	It("offers no game type choice in a private lobby", func() {
		s := testSnapshot(3)
		s.Matches[0].Lobby = simlog.Lobby{Code: "7KQ2MP"}
		s.Matches[0].Ranked = false
		v, tt := open(s)
		v.OpenMatch(s.Matches[0])
		tt.Frame()
		Expect(tt.HasText("Game type")).To(BeFalse(), fmt.Sprint(tt.Texts()))
		Expect(tt.HasText("Casual")).To(BeTrue())
	})

	It("saves the note when the box loses focus, and only once", func() {
		s := testSnapshot(3)
		v, tt := open(s)
		v.OpenMatch(s.Matches[0])
		tt.Frame()
		Expect(tt.HasText("Match note")).To(BeFalse())
		Expect(tt.Click("Add note")).To(Succeed())
		Expect(tt.Click("Match note")).To(Succeed())
		tt.Type("missed lethal on 7 ")
		tt.Frame()
		Expect(fa.notes).To(BeEmpty())

		Expect(tt.Click("Opening hands")).To(Succeed())
		Expect(fa.notes).To(Equal([]string{"0=missed lethal on 7"}))
		Expect(tt.Click("Back to overview")).To(Succeed())
		Expect(fa.notes).To(HaveLen(1))
	})

	It("saves the note being typed when the page changes", func() {
		s := testSnapshot(3)
		s.Matches[1].Note = "old note"
		v, tt := open(s)
		v.OpenMatch(s.Matches[1])
		tt.Frame()
		Expect(tt.Click("Match note")).To(Succeed())
		tt.Type(" and more")
		Expect(tt.Click("Overview")).To(Succeed())
		Expect(fa.notes).To(Equal([]string{"1=old note and more"}))
		Expect(tt.HasText("Match note")).To(BeFalse())
	})

	It("lists key moments with their cards in table talk", func() {
		s := testSnapshot(3)
		s.Matches[0].End = simlog.End{Kind: simlog.EndLethal, Loser: "Opp#2"}
		v, tt := open(s)
		v.OpenMatch(s.Matches[0])
		v.matches.SetTimeline(matches.MatchTimeline{Fingerprint: "0", Moments: []simlog.Moment{
			{
				Turn: 3, Player: me,
				Reasons:    []simlog.Reason{{Kind: simlog.ReasonBoard, Player: me, N: 3}},
				Played:     []string{"OP13-001", "OP17-058", "OP17-058"},
				Characters: []string{"OP13-001", "OP17-058", "OP17-058"},
				Board:      []string{"OP17-058", "OP13-001"},
				New:        2,
				Gone:       []string{"OP17-058"},
			},
			{
				Turn: 4, Player: "Opp#2",
				Reasons: []simlog.Reason{{Kind: simlog.ReasonRemoved, Player: "Opp#2", N: 2}},
				Removed: []string{"OP17-058", "OP17-058"},
			},
			{
				Turn: 5, Player: me,
				Reasons: []simlog.Reason{
					{Kind: simlog.ReasonLifeLost, Player: "Opp#2", N: 2},
					{Kind: simlog.ReasonAtZero, Player: "Opp#2"},
					{Kind: simlog.ReasonFinish, Player: "Opp#2"},
				},
			},
		}})
		tt.Frame()
		for _, text := range []string{
			"Turn 3", "You played Monkey D. Luffy and Kaido ×2", "Your board after the turn", "Also played",
			"Opp removed Kaido ×2", "Removed from you",
			"You hit Opp for 2 · Opp is at 0", "You dealt lethal",
		} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("says when no turn stood out", func() {
		v, tt := open(testSnapshot(3))
		v.OpenMatch(v.Snap.Matches[0])
		v.matches.SetTimeline(matches.MatchTimeline{Fingerprint: "0"})
		tt.Frame()
		Expect(tt.HasText("Nothing stood out in this game.")).To(BeTrue())
	})

	Describe("the Matches row menu", func() {
		rightClick := func() (*view, *ui.Tester) {
			v, tt := open(testSnapshot(3))
			Expect(tt.Click("Matches")).To(Succeed())
			Expect(tt.RightClick("19:00")).To(Succeed())
			tt.Frame()
			return v, tt
		}

		It("offers opening the match, replay, game type, note and filters", func() {
			_, tt := rightClick()
			for _, item := range []string{
				"Open match", "Watch replay", "Game type", "Add note…", "Bookmark", "Filter to games vs Opp",
				"Filter to Monkey D. Luffy vs Kaido",
			} {
				Expect(tt.HasText(item)).To(BeTrue(), item)
			}
		})

		It("opens the match", func() {
			_, tt := rightClick()
			Expect(choose(tt, "Open match")).To(Succeed())
			tt.Frame()
			Expect(tt.HasText("Opening hands")).To(BeTrue())
			Expect(fa.loaded).To(Equal([]string{"0"}))
		})

		It("bookmarks the match", func() {
			_, tt := rightClick()
			Expect(choose(tt, "Bookmark")).To(Succeed())
			Expect(fa.bookmarks).To(Equal([]string{"0=true"}))
		})

		It("sets the game type", func() {
			_, tt := rightClick()
			Expect(choose(tt, "Game type", "Friendly")).To(Succeed())
			Expect(fa.types).To(Equal([]string{"0=2"}))
		})

		It("opens the match to write a note", func() {
			_, tt := rightClick()
			Expect(choose(tt, "Add note…")).To(Succeed())
			Expect(fa.loaded).To(Equal([]string{"0"}))
			tt.Frame()
			tt.Type("close game")
			Expect(tt.Click("Opening hands")).To(Succeed())
			Expect(fa.notes).To(Equal([]string{"0=close game"}))
		})

		It("filters to the opponent, shown as a pill that removes it", func() {
			_, tt := rightClick()
			Expect(choose(tt, "Filter to games vs Opp")).To(Succeed())
			tt.Frame()
			Expect(tt.Click("Remove the filter vs Opp")).To(Succeed())
			_, pill := tt.Find("Remove the filter vs Opp")
			Expect(pill).To(BeFalse())
		})

		It("filters to the matchup", func() {
			_, tt := rightClick()
			Expect(choose(tt, "Filter to Monkey D. Luffy vs Kaido")).To(Succeed())
			tt.Frame()
			_, pill := tt.Find("Remove the filter vs Kaido")
			Expect(pill).To(BeTrue())
			Expect(tt.HasText("Monkey D. Luffy")).To(BeTrue())
		})
	})
})
