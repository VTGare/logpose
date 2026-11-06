package gui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/VTGare/logpose/internal/stats"
)

func (v *view) overviewPage(c *ui.Context) {
	o := v.snap.Overview
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			v.banner(c, o)
			ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
				turnOrderTile(c, o.TurnOrder).Grow(2).Basis(0)
				v.todayTile(c, o).Grow(1).Basis(0)
				v.streakTile(c, o).Grow(1).Basis(0)
			})
			v.rollingChart(c, o)
			ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
				v.facedTile(c, o).Grow(1).Basis(0)
				v.recentTile(c, o).Grow(1).Basis(0)
			})
			if len(o.OtherLeaders) > 0 {
				v.otherLeaders(c, o)
			}
		})
	})
}

// banner is the current deck: its leader's art beside a panel in the leader's colors.
func (v *view) banner(c *ui.Context, o stats.Overview) {
	leader := v.snap.card(o.Deck)
	cols := colorsOf(leader)
	ui.Row(c).Height(180).Radius(12).Clip().Background(cols[0].Deep).AlignItems(ui.Stretch).Children(func() {
		if art := v.art.get(o.Deck, artPortrait); art != nil {
			ui.Image(c, art).Width(300).Fit(ui.Cover)
		} else {
			ui.Box(c).Width(300).Background(cols[0].Mid)
		}
		ui.Column(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			ui.Row(c).Height(8).AlignItems(ui.Stretch).Children(func() {
				for i, col := range cols {
					ui.Box(c.Key(i)).Grow(1).Background(col.Mid)
				}
			})
			ui.Column(c).Grow(1).Padding(18, 22).Gap(4).Justify(ui.End).Children(func() {
				ui.Text(c, "Current deck").FontSize(12).TextColor(bannerTextMuted)
				ui.Text(c, leader.Name).FontSize(26).Bold().TextColor(bannerText)
				ui.Row(c).Gap(28).Margin(8, 0, 0, 0).Children(func() {
					bannerStat(c, "Win rate", rateOrTooFew(o.Record))
					bannerStat(c, "Record", record(o.Record))
					bannerStat(c, "Games", fmt.Sprint(o.Record.Games()))
				})
			})
		})
	})
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
			return
		}
		ui.Text(c, record(o.TodayRecord)).FontSize(24).Bold()
		resultSquares(c, o.Today)
		ui.Text(c, duration(o.TodayTime)+" played").FontSize(12).TextColor(t.TextMuted)
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
		word := "wins"
		if o.StreakResult == stats.Loss {
			word = "losses"
		}
		ui.Text(c, fmt.Sprintf("%d %s in a row", o.Streak, word)).FontSize(12).TextColor(t.TextMuted)
	})
}

func (*view) rollingChart(c *ui.Context, o stats.Overview) {
	t := c.Theme()
	label := fmt.Sprintf("Win rate, rolling %d games", stats.RollingWindow)
	tile(c, label, func() {
		if len(o.Rolling) < 2 {
			ui.Text(c, fmt.Sprintf("Shows up after %d games with this deck.", stats.RollingWindow+1)).TextColor(t.TextMuted)
			return
		}
		x := chart.NewTime(o.Rolling[0].At, o.Rolling[len(o.Rolling)-1].At)
		y := chart.NewLinear(0, 1)
		ticks := []chart.Tick{{At: 0, Label: "0%"}, {At: 0.5, Label: "50%"}, {At: 1, Label: "100%"}}
		xs := make([]float32, len(o.Rolling))
		ys := make([]float32, len(o.Rolling))
		for i, p := range o.Rolling {
			xs[i] = float32(x.At(p.At))
			ys[i] = float32(y.At(p.Rate))
		}
		chart.Plot(c, chart.Axes{Left: ticks, Bottom: x.Ticks(6)}, func(area ui.Element) {
			area.Label(label).Draw(func(p *ui.Painter, r ui.Rect) {
				for _, tick := range ticks {
					p.Line(r.X, chart.Y(r, tick.At), r.X+r.W, chart.Y(r, tick.At), 1, t.Border)
				}
				p.StrokePath(chart.Line(r, xs, ys, chart.CurveMonotone), 2, firstColor)
			})
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
			ui.Box(c.Key(i)).Grow(1).Background(col.Mid)
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

func (v *view) otherLeaders(c *ui.Context, o stats.Overview) {
	t := c.Theme()
	ui.Text(c, "Your other leaders").FontSize(12).TextColor(t.TextMuted)
	ui.Row(c).Gap(12).Wrap().Children(func() {
		for i, lr := range o.OtherLeaders {
			cols := colorsOf(v.snap.card(lr.Leader))
			ui.Column(c.Key(i)).Width(180).Radius(8).Clip().Background(t.Surface).AlignItems(ui.Stretch).Children(func() {
				ui.Row(c).Height(6).AlignItems(ui.Stretch).Children(func() {
					for j, col := range cols {
						ui.Box(c.Key(j)).Grow(1).Background(col.Mid)
					}
				})
				ui.Column(c).Padding(10, 12).Gap(2).Children(func() {
					ui.Text(c, v.snap.card(lr.Leader).Name).Bold().SingleLine()
					ui.Text(c, fmt.Sprintf("%s · %s", rateOrTooFew(lr.Record), games(lr.Record.Games()))).FontSize(12).TextColor(t.TextMuted)
				})
			})
		}
	})
}
