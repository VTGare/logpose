package gui

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/VTGare/logpose/internal/stats"
)

// overviewPage is the headline numbers and every day played by week on the left, and the decks
// and the leaders faced this week on the right.
func (v *view) overviewPage(c *ui.Context) {
	o := v.scoped().overview
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			v.headline(c, o)
			ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Gap(16).Children(func() {
					v.daysTile(c, o.Days)
				})
				ui.Column(c).Width(340).Gap(16).Children(func() {
					v.decksTile(c, o.Decks)
					v.facedTile(c, o.FacedThisWeek)
				})
			})
		})
	})
}

// headline is this week, the streak, the whole period and today, on the banner of the deck
// played last.
func (v *view) headline(c *ui.Context, o stats.Overview) {
	t := c.Theme()
	muted := func(text string) { ui.Text(c, text).FontSize(12).TextColor(t.TextMuted) }
	none := func(text string) { ui.Text(c, text).FontSize(28).Bold().TextColor(faintColor(t)) }
	big := func(text string, col ui.Color) { ui.Text(c, text).FontSize(28).Bold().TextColor(col) }
	block := func(i int, label string, body func()) {
		el := ui.Column(c.Key(label)).Grow(1).Basis(0).Gap(6).Padding(0, 18)
		if i > 0 {
			el.BorderStartWidth(1).BorderColor(t.Border)
		} else {
			el.Padding(0, 18, 0, 0)
		}
		el.Children(func() {
			ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
			body()
		})
	}
	var deck stats.Deck
	for _, d := range o.Decks {
		if d.Leader == o.Deck {
			deck = d
		}
	}
	v.banner(c, o.Deck, 170, func() {
		ui.Text(c, "Current deck").FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
			ui.Text(c, v.snap.card(o.Deck).Name).FontSize(22).Bold()
			ui.Text(c, fmt.Sprintf("%s · %s", setOf(o.Deck), games(deck.Record.Games()))).FontSize(13).TextColor(t.TextMuted).Margin(0, 0, 3, 0)
		})
		ui.Row(c).Margin(14, 0, 0, 0).AlignItems(ui.Stretch).Children(func() {
			block(0, "This week", func() {
				if o.Week.Games() == 0 {
					none("–")
					muted("No games")
					return
				}
				big(rateOrTooFew(o.Week), t.Text)
				change, d := weekChange(o)
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					muted(record(o.Week) + " ·")
					switch {
					case d > 0:
						ui.Icon(c, iconTrendUp).FontSize(12).TextColor(t.TextMuted).Label("Up")
					case d < 0:
						ui.Icon(c, iconTrendDown).FontSize(12).TextColor(t.TextMuted).Label("Down")
					}
					muted(change)
				})
			})
			block(1, "Streak", func() {
				if o.Streak == 0 {
					none("–")
					muted("None")
					return
				}
				big(fmt.Sprintf("%s%d", resultLetter(o.StreakResult), o.Streak), resultColor(o.StreakResult))
				muted(streakText(o))
			})
			block(2, v.periodTitle(), func() {
				if o.Total.Games() == 0 {
					none("–")
					muted("No games")
					return
				}
				big(rateOrTooFew(o.Total), t.Text)
				muted(record(o.Total))
			})
			block(3, "Today", func() {
				if len(o.Today) == 0 {
					none("–")
					muted("None yet")
					return
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					big(record(o.TodayRecord), t.Text)
					resultSquares(c, o.Today)
				})
				muted(fmt.Sprintf("%s · %s", games(len(o.Today)), duration(o.TodayTime)))
			})
		})
	})
}

func streakText(o stats.Overview) string {
	word := map[bool]string{true: "win", false: "wins"}[o.Streak == 1]
	if o.StreakResult == stats.Loss {
		word = map[bool]string{true: "loss", false: "losses"}[o.Streak == 1]
	}
	return fmt.Sprintf("%d %s in a row", o.Streak, word)
}

// weekChange compares this week's win rate with last week's, when both have enough games, and
// returns the change in points, which the headline shows as an arrow.
func weekChange(o stats.Overview) (string, float64) {
	now, ok := o.Week.Rate()
	before, okBefore := o.PrevWeek.Rate()
	if !ok || !okBefore {
		return games(o.Week.Games()), 0
	}
	d := math.Round((now - before) * 100)
	switch {
	case d > 0:
		return fmt.Sprintf("%.0f pts", d), d
	case d < 0:
		return fmt.Sprintf("%.0f pts", -d), d
	default:
		return "same as last week", 0
	}
}

// shownDays is how many days played the Overview lists. Older ones are on the Matches page.
const shownDays = 14

// daysTile is the latest days played under a header for each week. Each game is its opponent's
// leader art over a bar in the result's color.
func (v *view) daysTile(c *ui.Context, days []stats.Day) {
	t := c.Theme()
	days = days[:min(len(days), shownDays)]
	tile(c, "By day", func() {
		if len(days) == 0 {
			ui.Text(c, "No games").TextColor(t.TextMuted)
			return
		}
		var week time.Time
		for i, d := range days {
			if ws := stats.WeekStart(d.Date); !ws.Equal(week) {
				week = ws
				v.weekHeader(c.Key(fmt.Sprint("w", i)), ws, days[i:])
			}
			v.dayRow(c.Key(i), d)
		}
	})
}

// weekHeader names the week and gives its record over the days that follow in it.
func (v *view) weekHeader(c *ui.Context, start time.Time, days []stats.Day) {
	t := c.Theme()
	var r stats.Record
	for _, d := range days {
		if !stats.WeekStart(d.Date).Equal(start) {
			break
		}
		r.Wins += d.Record.Wins
		r.Losses += d.Record.Losses
	}
	this := stats.WeekStart(v.snap.Now)
	name := fmt.Sprintf("%s – %s", start.Format("Jan 2"), start.AddDate(0, 0, 6).Format("Jan 2"))
	switch {
	case start.Equal(this):
		name = "This week"
	case start.Equal(this.AddDate(0, 0, -7)):
		name = "Last week"
	}
	ui.Row(c).Gap(8).Padding(8, 0, 2, 0).AlignItems(ui.Center).Children(func() {
		v.periodLink(c, name, "Show only this week", 12, 700, func() { v.setRange(start, start.AddDate(0, 0, 6)) })
		ui.Text(c, fmt.Sprintf("%s · %s", record(r), pct(winShare(r)))).FontSize(12).TextColor(t.TextMuted)
		ui.Box(c).Grow(1).Height(1).Background(t.Border)
	})
}

// periodLink is a day's or a week's name in By day, which sets the period to it. A size of
// zero keeps the theme's.
func (*view) periodLink(c *ui.Context, name, tip string, size float32, weight int, set func()) {
	t := c.Theme()
	el := ui.Row(c).Label(tip + ", " + name).Cursor(ui.CursorPointer).Tooltip(tip)
	if el.Clicked() {
		set()
	}
	hover := el.Hovered()
	el.Children(func() {
		txt := ui.Text(c, name).SingleLine().FontWeight(weight)
		if size > 0 {
			txt.FontSize(size)
		}
		if hover {
			txt.TextColor(t.Accent).Underline()
		}
	})
}

// winShare is the share of games won, however few.
func winShare(r stats.Record) float64 {
	if r.Games() == 0 {
		return 0
	}
	return float64(r.Wins) / float64(r.Games())
}

func (v *view) dayRow(c *ui.Context, d stats.Day) {
	t := c.Theme()
	row := ui.Row(c).Gap(12).AlignItems(ui.Start).Padding(8, 8).Margin(0, -8).Radius(8)
	if v.dayName(d.Date) == "Today" {
		row.Background(t.Surface)
	}
	row.Children(func() {
		ui.Column(c).Width(170).Gap(3).Children(func() {
			v.periodLink(c, v.dayName(d.Date), "Show only this day", 0, 500, func() { v.setRange(d.Date, d.Date) })
			ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
				ui.Text(c, record(d.Record)).FontSize(18).Bold()
				ui.Text(c, fmt.Sprintf("%s · %s", pct(winShare(d.Record)), duration(d.Played))).FontSize(12).TextColor(faintColor(t)).Margin(0, 0, 2, 0)
			})
			v.dayDecks(c, d)
		})
		ui.Row(c).Gap(6).Grow(1).Basis(0).Wrap().Children(func() {
			for i, m := range d.Matches {
				v.gameTile(c.Key(m.Fingerprint), m, i)
			}
		})
	})
}

func (v *view) dayName(date time.Time) string {
	today := time.Date(v.snap.Now.Year(), v.snap.Now.Month(), v.snap.Now.Day(), 0, 0, 0, 0, date.Location())
	switch {
	case date.Equal(today):
		return "Today"
	case date.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	}
	return date.Format("Mon Jan 2")
}

// dayDecks is a chip for each deck played that day.
func (v *view) dayDecks(c *ui.Context, d stats.Day) {
	seen := make(map[string]bool)
	ui.Row(c).Gap(4).Children(func() {
		for i, m := range d.Matches {
			if !seen[m.Me.Leader] {
				seen[m.Me.Leader] = true
				v.artStrip(c.Key(i), m.Me.Leader, 22, 16)
			}
		}
	})
}

// gameTile is one game: the top of the opponent's leader art, with its corner and bottom edge
// in the result's color. It opens the match.
func (v *view) gameTile(c *ui.Context, m stats.Match, i int) {
	t := c.Theme()
	leader := v.snap.card(m.Opp.Leader).Name
	el := ui.Column(c).Label("Open the match against " + m.Opp.Name)
	if el.Clicked() {
		v.openMatch(m)
	}
	lift(el.Transition(tileEnter(i)), 7)
	el.Width(gameTileW).Height(gameTileH).Clip().Children(func() {
		if art := v.art.get(m.Opp.Leader, artCrop); art != nil {
			ui.Image(c, art).Size(gameTileW, gameTileH).Fit(ui.Cover)
		} else {
			v.leaderChip(c, m.Opp.Leader, gameTileH)
		}
		col := resultColor(m.Result)
		ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) {
			var corner ui.Path
			corner.MoveTo(r.X+r.W-16, r.Y).LineTo(r.X+r.W, r.Y).LineTo(r.X+r.W, r.Y+16).Close()
			p.FillPath(&corner, col)
			p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - 3, W: r.W, H: 3}, col, 0)
		})
	})
	ui.TooltipBase(c, el, func(tip ui.Element) {
		tip.Padding(6, 10).Radius(6).Background(t.Background).Border(1, t.Border).Children(func() {
			ui.Text(c, gameTip(m, leader)).FontSize(12).NoWrap()
		})
	})
}

const (
	gameTileW = 52
	gameTileH = 38
)

func gameTip(m stats.Match, leader string) string {
	text := fmt.Sprintf("%s vs %s · %s", resultWord(m.Result), leader, m.Opp.Name)
	if o := turnOrderShort(m); o != "" {
		text += " · " + o
	}
	return fmt.Sprintf("%s · %d turns · %s", text, m.Turns, m.PlayedAt.Format("15:04"))
}

// decksTile is each deck played, latest first, as a short row that opens it on the Leaders
// page.
func (v *view) decksTile(c *ui.Context, decks []stats.Deck) {
	t := c.Theme()
	tile(c, "Your decks", func() {
		for _, d := range decks {
			card := v.snap.card(d.Leader)
			el := ui.Row(c.Key(d.Leader)).Gap(12).AlignItems(ui.Center).Padding(5, 6).Margin(0, -6).Label("Show " + card.Name).
				Transition(slide)
			if el.Hovered() {
				el.Background(t.Surface)
			}
			el.Radius(8)
			if el.Clicked() {
				v.openLeader(d.Leader)
			}
			el.Children(func() {
				v.artStrip(c, d.Leader, 52, 36)
				ui.Column(c).Grow(1).Basis(0).Gap(1).Children(func() {
					ui.Text(c, card.Name).FontWeight(600).SingleLine()
					ui.Text(c, fmt.Sprintf("%s · %s", games(d.Record.Games()), ago(v.snap.Now, d.Last))).FontSize(12).TextColor(t.TextMuted)
				})
				ui.Column(c).AlignItems(ui.End).Gap(1).Children(func() {
					if rate, ok := d.Record.Rate(); ok {
						ui.Text(c, pct(rate)).FontSize(17).Bold()
					} else {
						ui.Text(c, "too few").FontSize(11).TextColor(faintColor(t))
					}
					ui.Text(c, record(d.Record)).FontSize(12).TextColor(t.TextMuted)
				})
			})
		}
	})
}

// facedTile is the leaders faced this week, most played first, with the share won as a bar. A
// row narrows every page to games against that leader.
func (v *view) facedTile(c *ui.Context, faced []stats.LeaderRecord) {
	t := c.Theme()
	n := 0
	for _, lr := range faced {
		n += lr.Record.Games()
	}
	titledTile(c, "Facing most this week", games(n), func() {
		if len(faced) == 0 {
			ui.Text(c, "No games this week").TextColor(t.TextMuted)
			return
		}
		for _, lr := range faced[:min(len(faced), 5)] {
			name := v.snap.card(lr.Leader).Name
			el := ui.Row(c.Key(lr.Leader)).Gap(12).AlignItems(ui.Center).Padding(4, 6).Margin(0, -6).Radius(8).
				Label("Show games vs " + name).Transition(slide)
			if el.Hovered() {
				el.Background(t.Surface)
			}
			if el.Clicked() {
				v.filter.oppLeader = lr.Leader
			}
			el.Children(func() {
				v.artStrip(c, lr.Leader, 44, 30)
				ui.Text(c, name).FontWeight(500).SingleLine().Grow(1).Basis(0)
				ui.Box(c).Width(80).Children(func() { splitBar(c, winShare(lr.Record)) })
				ui.Text(c, record(lr.Record)).Bold().Width(36).TextAlign(ui.End)
			})
		}
	})
}

// artStrip is the top of a leader's art, or the leader's colors while it loads.
func (v *view) artStrip(c *ui.Context, code string, w, h float32) {
	if art := v.art.get(code, artCrop); art != nil {
		ui.Image(c, art).Size(w, h).Fit(ui.Cover).Radius(4)
		return
	}
	ui.Row(c).Size(w, h).Radius(4).Clip().AlignItems(ui.Stretch).Children(func() {
		for i, col := range colorsOf(v.snap.card(code)) {
			ui.Box(c.Key(i)).Grow(1).Background(col.Card)
		}
	})
}

// periodTitle names the whole of the filtered games: the period, or the format when the
// period is all time.
func (v *view) periodTitle() string {
	if fr, ok := v.formatRange(); ok && v.filter.period == 0 {
		return "All of " + fr.Name
	}
	return v.periodLabel()
}

// bannerArtW is the width of the art on the left of a leader banner.
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

func rateOrTooFew(r stats.Record) string {
	if rate, ok := r.Rate(); ok {
		return pct(rate)
	}
	return "too few"
}

// rollingChart is the rolling win rate on an axis that fits it, with 50% marked, over every game
// as a win or loss square, grouped by day. games are oldest first, and sel is the point under
// the pointer.
func (*view) rollingChart(c *ui.Context, pts []stats.Point, games []stats.Match, sel *int) {
	t := c.Theme()
	label := fmt.Sprintf("Win rate, last %d games", stats.RollingWindow)
	tile(c, label, func() {
		if len(pts) < 2 || len(games) == 0 {
			ui.Text(c, fmt.Sprintf("Shows up after %d games.", stats.RollingWindow+1)).TextColor(t.TextMuted)
			return
		}
		n := len(games)
		gx := func(i int) float32 { return (float32(i) + 0.5) / float32(n) }
		lo := 1.0
		for _, p := range pts {
			lo = min(lo, p.Rate)
		}
		lo = max(0, math.Floor((lo-0.05)*5)/5)
		const height, band = 190, 26
		// yAt is a rate's height up the plot as a fraction, above the band of squares.
		yAt := func(rate float64) float32 {
			return (band + float32((rate-lo)/(1-lo))*(height-band-8)) / height
		}
		xs := make([]float32, len(pts))
		ys := make([]float32, len(pts))
		for i, p := range pts {
			xs[i], ys[i] = gx(p.Game-1), yAt(p.Rate)
		}
		var left []chart.Tick
		for r := lo; r <= 1.0001; r += 0.2 {
			left = append(left, chart.Tick{At: yAt(r), Label: pct(r)})
		}
		days := dayRuns(games)
		var bottom []chart.Tick
		for _, d := range days {
			at := (gx(d.start) + gx(d.end-1)) / 2
			// Labels closer than this would run into each other.
			if len(bottom) > 0 && at-bottom[len(bottom)-1].At < 0.09 {
				continue
			}
			bottom = append(bottom, chart.Tick{At: at, Label: d.date.Format("Jan 2")})
		}
		describe := func(i int) string {
			p := pts[i]
			return fmt.Sprintf("%s · game %d · %s, %d of the last %d won", p.At.Format("Jan 2 15:04"), p.Game, pct(p.Rate), p.Wins, stats.RollingWindow)
		}
		chart.Plot(c, chart.Axes{Left: left, Bottom: bottom, Label: axisLabel(c)}, func(area ui.Element) {
			drawn := grow(area, "draw", 1, drawTime)
			area.Label(label).Draw(func(p *ui.Painter, r ui.Rect) {
				for k, d := range days {
					if k%2 == 1 {
						x0, x1 := r.X+r.W*float32(d.start)/float32(n), r.X+r.W*float32(d.end)/float32(n)
						p.Fill(ui.Rect{X: x0, Y: r.Y, W: x1 - x0, H: r.H}, ui.RGBA(255, 255, 255, 0.025), 0)
					}
				}
				for _, tick := range left {
					p.Line(r.X, chart.Y(r, tick.At), r.X+r.W, chart.Y(r, tick.At), 1, t.Border.Alpha(0.6))
				}
				if lo < 0.5 {
					y := chart.Y(r, yAt(0.5))
					for x := r.X; x < r.X+r.W; x += 8 {
						p.Line(x, y, min(x+4, r.X+r.W), y, 1, t.TextMuted)
					}
				}
				p.Clip(ui.Rect{X: r.X - 4, Y: r.Y - 4, W: (r.W + 8) * drawn, H: r.H + 8}, 0, func() {
					p.StrokePath(chart.Line(r, xs, ys, chart.CurveLinear), 2.25, youColor)
				})
				size := min(11, r.W/float32(n)-2)
				for i, m := range games {
					cx := r.X + r.W*gx(i)
					p.Fill(ui.Rect{X: cx - size/2, Y: r.Y + r.H - 4 - size, W: size, H: size}, resultColor(m.Result), 2)
				}
			})
			last := len(pts) - 1
			chart.Point(c, xs[last], ys[last], 8, 8).Radius(4).Background(youColor)
			chart.Anchor(c, xs[last], ys[last], ui.End, ui.End, func() {
				ui.Text(c, pct(pts[last].Rate)).FontSize(12).Bold().Margin(0, 6, 4, 0)
			})
			chart.Select(c, area, sel, xs, describe)
			if i := *sel; i >= 0 && i < len(pts) {
				chart.RuleX(c, xs[i], 1).Background(t.TextMuted)
				chart.Point(c, xs[i], ys[i], 10, 10).Radius(5).Background(youColor)
				align := ui.Center
				switch {
				case xs[i] < 0.2:
					align = ui.Start
				case xs[i] > 0.8:
					align = ui.End
				}
				chart.Anchor(c, xs[i], 1, align, ui.Start, func() {
					ui.Text(c, describe(i)).NoWrap().FontSize(12).Padding(4, 8).Radius(6).Background(t.Surface).Border(1, t.Border)
				})
			}
		}).Height(height).Gap(6).Margin(6, 0, 0, 0)
	})
}

// dayRun is the games of one day, as indexes into the games oldest first.
type dayRun struct {
	date       time.Time
	start, end int
}

func dayRuns(games []stats.Match) []dayRun {
	var out []dayRun
	for i, m := range games {
		y, mo, d := m.PlayedAt.Date()
		day := time.Date(y, mo, d, 0, 0, 0, 0, m.PlayedAt.Location())
		if len(out) == 0 || !out[len(out)-1].date.Equal(day) {
			out = append(out, dayRun{date: day, start: i})
		}
		out[len(out)-1].end = i + 1
	}
	return out
}

// cardThumb is a whole card at the given width, or a block in its colors while the image
// loads or when there isn't one.
func (v *view) cardThumb(c *ui.Context, code string, width float32) {
	h := width * 7 / 5
	art := v.art.get(code, artFull)
	if art == nil {
		cols := colorsOf(v.snap.card(code))
		ui.Row(c).Size(width, h).Radius(5).Clip().AlignItems(ui.Stretch).Children(func() {
			for i, col := range cols {
				ui.Box(c.Key(i)).Grow(1).Background(col.Card)
			}
		})
		return
	}
	img := lift(ui.Image(c, art).Size(width, h).Fit(ui.Cover), 5)
	if width < previewMin {
		pw := previewWidth(c)
		ui.TooltipBase(c, img, func(tip ui.Element) {
			tip.Radius(10).Shadow(0, 6, 24, 0, ui.RGBA(0, 0, 0, 0.5)).Children(func() {
				ui.Image(c, v.largeArt(code, art)).Size(pw, pw*7/5).Fit(ui.Cover).Radius(10)
			})
		})
	}
}

// largeArt is the full-size card, or small while it loads. The sim's small images are 120 px
// wide and its full ones 480 px.
func (v *view) largeArt(code string, small *ui.Bitmap) *ui.Bitmap {
	if large := v.art.get(code, artLarge); large != nil {
		return large
	}
	return small
}

// previewWidth is the width of the card shown while the pointer rests on a small one. It takes
// about 40% of the window's height, and stays wide enough to read the card's text.
func previewWidth(c *ui.Context) float32 {
	const smallest, largest = 180, 300
	_, h := c.Size()
	return min(max(h*0.4*5/7, smallest), largest)
}

// previewMin is the card width from which a card is big enough to need no preview.
const previewMin = 150

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
