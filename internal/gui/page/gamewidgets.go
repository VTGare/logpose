package page

import (
	"fmt"
	"math"
	"time"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
)

// TooFew stands in for a rate from fewer than stats.MinGames games.
const TooFew = "too few"

func RateText(r stats.Record) string {
	if rate, ok := r.Rate(); ok {
		return kit.Pct(rate)
	}
	return TooFew
}

// SortRate is the rate a table sorts by. It's 0 below MinGames, so those rows keep their order
// among themselves.
func SortRate(r stats.Record) float64 {
	x, _ := r.Rate()
	return x
}

// RateRecord is a rate with its record, as "76% (28–9)", or "–" without games.
func RateRecord(r stats.Record) string {
	if r.Games() == 0 {
		return "–"
	}
	return fmt.Sprintf("%s (%s)", RateText(r), kit.Record(r))
}

// RateCell is the rate with the record beside it. Below MinGames the rate is greyed out, and
// without games the cell is "–".
func RateCell(c *ui.Context, r stats.Record) {
	t := c.Theme()
	if r.Games() == 0 {
		ui.Text(c, "–").FontSize(13).TextColor(kit.FaintColor(t))
		return
	}
	ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
		if rate, ok := r.Rate(); ok {
			ui.Text(c, kit.Pct(rate)).FontSize(13).FontWeight(600).SingleLine()
		} else {
			ui.Text(c, TooFew).FontSize(13).TextColor(kit.FaintColor(t)).SingleLine()
		}
		ui.Text(c, kit.Record(r)).FontSize(11).TextColor(kit.FaintColor(t)).SingleLine()
	})
}

// PointsApart is how many points rate a is above rate b, rounded.
func PointsApart(a, b float64) float64 { return math.Round((a - b) * 100) }

// Trend is an arrow up or down for a change of d points, none for no change, then text.
func Trend(c *ui.Context, d float64, text string) {
	t := c.Theme()
	switch {
	case d > 0:
		ui.Icon(c, kit.IconTrendUp).FontSize(12).TextColor(t.TextMuted).Label("Up")
	case d < 0:
		ui.Icon(c, kit.IconTrendDown).FontSize(12).TextColor(t.TextMuted).Label("Down")
	}
	ui.Text(c, text).FontSize(12).TextColor(t.TextMuted)
}

// SplitBar is the share won in the win color and the rest in the loss color.
// It grows from the left when the page opens, and slides when a filter changes it.
func (e *Env) SplitBar(c *ui.Context, won float64) {
	won = min(max(won, 0), 1)
	box := ui.Box(c).Height(6).Radius(3).Clip()
	shown := e.Grow(box, "won", float32(won), kit.BarTime)
	full := e.Grow(box, "full", 1, kit.BarTime)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		w := r.W * shown
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H}, kit.WinColor(c.Theme()), 0)
		p.Fill(ui.Rect{X: r.X + w, Y: r.Y, W: r.W*full - w, H: r.H}, kit.LossColor(c.Theme()).Alpha(0.75), 0)
	})
}

// TurnRows is a row for going first and one for going second, each with a filled or hollow mark,
// the share won as a bar with 50% marked, the rate and the record. Turn order has no color of
// its own, so the marks tell the rows apart.
func (e *Env) TurnRows(c *ui.Context, o stats.TurnOrder, chosen stats.TurnOrderFilter, pick func(stats.TurnOrderFilter)) {
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
			won := e.Grow(track, "won", float32(r.Share()), kit.BarTime)
			track.Draw(func(p *ui.Painter, rect ui.Rect) {
				p.Fill(ui.Rect{X: rect.X, Y: rect.Y, W: rect.W * won, H: rect.H}, t.Text.Alpha(0.75), 3)
				p.Fill(ui.Rect{X: rect.X + rect.W/2 - 0.5, Y: rect.Y - 3, W: 1, H: rect.H + 6}, t.TextMuted, 0)
			})
			if rate, ok := r.Rate(); ok {
				ui.Text(c, kit.Pct(rate)).FontSize(16).Bold().Width(44).TextAlign(ui.End)
			} else {
				ui.Text(c, TooFew).FontSize(11).TextColor(kit.FaintColor(t)).Width(44).TextAlign(ui.End)
			}
			ui.Text(c, kit.Record(r)).FontSize(12).TextColor(t.TextMuted).Width(40).TextAlign(ui.End)
		})
	}
	ui.Column(c).Gap(4).Children(func() {
		row(stats.GoingFirst, "first", "1st", o.First, true)
		row(stats.GoingSecond, "second", "2nd", o.Second, false)
	})
}

// ResultSquares are the results as squares of their color, in the order given. The gaps and
// corners grow with the size.
func ResultSquares(c *ui.Context, rs []stats.Result, size float32) ui.Element {
	gap := float32(math.Round(float64(size) / 4))
	return ui.Row(c).Gap(gap).Wrap().Children(func() {
		for i, r := range rs {
			ui.Box(c.Key(i)).Size(size, size).Radius(gap).Background(kit.ResultColor(c.Theme(), r))
		}
	})
}

func ResultBadge(c *ui.Context, r stats.Result) {
	ui.Text(c, kit.ResultLetter(r)).FontSize(11).Bold().TextColor(ui.Hex("#0B1209")).TextAlign(ui.Center).
		Size(20, 20).Radius(5).Background(kit.ResultColor(c.Theme(), r)).Padding(2, 0)
}

// TurnOrderShort is "1st" or "2nd", or "" when the log doesn't say.
func TurnOrderShort(m stats.Match) string {
	switch {
	case !m.TurnOrderKnown:
		return ""
	case m.WentFirst:
		return "1st"
	default:
		return "2nd"
	}
}

// TurnMark is 1st with a filled mark or 2nd with a hollow one, or nothing when the log doesn't
// say.
func TurnMark(c *ui.Context, m stats.Match) {
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
		ui.Text(c, TurnOrderShort(m)).FontSize(12).TextColor(t.TextMuted)
	})
}

// gameLink makes el open the match on a click, with the game's menu on right-click.
func (e *Env) gameLink(el ui.Element, m stats.Match) ui.Element {
	if el.Clicked() {
		e.Nav.OpenMatch(m)
	}
	return el.Label("Open the match against " + m.Opp.Name).PopoverMenu(e.Nav.GameMenu(m))
}

// GameLine is a row for one game that opens the match, lit while the pointer is over it.
func (e *Env) GameLine(c *ui.Context, m stats.Match) ui.Element {
	t := c.Theme()
	row := ui.ButtonBase(c).Row().AlignItems(ui.Center).Radius(6).
		Transition(ui.ElementTransition{Duration: kit.HoverTime, Colors: true})
	if row.Hovered() {
		row.Background(t.Surface.Alpha(0.5))
	} else {
		row.Background(ui.Transparent)
	}
	return e.gameLink(row, m)
}

// leaderChip is a small block in the leader's colors.
func (e *Env) leaderChip(c *ui.Context, code string, size float32) {
	cols := kit.ColorsOf(e.Snap.Card(code))
	ui.Row(c).Size(size, size).Radius(size / 4).Clip().AlignItems(ui.Stretch).Children(func() {
		for i, col := range cols {
			ui.Box(c.Key(i)).Grow(1).Background(col.Card)
		}
	})
}

func (e *Env) LeaderName(c *ui.Context, code string) {
	ui.Row(c).Gap(6).Children(func() {
		e.leaderChip(c, code, 12)
		ui.Text(c, e.Snap.Card(code).Name).SingleLine()
		ui.Text(c, cards.SetOf(code)).FontSize(11).TextColor(c.Theme().TextMuted)
	})
}

const bannerArtW = 300

// Banner is a leader's art on the left, fading into the panel, with the leader's colors glowing
// faintly from two corners and body beside the art.
func (e *Env) Banner(c *ui.Context, code string, height float32, body func()) ui.Element {
	t := c.Theme()
	cols := kit.ColorsOf(e.Snap.Card(code))
	art := e.Art.Get(code, artPortrait)
	strength := 0.5
	if art != nil {
		strength = 0.3
	}
	return ui.Row(c).Height(height).Radius(12).Clip().Background(kit.PanelColor(t)).Border(1, t.Border.Alpha(0.6)).
		Shadow(0, 6, 20, 0, kit.PanelShade).AlignItems(ui.Stretch).Draw(func(p *ui.Painter, rect ui.Rect) {
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
func glow(p *ui.Painter, r ui.Rect, cols []kit.LeaderColor, strength float64) {
	second := cols[0]
	if len(cols) > 1 {
		second = cols[1]
	}
	radius := r.W * 0.78
	p.FillRadialGradient(r, Light(cols[0].Glow, ui.Point{X: 0, Y: 1.1}, radius, strength), 0)
	p.FillRadialGradient(r, Light(second.Glow, ui.Point{X: 1, Y: -0.1}, radius, strength), 0)
}

// Light fades a color from strength at center to nothing at radius, along (1-t)^1.7.
func Light(col ui.Color, center ui.Point, radius float32, strength float64) ui.RadialGradient {
	g := ui.RadialGradient{Center: center, Radius: radius}
	for _, t := range []float32{0, 0.2, 0.4, 0.6, 0.8, 1} {
		c := col
		c.A = uint8(255 * strength * math.Pow(float64(1-t), 1.7))
		g.Stops = append(g.Stops, ui.GradientStop{Color: c, At: t})
	}
	return g
}

// AxisLabel builds tick labels that stay inside the plot. A label is centered on its tick, so
// the ones at the ends of the bottom axis would hang half outside. They line up with the end
// instead.
func AxisLabel(c *ui.Context) func(chart.Side, chart.Tick) {
	col := kit.FaintColor(c.Theme())
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

func (e *Env) DayRow(c *ui.Context, d stats.Day) {
	t := c.Theme()
	row := ui.Row(c).Gap(12).AlignItems(ui.Start).Padding(8, 8).Margin(0, -8).Radius(8)
	if d.Date.Equal(e.Today()) {
		row.Background(t.Surface)
	}
	row.Children(func() {
		ui.Column(c).Width(170).Gap(3).Children(func() {
			PeriodLink(c, e.DayName(d.Date), "Show only this day", 0, 500, func() { e.SetRange(d.Date, d.Date) })
			ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
				ui.Text(c, kit.Record(d.Record)).FontSize(18).Bold()
				ui.Text(c, fmt.Sprintf("%s · %s", kit.Pct(d.Record.Share()), kit.Duration(d.Played))).FontSize(12).TextColor(kit.FaintColor(t)).Margin(0, 0, 2, 0)
			})
			e.dayDecks(c, d)
		})
		ui.Row(c).Gap(6).Grow(1).Basis(0).Wrap().Children(func() {
			for i, m := range d.Matches {
				e.gameTile(c.Key(m.Fingerprint), m, i)
			}
		})
	})
}

func (e *Env) DayName(date time.Time) string {
	today := e.Today()
	switch {
	case date.Equal(today):
		return "Today"
	case date.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	}
	return e.Weekday(date)
}

func (e *Env) dayDecks(c *ui.Context, d stats.Day) {
	seen := make(map[string]bool)
	ui.Row(c).Gap(4).Children(func() {
		for i, m := range d.Matches {
			if !seen[m.Me.Leader] {
				seen[m.Me.Leader] = true
				e.ArtStrip(c.Key(i), m.Me.Leader, 22, 16)
			}
		}
	})
}

// gameTile is one game: the top of the opponent's leader art, with its corner and bottom edge
// in the result's color. It opens the match.
func (e *Env) gameTile(c *ui.Context, m stats.Match, i int) {
	t := c.Theme()
	leader := e.Snap.Card(m.Opp.Leader).Name
	el := e.gameLink(ui.Column(c), m)
	kit.Lift(el.Transition(kit.TileEnter(i)), 7)
	el.Width(gameTileW).Height(gameTileH).Clip().Children(func() {
		if art := e.Art.Get(m.Opp.Leader, artCrop); art != nil {
			ui.Image(c, art).Size(gameTileW, gameTileH).Fit(ui.Cover)
		} else {
			e.leaderChip(c, m.Opp.Leader, gameTileH)
		}
		col := kit.ResultColor(t, m.Result)
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

// PeriodLink is a day's or a week's name in By day, which sets the period to it. A size of
// zero keeps the theme's.
func PeriodLink(c *ui.Context, name, tip string, size float32, weight int, set func()) {
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

func gameTip(m stats.Match, leader string) string {
	text := fmt.Sprintf("%s vs %s · %s", ResultWord(m.Result), leader, m.Opp.Name)
	if o := TurnOrderShort(m); o != "" {
		text += " · " + o
	}
	return fmt.Sprintf("%s · %s · %s", text, kit.Turns(m.Turns), m.PlayedAt.Format("15:04"))
}

// DeckTabs is a tab for each deck with its art, set, games and rate, calling pick with the one
// clicked. label starts each tab's name for assistive technology.
func (e *Env) DeckTabs(c *ui.Context, all []stats.LeaderRecord, chosen, label string, pick func(string)) {
	t := c.Theme()
	ui.Row(c).Gap(8).Wrap().Children(func() {
		for i, lr := range all {
			name := e.Snap.Card(lr.Leader).Name
			on := lr.Leader == chosen
			tab := ui.Row(c.Key(i)).Gap(10).AlignItems(ui.Center).Padding(6, 12, 6, 6).
				Border(1, t.Border.Alpha(0.6)).Label(label + name).Transition(ui.ElementTransition{Duration: kit.HoverTime, Colors: true}).
				PopoverMenu(e.YourDeckMenu(lr.Leader))
			if on {
				tab = tab.Background(t.Surface).Border(1, t.Border)
			} else if tab.Hovered() {
				tab = tab.Background(t.Surface.Alpha(0.5))
			}
			kit.Lift(tab, 10)
			if tab.Clicked() {
				pick(lr.Leader)
			}
			tab.Children(func() {
				e.ArtStrip(c, lr.Leader, 48, 34)
				ui.Column(c).Gap(1).Children(func() {
					nameText := ui.Text(c, name).FontSize(13).FontWeight(600).SingleLine()
					if !on {
						nameText.TextColor(t.TextMuted)
					}
					meta := fmt.Sprintf("%s · %s · %s", cards.SetOf(lr.Leader), kit.Games(lr.Record.Games()), RateText(lr.Record))
					ui.Text(c, meta).FontSize(12).TextColor(kit.FaintColor(t))
				})
			})
		}
	})
}

const (
	gameTileW = 52
	gameTileH = 38
)

func ResultWord(r stats.Result) string {
	switch r {
	case stats.Win:
		return "Win"
	case stats.Loss:
		return "Loss"
	default:
		return "Unknown"
	}
}
