package gui

import (
	"fmt"

	"github.com/VTGare/logpose/internal/gui/guitest"

	"github.com/VTGare/logpose/internal/gui/page"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("turn order and result filters", func() {
	// mixedTurns has 10 games, 5–5. The first 4 went second, 2–2, and the last one's log doesn't
	// say who went first.
	mixedTurns := func() *page.Snapshot {
		s := guitest.Snapshot(10)
		for i := range 4 {
			s.Matches[i].WentFirst = false
		}
		s.Matches[9].TurnOrderKnown = false
		return s
	}

	It("narrows every page to the turn order chosen from its pill", func() {
		_, tt := tester(mixedTurns())
		Expect(tt.Click("Either turn")).To(Succeed())
		Expect(guitest.Choose(tt, "Going second")).To(Succeed())
		Expect(tt.Texts()).To(ContainElements("Going second", "4 games"))

		Expect(tt.Click("Going second")).To(Succeed())
		Expect(guitest.Choose(tt, "Going first")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("5 games"))
		Expect(tt.Click("Decks")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("3–2 · 5 games"))
	})

	It("narrows only the match list by result, and keeps it after leaving the page", func() {
		_, tt := tester(mixedTurns())
		Expect(tt.Click("Matches")).To(Succeed())
		Expect(tt.HasText("10 games · 5–5")).To(BeTrue())
		Expect(tt.Click("Wins")).To(Succeed())
		Expect(tt.HasText("5 games · 5–0")).To(BeTrue(), fmt.Sprint(tt.Texts()))
		Expect(tt.Texts()).To(ContainElement("10 games"), "the filter bar still counts every game")

		Expect(tt.Click("Overview")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("10 games"))
		Expect(tt.Texts()).NotTo(ContainElement("5 games"))
		Expect(tt.Click("Matches")).To(Succeed())
		Expect(tt.HasText("5 games · 5–0")).To(BeTrue())
	})

	It("clears the result switch along with the bar's filters", func() {
		_, tt := tester(mixedTurns())
		Expect(tt.Click("Matches")).To(Succeed())
		Expect(tt.HasText("Clear")).To(BeFalse())
		Expect(tt.Click("Losses")).To(Succeed())
		Expect(tt.HasText("Clear")).To(BeTrue())
		Expect(tt.Click("Either turn")).To(Succeed())
		Expect(guitest.Choose(tt, "Going second")).To(Succeed())
		Expect(tt.Click("Clear")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("Either turn"))
		Expect(tt.HasText("10 games · 5–5")).To(BeTrue())
	})

	It("sets the turn order from a half of the Decks page's tile, and clears it from the same half", func() {
		_, tt := tester(mixedTurns())
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Click("Show games going second")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("Going second"))
		Expect(tt.Texts()).NotTo(ContainElement("0 games"), "the first half still counts its games")
		Expect(tt.Click("Show games going first")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("Going first"))
		Expect(tt.Click("Show games going first")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("Either turn"))
	})
})
