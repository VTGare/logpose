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

// tile is a stat tile: a muted label over its content, on the surface color.
func tile(c *ui.Context, label string, body func()) ui.Element {
	t := c.Theme()
	return ui.Column(c).Gap(6).Padding(12).Radius(8).Background(t.Surface).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
		body()
	})
}

// rateTile shows a win rate, or "too few" below stats.MinGames, with a line under it.
func rateText(c *ui.Context, r stats.Record, size float32) {
	t := c.Theme()
	if rate, ok := r.Rate(); ok {
		ui.Text(c, pct(rate)).FontSize(size).Bold()
		return
	}
	ui.Text(c, "too few").FontSize(size*0.7).TextColor(t.TextMuted).Margin(size*0.15, 0)
}

// bar is a thin bar filled to a fraction.
func bar(c *ui.Context, fraction float64, color ui.Color) {
	t := c.Theme()
	ui.Box(c).Height(4).Radius(2).Background(t.Border).Draw(func(p *ui.Painter, r ui.Rect) {
		w := r.W * float32(min(max(fraction, 0), 1))
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H}, color, 2)
	})
}

// turnOrderTile is the first/second tile: both turn orders as equal halves.
func turnOrderTile(c *ui.Context, o stats.TurnOrder) ui.Element {
	t := c.Theme()
	half := func(label string, r stats.Record, color ui.Color) {
		ui.Column(c).Grow(1).Basis(0).Gap(6).Children(func() {
			ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
			rateText(c, r, 24)
			rate, _ := r.Rate()
			bar(c, rate, color)
			ui.Text(c, games(r.Games())).FontSize(12).TextColor(t.TextMuted)
		})
	}
	return tile(c, "Turn order", func() {
		ui.Row(c).Gap(14).Children(func() {
			half("First", o.First, firstColor)
			ui.Box(c).Width(1).Background(t.Border)
			half("Second", o.Second, secondColor)
		})
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
