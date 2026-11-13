package decks

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// The card column takes the rest.
const (
	rateColW   = 118
	playedColW = 150
	searchColW = 96
	rangeBarW  = 92
)

// cardsTab is how each card did in the games the scope counts: the latest list's cards first,
// then the ones no longer in it.
func cardsTab(c *ui.Context, e *page.Env, d *deckData) {
	t := c.Theme()
	gs := d.deckGames
	kit.Panel(c).Gap(0).Padding(8, 16, 16, 16).Children(func() {
		if len(gs) == 0 {
			noGames(c, d).Padding(8, 0)
			return
		}
		avg := d.stats.Record.Share()
		tv := kit.ArrangeTable(e, "decks.cards", 12, cardCols(e, t, gs[0].Facts.List, len(gs), avg), "", false)
		in := tv.Sorted(slices.DeleteFunc(slices.Clone(d.cards), func(cs stats.CardStats) bool { return !cs.Latest }))
		gone := tv.Sorted(slices.DeleteFunc(slices.Clone(d.cards), func(cs stats.CardStats) bool { return cs.Latest }))
		tv.Header(c, 8)
		for _, cs := range in {
			cardRow(c.Key(cs.Card), tv, cs, false)
		}
		if len(gone) > 0 {
			ui.Text(c.Key("cut"), "No longer in the list").FontSize(12).FontWeight(600).TextColor(t.TextMuted).Padding(14, 8, 4, 8)
		}
		for _, cs := range gone {
			cardRow(c.Key(cs.Card), tv, cs, true)
		}
		cardsNote(c, avg)
	})
}

// noGames says that the filters leave none of the scope's games.
func noGames(c *ui.Context, d *deckData) ui.Element {
	return ui.Text(c, "No games of "+d.name+" match these filters.").TextColor(c.Theme().TextMuted)
}

// cardCols are the Cards tab's columns. With no sort chosen, the cards keep the list's order.
// latest is the newest game's list, and total is the build's games, which a card's own count is
// shown against when it was in fewer.
func cardCols(e *page.Env, t *ui.Theme, latest map[string]int, total int, avg float64) []kit.TableCol[stats.CardStats] {
	share := func(r stats.Record, total int) float64 {
		if total == 0 {
			return 0
		}
		return float64(r.Games()) / float64(total)
	}
	cost := func(cs stats.CardStats) int { return e.Snap.Card(cs.Card).PaidCost() }
	return []kit.TableCol[stats.CardStats]{
		{
			ID: "card", Title: "Card", Asc: true,
			By: func(a, b stats.CardStats) int { return cmp.Compare(e.Snap.Card(a.Card).Name, e.Snap.Card(b.Card).Name) },
			Cell: func(c *ui.Context, cs stats.CardStats) {
				meta := cs.Card
				if copies := latest[cs.Card]; cs.Latest && copies > 0 {
					meta += fmt.Sprintf(" · ×%d", copies)
				}
				if cs.Games < total {
					meta += " · " + kit.Games(cs.Games)
				}
				cardNameCell(c, e, cs.Card, meta)
			},
		},
		{
			ID: "open", Title: "In starting hand", Width: rateColW,
			By:   func(a, b stats.CardStats) int { return cmp.Compare(page.SortRate(a.Opening), page.SortRate(b.Opening)) },
			Few:  func(cs stats.CardStats) bool { return !cs.Opening.Enough() },
			Cell: func(c *ui.Context, cs stats.CardStats) { rateBar(c.Key("open"), cs.Opening, avg, 0) },
		},
		{
			ID: "on", Title: "In hand by its turn", Width: rateColW,
			By:  func(a, b stats.CardStats) int { return cmp.Compare(page.SortRate(a.OnCurve), page.SortRate(b.OnCurve)) },
			Few: func(cs stats.CardStats) bool { return !cs.Curve || !cs.OnCurve.Enough() },
			Cell: func(c *ui.Context, cs stats.CardStats) {
				if !cs.Curve {
					sameAsOpening(c, cost(cs))
					return
				}
				rateBar(c.Key("on"), cs.OnCurve, avg, 0).Tooltip("In hand by " + CurveText(cost(cs)))
			},
		},
		{
			ID: "off", Title: "Not by its turn", Width: rateColW,
			By: func(a, b stats.CardStats) int {
				return cmp.Compare(page.SortRate(a.OffCurve), page.SortRate(b.OffCurve))
			},
			Few: func(cs stats.CardStats) bool { return !cs.Curve || !cs.OffCurve.Enough() },
			Cell: func(c *ui.Context, cs stats.CardStats) {
				if !cs.Curve {
					ui.Text(c, "–").FontSize(13).TextColor(kit.FaintColor(t))
					return
				}
				rateBar(c.Key("off"), cs.OffCurve, avg, 0).Tooltip("Not in hand by " + CurveText(cost(cs)))
			},
		},
		{
			ID: "played", Title: "Played", Width: playedColW,
			Tip: "From left to right: played on curve, played later, put out by an effect, never played.",
			By: func(a, b stats.CardStats) int {
				return cmp.Compare(share(a.PlayedOnCurve, a.Games), share(b.PlayedOnCurve, b.Games))
			},
			Cell: func(c *ui.Context, cs stats.CardStats) { playedCell(c, cs) },
		},
		{
			ID: "search", Title: "Searcher took it", Width: searchColW,
			By: func(a, b stats.CardStats) int {
				return cmp.Compare(float64(a.Taken)/float64(max(a.Shown, 1)), float64(b.Taken)/float64(max(b.Shown, 1)))
			},
			Few: func(cs stats.CardStats) bool { return cs.Shown == 0 },
			Cell: func(c *ui.Context, cs stats.CardStats) {
				search := "–"
				if cs.Shown > 0 {
					search = fmt.Sprintf("%d of %d", cs.Taken, cs.Shown)
				}
				ui.Text(c, search).FontSize(13).TextColor(map[bool]ui.Color{true: t.Text, false: kit.FaintColor(t)}[cs.Shown > 0])
			},
		},
	}
}

func cardRow(c *ui.Context, tv kit.TableView[stats.CardStats], cs stats.CardStats, cut bool) {
	t := c.Theme()
	row := ui.Row(c).Gap(tv.Gap).Padding(6, 8).AlignItems(ui.Center)
	if cut {
		row.Opacity(0.7)
	}
	row.Children(func() { tv.Cells(c, cs) })
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.3))
}

// rateBar is a rateCell over a bar of where the rate likely is, with a tick at avg, the build's
// own rate. The bar is barW wide, or as wide as there's room for when barW is 0. It's painted at
// the width it's laid out at, since a column can get less than it asks.
func rateBar(c *ui.Context, r stats.Record, avg float64, barW float32) ui.Element {
	t := c.Theme()
	col := ui.Column(c).Gap(1)
	if barW > 0 {
		col.Width(barW + rateColW - rangeBarW)
	} else {
		col.Grow(1).Basis(0).MinWidth(0)
	}
	return col.Children(func() {
		page.RateCell(c, r)
		if r.Games() == 0 {
			return
		}
		lo, hi := r.LikelyRange()
		tip := fmt.Sprintf("Likely %s–%s from %s. The tick is the %s these games win.", kit.Pct(lo), kit.Pct(hi), kit.Record(r), kit.Pct(avg))
		bar := ui.Box(c).Height(11).Tooltip(tip)
		if barW > 0 {
			bar.Width(barW)
		} else {
			bar.AlignSelf(ui.Stretch)
		}
		track, fill, tick := t.Surface, t.Accent.Alpha(0.7), t.Text
		bar.Draw(func(p *ui.Painter, rc ui.Rect) {
			y := rc.Y + 3
			p.Fill(ui.Rect{X: rc.X, Y: y, W: rc.W, H: 5}, track, 3)
			p.Fill(ui.Rect{X: rc.X + float32(lo)*rc.W, Y: y, W: max(float32(hi-lo)*rc.W, 2), H: 5}, fill, 3)
			p.Fill(ui.Rect{X: rc.X + float32(avg)*rc.W - 1, Y: rc.Y, W: 2, H: 11}, tick, 0)
		})
	})
}

// cardNameCell is a card's art and name with meta under the name, in a table's card column.
func cardNameCell(c *ui.Context, e *page.Env, code, meta string) {
	name := e.Snap.Card(code).Name
	ui.Row(c).Gap(10).AlignItems(ui.Center).MinWidth(0).Children(func() {
		e.Card(c, code, 30)
		ui.Column(c).Gap(1).MinWidth(0).Children(func() {
			ui.Text(c, name).FontSize(13).FontWeight(600).SingleLine().Tooltip(name)
			ui.Text(c, meta).FontSize(11).TextColor(kit.FaintColor(c.Theme())).SingleLine()
		})
	})
}

// playedCell splits the games into played on curve, played later, put out by an effect and
// never played, as a bar and the games in each.
func playedCell(c *ui.Context, cs stats.CardStats) {
	t := c.Theme()
	parts := []struct {
		r   stats.Record
		col ui.Color
	}{
		{cs.PlayedOnCurve, kit.WinColor(t)}, {cs.PlayedLater, kit.WarnColor}, {cs.ByEffect, t.Accent}, {cs.NeverPlayed, kit.FaintColor(t)},
	}
	total := 0
	for _, p := range parts {
		total += p.r.Games()
	}
	ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
		ui.Row(c).Height(8).AlignSelf(ui.Stretch).Margin(0, 20, 0, 0).Radius(4).Clip().Background(t.Surface).Children(func() {
			for i, p := range parts {
				if n := p.r.Games(); n > 0 && total > 0 {
					ui.Box(c.Key(i)).Grow(float32(n)).Basis(0).Height(8).Background(p.col)
				}
			}
		})
		ui.Row(c).Gap(8).Children(func() {
			for i, p := range parts {
				if n := p.r.Games(); n > 0 {
					ui.Text(c.Key(i), fmt.Sprint(n)).FontSize(11).TextColor(p.col)
				}
			}
		})
	})
}

func cardsNote(c *ui.Context, avg float64) {
	t := c.Theme()
	ui.Row(c).Gap(14).Padding(12, 8, 0, 8).Wrap().AlignItems(ui.Center).Children(func() {
		ui.Text(c, fmt.Sprintf("The bar is where the real win rate likely is, from the games so far. The tick is "+
			"the build's %s. When the bar misses it, the difference likely isn't luck.", kit.Pct(avg))).
			FontSize(12).TextColor(kit.FaintColor(t)).Wrap()
		legend := func(label string, col ui.Color) {
			ui.Row(c.Key(label)).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(2).Background(col)
				ui.Text(c, label).FontSize(12).TextColor(col)
			})
		}
		ui.Text(c, "Played:").FontSize(12).TextColor(kit.FaintColor(t))
		legend("on curve", kit.WinColor(t))
		legend("later", kit.WarnColor)
		legend("by an effect", t.Accent)
		legend("never", kit.FaintColor(t))
	})
}

// sameAsOpening spans both turn columns for a card cheap enough that its turn is the
// opening hand plus at most one draw.
func sameAsOpening(c *ui.Context, cost int) {
	t := c.Theme()
	ui.Text(c, "as starting hand").FontSize(12).TextColor(kit.FaintColor(t)).SingleLine().
		Tooltip(fmt.Sprintf("A cost %d card can be paid for by your first or second turn, so this is "+
			"the starting hand plus at most one draw.", cost))
}

// CurveText names the turn a card of this cost can first be paid for.
func CurveText(cost int) string {
	first, second := stats.PayTurn(cost, true), stats.PayTurn(cost, false)
	turns := fmt.Sprintf("your %s turn going first and %s going second", ordinal(first), ordinal(second))
	if first == second {
		turns = fmt.Sprintf("your %s turn either way", ordinal(first))
	}
	return fmt.Sprintf("the turn with %d DON!!, %s.", cost, turns)
}

func ordinal(n int) string {
	switch n {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	}
	return fmt.Sprintf("%dth", n)
}
