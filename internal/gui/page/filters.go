package page

import (
	"fmt"
	"time"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// filters is what the filter bar sets. It stays set while moving between pages.
type filters struct {
	// Format is formatCurrent, formatAll or a Format's ID.
	Format int64
	// Period indexes periods, or is periodCustom for the days from and to.
	Period int
	// from and to are the first and last day of a custom period, at local midnight. An ongoing
	// period ends today, whatever to says.
	from, to time.Time
	ongoing  bool
	// Deck is the local player's leader, or "" for every Deck.
	Deck string
	// gameType is a stats.GameType, an int for ui.Segmented.
	gameType int
	Turn     stats.TurnOrderFilter
	// Opp is an opponent's player ID and OppLeader their leader, or "" for any. The Matches
	// page's row menu sets them.
	Opp, OppLeader string
}

var periods = []struct {
	label string
	days  int
}{
	{"All time", 0},
	{"Last 7 days", 7},
	{"Last 30 days", 30},
	{"Last 90 days", 90},
}

var gameTypeLabels = []string{"All games", "Ranked", "Casual"}

// turnLabels are indexed by stats.TurnOrderFilter.
var turnLabels = []string{"Either turn", "Going first", "Going second"}

const (
	// FormatCurrent is the latest format, whichever that is when the user adds one.
	FormatCurrent int64 = 0
	FormatAll     int64 = -1
)

// startUnpicked is the format pages start on until the user picks one under "Start on".
const startUnpicked = FormatAll

// cleared is where Clear puts the filters: every one off and the format at the default.
func (e *Env) cleared() filters { return filters{Format: e.Prefs.StartFormat} }

// filtersActive reports whether the filters differ from where Clear puts them. Two format
// values that pick the same games count as the same.
func (e *Env) filtersActive() bool {
	f := e.Filter
	f.Format = e.Prefs.StartFormat
	return f != e.cleared() || !e.atStartFormat()
}

// canClear reports whether Clear has anything to do, the Matches page's result switch included.
func (e *Env) canClear() bool {
	return e.filtersActive() || e.Page == Matches && e.ListResult != AnyResult
}

func (e *Env) atStartFormat() bool {
	a, aok := e.rangeOf(e.Filter.Format)
	b, bok := e.rangeOf(e.Prefs.StartFormat)
	return aok == bok && a.ID == b.ID
}

// FormatRange is the format the filter picks. There's none with no formats or all of them.
func (e *Env) FormatRange() (stats.FormatRange, bool) { return e.rangeOf(e.Filter.Format) }

func (e *Env) rangeOf(format int64) (stats.FormatRange, bool) {
	fs := e.Snap.Formats
	switch {
	case len(fs) == 0 || format == FormatAll:
		return stats.FormatRange{}, false
	case format == FormatCurrent:
		return fs[len(fs)-1], true
	}
	for _, f := range fs {
		if f.ID == format {
			return f, true
		}
	}
	return stats.FormatRange{}, false
}

// startFormatName names the default format: the latest one's name or All formats.
func (e *Env) startFormatName() string {
	if fr, ok := e.rangeOf(e.Prefs.StartFormat); ok {
		return fr.Name
	}
	return "All formats"
}

// SetStartFormat saves the format pages start on. It leaves the format picked now alone.
func (e *Env) SetStartFormat(format int64) {
	prefStartFormat.Set(e, format)
}

// startOnMenu is the two defaults as radio items.
func (e *Env) startOnMenu(m *ui.Menu) {
	fs := e.Snap.Formats
	latest := "Latest format, now " + fs[len(fs)-1].Name
	if m.Item(latest).Radio(e.Prefs.StartFormat == FormatCurrent).Chosen() {
		e.SetStartFormat(FormatCurrent)
	}
	if m.Item("All formats").Radio(e.Prefs.StartFormat == FormatAll).Chosen() {
		e.SetStartFormat(FormatAll)
	}
}

// currentScope is the filters as a stats.Scope. A format holds ranked games only, since casual
// lobbies can be any format. A period and a format overlap to the days in both.
func (e *Env) currentScope() stats.Scope {
	s := e.scopeAnyPeriod()
	since, until := e.periodBounds()
	if since.After(s.Since) {
		s.Since = since
	}
	if !until.IsZero() && (s.Until.IsZero() || until.Before(s.Until)) {
		s.Until = until
	}
	return s
}

func (e *Env) scopeAnyPeriod() stats.Scope {
	f := e.Filter
	s := stats.Scope{Type: stats.GameType(f.gameType), Turn: f.Turn, Deck: f.Deck, Opp: f.Opp, OppLeader: f.OppLeader}
	if fr, ok := e.FormatRange(); ok {
		fs := fr.Scope()
		s.Type, s.Since, s.Until = fs.Type, fs.Since, fs.Until
	}
	return s
}

func (e *Env) formatLabel() string {
	if fr, ok := e.FormatRange(); ok {
		return fr.Name
	}
	return "All formats"
}

func (e *Env) formatMenu(m *ui.Menu) {
	fs := e.Snap.Formats
	for i := len(fs) - 1; i >= 0; i-- {
		f := fs[i]
		on := e.Filter.Format == f.ID || e.Filter.Format == FormatCurrent && i == len(fs)-1
		label := fmt.Sprintf("%s · since %s", f.Name, e.Date(f.Start))
		if m.Item(label).Radio(on).Chosen() {
			e.Filter.Format = f.ID
			if i == len(fs)-1 {
				e.Filter.Format = FormatCurrent
			}
		}
	}
	if m.Item("All formats").Radio(len(fs) == 0 || e.Filter.Format == FormatAll).Chosen() && len(fs) > 0 {
		e.Filter.Format = FormatAll
	}
	m.Separator()
	if len(fs) > 0 {
		m.Submenu("Start on", e.startOnMenu)
	}
	if m.Item("Manage formats").Chosen() {
		e.Page = Formats
	}
}

// scoped is the snapshot's matches in the filters' scope, with what the pages build from them.
// It's built again only when the snapshot, the filters or the day change, and gen counts the
// builds for the caches made from it.
type scoped struct {
	Snap    *Snapshot
	filters filters
	day     time.Time
	Gen     int

	Matches  []stats.Match
	Overview stats.Overview
	// AnyDeck is the scope without the deck, for choosing one, and AnyTurn the scope without
	// the deck or the turn order, for the turn order tile.
	AnyDeck, AnyTurn []stats.Match
	Decks            []stats.LeaderRecord
}

func (e *Env) Scoped() *scoped {
	sc := &e.scope
	now := e.Now()
	day := stats.DayStart(now)
	if sc.Snap == e.Snap && sc.filters == e.Filter && sc.day.Equal(day) {
		return sc
	}
	*sc = scoped{Snap: e.Snap, filters: e.Filter, day: day, Gen: sc.Gen + 1}
	s := e.currentScope()
	sc.Matches = s.Apply(e.Snap.Matches)
	sc.Overview = stats.BuildOverview(sc.Matches, now)
	s.Deck = ""
	sc.AnyDeck = s.Apply(e.Snap.Matches)
	sc.Decks = stats.Leaders(sc.AnyDeck)
	s.Turn = stats.EitherTurn
	sc.AnyTurn = s.Apply(e.Snap.Matches)
	return sc
}

func (e *Env) deckLabel() string {
	if e.Filter.Deck == "" {
		return "All decks"
	}
	return e.Snap.Card(e.Filter.Deck).Name
}

func (e *Env) deckMenu(m *ui.Menu) {
	if m.Item("All decks").Radio(e.Filter.Deck == "").Chosen() {
		e.SetDeck("")
	}
	m.Separator()
	for _, lr := range e.Scoped().Decks {
		label := fmt.Sprintf("%s · %s", e.Snap.Card(lr.Leader).Name, kit.Games(lr.Record.Games()))
		if m.Item(label).Radio(e.Filter.Deck == lr.Leader).Chosen() {
			e.SetDeck(lr.Leader)
		}
	}
}

func (e *Env) SetDeck(code string) {
	e.Filter.Deck = code
}

func (e *Env) clearFilters() {
	e.Filter = e.cleared()
	e.ListResult = AnyResult
}

// SetTurn sets the turn order filter, or clears it when it's already set to t.
func (e *Env) SetTurn(t stats.TurnOrderFilter) {
	if e.Filter.Turn == t {
		t = stats.EitherTurn
	}
	e.Filter.Turn = t
}

func (e *Env) turnMenu(m *ui.Menu) {
	for i, l := range turnLabels {
		if m.Item(l).Radio(int(e.Filter.Turn) == i).Chosen() {
			e.Filter.Turn = stats.TurnOrderFilter(i)
		}
	}
}

// removablePill is a set filter shown as a tinted pill that clears it when clicked.
func removablePill(c *ui.Context, key, label string, clear func()) {
	t := c.Theme()
	el := kit.Tinted(ui.ButtonBase(c.Key(key)).Label("Remove the filter "+label).Padding(6, 12).Radius(16), t)
	el.Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(13)
		ui.Icon(c, kit.IconClose).FontSize(13)
	})
	if el.Clicked() {
		clear()
	}
}

func (e *Env) oppName(id string) string {
	for _, m := range e.Snap.Matches {
		if m.Opp.ID == id {
			return m.Opp.Name
		}
	}
	return id
}

// FilterPill is a filter's button. A pill that narrows what the pages show is tinted.
func FilterPill(c *ui.Context, label string, on bool) ui.Element {
	t := c.Theme()
	el := ui.ButtonBase(c).Label(label).Padding(5, 12).Radius(16).Border(1, t.Border).Background(ui.Transparent)
	if on {
		el = kit.Tinted(el, t)
	}
	return el.Gap(6).AlignItems(ui.Center).Children(func() {
		col := t.TextMuted
		if on {
			col = t.Text
		}
		ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(col)
		ui.Icon(c, kit.IconChevron).FontSize(12).TextColor(col)
	})
}

// FilterBar is the filters over every page, as pills that each open a menu. Pills that narrow
// what the pages show are tinted.
func (e *Env) FilterBar(c *ui.Context) {
	t := c.Theme()
	pill := func(key, label string, on bool, build func(m *ui.Menu), clear func(*ui.Menu)) {
		pillMenu(c.Key(key), func() { FilterPill(c, label, on).PopoverMenuButton(build) }, clear)
	}
	ui.Row(c).Padding(14, 20, 0, 20).Gap(8).Children(func() {
		_, inFormat := e.FormatRange()
		pill("format", e.formatLabel(), inFormat, e.formatMenu,
			e.clearPillMenu("Back to "+e.startFormatName(), e.atStartFormat(), func() { e.Filter.Format = e.Prefs.StartFormat }))
		pillMenu(c.Key("period"), func() { e.periodPill(c) },
			e.clearPillMenu("Clear period", e.Filter.Period == 0, func() {
				e.Filter.Period, e.Filter.from, e.Filter.to, e.Filter.ongoing = 0, time.Time{}, time.Time{}, false
			}))
		pill("deck", e.deckLabel(), e.Filter.Deck != "", e.deckMenu,
			e.clearPillMenu("Clear deck", e.Filter.Deck == "", func() { e.Filter.Deck = "" }))
		if inFormat {
			ui.Text(c, "Ranked").FontSize(13).TextColor(t.TextMuted).Padding(6, 12).Radius(16).Border(1, t.Border)
		} else {
			pill("type", gameTypeLabels[e.Filter.gameType], e.Filter.gameType != 0, func(m *ui.Menu) {
				for i, l := range gameTypeLabels {
					if m.Item(l).Radio(e.Filter.gameType == i).Chosen() {
						e.Filter.gameType = i
					}
				}
			}, e.clearPillMenu("Clear game type", e.Filter.gameType == 0, func() { e.Filter.gameType = 0 }))
		}
		pill("turn", turnLabels[e.Filter.Turn], e.Filter.Turn != stats.EitherTurn, e.turnMenu,
			e.clearPillMenu("Clear turn order", e.Filter.Turn == stats.EitherTurn, func() { e.Filter.Turn = stats.EitherTurn }))
		// The opponent filters have no menu of their own, so they show only while set.
		if e.Filter.Opp != "" {
			removablePill(c, "opp", "vs "+e.oppName(e.Filter.Opp), func() { e.Filter.Opp = "" })
		}
		if e.Filter.OppLeader != "" {
			removablePill(c, "oppLeader", "vs "+e.Snap.Card(e.Filter.OppLeader).Name, func() { e.Filter.OppLeader = "" })
		}
		if e.canClear() && ui.Text(c, "Clear").FontSize(13).TextColor(t.Accent).Padding(6, 4).Clicked() {
			e.clearFilters()
		}
		ui.Spacer(c)
		ui.Text(c, kit.Games(len(e.Scoped().Matches))).FontSize(13).TextColor(kit.FaintColor(t))
	})
}

const AnyResult = 0
