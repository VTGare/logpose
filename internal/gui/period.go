package gui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// periodCustom is filters.period for a range of days the user picked.
const periodCustom = -1

type periodState struct {
	open bool
	// month is the first day of the left of the two months shown.
	month time.Time
	// anchor is set after a first click, when the next click makes a range.
	anchor bool
	// cursor is the day the arrow keys move.
	cursor time.Time
}

func (e *env) today() time.Time { return stats.DayStart(e.now()) }

// customEnd is the last day of the custom period.
func (e *env) customEnd() time.Time {
	if e.filter.ongoing {
		return e.today()
	}
	return e.filter.to
}

// periodBounds is when the period starts and ends. Either is zero when it doesn't.
func (e *env) periodBounds() (since, until time.Time) {
	f := e.filter
	if f.period == periodCustom {
		if f.ongoing {
			return f.from, time.Time{}
		}
		return f.from, f.to.AddDate(0, 0, 1)
	}
	if d := periods[f.period].days; d > 0 {
		since = e.now().AddDate(0, 0, -d)
	}
	return since, time.Time{}
}

func (e *env) periodLabel() string {
	f := e.filter
	switch {
	case f.period != periodCustom:
		return periods[f.period].label
	case f.ongoing:
		return "Since " + e.date(f.from)
	case f.from.Equal(f.to):
		return e.weekday(f.from)
	}
	return span(e.date(f.from), e.date(f.to))
}

func (e *env) setRange(first, last time.Time) {
	e.filter.period = periodCustom
	e.filter.from, e.filter.to = first, last
	e.filter.ongoing = false
	e.periodUI.anchor = false
}

// pickDay is a click on a day in the calendar. The first click picks one day and the second
// makes a range with it, in either order.
func (e *env) pickDay(d time.Time) {
	f := &e.filter
	p := &e.periodUI
	p.cursor = d
	switch {
	case f.period == periodCustom && f.ongoing:
		f.from = d
	case f.period == periodCustom && p.anchor:
		first, last := f.from, d
		if d.Before(first) {
			first, last = d, f.from
		}
		e.setRange(first, last)
		return
	default:
		e.setRange(d, d)
	}
	p.anchor = !f.ongoing
}

func (e *env) setOngoing(on bool) {
	f := &e.filter
	if f.period != periodCustom {
		f.period, f.from = periodCustom, e.today()
	}
	f.ongoing = on
	f.to = e.today()
	e.periodUI.anchor = false
}

// openPeriod shows the months holding the end of the period, or this month.
func (e *env) openPeriod() {
	p := &e.periodUI
	end := e.today()
	if e.filter.period == periodCustom {
		end = e.customEnd()
	}
	p.month = time.Date(end.Year(), end.Month()-1, 1, 0, 0, 0, 0, time.Local)
	p.cursor = end
	p.anchor = false
}

// playDays is the record of each day played, in the filters' scope without the period.
func (e *env) playDays() map[time.Time]stats.Record {
	out := make(map[time.Time]stats.Record)
	for _, d := range stats.Days(e.scopeAnyPeriod().Apply(e.snap.Matches)) {
		out[d.Date] = d.Record
	}
	return out
}

func (e *env) periodPill(c *ui.Context) {
	p := &e.periodUI
	el := filterPill(c.Key("period"), e.periodLabel(), e.filter.period != 0)
	if el.Clicked() {
		p.open = !p.open
		if p.open {
			e.openPeriod()
		}
	}
	ui.PopoverBase(c, el, &p.open, func(panel ui.Element) {
		t := c.Theme()
		popover(panel.Margin(6, 0, 0, 0).Padding(10), t)
		e.periodKeys(panel)
		ui.Row(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
			e.periodPresets(c)
			e.periodCalendar(c)
		})
	})
}

func (e *env) periodKeys(panel ui.Element) {
	p := &e.periodUI
	move := func(days int) func() {
		return func() {
			p.cursor = p.cursor.AddDate(0, 0, days)
			if p.cursor.After(e.today()) {
				p.cursor = e.today()
			}
			e.showCursor()
		}
	}
	panel.OnShortcut(0, ui.KeyLeft, move(-1))
	panel.OnShortcut(0, ui.KeyRight, move(1))
	panel.OnShortcut(0, ui.KeyUp, move(-7))
	panel.OnShortcut(0, ui.KeyDown, move(7))
	panel.OnShortcut(0, ui.KeyEnter, func() { e.pickDay(p.cursor) })
	panel.OnShortcut(0, ui.KeyPageUp, func() { e.shiftMonths(-1) })
	panel.OnShortcut(0, ui.KeyPageDown, func() { e.shiftMonths(1) })
}

// shiftMonths moves the months shown, up to this month on the right.
func (e *env) shiftMonths(n int) {
	p := &e.periodUI
	m := p.month.AddDate(0, n, 0)
	if last := time.Date(e.now().Year(), e.now().Month()-1, 1, 0, 0, 0, 0, time.Local); m.After(last) {
		m = last
	}
	p.month = m
}

// showCursor moves the months shown so the cursor is on one of them.
func (e *env) showCursor() {
	p := &e.periodUI
	first := time.Date(p.cursor.Year(), p.cursor.Month(), 1, 0, 0, 0, 0, time.Local)
	switch {
	case first.Before(p.month):
		p.month = first
	case first.After(p.month.AddDate(0, 1, 0)):
		p.month = first.AddDate(0, -1, 0)
	}
}

func (e *env) periodPresets(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(150).Gap(2).PaddingEnd(10).BorderEndWidth(1).BorderColor(t.Border).Children(func() {
		for i, pr := range periods {
			if presetRow(c.Key(i), pr.label, e.filter.period == i, true).Clicked() {
				e.filter.period = i
				e.filter.from, e.filter.to, e.filter.ongoing = time.Time{}, time.Time{}, false
				e.periodUI.open = false
			}
		}
		ui.Box(c).Height(1).Background(t.Border).Margin(4, 4)
		presetRow(c.Key("custom"), "Custom", e.filter.period == periodCustom, false)
	})
}

func presetRow(c *ui.Context, label string, on, button bool) ui.Element {
	t := c.Theme()
	var el ui.Element
	if button {
		el = ui.ButtonBase(c).Label(label).Background(ui.Transparent)
		if el.Hovered() {
			el.Background(t.Surface)
		}
	} else {
		el = ui.Row(c)
	}
	return el.Gap(8).AlignItems(ui.Center).Justify(ui.Start).Padding(6, 10).Radius(6).Children(func() {
		ui.Box(c).Width(14).Children(func() {
			if on {
				ui.Icon(c, iconCheck).FontSize(14).TextColor(t.Accent)
			}
		})
		col := t.Text
		if !button && !on {
			col = faintColor(t)
		}
		ui.Text(c, label).FontSize(13).TextColor(col).SingleLine()
	})
}

func (e *env) periodCalendar(c *ui.Context) {
	t := c.Theme()
	p := &e.periodUI
	played := e.playDays()
	ui.Column(c).Gap(10).Children(func() {
		ui.Row(c).Gap(14).AlignItems(ui.Start).Children(func() {
			monthArrow(c, "Earlier month", iconPrevStep, false, func() { e.shiftMonths(-1) })
			e.month(c.Key("m0"), p.month, played)
			e.month(c.Key("m1"), p.month.AddDate(0, 1, 0), played)
			atEnd := !p.month.AddDate(0, 1, 0).Before(time.Date(e.now().Year(), e.now().Month(), 1, 0, 0, 0, 0, time.Local))
			monthArrow(c, "Later month", iconNextStep, atEnd, func() { e.shiftMonths(1) })
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(10, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			e.periodSummary(c, played)
			ui.Spacer(c)
			was := e.filter.period == periodCustom && e.filter.ongoing
			on := was
			row := ui.Row(c).Gap(8).AlignItems(ui.Center).Cursor(ui.CursorPointer).Tooltip("Keep the end at today").Children(func() {
				ui.Switch(c, &on).Label("Ongoing")
				ui.Text(c, "Ongoing").FontSize(13).TextColor(t.TextMuted)
			})
			if row.Clicked() {
				on = !was
			}
			if on != was {
				e.setOngoing(on)
			}
		})
	})
}

// periodSummary is the custom period and what was played in it, or how to pick one.
func (e *env) periodSummary(c *ui.Context, played map[time.Time]stats.Record) {
	t := c.Theme()
	f := e.filter
	if f.period != periodCustom {
		ui.Text(c, "Click a day, then another to make a range.").FontSize(13).TextColor(t.TextMuted)
		return
	}
	end := e.customEnd()
	name := span(e.weekday(f.from), e.weekday(end))
	if f.ongoing {
		name = "Since " + e.weekday(f.from)
	}
	var r stats.Record
	n := 0
	for d, dr := range played {
		if !d.Before(f.from) && !d.After(end) {
			n++
			r = r.Plus(dr)
		}
	}
	ui.Text(c, name).FontSize(13).Bold()
	sum := "No games"
	if n > 0 {
		sum = fmt.Sprintf("%d days played · %s · %s", n, games(r.Games()), record(r))
		if n == 1 {
			sum = fmt.Sprintf("%s · %s", games(r.Games()), record(r))
		}
	}
	ui.Text(c, sum).FontSize(12).TextColor(faintColor(t))
}

func monthArrow(c *ui.Context, label string, icon *ui.SVG, off bool, fn func()) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(label)).Label(label).Size(26, 26).Radius(13).Center().Background(ui.Transparent).Disabled(off)
	if b.Hovered() && !off {
		b.Background(t.Surface)
	}
	if b.Clicked() {
		fn()
	}
	col := t.TextMuted
	if off {
		col = faintColor(t).Alpha(0.4)
	}
	b.Children(func() { ui.Icon(c, icon).FontSize(14).TextColor(col) })
}

const calCell = 38

// month is a month's days in weeks from Monday. Days played show their record.
func (e *env) month(c *ui.Context, first time.Time, played map[time.Time]stats.Record) {
	t := c.Theme()
	ui.Column(c).Gap(2).Children(func() {
		ui.Text(c, first.Format("January 2006")).Bold().FontSize(13).Padding(3, 4, 8, 4)
		ui.Row(c).Children(func() {
			for _, d := range []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"} {
				ui.Text(c, d).FontSize(11).TextColor(faintColor(t)).Width(calCell).TextAlign(ui.Center)
			}
		})
		start := stats.WeekStart(first)
		for w := range 6 {
			ui.Row(c.Key(w)).Children(func() {
				for i := range 7 {
					d := start.AddDate(0, 0, w*7+i)
					if d.Month() != first.Month() {
						ui.Box(c.Key(i)).Size(calCell, 40)
						continue
					}
					e.dayCell(c.Key(i), d, played)
				}
			})
		}
	})
}

// cellSpot is where a day sits in the custom period.
type cellSpot struct{ in, first, last bool }

func (e *env) spot(d time.Time) cellSpot {
	f := e.filter
	if f.period != periodCustom {
		return cellSpot{}
	}
	end := e.customEnd()
	return cellSpot{in: !d.Before(f.from) && !d.After(end), first: d.Equal(f.from), last: d.Equal(end)}
}

func (s cellSpot) edge() bool { return s.first || s.last }

// shape fills a day's cell: the ends of the period in the accent with their outer corners
// round, and the days between tinted.
func (s cellSpot) shape(b ui.Element, t *ui.Theme, hover bool) {
	switch {
	case s.edge():
		b.Background(t.Accent)
	case s.in:
		b.Background(t.Accent.Alpha(0.16))
	case hover:
		b.Background(t.Surface)
	}
	switch {
	case s.first && s.last:
		b.Radius(8)
	case s.first:
		b.Radius(8, 0, 0, 8)
	case s.last:
		b.Radius(0, 8, 8, 0)
	case s.in:
		b.Radius(0)
	default:
		b.Radius(8)
	}
}

func (e *env) dayCell(c *ui.Context, d time.Time, played map[time.Time]stats.Record) {
	t := c.Theme()
	sp := e.spot(d)
	future := d.After(e.today())
	b := ui.ButtonBase(c).Label("Pick "+e.weekday(d)).Size(calCell, 40).Background(ui.Transparent).Disabled(future)
	sp.shape(b, t, b.Hovered() && !future)
	if d.Equal(e.periodUI.cursor) && b.Focused() {
		b.Border(1, t.Accent)
	}
	if b.Clicked() {
		e.pickDay(d)
	}
	r, ok := played[d]
	ink := faintColor(t)
	switch {
	case sp.edge():
		ink = onAccent
	case ok:
		ink = t.Text
	}
	if future {
		ink = ink.Alpha(0.4)
	}
	b.Column().Center().Children(func() {
		num := ui.Text(c, fmt.Sprint(d.Day())).FontSize(13).TextColor(ink)
		switch {
		case ok:
			num.FontWeight(600)
			ui.Text(c, record(r)).FontSize(10).FontWeight(500).TextColor(recordInk(t, r, sp.edge()))
		case d.Equal(e.today()):
			dot := t.Accent
			if sp.edge() {
				dot = onAccent
			}
			ui.Box(c).Size(4, 4).Radius(2).Background(dot).Margin(2, 0, 0, 0)
		}
	})
}

// recordInk is a day's record in the result's color, or dark on the accent.
func recordInk(t *ui.Theme, r stats.Record, onFill bool) ui.Color {
	switch {
	case onFill:
		return onAccent
	case r.Losses > r.Wins:
		return lossColor(t)
	}
	return winColor(t)
}
