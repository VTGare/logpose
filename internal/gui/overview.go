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
		ui.Row(c).Padding(20).Gap(16).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Gap(16).Children(func() {
				v.headline(c, o)
				v.daysTile(c, o.Days)
			})
			ui.Column(c).Width(340).Gap(16).Children(func() {
				v.decksTile(c, o.Decks)
				v.facedTile(c, o.FacedThisWeek)
			})
		})
	})
}

// headline is today, the streak, this week and the whole period side by side in one tile.
func (v *view) headline(c *ui.Context, o stats.Overview) {
	t := c.Theme()
	block := func(label string, body func()) {
		ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
			ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
			body()
		})
	}
	muted := func(text string) { ui.Text(c, text).FontSize(12).TextColor(t.TextMuted) }
	none := func(text string) { ui.Text(c, text).FontSize(16).TextColor(t.TextMuted).Margin(6, 0) }
	sep := func() { ui.Box(c).Width(1).Background(t.Border) }
	ui.Row(c).Gap(16).Padding(14).Radius(8).Background(t.Surface).AlignItems(ui.Stretch).Children(func() {
		block("Today", func() {
			if len(o.Today) == 0 {
				none("No games yet")
				return
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, record(o.TodayRecord)).FontSize(24).Bold()
				resultSquares(c, o.Today)
			})
			muted(fmt.Sprintf("%s · %s", games(len(o.Today)), duration(o.TodayTime)))
		})
		sep()
		block("Streak", func() {
			if o.Streak == 0 {
				none("None")
				return
			}
			ui.Text(c, fmt.Sprintf("%s%d", resultLetter(o.StreakResult), o.Streak)).FontSize(24).Bold().TextColor(resultColor(o.StreakResult))
			muted(streakText(o))
		})
		sep()
		block("This week", func() {
			if o.Week.Games() == 0 {
				none("No games")
				return
			}
			rateLine(c, o.Week)
			muted(weekChange(o))
		})
		sep()
		block(v.periodTitle(), func() {
			if o.Total.Games() == 0 {
				none("No games")
				return
			}
			rateLine(c, o.Total)
			muted(games(o.Total.Games()))
		})
	})
}

// rateLine is a win rate with its record beside it.
func rateLine(c *ui.Context, r stats.Record) {
	ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
		rateText(c, r, 24)
		ui.Text(c, record(r)).FontSize(12).TextColor(c.Theme().TextMuted).Margin(0, 0, 4, 0)
	})
}

func streakText(o stats.Overview) string {
	word := map[bool]string{true: "win", false: "wins"}[o.Streak == 1]
	if o.StreakResult == stats.Loss {
		word = map[bool]string{true: "loss", false: "losses"}[o.Streak == 1]
	}
	return fmt.Sprintf("%d %s in a row", o.Streak, word)
}

// weekChange compares this week's win rate with last week's, when both have enough games.
func weekChange(o stats.Overview) string {
	now, ok := o.Week.Rate()
	before, okBefore := o.PrevWeek.Rate()
	if !ok || !okBefore {
		return games(o.Week.Games())
	}
	d := math.Round((now - before) * 100)
	switch {
	case d > 0:
		return fmt.Sprintf("▲ %.0f from last week", d)
	case d < 0:
		return fmt.Sprintf("▼ %.0f from last week", -d)
	default:
		return "Same as last week"
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
		ui.Text(c, name).FontSize(12).Bold().TextColor(t.TextMuted)
		ui.Text(c, fmt.Sprintf("%s · %s", record(r), pct(winShare(r)))).FontSize(12).TextColor(t.TextMuted)
		ui.Box(c).Grow(1).Height(1).Background(t.Border)
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
	row := ui.Row(c).Gap(12).AlignItems(ui.Start).Padding(4, 8).Margin(0, -8).Radius(6)
	if v.dayName(d.Date) == "Today" {
		row.Background(t.Background)
	}
	row.Children(func() {
		ui.Column(c).Width(150).Gap(2).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, v.dayName(d.Date)).SingleLine()
				v.dayDecks(c, d)
			})
			ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
				ui.Text(c, record(d.Record)).Bold()
				ui.Text(c, fmt.Sprintf("%s · %s", pct(winShare(d.Record)), duration(d.Played))).FontSize(12).TextColor(t.TextMuted)
			})
		})
		ui.Row(c).Gap(4).Grow(1).Basis(0).Wrap().Children(func() {
			for i, m := range d.Matches {
				v.gameTile(c.Key(i), m)
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
				v.leaderChip(c.Key(i), m.Me.Leader, 10)
			}
		}
	})
}

// gameTile is one game: the top of the opponent's leader art over a bar in the result's color.
// It opens the match.
func (v *view) gameTile(c *ui.Context, m stats.Match) {
	t := c.Theme()
	leader := v.snap.card(m.Opp.Leader).Name
	el := ui.Column(c).Gap(2).Label("Open the match against " + m.Opp.Name)
	if el.Clicked() {
		v.openMatch(m)
	}
	el.Children(func() {
		if art := v.art.get(m.Opp.Leader, artCrop); art != nil {
			ui.Image(c, art).Size(30, 20).Fit(ui.Cover).Radius(3)
		} else {
			v.leaderChip(c, m.Opp.Leader, 20)
		}
		ui.Box(c).Size(30, 3).Radius(1.5).Background(resultColor(m.Result))
	})
	ui.TooltipBase(c, el, func(tip ui.Element) {
		tip.Padding(6, 10).Radius(6).Background(t.Background).Border(1, t.Border).Children(func() {
			ui.Text(c, gameTip(m, leader)).FontSize(12).NoWrap()
		})
	})
}

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
		for i, d := range decks {
			card := v.snap.card(d.Leader)
			el := ui.Row(c.Key(i)).Gap(10).AlignItems(ui.Center).Padding(2, 0).Label("Show " + card.Name)
			if el.Clicked() {
				v.openLeader(d.Leader)
			}
			el.Children(func() {
				v.artStrip(c, d.Leader, 48, 30)
				ui.Column(c).Grow(1).Basis(0).Children(func() {
					ui.Text(c, card.Name).Bold().SingleLine()
					ui.Text(c, fmt.Sprintf("%s · %s", games(d.Record.Games()), ago(v.snap.Now, d.Last))).FontSize(11).TextColor(t.TextMuted)
				})
				if rate, ok := d.Record.Rate(); ok {
					ui.Text(c, pct(rate)).Bold()
				}
				ui.Text(c, record(d.Record)).FontSize(12).TextColor(t.TextMuted).Width(40).TextAlign(ui.End)
			})
		}
	})
}

// facedTile is the leaders faced this week, most played first, with the share won as a bar. A
// row narrows every page to games against that leader.
func (v *view) facedTile(c *ui.Context, faced []stats.LeaderRecord) {
	t := c.Theme()
	tile(c, "Facing most this week", func() {
		if len(faced) == 0 {
			ui.Text(c, "No games this week").TextColor(t.TextMuted)
			return
		}
		for i, lr := range faced[:min(len(faced), 5)] {
			name := v.snap.card(lr.Leader).Name
			el := ui.Row(c.Key(i)).Gap(8).AlignItems(ui.Center).Padding(2, 0).Label("Show games vs " + name)
			if el.Clicked() {
				v.filter.oppLeader = lr.Leader
			}
			el.Children(func() {
				v.artStrip(c, lr.Leader, 44, 28)
				ui.Text(c, name).SingleLine().Grow(1).Basis(0)
				ui.Text(c, record(lr.Record)).FontSize(12).TextColor(t.TextMuted).Width(34).TextAlign(ui.End)
				ui.Box(c).Width(70).Children(func() { bar(c, winShare(lr.Record), winColor) })
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
	return periods[v.filter.period].label
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
		chart.Plot(c, chart.Axes{Left: ticks, Bottom: dateTicks(pts, xs, 5), Label: axisLabel(c)}, func(area ui.Element) {
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
	img := ui.Image(c, art).Size(width, h).Fit(ui.Cover).Radius(5)
	if width < previewWidth/2 {
		ui.TooltipBase(c, img, func(tip ui.Element) {
			tip.Radius(10).Shadow(0, 6, 24, 0, ui.RGBA(0, 0, 0, 0.5)).Children(func() {
				ui.Image(c, v.largeArt(code, art)).Size(previewWidth, previewWidth*7/5).Fit(ui.Cover).Radius(10)
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

// previewWidth is the size a small card shows at while the pointer rests on it, large enough to
// read its text.
const previewWidth = 300

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
	return func(side chart.Side, t chart.Tick) {
		text := func() { ui.Text(c, t.Label).NoWrap() }
		switch {
		case side == chart.Bottom && t.At > 0.95:
			// The invisible copy keeps the axis row as tall as the text, and the real one ends
			// at the tick.
			ui.Box(c).Width(0).Children(func() {
				ui.Text(c, t.Label).NoWrap().Invisible()
				ui.Text(c, t.Label).NoWrap().Absolute().Right(0).Top(0)
			})
		case side == chart.Bottom && t.At < 0.05:
			ui.Row(c).Width(0).Justify(ui.Start).Children(text)
		default:
			text()
		}
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
