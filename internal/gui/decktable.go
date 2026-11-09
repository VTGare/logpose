package gui

import (
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
	panel(c).Gap(0).Padding(12, 16, 14, 16).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.End).Padding(0, 0, 6, 0).Children(func() {
			ui.Text(c, "Builds of "+v.snap.card(d.leader).Name).FontSize(13).FontWeight(600)
			ui.Text(c, "Click a row to see only its games. With about 30 games each, gaps under 10 points are likely luck.").
				FontSize(12).TextColor(t.TextMuted).Wrap()
		})
		head := func(label string, w float32) {
			flexCell(ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).SingleLine(), w)
		}
		ui.Row(c).Gap(tableGap).Padding(8, 8).Children(func() {
			head("Build", 0)
			head("Games", gamesColW)
			head("Win rate", rateColTW)
			head("Going first", turnColW)
			head("Going second", turnColW)
			head("Last 20", lastColW)
			head("Played", playedColTW)
		})
		ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
		for _, r := range rows {
			v.scopeTableRow(c.Key(r.name), r)
		}
	})
}

func (v *view) scopeTableRow(c *ui.Context, r scopeRow) {
	t := c.Theme()
	l := r.stats
	row := ui.ButtonBase(c).Label("Show "+r.name).Row().Gap(tableGap).Padding(9, 8).AlignItems(ui.Center).
		Background(ui.Transparent).Radius(6)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	if row.Clicked() {
		v.decks.scope, v.decks.suggesting = r.scope, false
	}
	rate := func(rec stats.Record, w float32) {
		ui.Row(c).Width(w).Gap(5).AlignItems(ui.End).Children(func() {
			ui.Text(c, rateOrTooFew(rec)).FontSize(13).FontWeight(600)
			ui.Text(c, record(rec)).FontSize(12).TextColor(faintColor(t))
		})
	}
	cell := func(text string, w float32, muted bool) {
		col := t.Text
		if muted {
			col = faintColor(t)
		}
		flexCell(ui.Text(c, text).FontSize(13).TextColor(col).SingleLine(), w)
	}
	row.Children(func() {
		ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
			v.coreThumbs(c, r.core)
			ui.Text(c, r.name).FontSize(13).FontWeight(600).SingleLine()
		})
		cell(fmt.Sprint(l.Record.Games()), gamesColW, false)
		rate(l.Record, rateColTW)
		rate(l.TurnOrder.First, turnColW)
		rate(l.TurnOrder.Second, turnColW)
		last := "–"
		if len(l.Rolling) > 0 {
			last = pct(l.Rolling[len(l.Rolling)-1].Rate)
		}
		cell(last, lastColW, last == "–")
		played := "–"
		if !l.First.IsZero() {
			played = l.First.Format("Jan 2") + " – " + l.Last.Format("Jan 2")
		}
		cell(played, playedColTW, true)
	})
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.3))
}

// flexCell sets a table cell's width, or shares out the row's spare width when w is 0.
func flexCell(el ui.Element, w float32) {
	if w > 0 {
		el.Width(w)
		return
	}
	el.Grow(1).Basis(0).MinWidth(0)
}
