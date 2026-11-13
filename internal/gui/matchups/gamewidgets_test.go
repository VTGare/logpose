package matchups

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/gui/decks"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/stats"
)

var _ = Describe("rate text", func() {
	It("shows a rate from 5 games and says too few below", func() {
		Expect(page.RateText(stats.Record{Wins: 3, Losses: 2})).To(Equal("60%"))
		Expect(page.RateText(stats.Record{Wins: 3, Losses: 1})).To(Equal("too few"))
		Expect(page.RateRecord(stats.Record{Wins: 3, Losses: 1})).To(Equal("too few (3–1)"))
		Expect(page.RateRecord(stats.Record{})).To(Equal("–"))
		Expect(decks.ScopeMeta(4, stats.Record{Wins: 3, Losses: 1})).To(Equal("4 games · 3–1 · too few"))
	})

	It("says too few for a matchup under 5 games", func() {
		_, tt := matchupsTester(matchupsSnapshot())
		Expect(tt.Texts()).To(ContainElement("too few"))
	})
})
