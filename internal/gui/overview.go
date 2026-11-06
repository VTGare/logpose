package gui

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/VTGare/logpose/internal/stats"
)

// overviewPage is the session and the week on the left, and the decks played and the latest
// games on the right.
func (v *view) overviewPage(c *ui.Context) {
	o := v.snap.Overview
	ui.Scroll(c).Fill().Children(func() {
		ui.Row(c).Padding(20).Gap(16).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Gap(16).Children(func() {
				ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
					v.todayTile(c, o).Grow(2).Basis(0)
					v.streakTile(c, o).Grow(1).Basis(0)
				})
				ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
					periodTile(c, "Last 7 days", o.Week, weekChange(o)).Grow(1).Basis(0)
					periodTile(c, "All time", o.Total, games(o.Total.Games())).Grow(1).Basis(0)
				})
				v.rollingChart(c, "Win rate, all leaders", o.Rolling, &v.rollingPoint)
				v.facedTile(c, o)
			})
			ui.Column(c).Width(340).Gap(16).Children(func() {
				tile(c, "Your decks", func() {
					for i, d := range o.Decks {
						v.deckRow(c.Key(i), d)
					}
				})
				v.recentTile(c, o)
			})
		})
	})
}

func periodTile(c *ui.Context, label string, r stats.Record, note string) ui.Element {
	t := c.Theme()
	return tile(c, label, func() {
		if r.Games() == 0 {
			ui.Text(c, "No games").TextColor(t.TextMuted)
			return
		}
		rateText(c, r, 24)
		ui.Text(c, record(r)).FontSize(12)
		if note != "" {
			ui.Text(c, note).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

// weekChange compares the last 7 days' win rate with the 7 days before, when both have enough
// games.
func weekChange(o stats.Overview) string {
	now, ok := o.Week.Rate()
	before, okBefore := o.PrevWeek.Rate()
	if !ok || !okBefore {
		return ""
	}
	d := math.Round((now - before) * 100)
	switch {
	case d > 0:
		return fmt.Sprintf("Up %.0f points on the week before", d)
	case d < 0:
		return fmt.Sprintf("Down %.0f points on the week before", -d)
	default:
		return "Same as the week before"
	}
}

// deckRow is one deck played: its art, name, games and win rate. It opens the leader on the
// Leaders page.
func (v *view) deckRow(c *ui.Context, d stats.Deck) {
	t := c.Theme()
	card := v.snap.card(d.Leader)
	el := ui.Row(c).Gap(10).Padding(4, 0).Label("Show " + card.Name)
	if el.Clicked() {
		v.openLeader(d.Leader)
	}
	el.Children(func() {
		if art := v.art.get(d.Leader, artCrop); art != nil {
			ui.Image(c, art).Size(72, 46).Fit(ui.Cover).Radius(6)
		} else {
			ui.Box(c).Size(72, 46).Radius(6).Background(bannerBase)
		}
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				v.leaderChip(c, d.Leader, 10)
				ui.Text(c, card.Name).Bold().SingleLine().Grow(1)
			})
			ui.Text(c, fmt.Sprintf("%s · %s", games(d.Record.Games()), ago(v.snap.Now, d.Last))).FontSize(12).TextColor(t.TextMuted)
		})
		ui.Column(c).Gap(2).AlignItems(ui.End).Children(func() {
			ui.Text(c, rateOrTooFew(d.Record)).Bold()
			ui.Text(c, record(d.Record)).FontSize(12).TextColor(t.TextMuted)
		})
	})
}

// bannerArtW is the width of the sharp art on the left of a leader banner.
const bannerArtW = 300

// leaderBanner is a leader's art over its own blurred backdrop, lit from the corners of the
// panel by the leader's card colors, with its record. Without art the panel is plain dark, and
// the glows are stronger.
func (v *view) leaderBanner(c *ui.Context, code, label string, r stats.Record) ui.Element {
	leader := v.snap.card(code)
	cols := colorsOf(leader)
	back := v.art.get(code, artBackdrop)
	strength := 0.95
	if back != nil {
		strength = 0.6
	}
	return ui.Row(c).Height(180).Radius(12).Clip().Background(bannerBase).AlignItems(ui.Stretch).Draw(func(p *ui.Painter, rect ui.Rect) {
		if back != nil {
			p.Image(back, rect, ui.Cover)
		}
		panel := ui.Rect{X: rect.X + bannerArtW, Y: rect.Y, W: rect.W - bannerArtW, H: rect.H}
		glow(p, panel, cols, strength)
	}).Children(func() {
		if art := v.art.get(code, artPortrait); art != nil {
			ui.Image(c, art).Width(bannerArtW).Fit(ui.Cover)
		} else {
			ui.Box(c).Width(bannerArtW)
		}
		ui.Column(c).Grow(1).Padding(18, 22).Gap(4).Justify(ui.End).Children(func() {
			ui.Text(c, label).FontSize(12).TextColor(bannerTextMuted)
			ui.Text(c, leader.Name).FontSize(26).Bold().TextColor(bannerText)
			ui.Row(c).Gap(28).Margin(8, 0, 0, 0).Children(func() {
				bannerStat(c, "Win rate", rateOrTooFew(r))
				bannerStat(c, "Record", record(r))
				bannerStat(c, "Games", fmt.Sprint(r.Games()))
			})
		})
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

func rateOrTooFew(r stats.Record) string {
	if rate, ok := r.Rate(); ok {
		return pct(rate)
	}
	return "too few"
}

func bannerStat(c *ui.Context, label, value string) {
	ui.Column(c).Gap(2).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(bannerTextMuted)
		ui.Text(c, value).FontSize(20).Bold().TextColor(bannerText)
	})
}

func (*view) todayTile(c *ui.Context, o stats.Overview) ui.Element {
	t := c.Theme()
	return tile(c, "Today", func() {
		if len(o.Today) == 0 {
			ui.Text(c, "No games yet").TextColor(t.TextMuted)
			if len(o.Last) > 0 {
				ui.Text(c, "Last 10 games").FontSize(12).TextColor(t.TextMuted)
				resultSquares(c, o.Last)
			}
			return
		}
		ui.Row(c).Gap(10).Children(func() {
			ui.Text(c, record(o.TodayRecord)).FontSize(24).Bold()
			if rate, ok := o.TodayRecord.Rate(); ok {
				ui.Text(c, pct(rate)).FontSize(16).TextColor(t.TextMuted)
			}
		})
		resultSquares(c, o.Today)
		ui.Text(c, fmt.Sprintf("%s over %s played", games(len(o.Today)), duration(o.TodayTime))).FontSize(12).TextColor(t.TextMuted)
	})
}

func (*view) streakTile(c *ui.Context, o stats.Overview) ui.Element {
	t := c.Theme()
	return tile(c, "Streak", func() {
		if o.Streak == 0 {
			ui.Text(c, "None").TextColor(t.TextMuted)
			return
		}
		ui.Text(c, fmt.Sprintf("%s%d", resultLetter(o.StreakResult), o.Streak)).FontSize(24).Bold().TextColor(resultColor(o.StreakResult))
		word := map[bool]string{true: "win", false: "wins"}[o.Streak == 1]
		if o.StreakResult == stats.Loss {
			word = map[bool]string{true: "loss", false: "losses"}[o.Streak == 1]
		}
		ui.Text(c, fmt.Sprintf("%d %s in a row", o.Streak, word)).FontSize(12).TextColor(t.TextMuted)
	})
}

// rollingChart is the rolling win rate. sel is the point under the pointer, kept per chart.
func (*view) rollingChart(c *ui.Context, title string, pts []stats.Point, sel *int) {
	t := c.Theme()
	label := fmt.Sprintf("%s, rolling %d games", title, stats.RollingWindow)
	tile(c, label, func() {
		if len(pts) < 2 {
			ui.Text(c, fmt.Sprintf("Shows up after %d games.", stats.RollingWindow+1)).TextColor(t.TextMuted)
			return
		}
		// Games, not time, along x: sessions are hours apart and games minutes apart, so a time
		// axis is mostly flat lines between sessions.
		x := chart.NewLinear(float64(pts[0].Game), float64(pts[len(pts)-1].Game))
		y := chart.NewLinear(0, 1)
		ticks := []chart.Tick{{At: 0, Label: "0%"}, {At: 0.5, Label: "50%"}, {At: 1, Label: "100%"}}
		xs := make([]float32, len(pts))
		ys := make([]float32, len(pts))
		for i, p := range pts {
			xs[i] = float32(x.At(float64(p.Game)))
			ys[i] = float32(y.At(p.Rate))
		}
		describe := func(i int) string {
			p := pts[i]
			return fmt.Sprintf("%s · game %d · %s, %d of the last %d won", p.At.Format("Jan 2 15:04"), p.Game, pct(p.Rate), p.Wins, stats.RollingWindow)
		}
		chart.Plot(c, chart.Axes{Left: ticks, Bottom: dateTicks(pts, xs, 5)}, func(area ui.Element) {
			area.Label(label).Draw(func(p *ui.Painter, r ui.Rect) {
				for _, tick := range ticks {
					p.Line(r.X, chart.Y(r, tick.At), r.X+r.W, chart.Y(r, tick.At), 1, t.Border)
				}
				p.StrokePath(chart.Line(r, xs, ys, chart.CurveMonotone), 2, firstColor)
			})
			chart.Select(c, area, sel, xs, describe)
			if i := *sel; i >= 0 && i < len(pts) {
				chart.RuleX(c, xs[i], 1).Background(t.TextMuted)
				chart.Point(c, xs[i], ys[i], 10, 10).Radius(5).Background(firstColor)
				align := ui.Center
				switch {
				case xs[i] < 0.2:
					align = ui.Start
				case xs[i] > 0.8:
					align = ui.End
				}
				chart.Anchor(c, xs[i], 1, align, ui.Start, func() {
					ui.Text(c, describe(i)).NoWrap().FontSize(12).Padding(4, 8).Radius(6).Background(t.Background).Border(1, t.Border)
				})
			}
		}).Height(170).Gap(6).Margin(8, 0, 0, 0)
	})
}

func (v *view) facedTile(c *ui.Context, o stats.Overview) ui.Element {
	t := c.Theme()
	return tile(c, "Facing most this week", func() {
		if len(o.FacedThisWeek) == 0 {
			ui.Text(c, "No games this week").TextColor(t.TextMuted)
			return
		}
		ui.Row(c).Gap(10).Children(func() {
			for i, lr := range o.FacedThisWeek[:min(len(o.FacedThisWeek), 5)] {
				ui.Column(c.Key(i)).Gap(4).Width(64).Children(func() {
					v.cardThumb(c, lr.Leader, 64)
					ui.Text(c, v.snap.card(lr.Leader).Name).FontSize(11).SingleLine()
					ui.Text(c, record(lr.Record)).FontSize(11).TextColor(t.TextMuted)
				})
			}
		})
	})
}

// cardThumb is a whole card at the given width, or a block in its colors while the image
// loads or when there isn't one.
func (v *view) cardThumb(c *ui.Context, code string, width float32) {
	h := width * 7 / 5
	if art := v.art.get(code, artFull); art != nil {
		ui.Image(c, art).Size(width, h).Fit(ui.Cover).Radius(5)
		return
	}
	cols := colorsOf(v.snap.card(code))
	ui.Row(c).Size(width, h).Radius(5).Clip().AlignItems(ui.Stretch).Children(func() {
		for i, col := range cols {
			ui.Box(c.Key(i)).Grow(1).Background(col.Card)
		}
	})
}

func (v *view) recentTile(c *ui.Context, o stats.Overview) ui.Element {
	t := c.Theme()
	return tile(c, "Recent matches", func() {
		for i, m := range o.Recent[:min(len(o.Recent), 6)] {
			ui.Row(c.Key(i)).Gap(8).Children(func() {
				ui.Text(c, resultLetter(m.Result)).Bold().TextColor(resultColor(m.Result)).Width(14)
				v.leaderChip(c, m.Opp.Leader, 10)
				ui.Text(c, v.snap.card(m.Opp.Leader).Name).SingleLine().Grow(1)
				ui.Text(c, turnOrderShort(m)).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, ago(v.snap.Now, m.PlayedAt)).FontSize(12).TextColor(t.TextMuted).Width(64).TextAlign(ui.End)
			})
		}
	})
}

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

// dateTicks labels about n evenly spread points with their dates.
func dateTicks(pts []stats.Point, xs []float32, n int) []chart.Tick {
	if len(pts) < 2 {
		return nil
	}
	var ticks []chart.Tick
	last := ""
	for k := range n {
		i := k * (len(pts) - 1) / (n - 1)
		label := pts[i].At.Format("Jan 2")
		if label == last {
			continue
		}
		last = label
		ticks = append(ticks, chart.Tick{At: xs[i], Label: label})
	}
	return ticks
}
