package decks

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/gui/guitest"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/store"
)

// fakeActions records what the page asks the app for.
type fakeActions struct {
	guitest.EnvActions
	builds     []store.Build
	deleted    []int64
	dismissals []store.Dismissal
}

// SaveBuild gives a new build the ID 100 and up.
func (f *fakeActions) SaveBuild(b store.Build, saved func(id int64)) {
	f.builds = append(f.builds, b)
	if b.ID == 0 {
		b.ID = int64(99 + len(f.builds))
	}
	if saved != nil {
		saved(b.ID)
	}
}

func (f *fakeActions) DeleteBuild(id int64) { f.deleted = append(f.deleted, id) }

func (f *fakeActions) DismissSuggestion(d store.Dismissal) { f.dismissals = append(f.dismissals, d) }

// view is the Decks page in a host, under the filter bar.
type view struct {
	*guitest.Host
	decks Page
}

func tester(s *page.Snapshot) (*view, *ui.Tester) {
	v := &view{Host: &guitest.Host{Bar: func() bool { return true }}}
	v.HandsShown, v.HandsOpener = v.decks.ShowsHands, v.decks.ShowHands
	_, tt := guitest.Start(v.Host, s, v.decks.Draw)
	v.Page = page.Decks
	return v, tt
}

func (v *view) setSnapshot(s *page.Snapshot) { v.Snap = s }
func (v *view) setPrefs(p page.Prefs)        { v.SetPrefs(p) }
func (v *view) setActions(f *fakeActions)    { v.Act, v.decks.Act = f, f }

// open shows what -page decks/<sub> opens.
func (v *view) open(id string) {
	_, sub, _ := strings.Cut(id, "/")
	v.decks.Open(&v.Env, sub)
}

// openDecks opens a Decks tab the way -page does, since the sidebar's Cards hides the tab's.
func openDecks(v *view, tt *ui.Tester, tab string) {
	v.open("decks/" + tab)
	tt.Frame()
}
