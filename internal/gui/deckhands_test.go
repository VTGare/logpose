package gui

import (
	"github.com/egoist/mygo/ui"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/stats"
)

// handsSnapshot is deckSnapshot's games, all going first. The even ones, all wins, keep a hand
// with Garp and Otama. The odd ones, all losses, are dealt Shanks with Garp and mulligan.
func handsSnapshot() *snapshot {
	s := deckSnapshot()
	s.Cards.Name("SHK", "Shanks")
	for i := range s.Matches {
		m := &s.Matches[i]
		if i%2 == 0 {
			m.Me.HandBefore = []string{"GARP", "OTA", "X-1", "X-2", "X-3"}
		} else {
			m.Me.Mulliganed = true
			m.Me.HandBefore = []string{"SHK", "GARP", "X-1", "X-2", "X-3"}
			m.Me.HandAfter = []string{"OTA", "X-1", "X-2", "X-3", "X-4"}
			m.Opp.Mulliganed = true
		}
	}
	return s
}

// handsTester opens the Hands tab in a window tall enough for its card rows.
func handsTester() (*view, *ui.Tester) {
	v, tt := tester(handsSnapshot())
	v.act = &fakeActions{}
	tt.SetSize(1180, 2000)
	v.open("decks/hands")
	tt.Frame()
	return v, tt
}

var _ = Describe("Hands tab", func() {
	It("compares kept hands with mulligans and says where they differ", func() {
		_, tt := handsTester()
		for _, text := range []string{
			"Keep or mulligan", "Going first, hands you kept won 100% and mulligans won 0%.",
			"Kept the first five", "5 of 10 sent back", "You send back hands you think are weak",
			"When the opponent mulliganed you won 0% (0–5), and 100% (5–0) when they kept.",
		} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("shows each card dealt with how often its hand was kept", func() {
		_, tt := handsTester()
		Expect(tt.Texts()).To(ContainElements("Cards in your first five", "Dealt", "Kept that hand", "Won, kept", "Won, sent back", "Shanks", "0 of 5", "5 of 10"))
	})

	It("opens the hands a card was dealt in and shows all of them on request", func() {
		v, tt := handsTester()
		Expect(tt.Click("Show the hands dealt Shanks")).To(Succeed())
		tt.Frame()
		Expect(v.decks.handsCard).To(Equal("SHK"))
		Expect(tt.HasText("The 5 games you were dealt Shanks, newest first.")).To(BeTrue())
		Expect(tt.HasText("First · ")).To(BeTrue())
		Expect(tt.Click("Hide the hands dealt Shanks")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("you were dealt Shanks")).To(BeFalse())

		Expect(tt.Click("Show the hands dealt Garp")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Show all 10")).To(BeTrue())
		Expect(tt.Click("Show all 10")).To(Succeed())
		tt.Frame()
		Expect(v.decks.allHands).To(BeTrue())
		Expect(tt.HasText("Show all 10")).To(BeFalse())
	})

	It("opens a match from its hand", func() {
		v, tt := handsTester()
		Expect(tt.Click("Show the hands dealt Shanks")).To(Succeed())
		tt.Frame()
		Expect(tt.Click("Open the match vs Kaido")).To(Succeed())
		tt.Frame()
		Expect(v.page).To(Equal(pageMatches))
	})

	It("says when there are too few games to compare", func() {
		mu := stats.Mulligans{All: stats.KeepSplit{Kept: stats.Record{Wins: 3}}}
		Expect(keepFinding(mu)).To(Equal("Too few games yet to compare kept hands with mulligans."))
	})

	It("says when keeping made little difference either way", func() {
		even := stats.KeepSplit{Kept: stats.Record{Wins: 3, Losses: 2}, Mulliganed: stats.Record{Wins: 3, Losses: 2}}
		Expect(keepFinding(stats.Mulligans{First: even, Second: even})).To(Equal("Going first or second, keeping or mulliganing made little difference."))
		apart := stats.KeepSplit{Kept: stats.Record{Wins: 5}, Mulliganed: stats.Record{Wins: 2, Losses: 3}}
		Expect(keepFinding(stats.Mulligans{First: apart, Second: even})).
			To(Equal("Going first, hands you kept won 100% and mulligans won 40%. Going second it made little difference."))
	})
})
