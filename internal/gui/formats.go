package gui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/VTGare/logpose/internal/stats"
)

type formatsState struct {
	name  string
	start time.Time
	err   string
	// editing is the ID of the format being renamed, and rename its new name.
	editing int64
	rename  string
	// day is the timeline's day under the pointer, or -1.
	day int
}

func (v *view) formatsPage(c *ui.Context) {
	t := c.Theme()
	s := &v.formats
	if s.start.IsZero() {
		s.start = stats.DayStart(v.now())
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			ui.Text(c, "Formats").FontSize(20).FontWeight(600)
			ui.Text(c, "A format runs from the day it starts until the next one starts, and holds the ranked games in it. Casual lobbies can be any format, so they only show under All formats. The sim's logs don't say which format a game was played in, so you set where each one begins.").TextColor(t.TextMuted).Wrap().MaxWidth(820)
			v.debutTiles(c)
			v.formatTimeline(c)
			v.formatList(c)
			v.addFormatTile(c)
		})
	})
}

// debutBanner offers to start a format for each set that just showed up, over the Overview.
func (v *view) debutBanner(c *ui.Context) {
	if len(v.snap.Debuts) == 0 {
		return
	}
	ui.Column(c).Padding(14, 20, 0, 20).Gap(8).Children(func() { v.debutTiles(c) })
}

func (v *view) debutTiles(c *ui.Context) {
	for _, d := range v.snap.Debuts {
		v.debutTile(c.Key(d.Set), d)
	}
}

func (v *view) debutTile(c *ui.Context, d stats.SetDebut) {
	t := c.Theme()
	day := stats.DayStart(d.First)
	ui.Row(c).Gap(12).Padding(12, 16).Radius(12).Background(panelColor(t)).Border(1, t.Accent).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, fmt.Sprintf("%s cards first showed up in your games on %s", d.Set, v.date(day))).Bold()
			ui.Text(c, "That usually means a new format started.").FontSize(12).TextColor(t.TextMuted)
		})
		if ui.PrimaryButton(c, "Start a format on "+v.date(day)).Clicked() {
			v.do(func(a actions) { a.AddFormat(d.Set, day) })
		}
		if ui.Button(c, "Dismiss").Clicked() {
			v.do(func(a actions) { a.DismissSet(d.Set) })
		}
	})
}

func (v *view) addFormatTile(c *ui.Context) {
	t := c.Theme()
	s := &v.formats
	tile(c, "Add a format", func() {
		ui.Row(c).Gap(10).Children(func() {
			ui.TextInput(c, &s.name).Placeholder("Name, like OP17 + EB05").Grow(1).Label("Format name")
			ui.DateInput(c, &s.start).Label("Starts on")
			if ui.PrimaryButton(c, "Add format").Label("Add").Clicked() {
				v.addFormat()
			}
		})
		if s.err != "" {
			ui.Text(c, s.err).FontSize(12).TextColor(lossColor(t))
		} else {
			ui.Text(c, "It starts at midnight on that day.").FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (v *view) addFormat() {
	s := &v.formats
	name := strings.TrimSpace(s.name)
	start := stats.DayStart(s.start)
	switch {
	case name == "":
		s.err = "Give the format a name."
		return
	case v.formatStartsOn(start):
		s.err = "A format already starts on that day."
		return
	}
	s.err, s.name = "", ""
	v.do(func(a actions) { a.AddFormat(name, start) })
}

func (v *view) formatStartsOn(day time.Time) bool {
	for _, f := range v.snap.Formats {
		if f.Start.Equal(day) {
			return true
		}
	}
	return false
}

// formatList is every format, newest first, with its dates and record.
func (v *view) formatList(c *ui.Context) {
	t := c.Theme()
	fs := v.snap.Formats
	if len(fs) == 0 {
		return
	}
	panel(c).Gap(8).Padding(14, 16).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Your formats").FontSize(13).FontWeight(600)
			ui.Spacer(c)
			ui.Text(c, "Pages start on").FontSize(12).TextColor(t.TextMuted)
			starts := []int64{formatCurrent, formatAll}
			start := max(slices.Index(starts, v.prefs.startFormat), 0)
			ui.Segmented(c, &start, "Latest format", "All formats").Label("Pages start on")
			if starts[start] != v.prefs.startFormat {
				v.setStartFormat(starts[start])
			}
		})
		for i := len(fs) - 1; i >= 0; i-- {
			v.formatRow(c.Key(fs[i].ID), fs[i])
		}
		before := stats.FormatRange{End: fs[0].Start}.Scope().Apply(v.snap.Matches)
		if len(before) > 0 {
			ui.Text(c, fmt.Sprintf("%d ranked %s before %s belong to no format.", len(before), plural(len(before), "game", "games"), v.date(fs[0].Start))).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (v *view) formatRow(c *ui.Context, f stats.FormatRange) {
	t := c.Theme()
	s := &v.formats
	ms := f.Scope().Apply(v.snap.Matches)
	r := stats.RecordOf(ms)
	end := "now"
	if !f.End.IsZero() {
		end = v.date(f.End)
	}
	current := f.End.IsZero()
	ui.Row(c).Gap(14).Padding(8, 0).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 36).Radius(3).Background(map[bool]ui.Color{true: t.Accent, false: t.Border}[current])
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			if s.editing == f.ID {
				ui.Row(c).Gap(8).Children(func() {
					in := ui.TextInput(c, &s.rename).Grow(1).Label("New name")
					if (in.Submitted() || ui.Button(c, "Save").Clicked()) && strings.TrimSpace(s.rename) != "" {
						g := f.Format
						g.Name = s.rename
						v.do(func(a actions) { a.UpdateFormat(g) })
						s.editing = 0
					}
					if ui.Button(c, "Cancel").Clicked() {
						s.editing = 0
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
			ui.Text(c, fmt.Sprintf("%s to %s · %s ranked · %s", v.date(f.Start), end, games(len(ms)), record(r))).FontSize(12).TextColor(t.TextMuted)
		})
		if rate, ok := r.Rate(); ok && s.editing != f.ID {
			ui.Text(c, pct(rate)).FontSize(18).Bold()
		}
		if s.editing != f.ID {
			if ui.Button(c, "Rename").Clicked() {
				s.editing, s.rename = f.ID, f.Name
			}
			if ui.Button(c, "Delete").Label("Delete " + f.Name).Clicked() {
				id := f.ID
				v.do(func(a actions) { a.DeleteFormat(id) })
			}
		}
	})
}

// formatTimeline is every day from the first game to today, with the day's ranked games over
// its casual ones and each format as a band over the days it covers. Clicking a day puts it in
// the add form as a start.
func (v *view) formatTimeline(c *ui.Context) {
	t := c.Theme()
	ms := v.snap.Matches
	if len(ms) == 0 {
		return
	}
	first := stats.DayStart(ms[len(ms)-1].PlayedAt)
	ranked, casual, top := gamesByDay(ms, first, stats.DayStart(v.now()))
	n := len(ranked)
	dayAt := func(i int) time.Time { return first.AddDate(0, 0, i) }
	inFormat := func(i int) bool {
		fs := v.snap.Formats
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
	s := &v.formats
	describe := func(i int) string {
		return fmt.Sprintf("%s · %d ranked, %d casual", v.weekday(dayAt(i)), ranked[i], casual[i])
	}
	tile(c, "Your games by day", func() {
		chart.Plot(c, chart.Axes{Bottom: bottom, Label: axisLabel(c)}, func(area ui.Element) {
			area.Label("Games by day").Cursor(ui.CursorPointer).Draw(func(p *ui.Painter, r ui.Rect) {
				const labelH = 20
				colW := r.W / float32(n)
				formatBands(p, r, t, v.snap.Formats, first, n, labelH)
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
			chart.Select(c, area, &s.day, xs, describe)
			if area.Clicked() && s.day >= 0 && s.day < n {
				s.start = dayAt(s.day)
			}
			if i := s.day; i >= 0 && i < n {
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
