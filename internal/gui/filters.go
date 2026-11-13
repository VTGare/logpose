package gui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// filters is what the filter bar sets. It stays set while moving between pages.
type filters struct {
	// format is formatCurrent, formatAll or a format's ID.
	format int64
	// period indexes periods, or is periodCustom for the days from and to.
	period int
	// from and to are the first and last day of a custom period, at local midnight. An ongoing
	// period ends today, whatever to says.
	from, to time.Time
	ongoing  bool
	// deck is the local player's leader, or "" for every deck.
	deck string
	// gameType is a stats.GameType, an int for ui.Segmented.
	gameType int
	turn     stats.TurnOrderFilter
	// opp is an opponent's player ID and oppLeader their leader, or "" for any. The Matches
	// page's row menu sets them.
	opp, oppLeader string
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
	// formatCurrent is the latest format, whichever that is when the user adds one.
	formatCurrent int64 = 0
	formatAll     int64 = -1
)

// startUnpicked is the format pages start on until the user picks one under "Start on".
const startUnpicked = formatAll

// cleared is where Clear puts the filters: every one off and the format at the default.
func (e *env) cleared() filters { return filters{format: e.prefs.startFormat} }

// filtersActive reports whether the filters differ from where Clear puts them. Two format
// values that pick the same games count as the same.
func (e *env) filtersActive() bool {
	f := e.filter
	f.format = e.prefs.startFormat
	return f != e.cleared() || !e.atStartFormat()
}

// canClear reports whether Clear has anything to do, the Matches page's result switch included.
func (e *env) canClear() bool {
	return e.filtersActive() || e.page == pageMatches && e.listResult != anyResult
}

func (e *env) atStartFormat() bool {
	a, aok := e.rangeOf(e.filter.format)
	b, bok := e.rangeOf(e.prefs.startFormat)
	return aok == bok && a.ID == b.ID
}

// formatRange is the format the filter picks. There's none with no formats or all of them.
func (e *env) formatRange() (stats.FormatRange, bool) { return e.rangeOf(e.filter.format) }

func (e *env) rangeOf(format int64) (stats.FormatRange, bool) {
	fs := e.snap.Formats
	switch {
	case len(fs) == 0 || format == formatAll:
		return stats.FormatRange{}, false
	case format == formatCurrent:
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
func (e *env) startFormatName() string {
	if fr, ok := e.rangeOf(e.prefs.startFormat); ok {
		return fr.Name
	}
	return "All formats"
}

// setStartFormat saves the format pages start on. It leaves the format picked now alone.
func (e *env) setStartFormat(format int64) {
	prefStartFormat.set(e, format)
}

// startOnMenu is the two defaults as radio items.
func (e *env) startOnMenu(m *ui.Menu) {
	fs := e.snap.Formats
	latest := "Latest format, now " + fs[len(fs)-1].Name
	if m.Item(latest).Radio(e.prefs.startFormat == formatCurrent).Chosen() {
		e.setStartFormat(formatCurrent)
	}
	if m.Item("All formats").Radio(e.prefs.startFormat == formatAll).Chosen() {
		e.setStartFormat(formatAll)
	}
}

// currentScope is the filters as a stats.Scope. A format holds ranked games only, since casual
// lobbies can be any format. A period and a format overlap to the days in both.
func (e *env) currentScope() stats.Scope {
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

func (e *env) scopeAnyPeriod() stats.Scope {
	f := e.filter
	s := stats.Scope{Type: stats.GameType(f.gameType), Turn: f.turn, Deck: f.deck, Opp: f.opp, OppLeader: f.oppLeader}
	if fr, ok := e.formatRange(); ok {
		fs := fr.Scope()
		s.Type, s.Since, s.Until = fs.Type, fs.Since, fs.Until
	}
	return s
}

func (e *env) formatLabel() string {
	if fr, ok := e.formatRange(); ok {
		return fr.Name
	}
	return "All formats"
}

func (e *env) formatMenu(m *ui.Menu) {
	fs := e.snap.Formats
	for i := len(fs) - 1; i >= 0; i-- {
		f := fs[i]
		on := e.filter.format == f.ID || e.filter.format == formatCurrent && i == len(fs)-1
		label := fmt.Sprintf("%s · since %s", f.Name, e.date(f.Start))
		if m.Item(label).Radio(on).Chosen() {
			e.filter.format = f.ID
			if i == len(fs)-1 {
				e.filter.format = formatCurrent
			}
		}
	}
	if m.Item("All formats").Radio(len(fs) == 0 || e.filter.format == formatAll).Chosen() && len(fs) > 0 {
		e.filter.format = formatAll
	}
	m.Separator()
	if len(fs) > 0 {
		m.Submenu("Start on", e.startOnMenu)
	}
	if m.Item("Manage formats").Chosen() {
		e.page = pageFormats
	}
}

// scoped is the snapshot's matches in the filters' scope, with what the pages build from them.
// It's built again only when the snapshot, the filters or the day change, and gen counts the
// builds for the caches made from it.
type scoped struct {
	snap    *snapshot
	filters filters
	day     time.Time
	gen     int

	matches  []stats.Match
	overview stats.Overview
	// anyDeck is the scope without the deck, for choosing one, and anyTurn the scope without
	// the deck or the turn order, for the turn order tile.
	anyDeck, anyTurn []stats.Match
	decks            []stats.LeaderRecord
}

func (e *env) scoped() *scoped {
	sc := &e.scope
	now := e.now()
	day := stats.DayStart(now)
	if sc.snap == e.snap && sc.filters == e.filter && sc.day.Equal(day) {
		return sc
	}
	*sc = scoped{snap: e.snap, filters: e.filter, day: day, gen: sc.gen + 1}
	s := e.currentScope()
	sc.matches = s.Apply(e.snap.Matches)
	sc.overview = stats.BuildOverview(sc.matches, now)
	s.Deck = ""
	sc.anyDeck = s.Apply(e.snap.Matches)
	sc.decks = stats.Leaders(sc.anyDeck)
	s.Turn = stats.EitherTurn
	sc.anyTurn = s.Apply(e.snap.Matches)
	return sc
}

func (e *env) deckLabel() string {
	if e.filter.deck == "" {
		return "All decks"
	}
	return e.snap.card(e.filter.deck).Name
}

func (e *env) deckMenu(m *ui.Menu) {
	if m.Item("All decks").Radio(e.filter.deck == "").Chosen() {
		e.setDeck("")
	}
	m.Separator()
	for _, lr := range e.scoped().decks {
		label := fmt.Sprintf("%s · %s", e.snap.card(lr.Leader).Name, games(lr.Record.Games()))
		if m.Item(label).Radio(e.filter.deck == lr.Leader).Chosen() {
			e.setDeck(lr.Leader)
		}
	}
}

func (e *env) setDeck(code string) {
	e.filter.deck = code
}

func (e *env) clearFilters() {
	e.filter = e.cleared()
	e.listResult = anyResult
}

// setTurn sets the turn order filter, or clears it when it's already set to t.
func (e *env) setTurn(t stats.TurnOrderFilter) {
	if e.filter.turn == t {
		t = stats.EitherTurn
	}
	e.filter.turn = t
}

func (e *env) turnMenu(m *ui.Menu) {
	for i, l := range turnLabels {
		if m.Item(l).Radio(int(e.filter.turn) == i).Chosen() {
			e.filter.turn = stats.TurnOrderFilter(i)
		}
	}
}

// removablePill is a set filter shown as a tinted pill that clears it when clicked.
func removablePill(c *ui.Context, key, label string, clear func()) {
	t := c.Theme()
	el := tinted(ui.ButtonBase(c.Key(key)).Label("Remove the filter "+label).Padding(6, 12).Radius(16), t)
	el.Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(13)
		ui.Icon(c, iconClose).FontSize(13)
	})
	if el.Clicked() {
		clear()
	}
}

func (e *env) oppName(id string) string {
	for _, m := range e.snap.Matches {
		if m.Opp.ID == id {
			return m.Opp.Name
		}
	}
	return id
}

// filterPill is a filter's button. A pill that narrows what the pages show is tinted.
func filterPill(c *ui.Context, label string, on bool) ui.Element {
	t := c.Theme()
	el := ui.ButtonBase(c).Label(label).Padding(5, 12).Radius(16).Border(1, t.Border).Background(ui.Transparent)
	if on {
		el = tinted(el, t)
	}
	return el.Gap(6).AlignItems(ui.Center).Children(func() {
		col := t.TextMuted
		if on {
			col = t.Text
		}
		ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(col)
		ui.Icon(c, iconChevron).FontSize(12).TextColor(col)
	})
}

// filterBar is the filters over every page, as pills that each open a menu. Pills that narrow
// what the pages show are tinted.
func (e *env) filterBar(c *ui.Context) {
	t := c.Theme()
	pill := func(key, label string, on bool, build func(m *ui.Menu), clear func(*ui.Menu)) {
		pillMenu(c.Key(key), func() { filterPill(c, label, on).PopoverMenuButton(build) }, clear)
	}
	ui.Row(c).Padding(14, 20, 0, 20).Gap(8).Children(func() {
		_, inFormat := e.formatRange()
		pill("format", e.formatLabel(), inFormat, e.formatMenu,
			e.clearPillMenu("Back to "+e.startFormatName(), e.atStartFormat(), func() { e.filter.format = e.prefs.startFormat }))
		pillMenu(c.Key("period"), func() { e.periodPill(c) },
			e.clearPillMenu("Clear period", e.filter.period == 0, func() {
				e.filter.period, e.filter.from, e.filter.to, e.filter.ongoing = 0, time.Time{}, time.Time{}, false
			}))
		pill("deck", e.deckLabel(), e.filter.deck != "", e.deckMenu,
			e.clearPillMenu("Clear deck", e.filter.deck == "", func() { e.filter.deck = "" }))
		if inFormat {
			ui.Text(c, "Ranked").FontSize(13).TextColor(t.TextMuted).Padding(6, 12).Radius(16).Border(1, t.Border)
		} else {
			pill("type", gameTypeLabels[e.filter.gameType], e.filter.gameType != 0, func(m *ui.Menu) {
				for i, l := range gameTypeLabels {
					if m.Item(l).Radio(e.filter.gameType == i).Chosen() {
						e.filter.gameType = i
					}
				}
			}, e.clearPillMenu("Clear game type", e.filter.gameType == 0, func() { e.filter.gameType = 0 }))
		}
		pill("turn", turnLabels[e.filter.turn], e.filter.turn != stats.EitherTurn, e.turnMenu,
			e.clearPillMenu("Clear turn order", e.filter.turn == stats.EitherTurn, func() { e.filter.turn = stats.EitherTurn }))
		// The opponent filters have no menu of their own, so they show only while set.
		if e.filter.opp != "" {
			removablePill(c, "opp", "vs "+e.oppName(e.filter.opp), func() { e.filter.opp = "" })
		}
		if e.filter.oppLeader != "" {
			removablePill(c, "oppLeader", "vs "+e.snap.card(e.filter.oppLeader).Name, func() { e.filter.oppLeader = "" })
		}
		if e.canClear() && ui.Text(c, "Clear").FontSize(13).TextColor(t.Accent).Padding(6, 4).Clicked() {
			e.clearFilters()
		}
		ui.Spacer(c)
		ui.Text(c, games(len(e.scoped().matches))).FontSize(13).TextColor(faintColor(t))
	})
}
