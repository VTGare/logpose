package gui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
)

// overviewPage is the headline numbers and every day played by week on the left, and the decks
// and the leaders faced this week on the right.
func (v *view) overviewPage(c *ui.Context) {
	o := v.scoped().overview
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			v.shortcutCard(c)
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
			ui.Text(c, fmt.Sprintf("%s · %s", cards.SetOf(o.Deck), games(deck.Record.Games()))).FontSize(13).TextColor(t.TextMuted).Margin(0, 0, 3, 0)
		})
		ui.Row(c).Margin(14, 0, 0, 0).AlignItems(ui.Stretch).Children(func() {
			block(0, "This week", func() {
				if o.Week.Games() == 0 {
					none("–")
					muted("No games")
					return
				}
				big(rateText(o.Week), t.Text)
				change, d := weekChange(o)
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					muted(record(o.Week) + " ·")
					trend(c, d, change)
				})
			})
			block(1, "Streak", func() {
				if o.Streak == 0 {
					none("–")
					muted("None")
					return
				}
				big(fmt.Sprintf("%s%d", resultLetter(o.StreakResult), o.Streak), resultColor(t, o.StreakResult))
				muted(streakText(o))
			})
			block(2, v.periodTitle(), func() {
				if o.Total.Games() == 0 {
					none("–")
					muted("No games")
					return
				}
				big(rateText(o.Total), t.Text)
				muted(record(o.Total))
			})
			block(3, "Today", func() {
				if len(o.Today) == 0 {
					none("–")
					muted("None yet")
					return
				}
				// The squares wrap before the record does. Two rows of them fit beside the record.
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, record(o.TodayRecord)).FontSize(28).Bold().TextColor(t.Text).SingleLine().Shrink(0)
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() { resultSquares(c, o.Today, 12) })
				})
				muted(fmt.Sprintf("%s · %s", games(len(o.Today)), duration(o.TodayTime)))
			})
		})
	})
}

func streakText(o stats.Overview) string {
	word := plural(o.Streak, "win", "wins")
	if o.StreakResult == stats.Loss {
		word = plural(o.Streak, "loss", "losses")
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
	d := pointsApart(now, before)
	switch {
	case d > 0:
		return fmt.Sprintf("%.0f pts", d), d
	case d < 0:
		return fmt.Sprintf("%.0f pts", -d), d
	default:
		return "no change", 0
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
		r = r.Plus(d.Record)
	}
	this := stats.WeekStart(v.now())
	name := span(v.date(start), v.date(start.AddDate(0, 0, 6)))
	switch {
	case start.Equal(this):
		name = "This week"
	case start.Equal(this.AddDate(0, 0, -7)):
		name = "Last week"
	}
	ui.Row(c).Gap(8).Padding(8, 0, 2, 0).AlignItems(ui.Center).Children(func() {
		v.periodLink(c, name, "Show only this week", 12, 700, func() { v.setRange(start, start.AddDate(0, 0, 6)) })
		ui.Text(c, fmt.Sprintf("%s · %s", record(r), pct(r.Share()))).FontSize(12).TextColor(t.TextMuted)
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

func (v *view) dayRow(c *ui.Context, d stats.Day) {
	t := c.Theme()
	row := ui.Row(c).Gap(12).AlignItems(ui.Start).Padding(8, 8).Margin(0, -8).Radius(8)
	if d.Date.Equal(v.today()) {
		row.Background(t.Surface)
	}
	row.Children(func() {
		ui.Column(c).Width(170).Gap(3).Children(func() {
			v.periodLink(c, v.dayName(d.Date), "Show only this day", 0, 500, func() { v.setRange(d.Date, d.Date) })
			ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
				ui.Text(c, record(d.Record)).FontSize(18).Bold()
				ui.Text(c, fmt.Sprintf("%s · %s", pct(d.Record.Share()), duration(d.Played))).FontSize(12).TextColor(faintColor(t)).Margin(0, 0, 2, 0)
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
	today := v.today()
	switch {
	case date.Equal(today):
		return "Today"
	case date.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	}
	return v.weekday(date)
}

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
	el := v.gameLink(ui.Column(c), m)
	lift(el.Transition(tileEnter(i)), 7)
	el.Width(gameTileW).Height(gameTileH).Clip().Children(func() {
		if art := v.art.get(m.Opp.Leader, artCrop); art != nil {
			ui.Image(c, art).Size(gameTileW, gameTileH).Fit(ui.Cover)
		} else {
			v.leaderChip(c, m.Opp.Leader, gameTileH)
		}
		col := resultColor(t, m.Result)
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
	return fmt.Sprintf("%s · %s · %s", text, turns(m.Turns), m.PlayedAt.Format("15:04"))
}

// decksTile is each deck played, latest first, as a short row that opens it on the Decks page.
func (v *view) decksTile(c *ui.Context, decks []stats.Deck) {
	t := c.Theme()
	tile(c, "Your decks", func() {
		for _, d := range decks {
			card := v.snap.card(d.Leader)
			el := ui.Row(c.Key(d.Leader)).Gap(12).AlignItems(ui.Center).Padding(5, 6).Margin(0, -6).Label("Show " + card.Name).
				Transition(slide).PopoverMenu(v.yourDeckMenu(d.Leader))
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
					ui.Text(c, fmt.Sprintf("%s · %s", games(d.Record.Games()), ago(v.now(), d.Last))).FontSize(12).TextColor(t.TextMuted)
				})
				ui.Column(c).AlignItems(ui.End).Gap(1).Children(func() {
					if rate, ok := d.Record.Rate(); ok {
						ui.Text(c, pct(rate)).FontSize(17).Bold()
					} else {
						ui.Text(c, tooFew).FontSize(11).TextColor(faintColor(t))
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
				Label("Show games vs " + name).Transition(slide).PopoverMenu(v.opponentMenu("", lr.Leader))
			if el.Hovered() {
				el.Background(t.Surface)
			}
			if el.Clicked() {
				v.filterOpponent(lr.Leader)
			}
			el.Children(func() {
				v.artStrip(c, lr.Leader, 44, 30)
				ui.Text(c, name).FontWeight(500).SingleLine().Grow(1).Basis(0)
				ui.Box(c).Width(80).Children(func() { v.splitBar(c, lr.Record.Share()) })
				ui.Text(c, record(lr.Record)).Bold().Width(36).TextAlign(ui.End)
			})
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
