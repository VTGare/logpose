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

// flowAxisWidth fits most sim names. Longer ones end in an ellipsis and show in full on hover.
const flowAxisWidth = 96

// flowTile shows life, hand or board by turn, picked by tabs, as pips laid out like the table.
func (v *view) flowTile(c *ui.Context, m stats.Match) ui.Element {
	t := c.Theme()
	sum := ""
	if l := v.detail.timeline; l != nil && len(l.Me) > 0 && len(l.Opp) == len(l.Me) {
		sum = fmt.Sprintf("Life at the end: you %d, %s %d", l.Me[len(l.Me)-1], m.Opp.Name, l.Opp[len(l.Opp)-1])
	}
	return v.foldTile(c, "match.flow", foldHead{label: "Turn by turn", summary: sum}, func() {
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
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Segmented(c, tab, labels...)
			ui.Spacer(c)
			pipLegend(c, r, m)
		})
		v.pips(c, m, l, r, rows)
	})
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

// pipGap is the room a pip takes in its column, and pipSpine the band between the two players
// that holds the turn numbers and the key moments' pins.
const (
	pipGap   = 15
	pipSize  = 11
	pipSpine = 34
)

// pipPlot is what drawing one tab of pips needs.
type pipPlot struct {
	t        *ui.Theme
	m        stats.Match
	r        flowSeries
	n, slots int
	half     float32
	asPips   bool
	// triggers holds "true 5" for a Trigger from your life on turn 5, and pinned the turns
	// with a key moment, whose pin stands in for the turn number.
	triggers map[string]bool
	pinned   map[int]bool
}

func newPipPlot(t *ui.Theme, m stats.Match, l *matchTimeline, r flowSeries) pipPlot {
	pp := pipPlot{
		t: t, m: m, r: r, n: len(r.me), slots: 5, asPips: r.label != "Hand", half: 30,
		triggers: make(map[string]bool), pinned: make(map[int]bool),
	}
	if pp.asPips {
		pp.slots = max(slices.Max(r.me), slices.Max(r.opp), 1)
		pp.half = float32(pp.slots*pipGap + 6)
	}
	for _, mk := range l.Marks {
		if r.marks == simlog.MarkTrigger && mk.Kind == simlog.MarkTrigger {
			pp.triggers[fmt.Sprint(mk.Player == m.Me.ID, mk.Turn)] = true
		}
	}
	for _, mo := range l.Moments {
		pp.pinned[mo.Turn] = true
	}
	return pp
}

func (pp pipPlot) height() float32 { return 2*pp.half + pipSpine }

func (pp pipPlot) draw(p *ui.Painter, rect ui.Rect, sel int) {
	colW := rect.W / float32(pp.n)
	for i := 1; i < pp.n; i++ {
		if mineTurn(pp.m, i) {
			p.Fill(ui.Rect{X: rect.X + float32(i)*colW + 1, Y: rect.Y, W: colW - 2, H: rect.H}, youColor.Alpha(0.07), 4)
		}
	}
	for i := range pp.n {
		cx := rect.X + (float32(i)+0.5)*colW
		pp.column(p, rect, cx, pp.r.opp, i, oppColor, false)
		pp.column(p, rect, cx, pp.r.me, i, youColor, true)
		if !pp.pinned[i] {
			label := fmt.Sprint(i)
			if i == 0 {
				label = "S"
			}
			p.Text(cx-float32(len(label))*3.3, rect.Y+pp.half+pipSpine/2-7, label, 11, faintColor(pp.t))
		}
	}
	if sel >= 0 && sel < pp.n {
		p.Stroke(ui.Rect{X: rect.X + float32(sel)*colW, Y: rect.Y - 2, W: colW, H: rect.H + 4}, pp.t.Text.Alpha(0.6), 5, 1.5)
	}
}

// column draws one player's count at the end of turn i: a number for hand, else a pip per
// card, with the ones lost that turn as rings.
func (pp pipPlot) column(p *ui.Painter, rect ui.Rect, cx float32, counts []int, i int, col ui.Color, mine bool) {
	v, prev := counts[i], counts[max(i-1, 0)]
	if !pp.asPips {
		y := rect.Y + pp.half/2
		if mine {
			y = rect.Y + pp.half + pipSpine + pp.half/2
		}
		s := fmt.Sprint(v)
		p.Text(cx-float32(len(s))*3.6, y-8, s, 14, col)
		return
	}
	ring := col
	if pp.r.marks == simlog.MarkRemoved {
		ring = lossColor
	}
	for k := range pp.slots {
		y := rect.Y + pp.half - 6 - float32(k)*pipGap - pipSize/2
		if mine {
			y = rect.Y + pp.half + pipSpine + 6 + float32(k)*pipGap + pipSize/2
		}
		dot := ui.Rect{X: cx - pipSize/2, Y: y - pipSize/2, W: pipSize, H: pipSize}
		switch {
		case k < v:
			p.Fill(dot, col, pipSize/2)
		case k < prev:
			p.Stroke(dot, ring, pipSize/2, 1.5)
			if k == prev-1 && pp.triggers[fmt.Sprint(mine, i)] {
				p.Text(cx-3, y-6, "T", 9, ring)
			}
		default:
			p.Fill(ui.Rect{X: cx - 1.5, Y: y - 1.5, W: 3, H: 3}, pp.t.Border, 1.5)
		}
	}
}

// pips lays the turns out like the table: the opponent's counts above, yours below and the turn
// numbers between. Life and board are drawn as pips, one per card, and a pip lost that turn is
// a ring for that turn only. Hand goes past what pips can show, so it's numbers. Your turns are
// shaded, and hovering a turn explains it.
func (v *view) pips(c *ui.Context, m stats.Match, l *matchTimeline, r flowSeries, rows []flowSeries) {
	pp := newPipPlot(c.Theme(), m, l, r)
	h := pp.height()
	xs := make([]float32, pp.n)
	for i := range xs {
		xs[i] = (float32(i) + 0.5) / float32(pp.n)
	}
	ticks := []chart.Tick{
		{At: 1 - pp.half/2/h, Label: m.Opp.Name},
		{At: pp.half / 2 / h, Label: "You"},
	}
	sel := &v.detail.lifePoint
	label := func(_ chart.Side, tick chart.Tick) {
		col := oppColor
		if tick.Label == "You" {
			col = youColor
		}
		ui.Text(c, tick.Label).SingleLine().Width(flowAxisWidth).TextAlign(ui.End).FontWeight(600).TextColor(col).Tooltip(tick.Label)
	}
	chart.Plot(c.Key(r.label), chart.Axes{Left: ticks, Label: label}, func(area ui.Element) {
		area.Label(r.label + " by turn").Draw(func(p *ui.Painter, rect ui.Rect) { pp.draw(p, rect, *sel) })
		for i, mo := range l.Moments {
			if mo.Turn < pp.n {
				chart.Anchor(c.Key(fmt.Sprint("pin", i)), xs[mo.Turn], 1-(pp.half+pipSpine/2)/h, ui.Center, ui.Center, func() {
					v.momentPin(c, m, i, mo)
				})
			}
		}
		chart.Select(c, area, sel, xs, func(i int) string { return flowSummary(m, rows, i) })
		if i := *sel; i >= 0 && i < pp.n {
			moment, marks := v.turnLines(m, l, i)
			pipTooltip(c, xs[i], 0.5/float32(pp.n), flowSummary(m, rows, i), moment, marks)
		}
	}).Height(h).Gap(8).Margin(10, 0, 0, 0).Transition(ui.ElementTransition{Duration: barTime, Colors: true, Enter: &ui.Motion{}})
}

// mineTurn reports whether turn i was the local player's, when the log says who went first.
func mineTurn(m stats.Match, i int) bool {
	if !m.TurnOrderKnown || i < 1 {
		return false
	}
	return (i%2 == 1) == m.WentFirst
}

// pipTooltip explains a turn beside its column, on the side with more room.
func pipTooltip(c *ui.Context, x, halfCol float32, summary string, moment, marks []string) {
	t := c.Theme()
	h, at := ui.Start, x+halfCol
	if x > 0.5 {
		h, at = ui.End, x-halfCol
	}
	chart.Anchor(c, at, 0.5, h, ui.Center, func() {
		ui.Column(c).Gap(3).Padding(8, 10).Margin(0, 6).Radius(8).Background(t.Surface).Border(1, t.Border).
			Shadow(0, 10, 24, 0, ui.RGBA(0, 0, 0, 0.55)).Children(func() {
			for k, line := range strings.Split(summary, "\n") {
				txt := ui.Text(c.Key(fmt.Sprint("sum", k)), line).NoWrap().FontSize(12)
				if k == 0 {
					txt.Bold()
				} else {
					txt.TextColor(t.TextMuted)
				}
			}
			for k, line := range moment {
				ui.Text(c.Key(k), line).NoWrap().FontSize(12)
			}
			for k, line := range marks {
				ui.Text(c.Key(fmt.Sprint("mark", k)), line).NoWrap().FontSize(12).TextColor(t.TextMuted)
			}
		})
	})
}

// pipLegend names the marks the tab draws.
func pipLegend(c *ui.Context, r flowSeries, m stats.Match) {
	t := c.Theme()
	item := func(mark func(), label string) {
		ui.Row(c.Key(label)).Gap(6).AlignItems(ui.Center).Children(func() {
			mark()
			ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
		})
	}
	if r.label != "Hand" {
		item(func() { ui.Box(c).Size(10, 10).Radius(5).Background(youColor) }, r.label)
		taken, col := "Taken that turn", youColor
		if r.marks == simlog.MarkRemoved {
			taken, col = "Removed that turn", lossColor
		}
		item(func() { ui.Box(c).Size(10, 10).Radius(5).Border(1.5, col) }, taken)
		if r.marks == simlog.MarkTrigger {
			item(func() { ui.Text(c, "T").FontSize(10).Bold().TextColor(youColor) }, "Trigger")
		}
	}
	if m.TurnOrderKnown {
		item(func() { ui.Box(c).Size(18, 10).Radius(3).Background(youColor.Alpha(0.25)) }, "Your turns")
	}
}
