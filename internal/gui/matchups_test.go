package gui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// matchupsSnapshot is 9 games against Kaido, 4–5, and the newest one a win against Buggy.
func matchupsSnapshot() *snapshot {
	s := testSnapshot(10)
	s.Matches[0].Opp.Leader = "OP09-042"
	s.Cards.Name("OP09-042", "Buggy")
	return s
}

func matchupsTester(s *snapshot) (*view, *ui.Tester) {
	v, tt := tester(s)
	Expect(tt.Click("Matchups")).To(Succeed())
	Expect(v.page).To(Equal(pageMatchups))
	return v, tt
}

var _ = Describe("Matchups page", func() {
	It("lists every opposing leader with its set and opens the most played one", func() {
		_, tt := matchupsTester(matchupsSnapshot())
		for _, text := range []string{
			"2 leaders faced", "Kaido", "OP17", "44%", "4–5", "Buggy", "OP09", "1–0",
			"Turn order", "How the games went", "Latest games", "Show in Matches",
		} {
			Expect(tt.HasText(text)).To(BeTrue(), fmt.Sprint(text, tt.Texts()))
		}
	})

	It("opens the row picked, and closes it when picked again", func() {
		v, tt := matchupsTester(matchupsSnapshot())
		Expect(tt.Click("Show the matchup vs Buggy OP09")).To(Succeed())
		Expect(v.matchups.pick).To(Equal("OP09-042"))
		tt.Frame()
		Expect(tt.HasText("Latest games")).To(BeTrue())
		Expect(tt.Click("Show the matchup vs Buggy OP09")).To(Succeed())
		tt.Frame()
		Expect(v.matchups.pick).To(Equal(matchupNone))
		Expect(tt.HasText("Latest games")).To(BeFalse())
	})

	It("opens Matches filtered to the matchup", func() {
		v, tt := matchupsTester(matchupsSnapshot())
		Expect(tt.Click("Show in Matches")).To(Succeed())
		Expect(v.page).To(Equal(pageMatches))
		Expect(v.filter.deck).To(Equal("OP13-001"))
		Expect(v.filter.oppLeader).To(Equal("OP17-058"))
		tt.Frame()
		Expect(tt.HasText("9 games · 4–5")).To(BeTrue())
	})

	It("counts only the games in the filters' scope", func() {
		s := matchupsSnapshot()
		for i := range 4 {
			s.Matches[i].Ranked = false
		}
		v, tt := matchupsTester(s)
		v.filter.gameType = int(stats.CasualGames)
		tt.Frame()
		_, list := v.matchupData()
		Expect(list).To(HaveLen(2))
		Expect(list[0].Record.Games()).To(Equal(3))
		Expect(tt.HasText("2 leaders faced")).To(BeTrue())
	})

	It("lists the matchups of the deck picked at the top", func() {
		s := matchupsSnapshot()
		s.Matches[5].Me.Leader = "EB05-010"
		s.Cards.Name("EB05-010", "Nico Robin")
		v, tt := matchupsTester(s)
		Expect(tt.Click("Show matchups with Nico Robin")).To(Succeed())
		Expect(v.filter.deck).To(Equal("EB05-010"))
		tt.Frame()
		Expect(tt.HasText("1 leader faced")).To(BeTrue())
	})
})
