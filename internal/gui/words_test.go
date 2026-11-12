package gui

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("words", func() {
	at := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 18, 50, 0, 0, time.Local) }
	today := at(2026, time.November, 11)

	It("writes a date with its year only when it isn't this year's", func() {
		Expect(dateText(today, at(2026, time.October, 25))).To(Equal("Oct 25"))
		Expect(dateText(today, at(2025, time.December, 30))).To(Equal("Dec 30, 2025"))
		Expect(weekdayText(today, at(2026, time.October, 25))).To(Equal("Sun Oct 25"))
		Expect(ago(today, at(2025, time.December, 30))).To(Equal("Dec 30, 2025"))
		Expect(dayAndTime(today, at(2025, time.December, 30))).To(Equal("Dec 30, 2025 at 18:50"))
	})

	It("writes a range once when both ends read the same", func() {
		Expect(span("Oct 25", "Oct 25")).To(Equal("Oct 25"))
		Expect(span("Oct 25", "Nov 2")).To(Equal("Oct 25 – Nov 2"))
	})

	It("counts with the right noun", func() {
		Expect(games(1)).To(Equal("1 game"))
		Expect(count(2, "loss", "losses")).To(Equal("2 losses"))
	})
})
