package gui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// periodCustom is filters.period for a range of days the user picked.
const periodCustom = -1

// periodState is the period panel's state.
type periodState struct {
	open bool
	// month is the first day of the left of the two months shown.
	month time.Time
	// anchor is set after a first click, when the next click makes a range.
	anchor bool
	// cursor is the day the arrow keys move.
	cursor time.Time
}

// day is t's local midnight.
func day(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func (v *view) today() time.Time { return day(v.snap.Now) }

// customEnd is the last day of the custom period.
func (v *view) customEnd() time.Time {
	if v.filter.ongoing {
		return v.today()
	}
	return v.filter.to
}

// periodBounds is when the period starts and ends. Either is zero when it doesn't.
func (v *view) periodBounds() (since, until time.Time) {
	f := v.filter
	if f.period == periodCustom {
		if f.ongoing {
			return f.from, time.Time{}
		}
		return f.from, f.to.AddDate(0, 0, 1)
	}
	if d := periods[f.period].days; d > 0 {
		since = v.snap.Now.AddDate(0, 0, -d)
	}
	return since, time.Time{}
}

// shortDay is "Oct 25", with the year when it isn't this one.
func (v *view) shortDay(d time.Time) string {
	if d.Year() != v.snap.Now.Year() {
		return d.Format("Jan 2, 2006")
	}
	return d.Format("Jan 2")
}

// longDay is "Sun Oct 25", with the year when it isn't this one.
func (v *view) longDay(d time.Time) string {
	return d.Format("Mon ") + v.shortDay(d)
}

func (v *view) periodLabel() string {
	f := v.filter
	switch {
	case f.period != periodCustom:
		return periods[f.period].label
	case f.ongoing:
		return "Since " + v.shortDay(f.from)
	case f.from.Equal(f.to):
		return v.longDay(f.from)
	}
	return v.shortDay(f.from) + " – " + v.shortDay(f.to)
}

// setRange sets a custom period from the first to the last day.
func (v *view) setRange(first, last time.Time) {
	v.filter.period = periodCustom
	v.filter.from, v.filter.to = first, last
	v.filter.ongoing = false
	v.periodUI.anchor = false
	v.resetPoints()
}

// pickDay is a click on a day in the calendar. The first click picks one day and the second
// makes a range with it, in either order.
func (v *view) pickDay(d time.Time) {
	f := &v.filter
	p := &v.periodUI
	p.cursor = d
	switch {
	case f.period == periodCustom && f.ongoing:
		f.from = d
	case f.period == periodCustom && p.anchor:
		first, last := f.from, d
		if d.Before(first) {
			first, last = d, f.from
		}
		v.setRange(first, last)
		return
	default:
		v.setRange(d, d)
	}
	p.anchor = !f.ongoing
	v.resetPoints()
}

func (v *view) setOngoing(on bool) {
	f := &v.filter
	if f.period != periodCustom {
		f.period, f.from = periodCustom, v.today()
	}
	f.ongoing = on
	f.to = v.today()
	v.periodUI.anchor = false
	v.resetPoints()
}

// openPeriod shows the months holding the end of the period, or this month.
func (v *view) openPeriod() {
	p := &v.periodUI
	end := v.today()
	if v.filter.period == periodCustom {
		end = v.customEnd()
	}
	p.month = time.Date(end.Year(), end.Month()-1, 1, 0, 0, 0, 0, time.Local)
	p.cursor = end
	p.anchor = false
}

// playDays is the record of each day played, in the filters' scope without the period.
func (v *view) playDays() map[time.Time]stats.Record {
	out := make(map[time.Time]stats.Record)
	for _, m := range v.scopeAnyPeriod().Apply(v.snap.Matches) {
		d := day(m.PlayedAt)
		r := out[d]
		r.Add(m.Result)
		out[d] = r
	}
	return out
}

func (v *view) periodPill(c *ui.Context) {
	p := &v.periodUI
	el := filterPill(c.Key("period"), v.periodLabel(), v.filter.period != 0)
	if el.Clicked() {
		p.open = !p.open
		if p.open {
			v.openPeriod()
		}
	}
	ui.PopoverBase(c, el, &p.open, func(panel ui.Element) {
		t := c.Theme()
		panel.Margin(6, 0, 0, 0).Padding(10).Radius(10).Background(t.Background).Border(1, t.Border).
			Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.45))
		v.periodKeys(panel)
		ui.Row(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
			v.periodPresets(c)
			v.periodCalendar(c)
		})
	})
}

func (v *view) periodKeys(panel ui.Element) {
	p := &v.periodUI
	move := func(days int) func() {
		return func() {
			p.cursor = p.cursor.AddDate(0, 0, days)
			if p.cursor.After(v.today()) {
				p.cursor = v.today()
			}
			v.showCursor()
		}
	}
	panel.OnShortcut(0, ui.KeyLeft, move(-1))
	panel.OnShortcut(0, ui.KeyRight, move(1))
	panel.OnShortcut(0, ui.KeyUp, move(-7))
	panel.OnShortcut(0, ui.KeyDown, move(7))
	panel.OnShortcut(0, ui.KeyEnter, func() { v.pickDay(p.cursor) })
	panel.OnShortcut(0, ui.KeyPageUp, func() { v.shiftMonths(-1) })
	panel.OnShortcut(0, ui.KeyPageDown, func() { v.shiftMonths(1) })
}

// shiftMonths moves the months shown, up to this month on the right.
func (v *view) shiftMonths(n int) {
	p := &v.periodUI
	m := p.month.AddDate(0, n, 0)
	if last := time.Date(v.snap.Now.Year(), v.snap.Now.Month()-1, 1, 0, 0, 0, 0, time.Local); m.After(last) {
		m = last
	}
	p.month = m
}

// showCursor moves the months shown so the cursor is on one of them.
func (v *view) showCursor() {
	p := &v.periodUI
	first := time.Date(p.cursor.Year(), p.cursor.Month(), 1, 0, 0, 0, 0, time.Local)
	switch {
	case first.Before(p.month):
		p.month = first
	case first.After(p.month.AddDate(0, 1, 0)):
		p.month = first.AddDate(0, -1, 0)
	}
}

func (v *view) periodPresets(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(150).Gap(2).PaddingEnd(10).BorderEndWidth(1).BorderColor(t.Border).Children(func() {
		for i, pr := range periods {
			if presetRow(c.Key(i), pr.label, v.filter.period == i, true).Clicked() {
				v.filter.period = i
				v.filter.from, v.filter.to, v.filter.ongoing = time.Time{}, time.Time{}, false
				v.periodUI.open = false
				v.resetPoints()
			}
		}
		ui.Box(c).Height(1).Background(t.Border).Margin(4, 4)
		presetRow(c.Key("custom"), "Custom", v.filter.period == periodCustom, false)
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

func (v *view) periodCalendar(c *ui.Context) {
	t := c.Theme()
	p := &v.periodUI
	played := v.playDays()
	ui.Column(c).Gap(10).Children(func() {
		ui.Row(c).Gap(14).AlignItems(ui.Start).Children(func() {
			monthArrow(c, "Earlier month", iconPrevStep, false, func() { v.shiftMonths(-1) })
			v.month(c.Key("m0"), p.month, played)
			v.month(c.Key("m1"), p.month.AddDate(0, 1, 0), played)
			atEnd := !p.month.AddDate(0, 1, 0).Before(time.Date(v.snap.Now.Year(), v.snap.Now.Month(), 1, 0, 0, 0, 0, time.Local))
			monthArrow(c, "Later month", iconNextStep, atEnd, func() { v.shiftMonths(1) })
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(10, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			v.periodSummary(c, played)
			ui.Spacer(c)
			was := v.filter.period == periodCustom && v.filter.ongoing
			on := was
			row := ui.Row(c).Gap(8).AlignItems(ui.Center).Cursor(ui.CursorPointer).Tooltip("Keep the end at today").Children(func() {
				ui.Switch(c, &on).Label("Ongoing")
				ui.Text(c, "Ongoing").FontSize(13).TextColor(t.TextMuted)
			})
			if row.Clicked() {
				on = !was
			}
			if on != was {
				v.setOngoing(on)
			}
		})
	})
}

// periodSummary is the custom period and what was played in it, or how to pick one.
func (v *view) periodSummary(c *ui.Context, played map[time.Time]stats.Record) {
	t := c.Theme()
	f := v.filter
	if f.period != periodCustom {
		ui.Text(c, "Click a day, then another to make a range.").FontSize(13).TextColor(t.TextMuted)
		return
	}
	end := v.customEnd()
	name := v.longDay(f.from)
	switch {
	case f.ongoing:
		name = "Since " + name
	case !end.Equal(f.from):
		name += " – " + v.longDay(end)
	}
	var r stats.Record
	n := 0
	for d, dr := range played {
		if !d.Before(f.from) && !d.After(end) {
			n++
			r.Wins += dr.Wins
			r.Losses += dr.Losses
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
func (v *view) month(c *ui.Context, first time.Time, played map[time.Time]stats.Record) {
	t := c.Theme()
	ui.Column(c).Gap(2).Children(func() {
		ui.Text(c, first.Format("January 2006")).Bold().FontSize(13).Padding(3, 4, 8, 4)
		ui.Row(c).Children(func() {
			for _, d := range []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"} {
				ui.Text(c, d).FontSize(11).TextColor(faintColor(t)).Width(calCell).TextAlign(ui.Center)
			}
		})
		lead := (int(first.Weekday()) + 6) % 7
		start := first.AddDate(0, 0, -lead)
		for w := range 6 {
			ui.Row(c.Key(w)).Children(func() {
				for i := range 7 {
					d := start.AddDate(0, 0, w*7+i)
					if d.Month() != first.Month() {
						ui.Box(c.Key(i)).Size(calCell, 40)
						continue
					}
					v.dayCell(c.Key(i), d, played)
				}
			})
		}
	})
}

// cellSpot is where a day sits in the custom period.
type cellSpot struct{ in, first, last bool }

func (v *view) spot(d time.Time) cellSpot {
	f := v.filter
	if f.period != periodCustom {
		return cellSpot{}
	}
	end := v.customEnd()
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

func (v *view) dayCell(c *ui.Context, d time.Time, played map[time.Time]stats.Record) {
	t := c.Theme()
	sp := v.spot(d)
	future := d.After(v.today())
	b := ui.ButtonBase(c).Label("Pick "+v.longDay(d)).Size(calCell, 40).Background(ui.Transparent).Disabled(future)
	sp.shape(b, t, b.Hovered() && !future)
	if d.Equal(v.periodUI.cursor) && b.Focused() {
		b.Border(1, t.Accent)
	}
	if b.Clicked() {
		v.pickDay(d)
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
			ui.Text(c, record(r)).FontSize(10).FontWeight(500).TextColor(recordInk(r, sp.edge()))
		case d.Equal(v.today()):
			dot := t.Accent
			if sp.edge() {
				dot = onAccent
			}
			ui.Box(c).Size(4, 4).Radius(2).Background(dot).Margin(2, 0, 0, 0)
		}
	})
}

// recordInk is a day's record in the result's color, or dark on the accent.
func recordInk(r stats.Record, onFill bool) ui.Color {
	switch {
	case onFill:
		return onAccent
	case r.Losses > r.Wins:
		return lossColor
	}
	return winColor
}
