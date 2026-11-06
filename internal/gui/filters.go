package gui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// filters is what the filter bar sets. It stays set while moving between pages.
type filters struct {
	// format is formatCurrent, formatAll or a format's ID.
	format int64
	// period indexes periods.
	period int
	// deck is the local player's leader, or "" for every deck.
	deck string
	// gameType is a stats.GameType, an int for ui.Segmented.
	gameType int
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

const (
	// formatCurrent is the latest format, whichever that is when the user adds one.
	formatCurrent int64 = 0
	formatAll     int64 = -1
)

// active reports whether the filters differ from where Clear puts them.
func (f filters) active() bool { return f != filters{} }

// formatRange is the format the filter picks. There's none with no formats or all of them.
func (v *view) formatRange() (stats.FormatRange, bool) {
	fs := v.snap.Formats
	switch {
	case len(fs) == 0 || v.filter.format == formatAll:
		return stats.FormatRange{}, false
	case v.filter.format == formatCurrent:
		return fs[len(fs)-1], true
	}
	for _, f := range fs {
		if f.ID == v.filter.format {
			return f, true
		}
	}
	return stats.FormatRange{}, false
}

// currentScope is the filters as a stats.Scope. A format holds ranked games only, since casual
// lobbies can be any format. A period inside a format starts at the later of the two.
func (v *view) currentScope() stats.Scope {
	f := v.filter
	s := stats.Scope{Type: stats.GameType(f.gameType), Deck: f.deck, Opp: f.opp, OppLeader: f.oppLeader}
	if d := periods[f.period].days; d > 0 {
		s.Since = v.snap.Now.AddDate(0, 0, -d)
	}
	if fr, ok := v.formatRange(); ok {
		if fr.Start.After(s.Since) {
			s.Since = fr.Start
		}
		s.Until = fr.End
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
	// anyDeck is the scope without the deck, for choosing one.
	anyDeck []stats.Match
	decks   []stats.LeaderRecord
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

func (v *view) periodMenu(m *ui.Menu) {
	for i, p := range periods {
		if m.Item(p.label).Radio(v.filter.period == i).Chosen() {
			v.filter.period = i
			v.resetPoints()
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
	v.filter = filters{}
	v.resetPoints()
}

// removablePill is a set filter shown as a tinted pill that clears it when clicked.
func (v *view) removablePill(c *ui.Context, key, label string, clear func()) {
	t := c.Theme()
	el := ui.ButtonBase(c.Key(key)).Label("Remove the filter "+label).Padding(6, 12).Radius(16).
		Background(t.Accent.Alpha(0.18)).Border(1, t.Accent)
	el.Children(func() { ui.Text(c, label+"  ✕").FontSize(13) })
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

// filterBar is the filters over every page, as pills that each open a menu. Pills that narrow
// what the pages show are tinted.
func (v *view) filterBar(c *ui.Context) {
	t := c.Theme()
	pill := func(key, label string, on bool, build func(m *ui.Menu)) {
		el := ui.ButtonBase(c.Key(key)).Label(label).Padding(6, 12).Radius(16).Border(1, t.Border)
		if on {
			el = el.Background(t.Accent.Alpha(0.18)).Border(1, t.Accent)
		}
		el.Children(func() {
			ui.Text(c, label+"  ▾").FontSize(13)
		})
		el.PopoverMenuButton(build)
	}
	ui.Row(c).Padding(14, 20, 0, 20).Gap(8).Children(func() {
		_, inFormat := v.formatRange()
		pill("format", v.formatLabel(), inFormat, v.formatMenu)
		pill("period", periods[v.filter.period].label, v.filter.period != 0, v.periodMenu)
		pill("deck", v.deckLabel(), v.filter.deck != "", v.deckMenu)
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
			})
		}
		// The opponent filters have no menu of their own, so they show only while set.
		if v.filter.opp != "" {
			v.removablePill(c, "opp", "vs "+v.oppName(v.filter.opp), func() { v.filter.opp = "" })
		}
		if v.filter.oppLeader != "" {
			v.removablePill(c, "oppLeader", "vs "+v.snap.card(v.filter.oppLeader).Name, func() { v.filter.oppLeader = "" })
		}
		if v.filter.active() && ui.Text(c, "Clear").FontSize(13).TextColor(t.Accent).Padding(6, 4).Clicked() {
			v.clearFilters()
		}
		ui.Spacer(c)
		ui.Text(c, games(len(v.scoped().matches))).FontSize(12).TextColor(t.TextMuted)
	})
}
