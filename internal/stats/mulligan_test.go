package stats_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

// handGame is a game dealt the hand, with the turn order known unless first is nil.
func handGame(won, mulliganed bool, first *bool, dealt ...string) stats.Match {
	res := stats.Loss
	if won {
		res = stats.Win
	}
	m := stats.Match{Me: store.GamePlayer{ID: me, Mulliganed: mulliganed, HandBefore: dealt}, Result: res}
	if first != nil {
		m.TurnOrderKnown, m.WentFirst = true, *first
	}
	return m
}

var _ = Describe("ForMulligans", func() {
	yes, no := true, false

	It("splits the games by mulligan, overall and by turn order", func() {
		mu := stats.ForMulligans([]stats.Match{
			handGame(true, false, &yes, "A"),
			handGame(true, false, &yes, "A"),
			handGame(false, true, &yes, "A"),
			handGame(false, false, &no, "A"),
			handGame(true, true, &no, "A"),
			handGame(true, true, nil, "A"),
		})
		Expect(mu.All).To(Equal(stats.KeepSplit{Kept: record(2, 1), Mulliganed: record(2, 1)}))
		Expect(mu.First).To(Equal(stats.KeepSplit{Kept: record(2, 0), Mulliganed: record(0, 1)}))
		Expect(mu.Second).To(Equal(stats.KeepSplit{Kept: record(0, 1), Mulliganed: record(1, 0)}))
	})

	It("splits the games by the opponent's mulligan", func() {
		m := handGame(true, false, nil)
		m.Opp.Mulliganed = true
		mu := stats.ForMulligans([]stats.Match{m, handGame(false, false, nil)})
		Expect(mu.Opp).To(Equal(stats.KeepSplit{Kept: record(0, 1), Mulliganed: record(1, 0)}))
	})

	It("counts each card dealt once a game, by whether the hand was kept", func() {
		mu := stats.ForMulligans([]stats.Match{
			handGame(true, false, nil, "A", "A", "B"),
			handGame(false, true, nil, "A", "C"),
			handGame(true, true, nil, "A"),
		})
		Expect(mu.Cards).To(Equal([]stats.DealtCard{
			{Card: "A", Kept: record(1, 0), SentBack: record(1, 1)},
			{Card: "B", Kept: record(1, 0)},
			{Card: "C", SentBack: record(0, 1)},
		}))
		Expect(mu.Cards[0].Dealt()).To(Equal(3))
	})

	It("leaves games without a result out of the cards", func() {
		m := handGame(true, false, nil, "A")
		m.Result = stats.Unknown
		Expect(stats.ForMulligans([]stats.Match{m}).Cards).To(BeEmpty())
	})
})
