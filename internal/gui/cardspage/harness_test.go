package cardspage

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/gui/guitest"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/sim"
)

// fakeActions records what the page asks the app for.
type fakeActions struct {
	guitest.EnvActions
	// artLoads holds each LoadArts as "<code> <gen>". Showing a card calls it on its own.
	artLoads []string
}

func (f *fakeActions) call(what string) { f.Calls = append(f.Calls, what) }

func (f *fakeActions) RefreshCards()              { f.call("refresh cards") }
func (f *fakeActions) ChooseSimFolder()           { f.call("choose sim") }
func (f *fakeActions) LoadSimArt(card cards.Card) { f.call("load art " + card.Code) }
func (f *fakeActions) LoadArts(card cards.Card, gen int) {
	f.artLoads = append(f.artLoads, fmt.Sprint(card.Code, " ", gen))
}
func (f *fakeActions) ScanSimArt() { f.call("scan art") }
func (f *fakeActions) UseSimArt(card cards.Card, choice, _ sim.ArtChoice) {
	f.call("use art " + card.Code + " " + choice.Key)
}
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
