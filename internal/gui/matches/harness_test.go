package matches

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/gui/guitest"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

// fakeActions records what the page asks the app for.
type fakeActions struct {
	guitest.EnvActions
	loaded    []string
	types     []string
	notes     []string
	bookmarks []string
	replays   []string
	exports   []string
}

func (f *fakeActions) LoadTimeline(fingerprint, _, _ string) {
	f.loaded = append(f.loaded, fingerprint)
}

func (f *fakeActions) SetGameType(fingerprint string, t store.GameType) {
	f.types = append(f.types, fingerprint+"="+fmt.Sprint(t))
}

func (f *fakeActions) SetGameNote(fingerprint, note string) {
	f.notes = append(f.notes, fingerprint+"="+note)
}

func (f *fakeActions) SetBookmark(fingerprint string, on bool) {
	f.bookmarks = append(f.bookmarks, fmt.Sprintf("%s=%t", fingerprint, on))
}

func (f *fakeActions) OpenReplay(fingerprint string, _ *simlog.Game) {
	f.replays = append(f.replays, fingerprint)
}

func (f *fakeActions) ExportToSim(fingerprint, name string, _ *simlog.Game) {
	f.exports = append(f.exports, fingerprint+" "+name)
}

// view is the Matches page in a host, with what the window does around an open match: the
// match takes the page's place and the filter bar goes, a replay takes the whole window, and
// leaving the page closes the match.
type view struct {
	*guitest.Host
	matches Page
}

func tester(s *page.Snapshot) (*view, *ui.Tester) {
	v := &view{Host: &guitest.Host{}}
	open := func() bool { return v.matches.Detail.Fingerprint != "" }
	v.Bar = func() bool { return !open() }
	v.Full = func() bool { return v.matches.Detail.Replay != nil }
	v.Shown = func() string {
		if open() {
			return "match " + v.matches.Detail.Fingerprint
		}
		return v.Page
	}
	v.Before = func(c *ui.Context) {
		if open() && v.Page != v.matches.Detail.From {
			v.matches.CloseMatch()
		}
		v.matches.ReplayKeys(c, &v.Env)
	}
	v.MatchOpener = func(m stats.Match) { v.matches.OpenMatch(&v.Env, m) }
	v.GameMenus = func(m stats.Match) func(*ui.Menu) { return v.matches.GameMenu(&v.Env, m) }
	_, tt := guitest.Start(v.Host, s, func(c *ui.Context, e *page.Env) {
		if open() {
			v.matches.MatchPage(c, e)
			return
		}
		if e.Page == page.Matches {
			v.matches.Draw(c, e)
		}
	})
	return v, tt
}

func (v *view) setActions(f *fakeActions) { v.Act, v.matches.Act = f, f }
