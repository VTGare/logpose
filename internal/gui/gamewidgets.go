package gui

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
)

// tooFew stands in for a rate from fewer than stats.MinGames games.
const tooFew = "too few"

func rateText(r stats.Record) string {
	if rate, ok := r.Rate(); ok {
		return pct(rate)
	}
	return tooFew
}

// sortRate is the rate a table sorts by. It's 0 below MinGames, so those rows keep their order
// among themselves.
func sortRate(r stats.Record) float64 {
	x, _ := r.Rate()
	return x
}

// rateRecord is a rate with its record, as "76% (28–9)", or "–" without games.
func rateRecord(r stats.Record) string {
	if r.Games() == 0 {
		return "–"
	}
	return fmt.Sprintf("%s (%s)", rateText(r), record(r))
}

// rateCell is the rate with the record beside it. Below MinGames the rate is greyed out, and
// without games the cell is "–".
func rateCell(c *ui.Context, r stats.Record) {
	t := c.Theme()
	if r.Games() == 0 {
		ui.Text(c, "–").FontSize(13).TextColor(faintColor(t))
		return
	}
	ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
		if rate, ok := r.Rate(); ok {
			ui.Text(c, pct(rate)).FontSize(13).FontWeight(600).SingleLine()
		} else {
			ui.Text(c, tooFew).FontSize(13).TextColor(faintColor(t)).SingleLine()
		}
		ui.Text(c, record(r)).FontSize(11).TextColor(faintColor(t)).SingleLine()
	})
}

// pointsApart is how many points rate a is above rate b, rounded.
func pointsApart(a, b float64) float64 { return math.Round((a - b) * 100) }

// trend is an arrow up or down for a change of d points, none for no change, then text.
func trend(c *ui.Context, d float64, text string) {
	t := c.Theme()
	switch {
	case d > 0:
		ui.Icon(c, iconTrendUp).FontSize(12).TextColor(t.TextMuted).Label("Up")
	case d < 0:
		ui.Icon(c, iconTrendDown).FontSize(12).TextColor(t.TextMuted).Label("Down")
	}
	ui.Text(c, text).FontSize(12).TextColor(t.TextMuted)
}

// splitBar is the share won in the win color and the rest in the loss color.
// It grows from the left when the page opens, and slides when a filter changes it.
func (v *view) splitBar(c *ui.Context, won float64) {
	won = min(max(won, 0), 1)
	box := ui.Box(c).Height(6).Radius(3).Clip()
	shown := v.grow(box, "won", float32(won), barTime)
	full := v.grow(box, "full", 1, barTime)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		w := r.W * shown
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H}, winColor(c.Theme()), 0)
		p.Fill(ui.Rect{X: r.X + w, Y: r.Y, W: r.W*full - w, H: r.H}, lossColor(c.Theme()).Alpha(0.75), 0)
	})
}

// turnRows is a row for going first and one for going second, each with a filled or hollow mark,
// the share won as a bar with 50% marked, the rate and the record. Turn order has no color of
// its own, so the marks tell the rows apart.
func (v *view) turnRows(c *ui.Context, o stats.TurnOrder, chosen stats.TurnOrderFilter, pick func(stats.TurnOrderFilter)) {
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
			won := v.grow(track, "won", float32(r.Share()), barTime)
			track.Draw(func(p *ui.Painter, rect ui.Rect) {
				p.Fill(ui.Rect{X: rect.X, Y: rect.Y, W: rect.W * won, H: rect.H}, t.Text.Alpha(0.75), 3)
				p.Fill(ui.Rect{X: rect.X + rect.W/2 - 0.5, Y: rect.Y - 3, W: 1, H: rect.H + 6}, t.TextMuted, 0)
			})
			if rate, ok := r.Rate(); ok {
				ui.Text(c, pct(rate)).FontSize(16).Bold().Width(44).TextAlign(ui.End)
			} else {
				ui.Text(c, tooFew).FontSize(11).TextColor(faintColor(t)).Width(44).TextAlign(ui.End)
			}
			ui.Text(c, record(r)).FontSize(12).TextColor(t.TextMuted).Width(40).TextAlign(ui.End)
		})
	}
	ui.Column(c).Gap(4).Children(func() {
		row(stats.GoingFirst, "first", "1st", o.First, true)
		row(stats.GoingSecond, "second", "2nd", o.Second, false)
	})
}

// resultSquares are the results as squares of their color, in the order given. The gaps and
// corners grow with the size.
func resultSquares(c *ui.Context, rs []stats.Result, size float32) ui.Element {
	gap := float32(math.Round(float64(size) / 4))
	return ui.Row(c).Gap(gap).Wrap().Children(func() {
		for i, r := range rs {
			ui.Box(c.Key(i)).Size(size, size).Radius(gap).Background(resultColor(c.Theme(), r))
		}
	})
}

func resultBadge(c *ui.Context, r stats.Result) {
	ui.Text(c, resultLetter(r)).FontSize(11).Bold().TextColor(ui.Hex("#0B1209")).TextAlign(ui.Center).
		Size(20, 20).Radius(5).Background(resultColor(c.Theme(), r)).Padding(2, 0)
}

// turnOrderShort is "1st" or "2nd", or "" when the log doesn't say.
func turnOrderShort(m stats.Match) string {
	switch {
	case !m.TurnOrderKnown:
		return ""
	case m.WentFirst:
		return "1st"
	default:
		return "2nd"
	}
}

// turnMark is 1st with a filled mark or 2nd with a hollow one, or nothing when the log doesn't
// say.
func turnMark(c *ui.Context, m stats.Match) {
	t := c.Theme()
	ui.Row(c).Gap(5).AlignItems(ui.Center).Width(40).Children(func() {
		if !m.TurnOrderKnown {
			return
		}
		mark := ui.Box(c).Size(8, 8).Radius(4)
		if m.WentFirst {
			mark.Background(t.TextMuted)
		} else {
			mark.Border(1.5, t.TextMuted)
		}
		ui.Text(c, turnOrderShort(m)).FontSize(12).TextColor(t.TextMuted)
	})
}

// gameLink makes el open the match on a click, with the game's menu on right-click.
func (v *view) gameLink(el ui.Element, m stats.Match) ui.Element {
	if el.Clicked() {
		v.openMatch(m)
	}
	return el.Label("Open the match against " + m.Opp.Name).PopoverMenu(v.gameMenu(m))
}

// gameLine is a row for one game that opens the match, lit while the pointer is over it.
func (v *view) gameLine(c *ui.Context, m stats.Match) ui.Element {
	t := c.Theme()
	row := ui.ButtonBase(c).Row().AlignItems(ui.Center).Radius(6).
		Transition(ui.ElementTransition{Duration: hoverTime, Colors: true})
	if row.Hovered() {
		row.Background(t.Surface.Alpha(0.5))
	} else {
		row.Background(ui.Transparent)
	}
	return v.gameLink(row, m)
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

func (v *view) leaderName(c *ui.Context, code string) {
	ui.Row(c).Gap(6).Children(func() {
		v.leaderChip(c, code, 12)
		ui.Text(c, v.snap.card(code).Name).SingleLine()
		ui.Text(c, cards.SetOf(code)).FontSize(11).TextColor(c.Theme().TextMuted)
	})
}

const bannerArtW = 300

// banner is a leader's art on the left, fading into the panel, with the leader's colors glowing
// faintly from two corners and body beside the art.
func (v *view) banner(c *ui.Context, code string, height float32, body func()) ui.Element {
	t := c.Theme()
	cols := colorsOf(v.snap.card(code))
	art := v.art.get(code, artPortrait)
	strength := 0.5
	if art != nil {
		strength = 0.3
	}
	return ui.Row(c).Height(height).Radius(12).Clip().Background(panelColor(t)).Border(1, t.Border.Alpha(0.6)).
		Shadow(0, 6, 20, 0, panelShade).AlignItems(ui.Stretch).Draw(func(p *ui.Painter, rect ui.Rect) {
		glow(p, rect, cols, strength)
	}).Children(func() {
		if art != nil {
			ui.Image(c, art).Width(bannerArtW).Fit(ui.Cover)
		} else {
			ui.Box(c).Width(bannerArtW / 3)
		}
		ui.Column(c).Grow(1).Basis(0).Padding(18, 22, 18, 4).Gap(2).Justify(ui.Center).Children(body)
	})
}

// glow lights the banner's dark panel with the leader's colors: the first from the bottom left
// and the second, or the first again, from the top right. Each is brightest in its corner and
// fades fast, so the text in the middle stays on the dark base. A color only reads as itself
// near full strength: yellow dimmed over dark turns olive.
func glow(p *ui.Painter, r ui.Rect, cols []leaderColor, strength float64) {
	second := cols[0]
	if len(cols) > 1 {
		second = cols[1]
	}
	radius := r.W * 0.78
	p.FillRadialGradient(r, light(cols[0].Glow, ui.Point{X: 0, Y: 1.1}, radius, strength), 0)
	p.FillRadialGradient(r, light(second.Glow, ui.Point{X: 1, Y: -0.1}, radius, strength), 0)
}

// light fades a color from strength at center to nothing at radius, along (1-t)^1.7.
func light(col ui.Color, center ui.Point, radius float32, strength float64) ui.RadialGradient {
	g := ui.RadialGradient{Center: center, Radius: radius}
	for _, t := range []float32{0, 0.2, 0.4, 0.6, 0.8, 1} {
		c := col
		c.A = uint8(255 * strength * math.Pow(float64(1-t), 1.7))
		g.Stops = append(g.Stops, ui.GradientStop{Color: c, At: t})
	}
	return g
}

// axisLabel builds tick labels that stay inside the plot. A label is centered on its tick, so
// the ones at the ends of the bottom axis would hang half outside. They line up with the end
// instead.
func axisLabel(c *ui.Context) func(chart.Side, chart.Tick) {
	col := faintColor(c.Theme())
	small := func(label string) ui.Element { return ui.Text(c, label).NoWrap().FontSize(11).TextColor(col) }
	return func(side chart.Side, t chart.Tick) {
		switch {
		case side == chart.Bottom && t.At > 0.95:
			// The invisible copy keeps the axis row as tall as the text, and the real one ends
			// at the tick.
			ui.Box(c).Width(0).Children(func() {
				small(t.Label).Invisible()
				small(t.Label).Absolute().Right(0).Top(0)
			})
		case side == chart.Bottom && t.At < 0.05:
			ui.Row(c).Width(0).Justify(ui.Start).Children(func() { small(t.Label) })
		default:
			small(t.Label)
		}
	}
}
