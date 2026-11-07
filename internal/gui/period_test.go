package gui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("custom period", func() {
	// 48 hourly games: 20 on Fri Nov 6, 24 on Thu Nov 5 and 4 on Wed Nov 4.
	var (
		v  *view
		tt *ui.Tester
	)
	date := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.Local) }
	click := func(label string) {
		GinkgoHelper()
		Expect(tt.Click(label)).To(Succeed())
		tt.Frame()
	}
	shows := func(texts ...string) {
		GinkgoHelper()
		for _, text := range texts {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	}

	BeforeEach(func() {
		v, tt = tester(testSnapshot(48))
		tt.Frame()
		v.periodUI.open = true
		v.openPeriod()
		tt.Frame()
	})

	It("picks a day, then a range in either order, then starts over", func() {
		shows("October 2026", "November 2026", "Click a day, then another to make a range.")
		click("Pick Thu Nov 5")
		shows("Thu Nov 5", "24 games · 12–12")
		click("Pick Wed Nov 4")
		Expect(v.filter.from).To(Equal(date(time.November, 4)))
		Expect(v.filter.to).To(Equal(date(time.November, 5)))
		shows("Nov 4 – Nov 5", "2 days played · 28 games · 14–14")
		Expect(v.scoped().matches).To(HaveLen(28))
		click("Pick Fri Nov 6")
		Expect(v.periodLabel()).To(Equal("Fri Nov 6"))
		Expect(v.scoped().matches).To(HaveLen(20))
	})

	It("doesn't pick a day after today", func() {
		click("Pick Thu Nov 5")
		_ = tt.Click("Pick Sat Nov 7")
		tt.Frame()
		Expect(v.periodLabel()).To(Equal("Thu Nov 5"))
	})

	It("keeps an ongoing period's end at today", func() {
		click("Pick Wed Nov 4")
		click("Ongoing")
		Expect(v.periodLabel()).To(Equal("Since Nov 4"))
		Expect(v.scoped().matches).To(HaveLen(48))
		v.snap.Now = now.AddDate(0, 0, 3)
		Expect(v.customEnd()).To(Equal(date(time.November, 9)))
	})

	It("drops the custom days for a preset", func() {
		click("Pick Thu Nov 5")
		click("Last 7 days")
		Expect(v.filter.period).To(Equal(1))
		Expect(v.filter.from.IsZero()).To(BeTrue())
		Expect(v.periodUI.open).To(BeFalse())
	})

	It("names the year only when it isn't this one", func() {
		v.setRange(date(time.December, 30).AddDate(-1, 0, 0), date(time.November, 5))
		Expect(v.periodLabel()).To(Equal("Dec 30, 2025 – Nov 5"))
	})

	It("overlaps a format to the days in both", func() {
		v.snap.Formats = []stats.FormatRange{{ID: 1, Name: "OP14", Start: now.Add(-30 * time.Hour)}}
		v.setRange(date(time.November, 4), date(time.November, 5))
		s := v.currentScope()
		Expect(s.Since).To(Equal(now.Add(-30 * time.Hour)))
		Expect(s.Until).To(Equal(date(time.November, 6)))
	})
})

var _ = Describe("By day", func() {
	It("sets the period to a day or a week", func() {
		v, tt := tester(testSnapshot(48))
		tt.Frame()
		Expect(tt.Click("Show only this day, Yesterday")).To(Succeed())
		tt.Frame()
		Expect(v.periodLabel()).To(Equal("Thu Nov 5"))
		Expect(v.scoped().matches).To(HaveLen(24))
		Expect(tt.Click("Show only this week, This week")).To(Succeed())
		tt.Frame()
		Expect(v.periodLabel()).To(Equal("Nov 2 – Nov 8"))
	})
})
