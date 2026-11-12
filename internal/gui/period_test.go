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
	pill := func(label string) {
		GinkgoHelper()
		Expect(tt.Texts()).To(ContainElement(label))
	}

	BeforeEach(func() {
		v, tt = tester(testSnapshot(48))
		tt.Frame()
		click("All time")
	})

	It("picks a day, then a range in either order, then starts over", func() {
		shows("October 2026", "November 2026", "Click a day, then another to make a range.")
		click("Pick Thu Nov 5")
		shows("Thu Nov 5", "24 games · 12–12")
		click("Pick Wed Nov 4")
		shows("Nov 4 – Nov 5", "2 days played · 28 games · 14–14")
		pill("28 games")
		click("Pick Fri Nov 6")
		pill("Fri Nov 6")
		pill("20 games")
	})

	It("doesn't pick a day after today", func() {
		click("Pick Thu Nov 5")
		_ = tt.Click("Pick Sat Nov 7")
		tt.Frame()
		pill("Thu Nov 5")
		pill("24 games")
	})

	It("keeps an ongoing period's end at today", func() {
		click("Pick Wed Nov 4")
		click("Ongoing")
		pill("Since Nov 4")
		pill("48 games")
		later := testSnapshot(48)
		m := later.Matches[0]
		m.Fingerprint, m.PlayedAt = "later", now.AddDate(0, 0, 2)
		later.Matches = append([]stats.Match{m}, later.Matches...)
		v.setSnapshot(later)
		v.clock = func() time.Time { return now.AddDate(0, 0, 3) }
		tt.Frame()
		pill("Since Nov 4")
		pill("49 games")
		shows("4 days played · 49 games")
	})

	It("drops the custom days for a preset", func() {
		click("Pick Thu Nov 5")
		click("Last 7 days")
		pill("Last 7 days")
		pill("48 games")
		Expect(tt.HasText("Click a day")).To(BeFalse())
		Expect(tt.HasText("November 2026")).To(BeFalse())
	})

	It("names the year only when it isn't this one", func() {
		click("Pick Thu Nov 5")
		for range 11 {
			click("Earlier month")
		}
		click("Pick Tue Dec 30, 2025")
		pill("Dec 30, 2025 – Nov 5")
	})

	It("overlaps a format to the days in both", func() {
		s := testSnapshot(48)
		s.Formats = []stats.FormatRange{{ID: 1, Name: "OP14", Start: now.Add(-30 * time.Hour)}}
		v.setSnapshot(s)
		v.setPrefs(prefsFrom(map[string]string{keyStartFormat: startLatest}))
		tt.Frame()
		click("Pick Wed Nov 4")
		click("Pick Thu Nov 5")
		pill("Nov 4 – Nov 5")
		pill("OP14")
		pill("10 games")
	})
})

var _ = Describe("By day", func() {
	It("sets the period to a day or a week", func() {
		_, tt := tester(testSnapshot(48))
		tt.Frame()
		Expect(tt.Click("Show only this day, Yesterday")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElements("Thu Nov 5", "24 games"))
		Expect(tt.Click("Show only this week, This week")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("Nov 2 – Nov 8"))
	})
})
