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

func (f *fakeActions) RefreshCards() { f.Calls = append(f.Calls, "refresh cards") }

// view is the Cards page in a host.
type view struct {
	*guitest.Host
	cards Page
}

func tester(s *page.Snapshot) (*view, *ui.Tester) {
	v := &view{Host: &guitest.Host{}}
	v.CardsOpener = func(code string) {
		v.cards.Select(code)
		v.Viewer.Close()
		v.Page = page.Cards
	}
	_, tt := guitest.Start(v.Host, s, func(c *ui.Context, e *page.Env) {
		if e.Page == page.Cards {
			v.cards.Draw(c, e)
		}
	})
	return v, tt
}

func (v *view) setPrefs(p page.Prefs)           { v.SetPrefs(p) }
func (v *view) setActions(f *fakeActions)       { v.Act, v.ArtAct, v.cards.Act = f, f, f }
func (v *view) addCardSync(p cards.SetProgress) { v.cards.CardSync.Add(p) }
