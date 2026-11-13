package overview

import (
	"time"

	"github.com/egoist/mygo/ui"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/gui/guitest"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

// tester shows the Overview under the filter bar.
func tester(s *page.Snapshot) (*guitest.Host, *ui.Tester) {
	return guitest.WithBar(s, func(c *ui.Context, e *page.Env) { Draw(c, e, func() {}) })
}

var _ = Describe("formats on the Overview", func() {
	// withFormats splits the 10 test games, an hour apart, at 4 hours ago.
	withFormats := func() *page.Snapshot {
		s := guitest.Snapshot(10)
		s.Formats = stats.Ranges([]store.Format{
			{ID: 1, Name: "Old format", Start: guitest.Now.Add(-48 * time.Hour)},
			{ID: 2, Name: "New format", Start: guitest.Now.Add(-4*time.Hour - 30*time.Minute)},
		})
		return s
	}

	latestFirst := func(s *page.Snapshot) *ui.Tester {
		v, tt := tester(s)
		v.SetPrefs(page.PrefsFrom(map[string]string{page.KeyStartFormat: page.StartLatest}))
		tt.Frame()
		return tt
	}

	It("starts on the current format and can show the older one or all", func() {
		tt := latestFirst(withFormats())
		Expect(tt.HasText("All of New format")).To(BeTrue())
		Expect(tt.Texts()).To(ContainElements("New format", "4 games"))
		Expect(tt.Click("New format")).To(Succeed())
		Expect(guitest.Choose(tt, "Old format · since Nov 4")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("6 games"))
		Expect(tt.Click("Old format")).To(Succeed())
		Expect(guitest.Choose(tt, "All formats")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("10 games"))
	})

	It("counts ranked games only in a format, with the type fixed to ranked", func() {
		s := withFormats()
		s.Matches[0].Ranked = false
		_, tt := tester(s)
		Expect(tt.Click("All games")).To(Succeed())
		Expect(guitest.Choose(tt, "Casual")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("1 game"))
		Expect(tt.Click("All formats")).To(Succeed())
		Expect(guitest.Choose(tt, "New format · since Nov 6")).To(Succeed())
		Expect(tt.Texts()).To(ContainElements("Ranked", "3 games"))
		Expect(tt.Texts()).NotTo(ContainElement("Casual"))
	})
})
