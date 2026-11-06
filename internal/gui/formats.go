package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

type formatsState struct {
	name  string
	start time.Time
	err   string
	// editing is the ID of the format being renamed, and rename its new name.
	editing int64
	rename  string
}

func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func (v *view) do(fn func(actions)) {
	if v.act != nil {
		fn(v.act)
	}
}

func (v *view) formatsPage(c *ui.Context) {
	t := c.Theme()
	s := &v.formats
	if s.start.IsZero() {
		s.start = dayStart(v.snap.Now)
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).MaxWidth(760).Children(func() {
			ui.Text(c, "Formats").FontSize(20).Bold()
			ui.Text(c, "A format runs from the day it starts until the next one starts, and holds the ranked games in it. Casual lobbies can be any format, so they only show under All formats. The sim's logs don't say which format a game was played in, so you set where each one begins.").TextColor(t.TextMuted).Wrap()
			for _, d := range v.snap.Debuts {
				v.debutTile(c.Key(d.Set), d)
			}
			v.addFormatTile(c)
			v.formatList(c)
		})
	})
}

// debutBanner offers to start a format for each set that just showed up, over the Overview.
func (v *view) debutBanner(c *ui.Context) {
	ui.Column(c).Padding(14, 20, 0, 20).Gap(8).Children(func() {
		for _, d := range v.snap.Debuts {
			v.debutTile(c.Key(d.Set), d)
		}
	})
}

func (v *view) debutTile(c *ui.Context, d stats.SetDebut) {
	t := c.Theme()
	day := dayStart(d.First)
	ui.Row(c).Gap(12).Padding(12, 16).Radius(8).Background(t.Surface).Border(1, t.Accent).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, fmt.Sprintf("%s cards first showed up in your games on %s", d.Set, day.Format("Jan 2"))).Bold()
			ui.Text(c, "That usually means a new format started.").FontSize(12).TextColor(t.TextMuted)
		})
		if ui.PrimaryButton(c, "Start a format on "+day.Format("Jan 2")).Clicked() {
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
			if ui.Button(c, "Add").Clicked() {
				v.addFormat()
			}
		})
		if s.err != "" {
			ui.Text(c, s.err).FontSize(12).TextColor(lossColor)
		} else {
			ui.Text(c, "It starts at midnight on that day.").FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (v *view) addFormat() {
	s := &v.formats
	name := strings.TrimSpace(s.name)
	start := dayStart(s.start)
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
	tile(c, "Your formats", func() {
		for i := len(fs) - 1; i >= 0; i-- {
			v.formatRow(c.Key(fs[i].ID), fs[i])
		}
		before := stats.Scope{Type: stats.RankedGames, Until: fs[0].Start}.Apply(v.snap.Matches)
		if len(before) > 0 {
			ui.Text(c, fmt.Sprintf("%d ranked %s before %s belong to no format.", len(before), map[bool]string{true: "game", false: "games"}[len(before) == 1], fs[0].Start.Format("Jan 2"))).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (v *view) formatRow(c *ui.Context, f stats.FormatRange) {
	t := c.Theme()
	s := &v.formats
	ms := stats.Scope{Type: stats.RankedGames, Since: f.Start, Until: f.End}.Apply(v.snap.Matches)
	var r stats.Record
	for _, m := range ms {
		r.Add(m.Result)
	}
	end := "now"
	if !f.End.IsZero() {
		end = f.End.Format("Jan 2")
	}
	ui.Row(c).Gap(12).Padding(6, 0).Children(func() {
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
				ui.Text(c, f.Name).Bold()
			}
			ui.Text(c, fmt.Sprintf("%s to %s · %s ranked · %s", f.Start.Format("Jan 2"), end, games(len(ms)), record(r))).FontSize(12).TextColor(t.TextMuted)
		})
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
