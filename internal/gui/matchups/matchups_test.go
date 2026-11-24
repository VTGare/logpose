package matchups

import (
	"fmt"

	"github.com/VTGare/logpose/internal/gui/guitest"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/egoist/mygo/ui"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// matchupsSnapshot is 9 games against Kaido, 4–5, and the newest one a win against Buggy.
func matchupsSnapshot() *page.Snapshot {
	s := guitest.Snapshot(10)
	s.Matches[0].Opp.Leader = "OP09-042"
	s.Cards.Name("OP09-042", "Buggy")
	return s
}

var _ = Describe("Matchups page", func() {
	It("lists every opposing leader with its set and opens the most played one", func() {
		_, tt := matchupsTester(matchupsSnapshot())
		for _, text := range []string{
			"2 leaders faced", "Kaido", "OP17", "44%", "4–5", "Buggy", "OP09", "1–0",
			"Turn order", "How the games went", "Latest games", "Show in Matches",
		} {
			Expect(tt.HasText(text)).To(BeTrue(), fmt.Sprint(text, tt.Texts()))
		}
	})

	It("opens the row picked, and closes it when picked again", func() {
		_, tt := matchupsTester(matchupsSnapshot())
		Expect(tt.Texts()).To(ContainElement("Filter to games vs Kaido"))
		Expect(tt.Click("Show the matchup vs Buggy OP09")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("Filter to games vs Buggy"))
		Expect(tt.Texts()).NotTo(ContainElement("Filter to games vs Kaido"))
		Expect(tt.Click("Show the matchup vs Buggy OP09")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Latest games")).To(BeFalse())
	})

	It("counts only the games in the filters' scope", func() {
		s := matchupsSnapshot()
		for i := range 4 {
			s.Matches[i].Ranked = false
		}
		_, tt := matchupsTester(s)
		Expect(tt.Click("All games")).To(Succeed())
		Expect(guitest.Choose(tt, "Casual")).To(Succeed())
		Expect(tt.Texts()).To(ContainElements("2 leaders faced", "Kaido", "1–2", "Buggy", "1–0"))
		Expect(tt.Texts()).NotTo(ContainElement("4–5"))
	})

	It("lists the matchups of the deck picked at the top", func() {
		s := matchupsSnapshot()
		s.Matches[5].Me.Leader = "EB05-010"
		s.Cards.Name("EB05-010", "Nico Robin")
		_, tt := matchupsTester(s)
		Expect(tt.Click("Show matchups with Nico Robin")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("1 leader faced")).To(BeTrue())
	})
})

// view is the Matchups page in a host, under the filter bar.
type view struct {
	*guitest.Host
	pg *Page
}

func (v *view) setPrefs(p page.Prefs) { v.SetPrefs(p) }

func (v *view) setActions(f *guitest.EnvActions) { v.Act = f }

func matchupsTester(s *page.Snapshot) (*view, *ui.Tester) {
	pg := &Page{}
	h, tt := guitest.WithBar(s, pg.Draw)
	h.Page = page.Matchups
	tt.Frame()
	return &view{h, pg}, tt
}

func tableTester() (*view, *ui.Tester, *guitest.EnvActions) {
	v, tt := matchupsTester(matchupsSnapshot())
	act := &guitest.EnvActions{}
	v.setActions(act)
	return v, tt, act
}
