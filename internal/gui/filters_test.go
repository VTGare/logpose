package gui

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/stats"
)

var _ = Describe("turn order and result filters", func() {
	// mixedTurns has 10 games, 5–5. The first 4 went second, 2–2, and the last one's log doesn't
	// say who went first.
	mixedTurns := func() *snapshot {
		s := testSnapshot(10)
		for i := range 4 {
			s.Matches[i].WentFirst = false
		}
		s.Matches[9].TurnOrderKnown = false
		return s
	}

	It("narrows every page to the turn order chosen from its pill", func() {
		v, tt := tester(mixedTurns())
		Expect(tt.Click("Either turn")).To(Succeed())
		Expect(tt.Click("Going second")).To(Succeed())
		Expect(v.filter.turn).To(Equal(stats.GoingSecond))
		Expect(v.scoped().matches).To(HaveLen(4))
		tt.Frame()
		Expect(tt.HasText("Going second")).To(BeTrue())
		Expect(tt.HasText("4 games")).To(BeTrue(), fmt.Sprint(tt.Texts()))

		v.filter.turn = stats.GoingFirst
		Expect(v.scoped().matches).To(HaveLen(5))
		Expect(tt.Click("Decks")).To(Succeed())
		l, _ := v.leaderData()
		Expect(l.Record.Games()).To(Equal(5))
	})

	It("narrows only the match list by result, and keeps it after leaving the page", func() {
		v, tt := tester(mixedTurns())
		Expect(tt.Click("Matches")).To(Succeed())
		Expect(tt.HasText("10 games · 5–5")).To(BeTrue())
		Expect(tt.Click("Wins")).To(Succeed())
		Expect(tt.HasText("5 games · 5–0")).To(BeTrue(), fmt.Sprint(tt.Texts()))
		Expect(v.scoped().matches).To(HaveLen(10))

		Expect(tt.Click("Overview")).To(Succeed())
		Expect(v.scoped().overview.Total.Games()).To(Equal(10))
		Expect(tt.Click("Matches")).To(Succeed())
		Expect(tt.HasText("5 games · 5–0")).To(BeTrue())
	})

	It("clears the result switch along with the bar's filters", func() {
		v, tt := tester(mixedTurns())
		Expect(tt.Click("Matches")).To(Succeed())
		Expect(tt.HasText("Clear")).To(BeFalse())
		Expect(tt.Click("Losses")).To(Succeed())
		Expect(tt.HasText("Clear")).To(BeTrue())
		v.filter.turn = stats.GoingSecond
		tt.Frame()
		Expect(tt.Click("Clear")).To(Succeed())
		Expect(v.matches.result).To(Equal(anyResult))
		Expect(v.filter.turn).To(Equal(stats.EitherTurn))
		Expect(tt.HasText("10 games · 5–5")).To(BeTrue())
	})

	It("sets the turn order from a half of the Decks page's tile, and clears it from the same half", func() {
		v, tt := tester(mixedTurns())
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("Show games going second")).To(Succeed())
		Expect(v.filter.turn).To(Equal(stats.GoingSecond))
		tt.Frame()
		Expect(tt.Texts()).NotTo(ContainElement("0 games"), "the first half still counts its games")
		Expect(tt.Click("Show games going first")).To(Succeed())
		Expect(v.filter.turn).To(Equal(stats.GoingFirst))
		Expect(tt.Click("Show games going first")).To(Succeed())
		Expect(v.filter.turn).To(Equal(stats.EitherTurn))
	})
})
