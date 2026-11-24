package cardspage

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/gui/guitest"
	"github.com/VTGare/logpose/internal/gui/page"
)

// fakeActions records what the page asks the app for.
type fakeActions struct {
	guitest.EnvActions
}

func (f *fakeActions) call(what string) { f.Calls = append(f.Calls, what) }

func (f *fakeActions) RefreshCards()      { f.call("refresh cards") }
func (f *fakeActions) SaveArt(key string) { f.call("save art " + key) }
func (f *fakeActions) CopyArt(key string) { f.call("copy art " + key) }
func (f *fakeActions) OpenURL(url string) { f.call("open url " + url) }

// view is the Cards page in a host. Off the Cards page it shows only the art viewer, which the
// window draws over any page.
type view struct {
	*guitest.Host
	cards Page
}

func tester(s *page.Snapshot) (*view, *ui.Tester) {
	v := &view{Host: &guitest.Host{}}
	v.CardsOpener = func(code string) {
		v.cards.Select(code, false)
		v.Page = page.Cards
	}
	v.ArtViewer = func(code string) { v.cards.Select(code, true) }
	v.ArtViewerAt = func(code, key string) { v.cards.SelectKey(code, key, true) }
	v.CardMenus = func(card cards.Card) func(*ui.Menu) { return v.cards.CardMenu(&v.Env, card) }
	_, tt := guitest.Start(v.Host, s, func(c *ui.Context, e *page.Env) {
		switch {
		case e.Page == page.Cards:
			v.cards.Draw(c, e)
		case v.cards.State.Viewer && e.Snap.Cards != nil:
			v.cards.ArtViewer(c, e, e.Snap.Cards.Card(v.cards.State.Selected))
		}
	})
	return v, tt
}

func (v *view) setPrefs(p page.Prefs)           { v.SetPrefs(p) }
func (v *view) setActions(f *fakeActions)       { v.Act, v.cards.Act = f, f }
func (v *view) addCardSync(p cards.SetProgress) { v.cards.CardSync.Add(p) }
