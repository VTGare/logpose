package gui

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// cheapCost is the highest cost that's paid by the player's first or second turn either way, so
// the card's turn is the opening hand plus at most one draw. The player going first doesn't
// draw on their first turn.
const cheapCost = 2

// The Cards tab's columns, in DIPs. The card column takes the rest.
const (
	rateColW   = 118
	playedColW = 150
	searchColW = 96
	rangeBarW  = 92
)

// cardsTab is how each card did in the games the scope counts: the latest list's cards first,
// then the ones no longer in it.
func (v *view) cardsTab(c *ui.Context, d leaderDecks) {
	t := c.Theme()
	gs := d.scopedGames(v.decks.scope)
	name := v.scopeName(d)
	if name == "" {
		name = v.snap.card(d.leader).Name
	}
	panel(c).Gap(0).Padding(8, 16, 16, 16).Children(func() {
		if len(gs) == 0 {
			ui.Text(c, "No games of "+name+" match these filters.").TextColor(t.TextMuted).Padding(8, 0)
			return
		}
		var all stats.Record
		for _, g := range gs {
			all.Add(g.Result)
		}
		avg := 0.5
		if all.Games() > 0 {
			avg = float64(all.Wins) / float64(all.Games())
		}
		latest := gs[len(gs)-1].Facts.List
		tv := v.cardsTable()
		list := stats.Cards(gs, v.snap.card)
		in := slices.IndexFunc(list, func(cs stats.CardStats) bool { return latest[cs.Card] == 0 })
		if in < 0 {
			in = len(list)
		}
		v.sortCards(list[:in], tv)
		v.sortCards(list[in:], tv)
		v.tableHeader(c, tv, 8)
		for i, cs := range list {
			if i == in {
				ui.Text(c.Key("cut"), "No longer in the list").FontSize(12).FontWeight(600).TextColor(t.TextMuted).Padding(14, 8, 4, 8)
			}
			v.cardRow(c.Key(cs.Card), tv, cs, latest[cs.Card], len(gs), avg, i >= in)
		}
		cardsNote(c, avg)
	})
}

func (v *view) cardsTable() tableView {
	return v.arrangeTable("decks.cards", 12, []tableCol{
		{id: "card", title: "Card", asc: true},
		{id: "open", title: "In starting hand", width: rateColW},
		{id: "on", title: "In hand by its turn", width: rateColW},
		{id: "off", title: "Not by its turn", width: rateColW},
		{id: "played", title: "Played", width: playedColW, tip: "From left to right: played on curve, played later, put out by an effect, never played."},
		{id: "search", title: "Searcher took it", width: searchColW},
	}, "", false)
}

// sortCards sorts by the column chosen. With none, the cards keep the list's order.
func (v *view) sortCards(cs []stats.CardStats, tv tableView) {
	rate := func(r stats.Record) float64 {
		x, _ := r.Rate()
		return x
	}
	share := func(r stats.Record, total int) float64 {
		if total == 0 {
			return 0
		}
		return float64(r.Games()) / float64(total)
	}
	cheap := func(cs stats.CardStats) bool { return max(v.snap.card(cs.Card).Cost, 0) <= cheapCost }
	sortRows(cs, tv.sort, tv.desc, map[string]func(a, b stats.CardStats) int{
		"card": func(a, b stats.CardStats) int { return cmp.Compare(v.snap.card(a.Card).Name, v.snap.card(b.Card).Name) },
		"open": func(a, b stats.CardStats) int { return cmp.Compare(rate(a.Opening), rate(b.Opening)) },
		"on":   func(a, b stats.CardStats) int { return cmp.Compare(rate(a.OnCurve), rate(b.OnCurve)) },
		"off":  func(a, b stats.CardStats) int { return cmp.Compare(rate(a.OffCurve), rate(b.OffCurve)) },
		"played": func(a, b stats.CardStats) int {
			return cmp.Compare(share(a.PlayedOnCurve, a.Games), share(b.PlayedOnCurve, b.Games))
		},
		"search": func(a, b stats.CardStats) int {
			return cmp.Compare(float64(a.Taken)/float64(max(a.Shown, 1)), float64(b.Taken)/float64(max(b.Shown, 1)))
		},
	}, map[string]func(stats.CardStats) bool{
		"open":   func(cs stats.CardStats) bool { return cs.Opening.Games() < stats.MinGames },
		"on":     func(cs stats.CardStats) bool { return cheap(cs) || cs.OnCurve.Games() < stats.MinGames },
		"off":    func(cs stats.CardStats) bool { return cheap(cs) || cs.OffCurve.Games() < stats.MinGames },
		"search": func(cs stats.CardStats) bool { return cs.Shown == 0 },
	}, nil)
}

// cardRow is one card's numbers. total is the build's games, which the card's own count is
// shown against when it was in fewer.
func (v *view) cardRow(c *ui.Context, tv tableView, cs stats.CardStats, copies, total int, avg float64, cut bool) {
	t := c.Theme()
	row := ui.Row(c).Gap(tv.gap).Padding(6, 8).AlignItems(ui.Center)
	if cut {
		row.Opacity(0.7)
	}
	row.Children(func() {
		for _, col := range tv.cols {
			tv.cell(c, col, func() { v.cardCell(c, col, cs, copies, total, avg) })
		}
	})
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.3))
}

func (v *view) cardCell(c *ui.Context, col tableCol, cs stats.CardStats, copies, total int, avg float64) {
	t := c.Theme()
	card := v.snap.card(cs.Card)
	cost := max(card.Cost, 0)
	switch col.id {
	case "card":
		ui.Row(c).Gap(10).AlignItems(ui.Center).MinWidth(0).Children(func() {
			v.cardArt(c, cs.Card, 30, 42)
			ui.Column(c).Gap(1).MinWidth(0).Children(func() {
				ui.Text(c, card.Name).FontSize(13).FontWeight(600).SingleLine().Tooltip(card.Name)
				meta := cs.Card
				if copies > 0 {
					meta += fmt.Sprintf(" · ×%d", copies)
				}
				if cs.Games < total {
					meta += " · " + games(cs.Games)
				}
				ui.Text(c, meta).FontSize(11).TextColor(faintColor(t)).SingleLine()
			})
		})
	case "open":
		rateIn(c.Key("open"), cs.Opening, avg)
	case "on":
		if cost <= cheapCost {
			sameAsOpening(c, cost)
			return
		}
		rateIn(c.Key("on"), cs.OnCurve, avg).Tooltip("In hand by " + curveText(cost))
	case "off":
		if cost <= cheapCost {
			ui.Text(c, "–").FontSize(13).TextColor(faintColor(t))
			return
		}
		rateIn(c.Key("off"), cs.OffCurve, avg).Tooltip("Not in hand by " + curveText(cost))
	case "played":
		playedCell(c, cs)
	case "search":
		search := "–"
		if cs.Shown > 0 {
			search = fmt.Sprintf("%d of %d", cs.Taken, cs.Shown)
		}
		ui.Text(c, search).FontSize(13).TextColor(map[bool]ui.Color{true: t.Text, false: faintColor(t)}[cs.Shown > 0])
	}
}

// rateIn is a win rate with its record, and a bar of where the rate likely is with a tick at
// avg, the build's own rate, as wide as its column. Below MinGames the rate reads "few".
func rateIn(c *ui.Context, r stats.Record, avg float64) ui.Element {
	return rateBar(c, r, avg, 0)
}

// rateBar is a rateIn whose bar is barW wide, or as wide as there's room for when barW is 0.
// The bar is painted at the width it's laid out at, since a column can get less than it asks.
func rateBar(c *ui.Context, r stats.Record, avg float64, barW float32) ui.Element {
	t := c.Theme()
	col := ui.Column(c).Gap(1)
	if barW > 0 {
		col.Width(barW + rateColW - rangeBarW)
	} else {
		col.Grow(1).Basis(0).MinWidth(0)
	}
	return col.Children(func() {
		if r.Games() == 0 {
			ui.Text(c, "–").FontSize(13).TextColor(faintColor(t))
			return
		}
		ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
			if rate, ok := r.Rate(); ok {
				ui.Text(c, pct(rate)).FontSize(13).FontWeight(600).SingleLine()
			} else {
				ui.Text(c, "few").FontSize(13).TextColor(faintColor(t)).SingleLine()
			}
			ui.Text(c, record(r)).FontSize(11).TextColor(faintColor(t)).SingleLine()
		})
		lo, hi := r.LikelyRange()
		tip := fmt.Sprintf("Likely %s–%s from %s. The tick is the %s these games win.", pct(lo), pct(hi), record(r), pct(avg))
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

// playedCell splits the games into played on curve, played later, put out by an effect and
// never played, as a bar and the games in each.
func playedCell(c *ui.Context, cs stats.CardStats) {
	t := c.Theme()
	parts := []struct {
		r   stats.Record
		col ui.Color
	}{
		{cs.PlayedOnCurve, winColor}, {cs.PlayedLater, warnColor}, {cs.ByEffect, t.Accent}, {cs.NeverPlayed, faintColor(t)},
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
			"the build's %s. When the bar misses it, the difference likely isn't luck.", pct(avg))).
			FontSize(12).TextColor(faintColor(t)).Wrap()
		legend := func(label string, col ui.Color) {
			ui.Row(c.Key(label)).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(2).Background(col)
				ui.Text(c, label).FontSize(12).TextColor(col)
			})
		}
		ui.Text(c, "Played:").FontSize(12).TextColor(faintColor(t))
		legend("on curve", winColor)
		legend("later", warnColor)
		legend("by an effect", t.Accent)
		legend("never", faintColor(t))
	})
}

// sameAsOpening spans both turn columns for a card cheap enough that its turn is the
// opening hand plus at most one draw.
func sameAsOpening(c *ui.Context, cost int) {
	t := c.Theme()
	ui.Text(c, "as starting hand").FontSize(12).TextColor(faintColor(t)).SingleLine().
		Tooltip(fmt.Sprintf("A cost %d card can be paid for by your first or second turn, so this is "+
			"the starting hand plus at most one draw.", cost))
}

// curveText names the turn a card of this cost can first be paid for. DON!! goes 1, 3, 5 and up
// to 10 for the player going first, and 2, 4, 6 for the other.
func curveText(cost int) string {
	first := (cost + 2) / 2
	second := max((cost+1)/2, 1)
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
