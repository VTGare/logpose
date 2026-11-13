package formats

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

// fakeActions records the formats the page asks for.
type fakeActions struct {
	added     []store.Format
	dismissed []string
}

func (f *fakeActions) AddFormat(name string, start time.Time) {
	f.added = append(f.added, store.Format{Name: name, Start: start})
}
func (*fakeActions) UpdateFormat(store.Format) {}
func (*fakeActions) DeleteFormat(int64)        {}
func (f *fakeActions) DismissSet(set string)   { f.dismissed = append(f.dismissed, set) }

// tester shows the Formats page, or with banner the banner it puts over the Overview.
func tester(s *page.Snapshot, banner bool) (*fakeActions, *ui.Tester) {
	pg, fa := New(), &fakeActions{}
	pg.Act = fa
	draw := pg.Draw
	if banner {
		draw = pg.DebutBanner
	}
	_, tt := guitest.New(s, draw)
	tt.Frame()
	return fa, tt
}

var _ = Describe("formats", func() {
	// withFormats splits the 10 test games, an hour apart, at 4 hours ago.
	withFormats := func() *page.Snapshot {
		s := guitest.Snapshot(10)
		s.Formats = stats.Ranges([]store.Format{
			{ID: 1, Name: "Old format", Start: guitest.Now.Add(-48 * time.Hour)},
			{ID: 2, Name: "New format", Start: guitest.Now.Add(-4*time.Hour - 30*time.Minute)},
		})
		return s
	}

	It("lists formats with their games, and the games before the first", func() {
		_, tt := tester(withFormats(), false)
		Expect(tt.HasText("New format")).To(BeTrue())
		Expect(tt.HasText("4 games ranked · 2–2")).To(BeTrue(), "%v", tt.Texts())
	})

	It("asks for a name before adding a format", func() {
		fa, tt := tester(withFormats(), false)
		Expect(tt.Click("Add")).To(Succeed())
		Expect(tt.HasText("Give the format a name.")).To(BeTrue())
		Expect(fa.added).To(BeEmpty())
		Expect(tt.Click("Format name")).To(Succeed())
		tt.Type(" OP18 ")
		Expect(tt.Click("Add")).To(Succeed())
		Expect(fa.added).To(HaveLen(1))
		Expect(fa.added[0].Name).To(Equal("OP18"))
	})

	It("offers a format where a new set showed up, on the overview", func() {
		s := guitest.Snapshot(10)
		s.Debuts = []stats.SetDebut{{Set: "OP18", First: time.Date(2026, 11, 2, 18, 30, 0, 0, time.Local)}}
		fa, tt := tester(s, true)
		Expect(tt.HasText("OP18 cards first showed up in your games on Nov 2")).To(BeTrue())
		Expect(tt.Click("Start a format on Nov 2")).To(Succeed())
		Expect(fa.added).To(Equal([]store.Format{{Name: "OP18", Start: time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local)}}))
		Expect(tt.Click("Dismiss")).To(Succeed())
		Expect(fa.dismissed).To(Equal([]string{"OP18"}))
	})
})
