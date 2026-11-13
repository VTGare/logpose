package decks

import (
	"cmp"
	"fmt"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

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
func (pg *Page) buildsTable(c *ui.Context, e *page.Env, d leaderDecks) {
	t := c.Theme()
	tv := kit.ArrangeTable(e, "decks.builds", tableGap, buildCols(e, t), "", false)
	rows := tv.Sorted(d.rows)
	e.FoldTile(c, "decks.builds", kit.FoldHead{Label: "Builds of " + e.Snap.Card(d.leader).Name}, func() {
		ui.Text(c, "Click a row to see only its games. With about 30 games each, gaps under 10 points are likely luck.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
		ui.Column(c).Children(func() {
			tv.Header(c, 8)
			for _, r := range rows {
				pg.scopeTableRow(c.Key(r.name), e, tv, d, r)
			}
		})
	})
}

func buildCols(e *page.Env, t *ui.Theme) []kit.TableCol[scopeRow] {
	last := func(l stats.Leader) float64 {
		if len(l.Rolling) == 0 {
			return -1
		}
		return l.Rolling[len(l.Rolling)-1].Rate
	}
	text := func(c *ui.Context, s string, muted bool) {
		col := t.Text
		if muted {
			col = kit.FaintColor(t)
		}
		ui.Text(c, s).FontSize(13).TextColor(col).SingleLine()
	}
	rate := func(id, title string, width float32, rec func(scopeRow) stats.Record) kit.TableCol[scopeRow] {
		return kit.TableCol[scopeRow]{
			ID: id, Title: title, Width: width,
			By:   func(a, b scopeRow) int { return cmp.Compare(page.SortRate(rec(a)), page.SortRate(rec(b))) },
			Few:  func(r scopeRow) bool { return !rec(r).Enough() },
			Cell: func(c *ui.Context, r scopeRow) { page.RateCell(c, rec(r)) },
		}
	}
	return []kit.TableCol[scopeRow]{
		{
			ID: "build", Title: "Build", Asc: true,
			By: func(a, b scopeRow) int { return cmp.Compare(a.name, b.name) },
			Cell: func(c *ui.Context, r scopeRow) {
				ui.Row(c).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
					coreThumbs(c, e, r.core)
					ui.Text(c, r.name).FontSize(13).FontWeight(600).SingleLine().Tooltip(r.name)
				})
			},
		},
		{
			ID: "games", Title: "Games", Width: gamesColW,
			By:   func(a, b scopeRow) int { return cmp.Compare(a.stats.Record.Games(), b.stats.Record.Games()) },
			Cell: func(c *ui.Context, r scopeRow) { text(c, fmt.Sprint(r.stats.Record.Games()), false) },
		},
		rate("rate", "Win rate", rateColTW, func(r scopeRow) stats.Record { return r.stats.Record }),
		rate("first", "Going first", turnColW, func(r scopeRow) stats.Record { return r.stats.TurnOrder.First }),
		rate("second", "Going second", turnColW, func(r scopeRow) stats.Record { return r.stats.TurnOrder.Second }),
		{
			ID: "last", Title: fmt.Sprintf("Last %d", stats.RollingWindow), Width: lastColW,
			Tip: fmt.Sprintf("The win rate of the latest %d games.", stats.RollingWindow),
			By:  func(a, b scopeRow) int { return cmp.Compare(last(a.stats), last(b.stats)) },
			Few: func(r scopeRow) bool { return len(r.stats.Rolling) == 0 },
			Cell: func(c *ui.Context, r scopeRow) {
				s := "–"
				if len(r.stats.Rolling) > 0 {
					s = kit.Pct(last(r.stats))
				}
				text(c, s, s == "–")
			},
		},
		{
			ID: "played", Title: "Played", Width: playedColTW,
			By: func(a, b scopeRow) int { return a.stats.Last.Compare(b.stats.Last) },
			Cell: func(c *ui.Context, r scopeRow) {
				s := "–"
				if !r.stats.First.IsZero() {
					s = kit.Span(e.Date(r.stats.First), e.Date(r.stats.Last))
				}
				text(c, s, true)
			},
		},
	}
}

func (pg *Page) scopeTableRow(c *ui.Context, e *page.Env, tv kit.TableView[scopeRow], d leaderDecks, r scopeRow) {
	t := c.Theme()
	row := ui.ButtonBase(c).Label("Show "+r.name).Row().Gap(tv.Gap).Padding(9, 8).AlignItems(ui.Center).
		Background(ui.Transparent).Radius(6)
	if b, ok := d.chosen(r.scope.build); ok && r.scope.kind == scopeBuild {
		row.PopoverMenu(func(m *ui.Menu) { pg.buildMenu(e, m, d, b) })
	}
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	if row.Clicked() {
		pg.scope, pg.suggesting = r.scope, false
	}
	row.Children(func() { tv.Cells(c, r) })
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.3))
}
