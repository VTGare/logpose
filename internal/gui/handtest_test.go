package gui

import (
	"math/rand/v2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("handTest", func() {
	list := map[string]int{"A": 4, "B": 4, "C": 42}
	var r *rand.Rand
	BeforeEach(func() { r = rand.New(rand.NewPCG(1, 2)) })

	It("deals five and leaves the rest of the list in the deck", func() {
		var h handTest
		h.deal(list, r)
		Expect(h.hand).To(HaveLen(5))
		Expect(h.deck).To(HaveLen(45))
	})

	It("mulligans once, before the first turn, keeping fifty cards", func() {
		var h handTest
		h.deal(list, r)
		first := h.hand
		h.mulligan(r)
		Expect(h.sent).To(Equal(first))
		Expect(h.hand).To(HaveLen(5))
		Expect(append(h.hand, h.deck...)).To(HaveLen(50))
		again := h.hand
		h.mulligan(r)
		Expect(h.hand).To(Equal(again))

		h.deal(list, r)
		h.next()
		Expect(h.canMulligan()).To(BeFalse())
	})

	It("skips the first draw going first and gives DON!! by turn order", func() {
		var h handTest
		h.deal(list, r)
		h.next()
		Expect(h.hand).To(HaveLen(5))
		Expect(h.don()).To(Equal(1))
		h.next()
		Expect(h.hand).To(HaveLen(6))
		Expect(h.don()).To(Equal(3))

		h.second = true
		h.deal(list, r)
		Expect(h.second).To(BeTrue())
		h.next()
		Expect(h.hand).To(HaveLen(6))
		Expect(h.don()).To(Equal(2))
		for range 10 {
			h.next()
		}
		Expect(h.turn).To(Equal(lastTurn))
		Expect(h.don()).To(Equal(10))
	})
})
