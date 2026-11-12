package gui

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/stats"
)

var _ = Describe("rate text", func() {
	It("shows a rate from 5 games and says too few below", func() {
		Expect(rateText(stats.Record{Wins: 3, Losses: 2})).To(Equal("60%"))
		Expect(rateText(stats.Record{Wins: 3, Losses: 1})).To(Equal("too few"))
		Expect(rateRecord(stats.Record{Wins: 3, Losses: 1})).To(Equal("too few (3–1)"))
		Expect(rateRecord(stats.Record{})).To(Equal("–"))
		Expect(scopeMeta(4, stats.Record{Wins: 3, Losses: 1})).To(Equal("4 games · 3–1 · too few"))
	})

	It("says too few for a matchup under 5 games", func() {
		_, tt := matchupsTester(matchupsSnapshot())
		Expect(tt.Texts()).To(ContainElement("too few"))
	})
})
