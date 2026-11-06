package gui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
)

// flowSeries is one tab of the turn-by-turn view: a count for each player at the end of each
// turn, and which marks sit under it.
type flowSeries struct {
	label   string
	me, opp []int
	marks   simlog.MarkKind
}

// flowAxisWidth is the width of the left axis on the chart and on the strip under it, so their
// turns line up.
const flowAxisWidth = 44

// flowTile shows life, hand or board by turn on one chart, picked by tabs, with the key moments'
// pins on top and the tab's marks under the lines. Under it, who was ahead on each at the end
// of every turn.
func (v *view) flowTile(c *ui.Context, m stats.Match) ui.Element {
	t := c.Theme()
	return tile(c, "Turn by turn", func() {
		l := v.detail.timeline
		switch {
		case l == nil:
			ui.Text(c, "Reading the log").TextColor(t.TextMuted)
			return
		case len(l.Me) < 2 || len(l.Opp) != len(l.Me):
			ui.Text(c, "The log doesn't track life for this game.").TextColor(t.TextMuted)
			return
		}
		rows := []flowSeries{
			{label: "Life", me: l.Me, opp: l.Opp, marks: simlog.MarkTrigger},
			{label: "Hand", me: l.MeHand, opp: l.OppHand},
			{label: "Board", me: l.MeBoard, opp: l.OppBoard, marks: simlog.MarkRemoved},
		}
		rows = slices.DeleteFunc(rows, func(r flowSeries) bool { return len(r.me) != len(l.Me) || len(r.opp) != len(l.Me) })
		labels := make([]string, len(rows))
		for i, r := range rows {
			labels[i] = r.label
		}
		tab := &v.detail.flowTab
		*tab = min(*tab, len(rows)-1)
		r := rows[*tab]
		ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
			ui.Segmented(c, tab, labels...)
			legend(c, firstColor, "You")
			legend(c, secondColor, m.Opp.Name)
			ui.Spacer(c)
			switch r.marks {
			case simlog.MarkTrigger:
				ui.Text(c, "○ Trigger").FontSize(12).TextColor(t.TextMuted)
			case simlog.MarkRemoved:
				ui.Text(c, "● Removed").FontSize(12).TextColor(t.TextMuted)
			}
		})
		x := chart.NewLinear(0, float64(len(l.Me)-1))
		xs := make([]float32, len(l.Me))
		for i := range xs {
			xs[i] = x.At(float64(i))
		}
		v.flowChart(c, m, l, r, xs, rows)
		v.aheadStrip(c, m, rows, xs)
	})
}

func (v *view) flowChart(c *ui.Context, m stats.Match, l *matchTimeline, r flowSeries, xs []float32, rows []flowSeries) {
	t := c.Theme()
	top := max(slices.Max(r.me), slices.Max(r.opp), 1)
	lo := 0.0
	if r.marks != 0 && slices.ContainsFunc(l.Marks, func(mk simlog.Mark) bool { return mk.Kind == r.marks }) {
		lo = -0.3 * float64(top)
	}
	// Headroom keeps the pins off the lines at the top.
	y := chart.NewLinear(lo, 1.25*float64(top))
	mine := make([]float32, len(xs))
	theirs := make([]float32, len(xs))
	for i := range xs {
		mine[i] = y.At(float64(r.me[i]))
		theirs[i] = y.At(float64(r.opp[i]))
	}
	var left []chart.Tick
	step := max(1, top/4)
	for n := 0; n <= top; n += step {
		left = append(left, chart.Tick{At: y.At(float64(n)), Label: fmt.Sprint(n)})
	}
	var bottom []chart.Tick
	turns := len(xs) - 1
	for i := 0; i <= turns; i += max(1, turns/12) {
		bottom = append(bottom, chart.Tick{At: xs[i], Label: fmt.Sprint(i)})
	}
	sel := &v.detail.lifePoint
	chart.Plot(c, chart.Axes{Left: left, Bottom: bottom, Label: flowAxisLabel(c)}, func(area ui.Element) {
		area.Label(r.label + " by turn").Draw(func(p *ui.Painter, rect ui.Rect) {
			for _, tick := range left {
				p.Line(rect.X, chart.Y(rect, tick.At), rect.X+rect.W, chart.Y(rect, tick.At), 1, t.Border)
			}
			p.StrokePath(chart.Line(rect, xs, theirs, chart.CurveLinear), 2, secondColor)
			p.StrokePath(chart.Line(rect, xs, mine, chart.CurveLinear), 2, firstColor)
		})
		for i, mo := range l.Moments {
			if mo.Turn < len(xs) {
				chart.Anchor(c.Key(fmt.Sprint("pin", i)), xs[mo.Turn], 1, ui.Center, ui.Start, func() {
					pin(c, i+1, playerColor(m, mo.Player))
				})
			}
		}
		if lo < 0 {
			chartMarks(c, m, l.Marks, r.marks, xs, y, lo)
		}
		chart.Select(c, area, sel, xs, func(i int) string { return flowSummary(m, rows, i) })
		if i := *sel; i >= 0 && i < len(xs) {
			chart.RuleX(c, xs[i], 1).Background(t.TextMuted)
			chart.Point(c, xs[i], theirs[i], 8, 8).Radius(4).Background(secondColor)
			chart.Point(c, xs[i], mine[i], 8, 8).Radius(4).Background(firstColor)
			moment, marks := v.turnLines(m, l, i)
			flowTooltip(c, xs[i], flowSummary(m, rows, i), moment, marks)
		}
	}).Height(200).Gap(4).Margin(8, 0, 0, 0)
}

// turnLines is a turn's key moment and the marks the moment doesn't already name.
func (v *view) turnLines(m stats.Match, l *matchTimeline, turn int) (moment, marks []string) {
	named := false
	for _, mo := range l.Moments {
		if mo.Turn == turn {
			moment = v.momentLines(m, mo)
			named = len(mo.Removed) > 0
		}
	}
	for _, line := range v.markLines(m, l.Marks, turn) {
		if !named || !strings.HasPrefix(line, removedPrefix) {
			marks = append(marks, line)
		}
	}
	return moment, marks
}

// flowAxisLabel gives left labels a fixed width, so the chart and the strip under it start at
// the same x.
func flowAxisLabel(c *ui.Context) func(chart.Side, chart.Tick) {
	bottom := axisLabel(c)
	return func(side chart.Side, tick chart.Tick) {
		if side == chart.Left {
			ui.Text(c, tick.Label).NoWrap().Width(flowAxisWidth).TextAlign(ui.End)
			return
		}
		bottom(side, tick)
	}
}

// aheadStrip has a row each for life, hand and board, with a cell per turn under the chart's
// turn in the color of whoever had more at its end. The turn under the pointer is outlined.
func (v *view) aheadStrip(c *ui.Context, m stats.Match, rows []flowSeries, xs []float32) {
	t := c.Theme()
	const rowH, gap = 14, 6
	sel := v.detail.lifePoint
	ticks := make([]chart.Tick, len(rows))
	h := float32(len(rows)*(rowH+gap) - gap)
	for i, r := range rows {
		ticks[i] = chart.Tick{At: 1 - (float32(i)*(rowH+gap)+rowH/2)/h, Label: r.label}
	}
	ui.Text(c, "Who was ahead").FontSize(12).TextColor(t.TextMuted).Margin(12, 0, 0, 0)
	chart.Plot(c, chart.Axes{Left: ticks, Label: flowAxisLabel(c)}, func(area ui.Element) {
		area.Label("Who was ahead by turn").Draw(func(p *ui.Painter, rect ui.Rect) {
			if len(xs) < 2 {
				return
			}
			w := rect.W / float32(len(xs)-1)
			for k, r := range rows {
				top := rect.Y + float32(k)*(rowH+gap)
				for i := 1; i < len(xs); i++ {
					col := t.Border
					switch {
					case r.me[i] > r.opp[i]:
						col = firstColor
					case r.me[i] < r.opp[i]:
						col = secondColor
					}
					cx := rect.X + rect.W*xs[i]
					x0, x1 := max(rect.X, cx-w/2+1), min(rect.X+rect.W, cx+w/2-1)
					p.Fill(ui.Rect{X: x0, Y: top, W: x1 - x0, H: rowH}, col, 2)
				}
			}
			if sel > 0 && sel < len(xs) {
				cx := rect.X + rect.W*xs[sel]
				x0, x1 := max(rect.X, cx-w/2), min(rect.X+rect.W, cx+w/2)
				p.Stroke(ui.Rect{X: x0 - 1, Y: rect.Y - 2, W: x1 - x0 + 2, H: rect.H + 4}, t.Text, 3, 1.5)
			}
		})
	}).Height(h).Gap(4)
	ui.Text(c, aheadCounts(m, rows)).FontSize(12).TextColor(t.TextMuted).Margin(4, 0, 0, flowAxisWidth+4).Wrap()
}

// aheadCounts says how many turns each player ended ahead, as "Turns ahead · life: you 2,
// emog 11 · …".
func aheadCounts(m stats.Match, rows []flowSeries) string {
	parts := []string{"Turns ahead"}
	for _, r := range rows {
		mine, theirs := 0, 0
		for i := 1; i < len(r.me); i++ {
			switch {
			case r.me[i] > r.opp[i]:
				mine++
			case r.me[i] < r.opp[i]:
				theirs++
			}
		}
		parts = append(parts, fmt.Sprintf("%s: you %d, %s %d", strings.ToLower(r.label), mine, m.Opp.Name, theirs))
	}
	return strings.Join(parts, " · ")
}

// flowSummary is a turn's counts for both players, as "End of turn 9 · You: life 2, hand 7,
// board 3 · emog: …".
func flowSummary(m stats.Match, rows []flowSeries, i int) string {
	side := func(name string, mine bool) string {
		parts := make([]string, 0, len(rows))
		for _, r := range rows {
			n := r.opp[i]
			if mine {
				n = r.me[i]
			}
			parts = append(parts, fmt.Sprintf("%s %d", strings.ToLower(r.label), n))
		}
		return name + ": " + strings.Join(parts, ", ")
	}
	when := fmt.Sprintf("End of turn %d", i)
	if i == 0 {
		when = "Start"
	}
	return when + "\n" + side("You", true) + "\n" + side(m.Opp.Name, false)
}

// flowTooltip shows a turn's counts, then its moment in bold and its marks muted, kept inside
// the chart near its edges.
func flowTooltip(c *ui.Context, x float32, summary string, moment, marks []string) {
	t := c.Theme()
	align := ui.Center
	switch {
	case x < 0.2:
		align = ui.Start
	case x > 0.8:
		align = ui.End
	}
	chart.Anchor(c, x, 1, align, ui.Start, func() {
		ui.Column(c).Gap(3).Padding(4, 8).Radius(6).Background(t.Background).Border(1, t.Border).Children(func() {
			for k, line := range strings.Split(summary, "\n") {
				ui.Text(c.Key(fmt.Sprint("sum", k)), line).NoWrap().FontSize(12)
			}
			for k, line := range moment {
				ui.Text(c.Key(k), line).NoWrap().FontSize(12).Bold()
			}
			for k, line := range marks {
				ui.Text(c.Key(fmt.Sprint("mark", k)), line).NoWrap().FontSize(12).TextColor(t.TextMuted)
			}
		})
	})
}

// chartMarks draws each turn's marks of one kind under the lines, a row per player in the
// color of the card's owner: dots for removals and rings for Triggers.
func chartMarks(c *ui.Context, m stats.Match, marks []simlog.Mark, kind simlog.MarkKind, xs []float32, y chart.Linear, lo float64) {
	type key struct {
		turn int
		mine bool
	}
	count := make(map[key]int)
	for _, mk := range marks {
		if mk.Kind == kind {
			count[key{mk.Turn, mk.Player == m.Me.ID}]++
		}
	}
	for turn := range xs {
		for _, mine := range []bool{true, false} {
			k := key{turn, mine}
			if count[k] == 0 {
				continue
			}
			row, col := lo*0.72, secondColor
			if mine {
				row, col = lo*0.32, firstColor
			}
			chart.Anchor(c.Key(fmt.Sprint(turn, mine)), xs[turn], y.At(row), ui.Center, ui.Center, func() {
				ui.Row(c).Gap(2).Children(func() {
					for range count[k] {
						dot := ui.Box(c).Size(6, 6).Radius(3)
						if kind == simlog.MarkTrigger {
							dot.Size(7, 7).Radius(4).Border(1.5, col)
						} else {
							dot.Background(col)
						}
					}
				})
			})
		}
	}
}

func (v *view) markLines(m stats.Match, marks []simlog.Mark, turn int) []string {
	var removed, triggers []string
	for _, mk := range marks {
		if mk.Turn != turn {
			continue
		}
		name := v.snap.card(mk.Card).Name
		if mk.Player != m.Me.ID {
			name += " (" + m.Opp.Name + ")"
		}
		if mk.Kind == simlog.MarkRemoved {
			removed = append(removed, name)
		} else {
			triggers = append(triggers, name)
		}
	}
	var out []string
	if len(removed) > 0 {
		out = append(out, removedPrefix+strings.Join(removed, ", "))
	}
	if len(triggers) > 0 {
		out = append(out, "Trigger: "+strings.Join(triggers, ", "))
	}
	return out
}

const removedPrefix = "Removed: "
