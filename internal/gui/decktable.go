package gui

import (
	"cmp"
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// The builds table's columns, in DIPs. The build column takes the rest.
const (
	gamesColW   = 44
	rateColTW   = 96
	turnColW    = 88
	lastColW    = 52
	playedColTW = 104
	tableGap    = 10
)

// scopeRow is one row of the builds table: a build, or the games in no build.
type scopeRow struct {
	name  string
	core  []string
	scope deckScope
	stats stats.Leader
}

// scopeRows are the leader's builds, then No build when it has games, each with its stats.
func (d leaderDecks) scopeRows() []scopeRow {
	var rows []scopeRow
	add := func(name string, core []string, sc deckScope) {
		ms := reversed(d.scopedMatches(sc, d.matches))
		rows = append(rows, scopeRow{name: name, core: core, scope: sc, stats: stats.ForLeader(ms, d.leader)})
	}
	for _, b := range d.builds {
		add(b.Name, b.Core, buildScope(b.ID))
	}
	if d.unbuilt > 0 {
		add("No build", nil, deckScope{kind: scopeNoBuild})
	}
	return rows
}

// buildsTable compares the leader's builds and the games in no build. Clicking a row chooses it.
func (v *view) buildsTable(c *ui.Context, d leaderDecks) {
	t := c.Theme()
	rows := d.scopeRows()
	tv := v.arrangeTable("decks.builds", tableGap, []tableCol{
		{id: "build", title: "Build", asc: true},
		{id: "games", title: "Games", width: gamesColW},
		{id: "rate", title: "Win rate", width: rateColTW},
		{id: "first", title: "Going first", width: turnColW},
		{id: "second", title: "Going second", width: turnColW},
		{id: "last", title: "Last 20", width: lastColW, tip: "The win rate of the latest 20 games."},
		{id: "played", title: "Played", width: playedColTW},
	}, "", false)
	rate := func(r stats.Record) float64 {
		x, _ := r.Rate()
		return x
	}
	last := func(l stats.Leader) float64 {
		if len(l.Rolling) == 0 {
			return -1
		}
		return l.Rolling[len(l.Rolling)-1].Rate
	}
	few := func(r stats.Record) bool { return r.Games() < stats.MinGames }
	sortRows(rows, tv.sort, tv.desc, map[string]func(a, b scopeRow) int{
		"build": func(a, b scopeRow) int { return cmp.Compare(a.name, b.name) },
		"games": func(a, b scopeRow) int { return cmp.Compare(a.stats.Record.Games(), b.stats.Record.Games()) },
		"rate":  func(a, b scopeRow) int { return cmp.Compare(rate(a.stats.Record), rate(b.stats.Record)) },
		"first": func(a, b scopeRow) int {
			return cmp.Compare(rate(a.stats.TurnOrder.First), rate(b.stats.TurnOrder.First))
		},
		"second": func(a, b scopeRow) int {
			return cmp.Compare(rate(a.stats.TurnOrder.Second), rate(b.stats.TurnOrder.Second))
		},
		"last":   func(a, b scopeRow) int { return cmp.Compare(last(a.stats), last(b.stats)) },
		"played": func(a, b scopeRow) int { return a.stats.Last.Compare(b.stats.Last) },
	}, map[string]func(scopeRow) bool{
		"rate":   func(r scopeRow) bool { return few(r.stats.Record) },
		"first":  func(r scopeRow) bool { return few(r.stats.TurnOrder.First) },
		"second": func(r scopeRow) bool { return few(r.stats.TurnOrder.Second) },
		"last":   func(r scopeRow) bool { return len(r.stats.Rolling) == 0 },
	}, nil)
	v.foldTile(c, "decks.builds", foldHead{label: "Builds of " + v.snap.card(d.leader).Name}, func() {
		ui.Text(c, "Click a row to see only its games. With about 30 games each, gaps under 10 points are likely luck.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
		ui.Column(c).Children(func() {
			v.tableHeader(c, tv, 8)
			for _, r := range rows {
				v.scopeTableRow(c.Key(r.name), tv, r)
			}
		})
	})
}

func (v *view) scopeTableRow(c *ui.Context, tv tableView, r scopeRow) {
	t := c.Theme()
	l := r.stats
	row := ui.ButtonBase(c).Label("Show "+r.name).Row().Gap(tv.gap).Padding(9, 8).AlignItems(ui.Center).
		Background(ui.Transparent).Radius(6)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	if row.Clicked() {
		v.decks.scope, v.decks.suggesting = r.scope, false
	}
	rate := func(rec stats.Record) {
		ui.Row(c).Gap(5).AlignItems(ui.End).Children(func() {
			ui.Text(c, rateOrTooFew(rec)).FontSize(13).FontWeight(600).SingleLine()
			ui.Text(c, record(rec)).FontSize(12).TextColor(faintColor(t)).SingleLine()
		})
	}
	text := func(s string, muted bool) {
		col := t.Text
		if muted {
			col = faintColor(t)
		}
		ui.Text(c, s).FontSize(13).TextColor(col).SingleLine()
	}
	row.Children(func() {
		for _, col := range tv.cols {
			tv.cell(c, col, func() {
				switch col.id {
				case "build":
					ui.Row(c).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
						v.coreThumbs(c, r.core)
						ui.Text(c, r.name).FontSize(13).FontWeight(600).SingleLine().Tooltip(r.name)
					})
				case "games":
					text(fmt.Sprint(l.Record.Games()), false)
				case "rate":
					rate(l.Record)
				case "first":
					rate(l.TurnOrder.First)
				case "second":
					rate(l.TurnOrder.Second)
				case "last":
					s := "–"
					if len(l.Rolling) > 0 {
						s = pct(l.Rolling[len(l.Rolling)-1].Rate)
					}
					text(s, s == "–")
				case "played":
					s := "–"
					if !l.First.IsZero() {
						s = l.First.Format("Jan 2") + " – " + l.Last.Format("Jan 2")
					}
					text(s, true)
				}
			})
		}
	})
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.3))
}
