package page

import (
	"fmt"
	"time"

	"github.com/VTGare/logpose/internal/gui/kit"

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

func (e *Env) Today() time.Time { return stats.DayStart(e.Now()) }

// customEnd is the last day of the custom period.
func (e *Env) customEnd() time.Time {
	if e.Filter.ongoing {
		return e.Today()
	}
	return e.Filter.to
}

// periodBounds is when the period starts and ends. Either is zero when it doesn't.
func (e *Env) periodBounds() (since, until time.Time) {
	f := e.Filter
	if f.Period == periodCustom {
		if f.ongoing {
			return f.from, time.Time{}
		}
		return f.from, f.to.AddDate(0, 0, 1)
	}
	if d := periods[f.Period].days; d > 0 {
		since = e.Now().AddDate(0, 0, -d)
	}
	return since, time.Time{}
}

func (e *Env) PeriodLabel() string {
	f := e.Filter
	switch {
	case f.Period != periodCustom:
		return periods[f.Period].label
	case f.ongoing:
		return "Since " + e.Date(f.from)
	case f.from.Equal(f.to):
		return e.Weekday(f.from)
	}
	return kit.Span(e.Date(f.from), e.Date(f.to))
}

func (e *Env) SetRange(first, last time.Time) {
	e.Filter.Period = periodCustom
	e.Filter.from, e.Filter.to = first, last
	e.Filter.ongoing = false
	e.periodUI.anchor = false
}

// pickDay is a click on a day in the calendar. The first click picks one day and the second
// makes a range with it, in either order.
func (e *Env) pickDay(d time.Time) {
	f := &e.Filter
	p := &e.periodUI
	p.cursor = d
	switch {
	case f.Period == periodCustom && f.ongoing:
		f.from = d
	case f.Period == periodCustom && p.anchor:
		first, last := f.from, d
		if d.Before(first) {
			first, last = d, f.from
		}
		e.SetRange(first, last)
		return
	default:
		e.SetRange(d, d)
	}
	p.anchor = !f.ongoing
}

func (e *Env) setOngoing(on bool) {
	f := &e.Filter
	if f.Period != periodCustom {
		f.Period, f.from = periodCustom, e.Today()
	}
	f.ongoing = on
	f.to = e.Today()
	e.periodUI.anchor = false
}

// openPeriod shows the months holding the end of the period, or this month.
func (e *Env) openPeriod() {
	p := &e.periodUI
	end := e.Today()
	if e.Filter.Period == periodCustom {
		end = e.customEnd()
	}
	p.month = time.Date(end.Year(), end.Month()-1, 1, 0, 0, 0, 0, time.Local)
	p.cursor = end
	p.anchor = false
}

// playDays is the record of each day played, in the filters' scope without the period.
func (e *Env) playDays() map[time.Time]stats.Record {
	out := make(map[time.Time]stats.Record)
	for _, d := range stats.Days(e.scopeAnyPeriod().Apply(e.Snap.Matches)) {
		out[d.Date] = d.Record
	}
	return out
}

func (e *Env) periodPill(c *ui.Context) {
	p := &e.periodUI
	el := FilterPill(c.Key("period"), e.PeriodLabel(), e.Filter.Period != 0)
	if el.Clicked() {
		p.open = !p.open
		if p.open {
			e.openPeriod()
		}
	}
	ui.PopoverBase(c, el, &p.open, func(panel ui.Element) {
		t := c.Theme()
		kit.Popover(panel.Margin(6, 0, 0, 0).Padding(10), t)
		e.periodKeys(panel)
		ui.Row(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
			e.periodPresets(c)
			e.periodCalendar(c)
		})
	})
}

func (e *Env) periodKeys(panel ui.Element) {
	p := &e.periodUI
	move := func(days int) func() {
		return func() {
			p.cursor = p.cursor.AddDate(0, 0, days)
			if p.cursor.After(e.Today()) {
				p.cursor = e.Today()
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
func (e *Env) shiftMonths(n int) {
	p := &e.periodUI
	m := p.month.AddDate(0, n, 0)
	if last := time.Date(e.Now().Year(), e.Now().Month()-1, 1, 0, 0, 0, 0, time.Local); m.After(last) {
		m = last
	}
	p.month = m
}

// showCursor moves the months shown so the cursor is on one of them.
func (e *Env) showCursor() {
	p := &e.periodUI
	first := time.Date(p.cursor.Year(), p.cursor.Month(), 1, 0, 0, 0, 0, time.Local)
	switch {
	case first.Before(p.month):
		p.month = first
	case first.After(p.month.AddDate(0, 1, 0)):
		p.month = first.AddDate(0, -1, 0)
	}
}

func (e *Env) periodPresets(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(150).Gap(2).PaddingEnd(10).BorderEndWidth(1).BorderColor(t.Border).Children(func() {
		for i, pr := range periods {
			if presetRow(c.Key(i), pr.label, e.Filter.Period == i, true).Clicked() {
				e.Filter.Period = i
				e.Filter.from, e.Filter.to, e.Filter.ongoing = time.Time{}, time.Time{}, false
				e.periodUI.open = false
			}
		}
		ui.Box(c).Height(1).Background(t.Border).Margin(4, 4)
		presetRow(c.Key("custom"), "Custom", e.Filter.Period == periodCustom, false)
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
				ui.Icon(c, kit.IconCheck).FontSize(14).TextColor(t.Accent)
			}
		})
		col := t.Text
		if !button && !on {
			col = kit.FaintColor(t)
		}
		ui.Text(c, label).FontSize(13).TextColor(col).SingleLine()
	})
}

func (e *Env) periodCalendar(c *ui.Context) {
	t := c.Theme()
	p := &e.periodUI
	played := e.playDays()
	ui.Column(c).Gap(10).Children(func() {
		ui.Row(c).Gap(14).AlignItems(ui.Start).Children(func() {
			monthArrow(c, "Earlier month", kit.IconPrevStep, false, func() { e.shiftMonths(-1) })
			e.month(c.Key("m0"), p.month, played)
			e.month(c.Key("m1"), p.month.AddDate(0, 1, 0), played)
			atEnd := !p.month.AddDate(0, 1, 0).Before(time.Date(e.Now().Year(), e.Now().Month(), 1, 0, 0, 0, 0, time.Local))
			monthArrow(c, "Later month", kit.IconNextStep, atEnd, func() { e.shiftMonths(1) })
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(10, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			e.periodSummary(c, played)
			ui.Spacer(c)
			was := e.Filter.Period == periodCustom && e.Filter.ongoing
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
func (e *Env) periodSummary(c *ui.Context, played map[time.Time]stats.Record) {
	t := c.Theme()
	f := e.Filter
	if f.Period != periodCustom {
		ui.Text(c, "Click a day, then another to make a range.").FontSize(13).TextColor(t.TextMuted)
		return
	}
	end := e.customEnd()
	name := kit.Span(e.Weekday(f.from), e.Weekday(end))
	if f.ongoing {
		name = "Since " + e.Weekday(f.from)
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
		sum = fmt.Sprintf("%d days played · %s · %s", n, kit.Games(r.Games()), kit.Record(r))
		if n == 1 {
			sum = fmt.Sprintf("%s · %s", kit.Games(r.Games()), kit.Record(r))
		}
	}
	ui.Text(c, sum).FontSize(12).TextColor(kit.FaintColor(t))
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
		col = kit.FaintColor(t).Alpha(0.4)
	}
	b.Children(func() { ui.Icon(c, icon).FontSize(14).TextColor(col) })
}

const calCell = 38

// month is a month's days in weeks from Monday. Days played show their record.
func (e *Env) month(c *ui.Context, first time.Time, played map[time.Time]stats.Record) {
	t := c.Theme()
	ui.Column(c).Gap(2).Children(func() {
		ui.Text(c, first.Format("January 2006")).Bold().FontSize(13).Padding(3, 4, 8, 4)
		ui.Row(c).Children(func() {
			for _, d := range []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"} {
				ui.Text(c, d).FontSize(11).TextColor(kit.FaintColor(t)).Width(calCell).TextAlign(ui.Center)
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

func (e *Env) spot(d time.Time) cellSpot {
	f := e.Filter
	if f.Period != periodCustom {
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

func (e *Env) dayCell(c *ui.Context, d time.Time, played map[time.Time]stats.Record) {
	t := c.Theme()
	sp := e.spot(d)
	future := d.After(e.Today())
	b := ui.ButtonBase(c).Label("Pick "+e.Weekday(d)).Size(calCell, 40).Background(ui.Transparent).Disabled(future)
	sp.shape(b, t, b.Hovered() && !future)
	if d.Equal(e.periodUI.cursor) && b.Focused() {
		b.Border(1, t.Accent)
	}
	if b.Clicked() {
		e.pickDay(d)
	}
	r, ok := played[d]
	ink := kit.FaintColor(t)
	switch {
	case sp.edge():
		ink = kit.OnAccent
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
			ui.Text(c, kit.Record(r)).FontSize(10).FontWeight(500).TextColor(recordInk(t, r, sp.edge()))
		case d.Equal(e.Today()):
			dot := t.Accent
			if sp.edge() {
				dot = kit.OnAccent
			}
			ui.Box(c).Size(4, 4).Radius(2).Background(dot).Margin(2, 0, 0, 0)
		}
	})
}

// recordInk is a day's record in the result's color, or dark on the accent.
func recordInk(t *ui.Theme, r stats.Record, onFill bool) ui.Color {
	switch {
	case onFill:
		return kit.OnAccent
	case r.Losses > r.Wins:
		return kit.LossColor(t)
	}
	return kit.WinColor(t)
}
