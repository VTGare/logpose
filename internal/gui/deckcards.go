package gui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
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

// cardsTab is how each card of the chosen build did in its games in scope: the latest list's
// cards first, then the ones no longer in it.
func (v *view) cardsTab(c *ui.Context, d leaderDecks) {
	t := c.Theme()
	var b store.Build
	for _, x := range d.builds {
		if x.ID == v.decks.build {
			b = x
		}
	}
	gs := d.assignment.Builds[b.ID]
	panel(c).Gap(0).Padding(8, 16, 16, 16).Children(func() {
		if len(gs) == 0 {
			ui.Text(c, "No games of "+b.Name+" match these filters.").TextColor(t.TextMuted).Padding(8, 0)
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
		cardsHeader(c)
		cut := false
		for _, cs := range stats.Cards(gs, v.snap.card) {
			if latest[cs.Card] == 0 && !cut {
				cut = true
				ui.Text(c.Key("cut"), "No longer in the list").FontSize(12).FontWeight(600).TextColor(t.TextMuted).Padding(14, 8, 4, 8)
			}
			v.cardRow(c.Key(cs.Card), cs, latest[cs.Card], len(gs), avg, cut)
		}
		cardsNote(c, avg)
	})
}

func cardsHeader(c *ui.Context) {
	t := c.Theme()
	head := func(label string) ui.Element {
		return ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
	}
	ui.Row(c).Gap(12).Padding(10, 8).AlignItems(ui.End).Children(func() {
		head("Card").Grow(1).Basis(0)
		head("In opening hand").Width(rateColW)
		head("In hand by its turn").Width(rateColW)
		head("Not by its turn").Width(rateColW)
		head("Played").Width(playedColW)
		head("Searcher took it").Width(searchColW)
	})
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
}

// cardRow is one card's numbers. total is the build's games, which the card's own count is
// shown against when it was in fewer.
func (v *view) cardRow(c *ui.Context, cs stats.CardStats, copies, total int, avg float64, cut bool) {
	t := c.Theme()
	card := v.snap.card(cs.Card)
	row := ui.Row(c).Gap(12).Padding(6, 8).AlignItems(ui.Center)
	if cut {
		row.Opacity(0.7)
	}
	row.Children(func() {
		ui.Row(c).Grow(1).Basis(0).Gap(10).AlignItems(ui.Center).MinWidth(0).Children(func() {
			v.cardArt(c, cs.Card, 30, 42)
			ui.Column(c).Gap(1).MinWidth(0).Children(func() {
				ui.Text(c, card.Name).FontSize(13).FontWeight(600).SingleLine()
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
		rateCell(c.Key("open"), cs.Opening, avg)
		if cost := max(card.Cost, 0); cost <= cheapCost {
			sameAsOpening(c, cost)
		} else {
			turn := curveText(cost)
			rateCell(c.Key("on"), cs.OnCurve, avg).Tooltip("In hand by " + turn)
			rateCell(c.Key("off"), cs.OffCurve, avg).Tooltip("Not in hand by " + turn)
		}
		playedCell(c, cs)
		search := "–"
		if cs.Shown > 0 {
			search = fmt.Sprintf("%d of %d", cs.Taken, cs.Shown)
		}
		ui.Text(c, search).FontSize(13).Width(searchColW).TextColor(map[bool]ui.Color{true: t.Text, false: faintColor(t)}[cs.Shown > 0])
	})
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.3))
}

// rateCell is a win rate with its record, and a bar of where the rate likely is with a tick at
// avg, the build's own rate. Below MinGames the rate reads "few".
func rateCell(c *ui.Context, r stats.Record, avg float64) ui.Element {
	t := c.Theme()
	return ui.Column(c).Width(rateColW).Gap(4).Children(func() {
		if r.Games() == 0 {
			ui.Text(c, "–").FontSize(13).TextColor(faintColor(t))
			return
		}
		ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
			if rate, ok := r.Rate(); ok {
				ui.Text(c, pct(rate)).FontSize(13).FontWeight(600)
			} else {
				ui.Text(c, "few").FontSize(13).TextColor(faintColor(t))
			}
			ui.Text(c, record(r)).FontSize(11).TextColor(faintColor(t))
		})
		lo, hi := r.LikelyRange()
		tip := fmt.Sprintf("Likely %s–%s from %s. The build wins %s.", pct(lo), pct(hi), record(r), pct(avg))
		ui.Box(c).Size(rangeBarW, 5).Radius(3).Background(t.Surface).Tooltip(tip).Children(func() {
			ui.Box(c).Absolute().Left(float32(lo)*rangeBarW).Top(0).Size(max(float32(hi-lo)*rangeBarW, 2), 5).
				Radius(3).Background(t.Accent.Alpha(0.7))
			ui.Box(c).Absolute().Left(float32(avg)*rangeBarW-1).Top(-3).Size(2, 11).Background(t.Text)
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
	ui.Column(c).Width(playedColW).Gap(4).Children(func() {
		ui.Row(c).Size(playedColW-20, 8).Radius(4).Clip().Background(t.Surface).Children(func() {
			for i, p := range parts {
				if n := p.r.Games(); n > 0 && total > 0 {
					ui.Box(c.Key(i)).Width(float32(n) / float32(total) * (playedColW - 20)).Height(8).Background(p.col)
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
	ui.Text(c, "same as opening hand").FontSize(12).TextColor(faintColor(t)).Width(2*rateColW + 12).SingleLine().
		Tooltip(fmt.Sprintf("A cost %d card can be paid for by your first or second turn, so this is "+
			"the opening hand plus at most one draw.", cost))
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
