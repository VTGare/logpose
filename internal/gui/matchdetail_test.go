package gui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/simlog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("match detail", func() {
	var fa *fakeActions

	open := func(s *snapshot) (*view, *ui.Tester) {
		v, tt := tester(s)
		fa = &fakeActions{}
		v.act = fa
		return v, tt
	}

	It("opens a match from the Matches page and goes back to the list", func() {
		s := testSnapshot(10)
		s.Matches[0].Me.HandBefore = []string{"OP13-001", "OP17-058"}
		s.Matches[0].Me.Mulliganed = true
		s.Matches[0].Me.HandAfter = []string{"OP17-058"}
		s.Matches[0].Opp.HandBefore = []string{"OP17-058"}
		v, tt := open(s)
		Expect(tt.Click("Matches")).To(Succeed())
		Expect(tt.Click("19:00")).To(Succeed())
		Expect(v.detail.fingerprint).To(Equal("0"))
		Expect(fa.loaded).To(Equal([]string{"0"}))
		for _, text := range []string{"Win", "12 turns · Concession", "Opening hands", "You mulliganed", "Sent back", "Opp kept", "went first", "went second", "Reading the log"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("All games")).To(BeFalse())

		Expect(tt.Click("Back to matches")).To(Succeed())
		Expect(v.detail.fingerprint).To(BeEmpty())
		Expect(tt.HasText("10 games · 5–5")).To(BeTrue())
	})

	It("opens a match from its tile on the overview and goes back there", func() {
		v, tt := open(testSnapshot(1))
		Expect(tt.Click("Open the match against Opp")).To(Succeed())
		Expect(v.page).To(Equal(pageOverview))
		Expect(v.detail.fingerprint).To(Equal("0"))
		Expect(tt.Click("Back to overview")).To(Succeed())
		Expect(v.detail.fingerprint).To(BeEmpty())
		Expect(v.page).To(Equal(pageOverview))
	})

	It("closes the match when its page is picked in the sidebar", func() {
		v, tt := open(testSnapshot(1))
		Expect(tt.Click("Open the match against Opp")).To(Succeed())
		Expect(tt.Click("Overview")).To(Succeed())
		Expect(v.detail.fingerprint).To(BeEmpty())
	})

	It("closes the match when another page is picked", func() {
		v, tt := open(testSnapshot(1))
		Expect(tt.Click("Open the match against Opp")).To(Succeed())
		Expect(tt.Click("Matches")).To(Succeed())
		tt.Frame()
		Expect(v.detail.fingerprint).To(BeEmpty())
		Expect(tt.HasText("Back to overview")).To(BeFalse())
	})

	It("shows life, hand and board by turn once the log is read", func() {
		v, tt := open(testSnapshot(3))
		v.openMatch(v.snap.Matches[1])
		v.setTimeline(matchTimeline{Fingerprint: "0", Me: []int{9}})
		tt.Frame()
		Expect(v.detail.timeline).To(BeNil())

		v.setTimeline(matchTimeline{
			Fingerprint: "1",
			Me:          []int{5, 5, 4}, Opp: []int{4, 3, 3},
			MeHand: []int{5, 6, 4}, OppHand: []int{5, 7, 6},
			MeBoard: []int{0, 2, 1}, OppBoard: []int{0, 1, 3},
		})
		v.detail.lifePoint = 1
		tt.Frame()
		for _, text := range []string{
			"Life", "Hand", "Board", "End of turn 1", "You: life 5, hand 6, board 2", "Opp: life 3, hand 7, board 1",
			"Taken that turn",
		} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.Click("Board")).To(Succeed())
		Expect(v.detail.flowTab).To(Equal(2))
	})

	It("bookmarks the match from its page", func() {
		s := testSnapshot(3)
		v, tt := open(s)
		v.openMatch(s.Matches[0])
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
		v, tt := open(s)
		Expect(tt.Click("Matches")).To(Succeed())
		_, starred := tt.Find("Bookmarked")
		Expect(starred).To(BeTrue())
		Expect(tt.HasText("3 games · 2–1")).To(BeTrue())
		Expect(tt.Click("Bookmarked · 1")).To(Succeed())
		Expect(v.matches.bookmarked).To(Equal(1))
		Expect(tt.HasText("1 game · 0–1")).To(BeTrue(), fmt.Sprint(tt.Texts()))
	})

	It("says when the log doesn't track life", func() {
		v, tt := open(testSnapshot(3))
		v.openMatch(v.snap.Matches[0])
		v.setTimeline(matchTimeline{Fingerprint: "0"})
		tt.Frame()
		Expect(tt.HasText("The log doesn't track life for this game.")).To(BeTrue())
	})

	It("sets the game type of a game in a room lobby from the top bar", func() {
		s := testSnapshot(3)
		v, tt := open(s)
		v.openMatch(s.Matches[0])
		tt.Frame()
		Expect(tt.HasText("Auto: Ranked")).To(BeTrue())
		Expect(tt.Click("Auto: Ranked")).To(Succeed())
		Expect(tt.Click("Friendly")).To(Succeed())
		Expect(fa.types).To(Equal([]string{"0=2"}))
		Expect(v.typeNote(s.Matches[0])).To(Equal("Counted as a friendly, as you set it."))
	})

	It("offers no game type choice in a private lobby", func() {
		s := testSnapshot(3)
		s.Matches[0].Lobby = simlog.Lobby{Code: "7KQ2MP"}
		s.Matches[0].Ranked = false
		v, tt := open(s)
		v.openMatch(s.Matches[0])
		tt.Frame()
		Expect(tt.HasText("Game type")).To(BeFalse(), fmt.Sprint(tt.Texts()))
		Expect(tt.HasText("Casual")).To(BeTrue())
	})

	It("saves the note when the box loses focus, and only once", func() {
		s := testSnapshot(3)
		v, tt := open(s)
		v.openMatch(s.Matches[0])
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
		v.openMatch(s.Matches[1])
		tt.Frame()
		Expect(v.detail.note).To(Equal("old note"))
		Expect(tt.Click("Match note")).To(Succeed())
		tt.Type(" and more")
		Expect(tt.Click("Overview")).To(Succeed())
		Expect(fa.notes).To(Equal([]string{"1=old note and more"}))
		Expect(v.detail.fingerprint).To(BeEmpty())
	})

	It("lists key moments with their cards in table talk", func() {
		s := testSnapshot(3)
		s.Matches[0].End = simlog.End{Kind: simlog.EndLethal, Loser: "Opp#2"}
		v, tt := open(s)
		v.openMatch(s.Matches[0])
		v.setTimeline(matchTimeline{Fingerprint: "0", Moments: []simlog.Moment{
			{
				Turn: 3, Player: me,
				Reasons: []simlog.Reason{{Kind: simlog.ReasonBoard, Player: me, N: 3}},
				Played:  []string{"OP13-001", "OP17-058", "OP17-058"},
				Board:   []string{"OP17-058", "OP13-001"},
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
		v.openMatch(v.snap.Matches[0])
		v.setTimeline(matchTimeline{Fingerprint: "0"})
		tt.Frame()
		Expect(tt.HasText("Nothing stood out in this game.")).To(BeTrue())
	})

	Describe("the Matches row menu", func() {
		rightClick := func() (*view, *ui.Tester) {
			v, tt := open(testSnapshot(3))
			Expect(tt.Click("Matches")).To(Succeed())
			Expect(tt.RightClick("19:00")).To(Succeed())
			return v, tt
		}

		It("offers game type, note, replay and filters", func() {
			_, tt := rightClick()
			Expect(tt.Menu()).To(ContainElements(
				"Game type", "Add note…", "Bookmark", "Watch replay", "Filter to games vs Opp", "Filter to Monkey D. Luffy vs Kaido",
			))
		})

		It("bookmarks the match", func() {
			_, tt := rightClick()
			Expect(tt.ChooseMenuItem("Bookmark")).To(Succeed())
			Expect(fa.bookmarks).To(Equal([]string{"0=true"}))
		})

		It("sets the game type", func() {
			_, tt := rightClick()
			Expect(tt.ChooseMenuItem("Game type", "Friendly")).To(Succeed())
			Expect(fa.types).To(Equal([]string{"0=2"}))
		})

		It("opens the match to write a note", func() {
			v, tt := rightClick()
			Expect(tt.ChooseMenuItem("Add note…")).To(Succeed())
			Expect(v.detail.fingerprint).To(Equal("0"))
			tt.Frame()
			tt.Type("close game")
			Expect(v.detail.note).To(Equal("close game"))
		})

		It("filters to the opponent, shown as a pill that removes it", func() {
			v, tt := rightClick()
			Expect(tt.ChooseMenuItem("Filter to games vs Opp")).To(Succeed())
			Expect(v.filter.opp).To(Equal("Opp#2"))
			tt.Frame()
			Expect(tt.Click("Remove the filter vs Opp")).To(Succeed())
			Expect(v.filter.opp).To(BeEmpty())
		})

		It("filters to the matchup", func() {
			v, tt := rightClick()
			Expect(tt.ChooseMenuItem("Filter to Monkey D. Luffy vs Kaido")).To(Succeed())
			Expect(v.filter.deck).To(Equal("OP13-001"))
			Expect(v.filter.oppLeader).To(Equal("OP17-058"))
		})
	})
})
