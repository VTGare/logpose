package gui

import (
	"cmp"
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// The build column takes the rest.
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
		ms := d.scopedMatches(sc, d.matches)
		rows = append(rows, scopeRow{name: name, core: core, scope: sc, stats: stats.ForLeader(ms, d.leader, stats.EitherTurn)})
	}
	for _, b := range d.builds {
		add(b.Name, b.Core, buildScope(b.ID))
	}
	if d.unbuilt > 0 {
		add(noBuild, nil, deckScope{kind: scopeNoBuild})
	}
	return rows
}

// buildsTable compares the leader's builds and the games in no build. Clicking a row chooses it.
func (v *view) buildsTable(c *ui.Context, d leaderDecks) {
	t := c.Theme()
	tv := arrangeTable(v, "decks.builds", tableGap, v.buildCols(t), "", false)
	rows := tv.sorted(d.rows)
	v.foldTile(c, "decks.builds", foldHead{label: "Builds of " + v.snap.card(d.leader).Name}, func() {
		ui.Text(c, "Click a row to see only its games. With about 30 games each, gaps under 10 points are likely luck.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
		ui.Column(c).Children(func() {
			tv.header(c, 8)
			for _, r := range rows {
				v.scopeTableRow(c.Key(r.name), tv, d, r)
			}
		})
	})
}

func (v *view) buildCols(t *ui.Theme) []tableCol[scopeRow] {
	last := func(l stats.Leader) float64 {
		if len(l.Rolling) == 0 {
			return -1
		}
		return l.Rolling[len(l.Rolling)-1].Rate
	}
	text := func(c *ui.Context, s string, muted bool) {
		col := t.Text
		if muted {
			col = faintColor(t)
		}
		ui.Text(c, s).FontSize(13).TextColor(col).SingleLine()
	}
	rate := func(id, title string, width float32, rec func(scopeRow) stats.Record) tableCol[scopeRow] {
		return tableCol[scopeRow]{
			id: id, title: title, width: width,
			by:   func(a, b scopeRow) int { return cmp.Compare(sortRate(rec(a)), sortRate(rec(b))) },
			few:  func(r scopeRow) bool { return !rec(r).Enough() },
			cell: func(c *ui.Context, r scopeRow) { rateCell(c, rec(r)) },
		}
	}
	return []tableCol[scopeRow]{
		{
			id: "build", title: "Build", asc: true,
			by: func(a, b scopeRow) int { return cmp.Compare(a.name, b.name) },
			cell: func(c *ui.Context, r scopeRow) {
				ui.Row(c).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
					v.coreThumbs(c, r.core)
					ui.Text(c, r.name).FontSize(13).FontWeight(600).SingleLine().Tooltip(r.name)
				})
			},
		},
		{
			id: "games", title: "Games", width: gamesColW,
			by:   func(a, b scopeRow) int { return cmp.Compare(a.stats.Record.Games(), b.stats.Record.Games()) },
			cell: func(c *ui.Context, r scopeRow) { text(c, fmt.Sprint(r.stats.Record.Games()), false) },
		},
		rate("rate", "Win rate", rateColTW, func(r scopeRow) stats.Record { return r.stats.Record }),
		rate("first", "Going first", turnColW, func(r scopeRow) stats.Record { return r.stats.TurnOrder.First }),
		rate("second", "Going second", turnColW, func(r scopeRow) stats.Record { return r.stats.TurnOrder.Second }),
		{
			id: "last", title: fmt.Sprintf("Last %d", stats.RollingWindow), width: lastColW,
			tip: fmt.Sprintf("The win rate of the latest %d games.", stats.RollingWindow),
			by:  func(a, b scopeRow) int { return cmp.Compare(last(a.stats), last(b.stats)) },
			few: func(r scopeRow) bool { return len(r.stats.Rolling) == 0 },
			cell: func(c *ui.Context, r scopeRow) {
				s := "–"
				if len(r.stats.Rolling) > 0 {
					s = pct(last(r.stats))
				}
				text(c, s, s == "–")
			},
		},
		{
			id: "played", title: "Played", width: playedColTW,
			by: func(a, b scopeRow) int { return a.stats.Last.Compare(b.stats.Last) },
			cell: func(c *ui.Context, r scopeRow) {
				s := "–"
				if !r.stats.First.IsZero() {
					s = span(v.date(r.stats.First), v.date(r.stats.Last))
				}
				text(c, s, true)
			},
		},
	}
}

func (v *view) scopeTableRow(c *ui.Context, tv tableView[scopeRow], d leaderDecks, r scopeRow) {
	t := c.Theme()
	row := ui.ButtonBase(c).Label("Show "+r.name).Row().Gap(tv.gap).Padding(9, 8).AlignItems(ui.Center).
		Background(ui.Transparent).Radius(6)
	if b, ok := d.chosen(r.scope.build); ok && r.scope.kind == scopeBuild {
		row.PopoverMenu(func(m *ui.Menu) { v.buildMenu(m, d, b) })
	}
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	if row.Clicked() {
		v.decks.scope, v.decks.suggesting = r.scope, false
	}
	row.Children(func() { tv.cells(c, r) })
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.3))
}
