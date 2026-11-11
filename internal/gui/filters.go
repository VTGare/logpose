package gui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
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

// startFormatFrom reads store.SettingStartFormat.
func startFormatFrom(s string) int64 {
	switch s {
	case store.StartLatestFormat:
		return formatCurrent
	case store.StartAllFormats:
		return formatAll
	}
	return startUnpicked
}

// cleared is where Clear puts the filters: every one off and the format at the default.
func (v *view) cleared() filters { return filters{format: v.prefs.startFormat} }

// filtersActive reports whether the filters differ from where Clear puts them. Two format
// values that pick the same games count as the same.
func (v *view) filtersActive() bool {
	f := v.filter
	f.format = v.prefs.startFormat
	return f != v.cleared() || !v.atStartFormat()
}

// canClear reports whether Clear has anything to do, the Matches page's result switch included.
func (v *view) canClear() bool {
	return v.filtersActive() || v.page == pageMatches && v.matches.result != anyResult
}

func (v *view) atStartFormat() bool {
	a, aok := v.rangeOf(v.filter.format)
	b, bok := v.rangeOf(v.prefs.startFormat)
	return aok == bok && a.ID == b.ID
}

// formatRange is the format the filter picks. There's none with no formats or all of them.
func (v *view) formatRange() (stats.FormatRange, bool) { return v.rangeOf(v.filter.format) }

func (v *view) rangeOf(format int64) (stats.FormatRange, bool) {
	fs := v.snap.Formats
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
func (v *view) startFormatName() string {
	if fr, ok := v.rangeOf(v.prefs.startFormat); ok {
		return fr.Name
	}
	return "All formats"
}

// setStartFormat saves the format pages start on. It leaves the format picked now alone.
func (v *view) setStartFormat(format int64) {
	v.prefs.startFormat = format
	value := store.StartAllFormats
	if format == formatCurrent {
		value = store.StartLatestFormat
	}
	v.do(func(a actions) { a.SetSetting(store.SettingStartFormat, value) })
}

// startOnMenu is the two defaults as radio items.
func (v *view) startOnMenu(m *ui.Menu) {
	fs := v.snap.Formats
	latest := "Latest format, now " + fs[len(fs)-1].Name
	if m.Item(latest).Radio(v.prefs.startFormat == formatCurrent).Chosen() {
		v.setStartFormat(formatCurrent)
	}
	if m.Item("All formats").Radio(v.prefs.startFormat == formatAll).Chosen() {
		v.setStartFormat(formatAll)
	}
}

// currentScope is the filters as a stats.Scope. A format holds ranked games only, since casual
// lobbies can be any format. A period and a format overlap to the days in both.
func (v *view) currentScope() stats.Scope {
	s := v.scopeAnyPeriod()
	since, until := v.periodBounds()
	if since.After(s.Since) {
		s.Since = since
	}
	if !until.IsZero() && (s.Until.IsZero() || until.Before(s.Until)) {
		s.Until = until
	}
	return s
}

// scopeAnyPeriod is the filters as a stats.Scope, without the period.
func (v *view) scopeAnyPeriod() stats.Scope {
	f := v.filter
	s := stats.Scope{Type: stats.GameType(f.gameType), Turn: f.turn, Deck: f.deck, Opp: f.opp, OppLeader: f.oppLeader}
	if fr, ok := v.formatRange(); ok {
		s.Since, s.Until = fr.Start, fr.End
		s.Type = stats.RankedGames
	}
	return s
}

func (v *view) formatLabel() string {
	if fr, ok := v.formatRange(); ok {
		return fr.Name
	}
	return "All formats"
}

func (v *view) formatMenu(m *ui.Menu) {
	fs := v.snap.Formats
	for i := len(fs) - 1; i >= 0; i-- {
		f := fs[i]
		on := v.filter.format == f.ID || v.filter.format == formatCurrent && i == len(fs)-1
		label := fmt.Sprintf("%s · since %s", f.Name, f.Start.Format("Jan 2"))
		if m.Item(label).Radio(on).Chosen() {
			v.filter.format = f.ID
			if i == len(fs)-1 {
				v.filter.format = formatCurrent
			}
			v.resetPoints()
		}
	}
	if m.Item("All formats").Radio(len(fs) == 0 || v.filter.format == formatAll).Chosen() && len(fs) > 0 {
		v.filter.format = formatAll
		v.resetPoints()
	}
	m.Separator()
	if len(fs) > 0 {
		m.Submenu("Start on", v.startOnMenu)
	}
	if m.Item("Manage formats").Chosen() {
		v.page = pageFormats
	}
}

// scoped is the snapshot's matches in the filters' scope, with what the pages build from them.
// It's built again only when the snapshot or the filters change.
type scoped struct {
	snap    *snapshot
	filters filters

	matches  []stats.Match
	overview stats.Overview
	// anyDeck is the scope without the deck, for choosing one, and anyTurn the scope without
	// the deck or the turn order, for the turn order tile.
	anyDeck, anyTurn []stats.Match
	decks            []stats.LeaderRecord
}

func (v *view) scoped() *scoped {
	sc := &v.scope
	if sc.snap == v.snap && sc.filters == v.filter {
		return sc
	}
	*sc = scoped{snap: v.snap, filters: v.filter}
	s := v.currentScope()
	sc.matches = s.Apply(v.snap.Matches)
	sc.overview = stats.BuildOverview(sc.matches, v.snap.Now)
	s.Deck = ""
	sc.anyDeck = s.Apply(v.snap.Matches)
	sc.decks = stats.Leaders(sc.anyDeck)
	s.Turn = stats.EitherTurn
	sc.anyTurn = s.Apply(v.snap.Matches)
	return sc
}

func (v *view) deckLabel() string {
	if v.filter.deck == "" {
		return "All decks"
	}
	return v.snap.card(v.filter.deck).Name
}

func (v *view) deckMenu(m *ui.Menu) {
	if m.Item("All decks").Radio(v.filter.deck == "").Chosen() {
		v.setDeck("")
	}
	m.Separator()
	for _, lr := range v.scoped().decks {
		label := fmt.Sprintf("%s · %s", v.snap.card(lr.Leader).Name, games(lr.Record.Games()))
		if m.Item(label).Radio(v.filter.deck == lr.Leader).Chosen() {
			v.setDeck(lr.Leader)
		}
	}
}

func (v *view) setDeck(code string) {
	v.filter.deck = code
	v.resetPoints()
}

func (v *view) resetPoints() {
	v.leaderPoint = -1
}

func (v *view) clearFilters() {
	v.filter = v.cleared()
	v.matches.result = anyResult
	v.resetPoints()
}

// setTurn sets the turn order filter, or clears it when it's already set to t.
func (v *view) setTurn(t stats.TurnOrderFilter) {
	if v.filter.turn == t {
		t = stats.EitherTurn
	}
	v.filter.turn = t
	v.resetPoints()
}

func (v *view) turnMenu(m *ui.Menu) {
	for i, l := range turnLabels {
		if m.Item(l).Radio(int(v.filter.turn) == i).Chosen() {
			v.filter.turn = stats.TurnOrderFilter(i)
			v.resetPoints()
		}
	}
}

// removablePill is a set filter shown as a tinted pill that clears it when clicked.
func (v *view) removablePill(c *ui.Context, key, label string, clear func()) {
	t := c.Theme()
	el := ui.ButtonBase(c.Key(key)).Label("Remove the filter "+label).Padding(6, 12).Radius(16).
		Background(t.Accent.Alpha(0.18)).Border(1, t.Accent)
	el.Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(13)
		ui.Icon(c, iconClose).FontSize(13)
	})
	if el.Clicked() {
		clear()
		v.resetPoints()
	}
}

func (v *view) oppName(id string) string {
	for _, m := range v.snap.Matches {
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
		el = el.Background(t.Accent.Alpha(0.18)).Border(1, t.Accent)
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
func (v *view) filterBar(c *ui.Context) {
	t := c.Theme()
	pill := func(key, label string, on bool, build func(m *ui.Menu), clear func(*ui.Menu)) {
		pillMenu(c.Key(key), func() { filterPill(c, label, on).PopoverMenuButton(build) }, clear)
	}
	ui.Row(c).Padding(14, 20, 0, 20).Gap(8).Children(func() {
		_, inFormat := v.formatRange()
		pill("format", v.formatLabel(), inFormat, v.formatMenu,
			v.clearPillMenu("Back to "+v.startFormatName(), v.atStartFormat(), func() { v.filter.format = v.prefs.startFormat }))
		pillMenu(c.Key("period"), func() { v.periodPill(c) },
			v.clearPillMenu("Clear period", v.filter.period == 0, func() {
				v.filter.period, v.filter.from, v.filter.to, v.filter.ongoing = 0, time.Time{}, time.Time{}, false
			}))
		pill("deck", v.deckLabel(), v.filter.deck != "", v.deckMenu,
			v.clearPillMenu("Clear deck", v.filter.deck == "", func() { v.filter.deck = "" }))
		if inFormat {
			ui.Text(c, "Ranked").FontSize(13).TextColor(t.TextMuted).Padding(6, 12).Radius(16).Border(1, t.Border)
		} else {
			pill("type", gameTypeLabels[v.filter.gameType], v.filter.gameType != 0, func(m *ui.Menu) {
				for i, l := range gameTypeLabels {
					if m.Item(l).Radio(v.filter.gameType == i).Chosen() {
						v.filter.gameType = i
						v.resetPoints()
					}
				}
			}, v.clearPillMenu("Clear game type", v.filter.gameType == 0, func() { v.filter.gameType = 0 }))
		}
		pill("turn", turnLabels[v.filter.turn], v.filter.turn != stats.EitherTurn, v.turnMenu,
			v.clearPillMenu("Clear turn order", v.filter.turn == stats.EitherTurn, func() { v.filter.turn = stats.EitherTurn }))
		// The opponent filters have no menu of their own, so they show only while set.
		if v.filter.opp != "" {
			v.removablePill(c, "opp", "vs "+v.oppName(v.filter.opp), func() { v.filter.opp = "" })
		}
		if v.filter.oppLeader != "" {
			v.removablePill(c, "oppLeader", "vs "+v.snap.card(v.filter.oppLeader).Name, func() { v.filter.oppLeader = "" })
		}
		if v.canClear() && ui.Text(c, "Clear").FontSize(13).TextColor(t.Accent).Padding(6, 4).Clicked() {
			v.clearFilters()
		}
		ui.Spacer(c)
		ui.Text(c, games(len(v.scoped().matches))).FontSize(13).TextColor(faintColor(t))
	})
}
