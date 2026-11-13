package page

import (
	"time"

	"github.com/VTGare/logpose/internal/gui/kit"
	"github.com/VTGare/logpose/internal/store"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
)

// EnvActions are the changes the env asks the app for.
type EnvActions interface {
	SetSetting(key, value string)
	ApplyPrefs(p Prefs)
	// After runs fn on the main thread once d has passed.
	After(d time.Duration, fn func())
}

// Env is what the frame hands every page: the games, the prefs and the clock.
type Env struct {
	Snap  *Snapshot
	Prefs Prefs
	Act   EnvActions
	Clock func() time.Time
	// pending counts the calls to later by key, so only the last one runs.
	pending map[string]int
	// Entered is true in a page's first frame, when bars grow from zero and charts draw in.
	Entered bool
	Frame   int

	Art      *ArtCache
	Filter   filters
	scope    scoped
	periodUI periodState
	// Page is the id of the Page shown.
	Page string
	// ListResult indexes resultLabels. It narrows the Matches list only, since on the other pages
	// it would make every win rate 100% or 0%.
	ListResult int
	Nav        nav
	// Toast is a message for the next frame to show.
	Toast *ui.Toast
}

// nav is how the shared widgets reach what one page owns.
type nav interface {
	OpenMatch(m stats.Match)
	OpenInCards(code string)
	ViewArt(code string)
	GameMenu(m stats.Match) func(*ui.Menu)
	CardMenu(card cards.Card) func(*ui.Menu)
	ShowsHands() bool
	ShowHands(code string)
	// PageLabel is the name the sidebar gives the page with the id.
	PageLabel(id string) string
}

func (e *Env) Now() time.Time { return e.Clock() }

func (e *Env) OpenLeader(code string) {
	e.Page = Decks
	e.SetDeck(code)
}

// DragPane moves the width e holds by moved through snap, and saves it once the drag stops.
func (e *Env) DragPane(p prefEntry[float32], d *PaneDrag, moved float32, snap func(float32) float32) {
	width := p.field(&e.Prefs)
	if d.at == 0 {
		d.at = *width
	}
	d.at += moved
	*width = snap(d.at)
	e.later(p.key, func() {
		d.at = 0
		e.savePref(p)
	})
}

func (e *Env) Date(t time.Time) string { return kit.DateText(e.Now(), t) }

func (e *Env) Weekday(t time.Time) string { return kit.WeekdayText(e.Now(), t) }

// DateTime is the date and the time, such as "Oct 25 18:50".
func (e *Env) DateTime(t time.Time) string { return e.Date(t) + " " + t.Format("15:04") }

// Grow is a value that goes to target over d, from zero in a page's first frame, so bars Grow
// when a page opens and follow the filters after.
func (e *Env) Grow(el ui.Element, key any, target float32, d time.Duration) float32 {
	if e.Entered {
		el.Jump(key, 0)
	}
	return el.AnimateTo(key, target, ui.Animation{Duration: d})
}

// FoldTile is a fold tile kept collapsed across restarts by id.
func (e *Env) FoldTile(c *ui.Context, id string, h kit.FoldHead, body func()) ui.Element {
	folded := e.Prefs.folded[id]
	return kit.FoldTile(c, h, folded, func() { e.setFolded(id, !folded) }, body)
}

func (e *Env) setFolded(id string, folded bool) {
	if folded {
		e.Prefs.folded[id] = true
	} else {
		delete(e.Prefs.folded, id)
	}
	e.savePref(prefFolded)
}

func (e *Env) Layout(id string) *kit.TableLayout { return e.Prefs.tables[id] }

// SaveLayout stores every table's layout once the changes stop for a moment, since a
// resize changes it on each frame of the drag.
func (e *Env) SaveLayout(id string, lay *kit.TableLayout) {
	e.Prefs.tables[id] = lay
	e.later(KeyTables, func() { e.savePref(prefTables) })
}

// Snapshot is what the window shows. The engine builds a new one off the main thread, and the
// window swaps it in.
type Snapshot struct {
	Loaded   bool
	SimFound bool
	Matches  []stats.Match
	Overview stats.Overview
	Cards    *cards.DB
	// Formats are the user's formats, oldest first, and Debuts the sets that look like they
	// started one.
	Formats []stats.FormatRange
	Debuts  []stats.SetDebut
	// Stored is how many games the store holds, the local player's or not.
	Stored int
	// Decks are the matches with what the local player's deck did, oldest first, keyed by
	// fingerprint. Builds are the user's builds, and Dismissed the cores of suggestions they said
	// aren't builds, by leader.
	Decks     map[string]stats.DeckGame
	Builds    []store.Build
	Dismissed map[string][][]string
}

func (s *Snapshot) Card(code string) cards.Card {
	if s.Cards == nil {
		return cards.Card{Code: code, Name: code}
	}
	return s.Cards.Card(code)
}

const (
	Overview = "overview"
	Matches  = "matches"
	Decks    = "decks"
	Matchups = "matchups"
	Formats  = "formats"
	Settings = "settings"
	Cards    = "cards"
)

// PaneDrag follows a drag of a split's edge. The split moves the edge a few DIPs a frame from
// where it shows it, so the drag's own position is kept apart from the snapped width, or each
// small move would snap straight back.
type PaneDrag struct {
	at float32
}
