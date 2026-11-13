package overview

import (
	"fmt"
	"time"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
)

// Draw is the headline numbers and every day played by week on the left, and the decks
// and the leaders faced this week on the right.
func Draw(c *ui.Context, e *page.Env, top func()) {
	o := e.Scoped().Overview
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			top()
			headline(c, e, o)
			ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Gap(16).Children(func() {
					daysTile(c, e, o.Days)
				})
				ui.Column(c).Width(340).Gap(16).Children(func() {
					decksTile(c, e, o.Decks)
					facedTile(c, e, o.FacedThisWeek)
				})
			})
		})
	})
}

// headline is this week, the streak, the whole period and today, on the banner of the deck
// played last.
func headline(c *ui.Context, e *page.Env, o stats.Overview) {
	t := c.Theme()
	muted := func(text string) { ui.Text(c, text).FontSize(12).TextColor(t.TextMuted) }
	none := func(text string) { ui.Text(c, text).FontSize(28).Bold().TextColor(kit.FaintColor(t)) }
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
	e.Banner(c, o.Deck, 170, func() {
		ui.Text(c, "Current deck").FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
			ui.Text(c, e.Snap.Card(o.Deck).Name).FontSize(22).Bold()
			ui.Text(c, fmt.Sprintf("%s · %s", cards.SetOf(o.Deck), kit.Games(deck.Record.Games()))).FontSize(13).TextColor(t.TextMuted).Margin(0, 0, 3, 0)
		})
		ui.Row(c).Margin(14, 0, 0, 0).AlignItems(ui.Stretch).Children(func() {
			block(0, "This week", func() {
				if o.Week.Games() == 0 {
					none("–")
					muted("No games")
					return
				}
				big(page.RateText(o.Week), t.Text)
				change, d := WeekChange(o)
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					muted(kit.Record(o.Week) + " ·")
					page.Trend(c, d, change)
				})
			})
			block(1, "Streak", func() {
				if o.Streak == 0 {
					none("–")
					muted("None")
					return
				}
				big(fmt.Sprintf("%s%d", kit.ResultLetter(o.StreakResult), o.Streak), kit.ResultColor(t, o.StreakResult))
				muted(streakText(o))
			})
			block(2, periodTitle(e), func() {
				if o.Total.Games() == 0 {
					none("–")
					muted("No games")
					return
				}
				big(page.RateText(o.Total), t.Text)
				muted(kit.Record(o.Total))
			})
			block(3, "Today", func() {
				if len(o.Today) == 0 {
					none("–")
					muted("None yet")
					return
				}
				// The squares wrap before the record does. Two rows of them fit beside the record.
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, kit.Record(o.TodayRecord)).FontSize(28).Bold().TextColor(t.Text).SingleLine().Shrink(0)
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() { page.ResultSquares(c, o.Today, 12) })
				})
				muted(fmt.Sprintf("%s · %s", kit.Games(len(o.Today)), kit.Duration(o.TodayTime)))
			})
		})
	})
}

func streakText(o stats.Overview) string {
	word := kit.Plural(o.Streak, "win", "wins")
	if o.StreakResult == stats.Loss {
		word = kit.Plural(o.Streak, "loss", "losses")
	}
	return fmt.Sprintf("%d %s in a row", o.Streak, word)
}

// WeekChange compares this week's win rate with last week's, when both have enough games, and
// returns the change in points, which the headline shows as an arrow.
func WeekChange(o stats.Overview) (string, float64) {
	now, ok := o.Week.Rate()
	before, okBefore := o.PrevWeek.Rate()
	if !ok || !okBefore {
		return kit.Games(o.Week.Games()), 0
	}
	d := page.PointsApart(now, before)
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
func daysTile(c *ui.Context, e *page.Env, days []stats.Day) {
	t := c.Theme()
	days = days[:min(len(days), shownDays)]
	kit.Tile(c, "By day", func() {
		if len(days) == 0 {
			ui.Text(c, "No games").TextColor(t.TextMuted)
			return
		}
		var week time.Time
		for i, d := range days {
			if ws := stats.WeekStart(d.Date); !ws.Equal(week) {
				week = ws
				weekHeader(c.Key(fmt.Sprint("w", i)), e, ws, days[i:])
			}
			e.DayRow(c.Key(i), d)
		}
	})
}

// weekHeader names the week and gives its record over the days that follow in it.
func weekHeader(c *ui.Context, e *page.Env, start time.Time, days []stats.Day) {
	t := c.Theme()
	var r stats.Record
	for _, d := range days {
		if !stats.WeekStart(d.Date).Equal(start) {
			break
		}
		r = r.Plus(d.Record)
	}
	this := stats.WeekStart(e.Now())
	name := kit.Span(e.Date(start), e.Date(start.AddDate(0, 0, 6)))
	switch {
	case start.Equal(this):
		name = "This week"
	case start.Equal(this.AddDate(0, 0, -7)):
		name = "Last week"
	}
	ui.Row(c).Gap(8).Padding(8, 0, 2, 0).AlignItems(ui.Center).Children(func() {
		page.PeriodLink(c, name, "Show only this week", 12, 700, func() { e.SetRange(start, start.AddDate(0, 0, 6)) })
		ui.Text(c, fmt.Sprintf("%s · %s", kit.Record(r), kit.Pct(r.Share()))).FontSize(12).TextColor(t.TextMuted)
		ui.Box(c).Grow(1).Height(1).Background(t.Border)
	})
}

// decksTile is each deck played, latest first, as a short row that opens it on the Decks page.
func decksTile(c *ui.Context, e *page.Env, decks []stats.Deck) {
	t := c.Theme()
	kit.Tile(c, "Your decks", func() {
		for _, d := range decks {
			card := e.Snap.Card(d.Leader)
			el := ui.Row(c.Key(d.Leader)).Gap(12).AlignItems(ui.Center).Padding(5, 6).Margin(0, -6).Label("Show " + card.Name).
				Transition(kit.Slide).PopoverMenu(e.YourDeckMenu(d.Leader))
			if el.Hovered() {
				el.Background(t.Surface)
			}
			el.Radius(8)
			if el.Clicked() {
				e.OpenLeader(d.Leader)
			}
			el.Children(func() {
				e.ArtStrip(c, d.Leader, 52, 36)
				ui.Column(c).Grow(1).Basis(0).Gap(1).Children(func() {
					ui.Text(c, card.Name).FontWeight(600).SingleLine()
					ui.Text(c, fmt.Sprintf("%s · %s", kit.Games(d.Record.Games()), kit.Ago(e.Now(), d.Last))).FontSize(12).TextColor(t.TextMuted)
				})
				ui.Column(c).AlignItems(ui.End).Gap(1).Children(func() {
					if rate, ok := d.Record.Rate(); ok {
						ui.Text(c, kit.Pct(rate)).FontSize(17).Bold()
					} else {
						ui.Text(c, page.TooFew).FontSize(11).TextColor(kit.FaintColor(t))
					}
					ui.Text(c, kit.Record(d.Record)).FontSize(12).TextColor(t.TextMuted)
				})
			})
		}
	})
}

// facedTile is the leaders faced this week, most played first, with the share won as a bar. A
// row narrows every page to games against that leader.
func facedTile(c *ui.Context, e *page.Env, faced []stats.LeaderRecord) {
	t := c.Theme()
	n := 0
	for _, lr := range faced {
		n += lr.Record.Games()
	}
	kit.TitledTile(c, "Facing most this week", kit.Games(n), func() {
		if len(faced) == 0 {
			ui.Text(c, "No games this week").TextColor(t.TextMuted)
			return
		}
		for _, lr := range faced[:min(len(faced), 5)] {
			name := e.Snap.Card(lr.Leader).Name
			el := ui.Row(c.Key(lr.Leader)).Gap(12).AlignItems(ui.Center).Padding(4, 6).Margin(0, -6).Radius(8).
				Label("Show games vs " + name).Transition(kit.Slide).PopoverMenu(e.OpponentMenu("", lr.Leader))
			if el.Hovered() {
				el.Background(t.Surface)
			}
			if el.Clicked() {
				e.FilterOpponent(lr.Leader)
			}
			el.Children(func() {
				e.ArtStrip(c, lr.Leader, 44, 30)
				ui.Text(c, name).FontWeight(500).SingleLine().Grow(1).Basis(0)
				ui.Box(c).Width(80).Children(func() { e.SplitBar(c, lr.Record.Share()) })
				ui.Text(c, kit.Record(lr.Record)).Bold().Width(36).TextAlign(ui.End)
			})
		}
	})
}

// periodTitle names the whole of the filtered games: the period, or the format when the
// period is all time.
func periodTitle(e *page.Env) string {
	if fr, ok := e.FormatRange(); ok && e.Filter.Period == 0 {
		return "All of " + fr.Name
	}
	return e.PeriodLabel()
}
