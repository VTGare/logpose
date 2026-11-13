package gui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

type formatActions interface {
	AddFormat(name string, start time.Time)
	UpdateFormat(f store.Format)
	DeleteFormat(id int64)
	DismissSet(set string)
}

type formatsPage struct {
	act formatActions

	name  string
	start time.Time
	err   string
	// editing is the ID of the format being renamed, and rename its new name.
	editing int64
	rename  string
	// day is the timeline's day under the pointer, or -1.
	day int
}

func (pg *formatsPage) do(fn func(formatActions)) {
	if pg.act != nil {
		fn(pg.act)
	}
}

func (pg *formatsPage) draw(c *ui.Context, e *env) {
	t := c.Theme()
	if pg.start.IsZero() {
		pg.start = stats.DayStart(e.now())
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			ui.Text(c, "Formats").FontSize(20).FontWeight(600)
			ui.Text(c, "A format runs from the day it starts until the next one starts, and holds the ranked games in it. Casual lobbies can be any format, so they only show under All formats. The sim's logs don't say which format a game was played in, so you set where each one begins.").TextColor(t.TextMuted).Wrap().MaxWidth(820)
			pg.debutTiles(c, e)
			pg.formatTimeline(c, e)
			pg.formatList(c, e)
			pg.addFormatTile(c, e)
		})
	})
}

// debutBanner offers to start a format for each set that just showed up, over the Overview.
func (pg *formatsPage) debutBanner(c *ui.Context, e *env) {
	if len(e.snap.Debuts) == 0 {
		return
	}
	ui.Column(c).Padding(14, 20, 0, 20).Gap(8).Children(func() { pg.debutTiles(c, e) })
}

func (pg *formatsPage) debutTiles(c *ui.Context, e *env) {
	for _, d := range e.snap.Debuts {
		pg.debutTile(c.Key(d.Set), e, d)
	}
}

func (pg *formatsPage) debutTile(c *ui.Context, e *env, d stats.SetDebut) {
	t := c.Theme()
	day := stats.DayStart(d.First)
	ui.Row(c).Gap(12).Padding(12, 16).Radius(12).Background(panelColor(t)).Border(1, t.Accent).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, fmt.Sprintf("%s cards first showed up in your games on %s", d.Set, e.date(day))).Bold()
			ui.Text(c, "That usually means a new format started.").FontSize(12).TextColor(t.TextMuted)
		})
		if ui.PrimaryButton(c, "Start a format on "+e.date(day)).Clicked() {
			pg.do(func(a formatActions) { a.AddFormat(d.Set, day) })
		}
		if ui.Button(c, "Dismiss").Clicked() {
			pg.do(func(a formatActions) { a.DismissSet(d.Set) })
		}
	})
}

func (pg *formatsPage) addFormatTile(c *ui.Context, e *env) {
	t := c.Theme()
	tile(c, "Add a format", func() {
		ui.Row(c).Gap(10).Children(func() {
			ui.TextInput(c, &pg.name).Placeholder("Name, like OP17 + EB05").Grow(1).Label("Format name")
			ui.DateInput(c, &pg.start).Label("Starts on")
			if ui.PrimaryButton(c, "Add format").Label("Add").Clicked() {
				pg.addFormat(e)
			}
		})
		if pg.err != "" {
			ui.Text(c, pg.err).FontSize(12).TextColor(lossColor(t))
		} else {
			ui.Text(c, "It starts at midnight on that day.").FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (pg *formatsPage) addFormat(e *env) {
	name := strings.TrimSpace(pg.name)
	start := stats.DayStart(pg.start)
	switch {
	case name == "":
		pg.err = "Give the format a name."
		return
	case formatStartsOn(e.snap.Formats, start):
		pg.err = "A format already starts on that day."
		return
	}
	pg.err, pg.name = "", ""
	pg.do(func(a formatActions) { a.AddFormat(name, start) })
}

func formatStartsOn(fs []stats.FormatRange, day time.Time) bool {
	for _, f := range fs {
		if f.Start.Equal(day) {
			return true
		}
	}
	return false
}

// formatList is every format, newest first, with its dates and record.
func (pg *formatsPage) formatList(c *ui.Context, e *env) {
	t := c.Theme()
	fs := e.snap.Formats
	if len(fs) == 0 {
		return
	}
	panel(c).Gap(8).Padding(14, 16).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Your formats").FontSize(13).FontWeight(600)
			ui.Spacer(c)
			ui.Text(c, "Pages start on").FontSize(12).TextColor(t.TextMuted)
			starts := []int64{formatCurrent, formatAll}
			start := max(slices.Index(starts, e.prefs.startFormat), 0)
			ui.Segmented(c, &start, "Latest format", "All formats").Label("Pages start on")
			if starts[start] != e.prefs.startFormat {
				e.setStartFormat(starts[start])
			}
		})
		for i := len(fs) - 1; i >= 0; i-- {
			pg.formatRow(c.Key(fs[i].ID), e, fs[i])
		}
		before := stats.FormatRange{End: fs[0].Start}.Scope().Apply(e.snap.Matches)
		if len(before) > 0 {
			ui.Text(c, fmt.Sprintf("%d ranked %s before %s belong to no format.", len(before), plural(len(before), "game", "games"), e.date(fs[0].Start))).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (pg *formatsPage) formatRow(c *ui.Context, e *env, f stats.FormatRange) {
	t := c.Theme()
	ms := f.Scope().Apply(e.snap.Matches)
	r := stats.RecordOf(ms)
	end := "now"
	if !f.End.IsZero() {
		end = e.date(f.End)
	}
	current := f.End.IsZero()
	ui.Row(c).Gap(14).Padding(8, 0).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 36).Radius(3).Background(map[bool]ui.Color{true: t.Accent, false: t.Border}[current])
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			if pg.editing == f.ID {
				ui.Row(c).Gap(8).Children(func() {
					in := ui.TextInput(c, &pg.rename).Grow(1).Label("New name")
					if (in.Submitted() || ui.Button(c, "Save").Clicked()) && strings.TrimSpace(pg.rename) != "" {
						g := f.Format
						g.Name = pg.rename
						pg.do(func(a formatActions) { a.UpdateFormat(g) })
						pg.editing = 0
					}
					if ui.Button(c, "Cancel").Clicked() {
						pg.editing = 0
					}
				})
			} else {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, f.Name).FontWeight(600)
					if current {
						ui.Text(c, "Current").FontSize(11).FontWeight(600).TextColor(t.AccentText).Padding(1, 7).Radius(9).Background(t.Accent)
					}
				})
			}
			ui.Text(c, fmt.Sprintf("%s to %s · %s ranked · %s", e.date(f.Start), end, games(len(ms)), record(r))).FontSize(12).TextColor(t.TextMuted)
		})
		if rate, ok := r.Rate(); ok && pg.editing != f.ID {
			ui.Text(c, pct(rate)).FontSize(18).Bold()
		}
		if pg.editing != f.ID {
			if ui.Button(c, "Rename").Clicked() {
				pg.editing, pg.rename = f.ID, f.Name
			}
			if ui.Button(c, "Delete").Label("Delete " + f.Name).Clicked() {
				id := f.ID
				pg.do(func(a formatActions) { a.DeleteFormat(id) })
			}
		}
	})
}

// formatTimeline is every day from the first game to today, with the day's ranked games over
// its casual ones and each format as a band over the days it covers. Clicking a day puts it in
// the add form as a start.
func (pg *formatsPage) formatTimeline(c *ui.Context, e *env) {
	t := c.Theme()
	ms := e.snap.Matches
	if len(ms) == 0 {
		return
	}
	first := stats.DayStart(ms[len(ms)-1].PlayedAt)
	ranked, casual, top := gamesByDay(ms, first, stats.DayStart(e.now()))
	n := len(ranked)
	dayAt := func(i int) time.Time { return first.AddDate(0, 0, i) }
	inFormat := func(i int) bool {
		fs := e.snap.Formats
		return len(fs) > 0 && !dayAt(i).Before(fs[0].Start)
	}
	xs := make([]float32, n)
	for i := range xs {
		xs[i] = (float32(i) + 0.5) / float32(n)
	}
	var bottom []chart.Tick
	for i := range n {
		if d := dayAt(i); d.Weekday() == time.Monday || i == 0 {
			bottom = append(bottom, chart.Tick{At: xs[i], Label: d.Format("Jan 2")})
		}
	}
	describe := func(i int) string {
		return fmt.Sprintf("%s · %d ranked, %d casual", e.weekday(dayAt(i)), ranked[i], casual[i])
	}
	tile(c, "Your games by day", func() {
		chart.Plot(c, chart.Axes{Bottom: bottom, Label: axisLabel(c)}, func(area ui.Element) {
			area.Label("Games by day").Cursor(ui.CursorPointer).Draw(func(p *ui.Painter, r ui.Rect) {
				const labelH = 20
				colW := r.W / float32(n)
				formatBands(p, r, t, e.snap.Formats, first, n, labelH)
				h := r.H - labelH
				unit := h / float32(top)
				bw := min(14, colW*0.7)
				for i := range n {
					cx := r.X + (float32(i)+0.5)*colW
					rh, ch := float32(ranked[i])*unit, float32(casual[i])*unit
					col := t.TextMuted
					if inFormat(i) {
						col = t.Accent
					}
					p.Fill(ui.Rect{X: cx - bw/2, Y: r.Y + r.H - rh, W: bw, H: rh}, col, 2)
					p.Fill(ui.Rect{X: cx - bw/2, Y: r.Y + r.H - rh - ch, W: bw, H: max(ch-1, 0)}, t.Border, 2)
				}
				p.Line(r.X, r.Y+r.H, r.X+r.W, r.Y+r.H, 1, t.Border)
			})
			chart.Select(c, area, &pg.day, xs, describe)
			if area.Clicked() && pg.day >= 0 && pg.day < n {
				pg.start = dayAt(pg.day)
			}
			if i := pg.day; i >= 0 && i < n {
				align := ui.Center
				switch {
				case xs[i] < 0.2:
					align = ui.Start
				case xs[i] > 0.8:
					align = ui.End
				}
				chart.Anchor(c, xs[i], 1, align, ui.Start, func() {
					ui.Text(c, describe(i)+" · click to start a format here").NoWrap().FontSize(12).Padding(4, 8).Radius(6).Background(t.Surface).Border(1, t.Border)
				})
			}
		}).Height(150).Gap(6).Margin(6, 0, 0, 0)
		ui.Row(c).Gap(18).AlignItems(ui.Center).Children(func() {
			key := func(col ui.Color, label string) {
				ui.Row(c.Key(label)).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(10, 10).Radius(2).Background(col)
					ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
				})
			}
			key(t.Accent, "Ranked, in a format")
			key(t.TextMuted, "Ranked, before any format")
			key(t.Border, "Casual, any format")
		})
	})
}

// dayIndex is how many days day comes after the midnight first. Both are midnights, and the
// hours are rounded since a day with a clock change is 23 or 25 hours long.
func dayIndex(first, day time.Time) int {
	return int(day.Sub(first).Hours()/24 + 0.5)
}

// gamesByDay counts ranked and casual games on each day from first to last, and the most on any
// day.
func gamesByDay(ms []stats.Match, first, last time.Time) (ranked, casual []int, top int) {
	n := dayIndex(first, last) + 1
	ranked, casual, top = make([]int, n), make([]int, n), 1
	for _, m := range ms {
		i := dayIndex(first, stats.DayStart(m.PlayedAt))
		if i < 0 || i >= n {
			continue
		}
		if m.Ranked {
			ranked[i]++
		} else {
			casual[i]++
		}
		top = max(top, ranked[i]+casual[i])
	}
	return ranked, casual, top
}

// formatBands shades the days each format covers on the timeline, with its name over it.
func formatBands(p *ui.Painter, r ui.Rect, t *ui.Theme, fs []stats.FormatRange, first time.Time, n int, labelH float32) {
	colW := r.W / float32(n)
	for _, f := range fs {
		i0 := max(0, dayIndex(first, f.Start))
		i1 := n
		if !f.End.IsZero() {
			i1 = min(n, dayIndex(first, f.End))
		}
		if i0 >= n || i1 <= i0 {
			continue
		}
		x0, x1 := r.X+float32(i0)*colW, r.X+float32(i1)*colW
		p.Fill(ui.Rect{X: x0, Y: r.Y + labelH - 4, W: x1 - x0, H: r.H - labelH + 4}, t.Accent.Alpha(0.10), 6)
		p.Fill(ui.Rect{X: x0, Y: r.Y + labelH - 4, W: 2, H: r.H - labelH + 4}, t.Accent, 0)
		p.Text(x0+6, r.Y, f.Name, 12, t.Accent)
	}
}
