package gui

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

func pct(rate float64) string { return fmt.Sprintf("%.0f%%", math.Round(rate*100)) }

// record is written "4–7", with an en dash.
func record(r stats.Record) string { return fmt.Sprintf("%d–%d", r.Wins, r.Losses) }

func games(n int) string {
	if n == 1 {
		return "1 game"
	}
	return fmt.Sprintf("%d games", n)
}

// tile is a panel with a title over its content.
func tile(c *ui.Context, label string, body func()) ui.Element {
	return titledTile(c, label, "", body)
}

// titledTile is a tile with a muted note at the right of its title, such as a count.
func titledTile(c *ui.Context, label, note string, body func()) ui.Element {
	return tagTile(c, label, "", note, body)
}

// tagTile is a titled tile whose title names the games it counts after a dot, such as
// "Matchups · Wano/Miracle".
func tagTile(c *ui.Context, label, tag, note string, body func()) ui.Element {
	t := c.Theme()
	return panel(c).Gap(8).Padding(14, 16).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(13).FontWeight(600)
			if tag != "" {
				ui.Text(c, "· "+tag).FontSize(12).TextColor(t.TextMuted).SingleLine()
			}
			if note != "" {
				ui.Spacer(c)
				ui.Text(c, note).FontSize(12).TextColor(t.TextMuted)
			}
		})
		body()
	})
}

// panel is the raised surface tiles sit on: a step above the page, with a hairline edge and a
// soft shadow.
func panel(c *ui.Context) ui.Element {
	t := c.Theme()
	return ui.Column(c).Radius(12).Background(panelColor(t)).Border(1, t.Border.Alpha(0.6)).
		Shadow(0, 6, 20, 0, panelShade)
}

// splitBar is the share won in the win color and the rest in the loss color.
// It grows from the left when the page opens, and slides when a filter changes it.
func splitBar(c *ui.Context, won float64) {
	won = min(max(won, 0), 1)
	box := ui.Box(c).Height(6).Radius(3).Clip()
	shown := grow(box, "won", float32(won), barTime)
	full := grow(box, "full", 1, barTime)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		w := r.W * shown
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H}, winColor, 0)
		p.Fill(ui.Rect{X: r.X + w, Y: r.Y, W: r.W*full - w, H: r.H}, lossColor.Alpha(0.75), 0)
	})
}

// turnOrderTile is both turn orders as rows. Clicking a row calls pick with its turn order, and
// the row chosen is tinted.
func turnOrderTile(c *ui.Context, tag string, o stats.TurnOrder, chosen stats.TurnOrderFilter, pick func(stats.TurnOrderFilter)) ui.Element {
	return tagTile(c, "Turn order", tag, "", func() { turnRows(c, o, chosen, pick) })
}

// turnRows is a row for going first and one for going second, each with a filled or hollow mark,
// the share won as a bar with 50% marked, the rate and the record. Turn order has no color of
// its own, so the marks tell the rows apart.
func turnRows(c *ui.Context, o stats.TurnOrder, chosen stats.TurnOrderFilter, pick func(stats.TurnOrderFilter)) {
	t := c.Theme()
	row := func(f stats.TurnOrderFilter, word, label string, r stats.Record, filled bool) {
		el := ui.Row(c.Key(word)).Gap(10).AlignItems(ui.Center).Padding(5, 6).Margin(0, -6).Radius(6).
			Label("Show games going " + word)
		if f == chosen {
			el = el.Background(t.Accent.Alpha(0.18))
		}
		if el.Clicked() {
			pick(f)
		}
		el.Children(func() {
			mark := ui.Box(c).Size(9, 9).Radius(4.5)
			if filled {
				mark.Background(t.Text)
			} else {
				mark.Border(1.5, t.Text)
			}
			ui.Text(c, label).FontWeight(600).Width(28)
			track := ui.Box(c).Grow(1).Height(6).Radius(3).Background(t.Surface)
			won := grow(track, "won", float32(winShare(r)), barTime)
			track.Draw(func(p *ui.Painter, rect ui.Rect) {
				p.Fill(ui.Rect{X: rect.X, Y: rect.Y, W: rect.W * won, H: rect.H}, t.Text.Alpha(0.75), 3)
				p.Fill(ui.Rect{X: rect.X + rect.W/2 - 0.5, Y: rect.Y - 3, W: 1, H: rect.H + 6}, t.TextMuted, 0)
			})
			if rate, ok := r.Rate(); ok {
				ui.Text(c, pct(rate)).FontSize(16).Bold().Width(44).TextAlign(ui.End)
			} else {
				ui.Text(c, "too few").FontSize(11).TextColor(faintColor(t)).Width(44).TextAlign(ui.End)
			}
			ui.Text(c, record(r)).FontSize(12).TextColor(t.TextMuted).Width(40).TextAlign(ui.End)
		})
	}
	ui.Column(c).Gap(4).Children(func() {
		row(stats.GoingFirst, "first", "1st", o.First, true)
		row(stats.GoingSecond, "second", "2nd", o.Second, false)
	})
}

// resultSquares shows results as small colored squares, oldest first.
func resultSquares(c *ui.Context, rs []stats.Result) {
	ui.Row(c).Gap(4).Wrap().Children(func() {
		for i, r := range rs {
			ui.Box(c.Key(i)).Size(16, 16).Radius(4).Background(resultColor(r))
		}
	})
}

// leaderChip is a small block in the leader's colors.
func (v *view) leaderChip(c *ui.Context, code string, size float32) {
	cols := colorsOf(v.snap.card(code))
	ui.Row(c).Size(size, size).Radius(size / 4).Clip().AlignItems(ui.Stretch).Children(func() {
		for i, col := range cols {
			ui.Box(c.Key(i)).Grow(1).Background(col.Card)
		}
	})
}

// leaderName is the leader's chip and name on one line.
func (v *view) leaderName(c *ui.Context, code string) {
	ui.Row(c).Gap(6).Children(func() {
		v.leaderChip(c, code, 12)
		ui.Text(c, v.snap.card(code).Name).SingleLine()
		ui.Text(c, setOf(code)).FontSize(11).TextColor(c.Theme().TextMuted)
	})
}

func ago(now, then time.Time) string {
	d := now.Sub(then)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	default:
		return then.Format("Jan 2")
	}
}

func duration(d time.Duration) string {
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %02dm", h, m)
}
