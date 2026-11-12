package stats_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/stats"
)

var _ = Describe("DON!! curve", func() {
	It("gives 1 DON!! and then 2 a turn going first, 2 a turn going second, up to 10", func() {
		Expect([]int{stats.Don(0, true), stats.Don(1, true), stats.Don(2, true), stats.Don(6, true)}).To(Equal([]int{0, 1, 3, 10}))
		Expect([]int{stats.Don(1, false), stats.Don(2, false), stats.Don(9, false)}).To(Equal([]int{2, 4, 10}))
	})

	It("finds the first turn a cost is paid for", func() {
		Expect(stats.PayTurn(10, true)).To(Equal(6))
		Expect(stats.PayTurn(10, false)).To(Equal(5))
		Expect(stats.PayTurn(5, true)).To(Equal(3))
		Expect(stats.PayTurn(0, false)).To(Equal(1))
	})

	It("draws from the first turn going second and the second going first", func() {
		Expect(stats.Draws(1, true)).To(BeZero())
		Expect(stats.Draws(1, false)).To(Equal(1))
	})

	It("takes costs up to 2 as paid for from about the opening hand", func() {
		Expect(stats.FromOpening(2)).To(BeTrue())
		Expect(stats.FromOpening(3)).To(BeFalse())
	})
})
