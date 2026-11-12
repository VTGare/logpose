package cards_test

import (
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/cards"
)

var _ = Describe("codes", func() {
	It("reads a set's kind and number", func() {
		kind, n, ok := cards.SplitSet("OP13")
		Expect([]any{kind, n, ok}).To(Equal([]any{"OP", 13, true}))
		kind, _, ok = cards.SplitSet("P")
		Expect(kind).To(Equal("P"))
		Expect(ok).To(BeFalse())
		Expect(cards.SetOf("PRB02-005_p1")).To(Equal("PRB02"))
	})

	It("orders codes newest kind and set first, then by number", func() {
		codes := []string{"P-001", "ST01-002", "OP01-003", "EB02-001", "OP13-002", "OP13-001", "PRB02-001"}
		slices.SortFunc(codes, cards.CompareCodes)
		Expect(codes).To(Equal([]string{"OP13-001", "OP13-002", "OP01-003", "EB02-001", "PRB02-001", "ST01-002", "P-001"}))
	})

	It("tells printing keys from reprints", func() {
		Expect(cards.IsPrintKey("OP07-021_p1")).To(BeTrue())
		Expect(cards.IsPrintKey("OP07-021_r1")).To(BeFalse())
	})

	It("knows leaders and the cards that print power", func() {
		Expect(cards.Card{Type: cards.Leader}.IsLeader()).To(BeTrue())
		Expect(cards.Card{Type: cards.Character}.HasPower()).To(BeTrue())
		Expect(cards.Card{Type: cards.Event}.HasPower()).To(BeFalse())
	})
})
