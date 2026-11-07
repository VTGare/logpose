package gui

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
)

// stepView is one step of a replay as the page shows it, with the user's side apart.
type stepView struct {
	m        stats.Match
	g        *simlog.Game
	r        *replay.Replay
	index    int
	step     replay.Step
	me, them *replay.Side
	// glideFor is how long cards that moved take to get to their new place, or 0 when they
	// jump there.
	glideFor time.Duration
}

func newStepView(m stats.Match, rs *replayState) *stepView {
	sv := &stepView{m: m, g: rs.game, r: rs.r, index: rs.step, step: rs.cur(), glideFor: rs.glide}
	b := &sv.step.Board
	sv.me, sv.them = b.Side(1), b.Side(2)
	if sv.them.Player == m.Me.ID {
		sv.me, sv.them = sv.them, sv.me
	}
	return sv
}

func (sv *stepView) who(id string) string {
	if id == sv.m.Me.ID {
		return "You"
	}
	name, _, _ := strings.Cut(id, "#")
	return name
}

func (sv *stepView) color(id string) ui.Color { return playerColor(sv.m, id) }

// replayPage is the replay in the sim's arrangement: hands in the left corners with the game
// log between them, the playmat in the middle, and the turn, controls and every step on the
// right.
func (v *view) replayPage(c *ui.Context, m stats.Match) {
	rs := v.detail.replay
	if !rs.ready() {
		v.replayWaiting(c, rs)
		return
	}
	sv := newStepView(m, rs)
	moved := rs.moved
	rs.moved = false
	page := ui.Row(c).Fill().Padding(14).Gap(14).AlignItems(ui.Stretch)
	side := min(max(rs.pageBox.W*0.2, 250), 340)
	page.Children(func() {
		v.replayLeft(c, sv, side, moved)
		center := ui.Column(c).Grow(1).Basis(0).MinWidth(0).MinHeight(0).Clip().AlignItems(ui.Center).Justify(ui.Center)
		center.Children(func() { v.playmat(c, fitMat(rs.matBox), sv) })
		v.replayRight(c, sv, rs, side, moved)
		rs.measure(c, page.Bounds(), center.Bounds())
	})
}

// measure keeps the page's and the mat area's sizes from the frame before, which size the
// side columns and the mat, and draws again when they change.
func (rs *replayState) measure(c *ui.Context, page, mat ui.Rect) {
	if page.W == rs.pageBox.W && page.H == rs.pageBox.H && mat.W == rs.matBox.W && mat.H == rs.matBox.H {
		return
	}
	rs.pageBox, rs.matBox = page, mat
	c.Invalidate()
}

func (v *view) replayWaiting(c *ui.Context, rs *replayState) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
		if ui.Button(c, "Back to match").Clicked() {
			v.closeReplay()
		}
		msg := "Building the replay"
		switch {
		case errors.Is(rs.err, replay.ErrNoMoves):
			msg = noMovesText
		case rs.err != nil:
			msg = "The game's log couldn't be read: " + rs.err.Error()
		}
		ui.Text(c, msg).TextColor(t.TextMuted).Wrap()
	})
}

const noMovesText = "This game's log comes from a sim version without the move stream, so it can't be replayed."

func (v *view) replayLeft(c *ui.Context, sv *stepView, side float32, _ bool) {
	t := c.Theme()
	fw := side * 0.19
	ui.Column(c).Width(side).Gap(8).Children(func() {
		back := ui.Row(c).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer).Label("Back to match")
		if back.Clicked() {
			v.closeReplay()
		}
		back.Children(func() {
			ui.Icon(c, iconPrevStep).TextColor(t.TextMuted)
			ui.Text(c, "Back to match").TextColor(t.TextMuted)
		})
		v.replaySideName(c, sv, sv.them)
		v.fan(c, sv, sv.them.Hand, side, fw, true)
		panel(c).Grow(1).Basis(0).Padding(12).Clip().Children(func() { v.inspector(c, sv, side-24) })
		v.fan(c, sv, sv.me.Hand, side, fw, false)
		v.replaySideName(c, sv, sv.me)
	})
}

func (v *view) replaySideName(c *ui.Context, sv *stepView, s *replay.Side) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(8, 8).Radius(4).Background(sv.color(s.Player))
		ui.Text(c, sv.who(s.Player)).Bold().FontSize(13)
		ui.Text(c, fmt.Sprintf("%s · %d in hand", v.snap.card(s.Leader.Code).Name, len(s.Hand))).FontSize(12).TextColor(t.TextMuted).SingleLine()
	})
}

// fan is a hand as overlapping cards turned a little apart. Cards drawn from the deck fade in
// from the mat's side.
func (v *view) fan(c *ui.Context, sv *stepView, hand []replay.Card, width, fw float32, top bool) ui.Element {
	fh := fw * 1.4
	n := len(hand)
	stepX := fw * 0.62
	if n > 1 {
		stepX = min(stepX, (width-fw-8)/float32(n-1))
	}
	// The outer cards turn at most 15° and drop at most a fifth of a card, however big the hand.
	half := max(float32(n-1)/2, 1)
	turn := min(3.5, 15/half)
	drop := min(1.2, fw*0.2/(half*half))
	maxTurn := float64(turn*half) * math.Pi / 180
	// A turned card reaches past its own top and bottom by this much.
	reach := fw/2*float32(math.Sin(maxTurn)) + fh/2*(float32(math.Cos(maxTurn))-1)
	reach = max(reach, 0) + 2
	curve := drop * half * half
	return ui.Box(c).Size(width, fh+curve+2*reach).Children(func() {
		toMat := float32(-fh)
		top0 := reach
		if top {
			toMat = fh
			top0 += curve
		}
		for i, card := range hand {
			off := float32(i) - float32(n-1)/2
			angle, dy := off*turn, off*off*drop
			if top {
				angle, dy = -angle, -dy
			}
			box := ui.Box(c.Key(cardMatch(card.ID))).Absolute().Left(4 + float32(i)*stepX).Top(top0 + dy).Rotate(angle)
			if sv.glideFor > 0 {
				box.Transition(ui.ElementTransition{
					Duration: sv.glideFor, Position: true, Match: cardMatch(card.ID),
					Enter: &ui.Motion{Y: toMat},
				})
			}
			box.Children(func() { v.cardArt(c, card.Code, fw, fh) })
		}
	})
}

// printout reports the sim's board printouts, which the log panel leaves out.
func printout(l simlog.Line) bool {
	for _, p := range []string{"Hand: [", "Board: [", "Trash: [", "Life: "} {
		if strings.HasPrefix(l.Text, p) {
			return true
		}
	}
	return false
}

// gameLog is the sim's log up to the step, with the step's lines shaded and the last of them
// brought into view when the step changes.
func gameLog(c *ui.Context, sv *stepView, moved bool) {
	t := c.Theme()
	ui.Column(c).Gap(2).Padding(8).Children(func() {
		for i, s := range sv.r.Steps[:sv.index+1] {
			cur := i == sv.index
			last := -1
			for _, li := range s.Lines {
				if !printout(sv.g.Lines[li]) {
					last = li
				}
			}
			for _, li := range s.Lines {
				l := sv.g.Lines[li]
				if printout(l) {
					continue
				}
				row := ui.Box(c.Key(li)).Padding(2, 6).Radius(4)
				text := t.TextMuted
				if cur {
					row.Background(t.Accent.Alpha(0.16)).BorderStartWidth(2).BorderColor(t.Accent)
					text = t.Text
				}
				row.Children(func() { ui.RichText(c, logSpans(sv, l, 12, text)...) })
				if cur && moved && li == last {
					row.ScrollIntoView()
				}
			}
		}
	})
}

var cardGold = ui.Hex("#F0C05A")

// logSpans is a log line with its player in their color and card names in gold.
func logSpans(sv *stepView, l simlog.Line, size float32, text ui.Color) []ui.Span {
	var spans []ui.Span
	if l.Player != "" && l.Player != "You" {
		spans = append(spans, ui.Span{Text: sv.who(l.Player) + "  ", Size: size, Weight: 700, Color: sv.color(l.Player)})
	}
	rest := l.Text
	for _, code := range l.Cards {
		name := sv.g.CardNames[code]
		tag := name + " [" + code + "]"
		i := strings.Index(rest, tag)
		if i < 0 || name == "" {
			continue
		}
		spans = append(spans,
			ui.Span{Text: rest[:i], Size: size, Color: text},
			ui.Span{Text: name, Size: size, Weight: 600, Color: cardGold})
		rest = rest[i+len(tag):]
	}
	return append(spans, ui.Span{Text: replay.LineText(simlog.Line{Text: rest}), Size: size, Color: text})
}

func (v *view) replayRight(c *ui.Context, sv *stepView, rs *replayState, side float32, moved bool) {
	t := c.Theme()
	s := sv.step
	ui.Column(c).Width(side).Gap(12).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				turn := "Setup"
				if s.Turn > 0 {
					turn = fmt.Sprintf("Turn %d", s.Turn)
				}
				ui.Text(c, turn).FontSize(12).TextColor(t.TextMuted)
				// A block or counter is the defender's step, but the turn stays the attacker's.
				if p := sv.r.Steps[sv.r.FirstOfTurn(s.Turn)].Player; p != "" && s.Turn > 0 {
					ui.Text(c, "· "+turnOf(sv.who(p))).FontSize(12).FontWeight(600).TextColor(sv.color(p))
				}
				ui.Spacer(c)
				ui.Text(c, fmt.Sprintf("Step %d of %d", rs.step+1, len(rs.r.Steps))).FontSize(12).TextColor(faintColor(t))
			})
			ui.Text(c, phaseWord(sv.r.Steps, sv.index)).FontSize(22).FontWeight(700)
			if s.Title != "" {
				ui.Text(c, s.Title).FontSize(12).TextColor(t.TextMuted).Wrap()
			}
			if s.Result != "" {
				ui.Text(c, s.Result).FontSize(13).Bold().TextColor(cardGold).Wrap()
			}
		})
		v.controls(c, sv, rs)
		ui.Segmented(c, &rs.tab, "Actions", "Game log").Label("Show")
		ui.Scroll(c).Grow(1).Basis(0).Children(func() {
			if rs.tab == 1 {
				gameLog(c, sv, moved)
				return
			}
			v.actionList(c, sv, rs, moved)
		})
	})
}

// controls are the turn and step buttons around play, the whole game as a scrubber, and the
// restart and the speed.
func (v *view) controls(c *ui.Context, sv *stepView, rs *replayState) {
	t := c.Theme()
	s := sv.step
	round := func(icon *ui.SVG, label, tip string, size float32, primary, off bool) ui.Element {
		b := ui.ButtonBase(c.Key(label)).Size(size, size).Radius(size / 2).Center().Label(label).Tooltip(tip).Disabled(off)
		col := t.Text
		if primary {
			b.Background(t.Accent)
			col = t.AccentText
		}
		b.Children(func() { ui.Icon(c, icon).Size(size*0.45, size*0.45).TextColor(col) })
		return b
	}
	ui.Column(c).Gap(10).Padding(12).Radius(12).Background(t.Surface).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			if round(iconPrevTurn, "Previous turn", "Previous turn (Page Up)", 34, false, s.Turn == 0).Clicked() {
				rs.prevTurn()
			}
			if round(iconPrevStep, "Previous step", "Previous step (Left)", 34, false, rs.step == 0).Clicked() {
				rs.jump(rs.step - 1)
			}
			icon, label := iconPlay, "Play"
			if rs.playing {
				icon, label = iconPause, "Pause"
			}
			if round(icon, label, "Space", 44, true, false).Clicked() {
				v.togglePlay()
			}
			if round(iconNextStep, "Next step", "Next step (Right)", 34, false, rs.step == len(rs.r.Steps)-1).Clicked() {
				rs.jump(rs.step + 1)
			}
			if round(iconNextTurn, "Next turn", "Next turn (Page Down)", 34, false, s.Turn == rs.lastTurn()).Clicked() {
				rs.nextTurn()
			}
		})
		v.scrubber(c, sv, rs)
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			restart := ui.ButtonBase(c).Label("Restart this turn").Tooltip("Restart this turn").Padding(2, 4).Radius(6).
				Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconRestart).TextColor(t.TextMuted)
						ui.Text(c, "Restart turn").FontSize(12).TextColor(t.TextMuted)
					})
				})
			if restart.Clicked() {
				rs.restartTurn()
			}
			ui.Spacer(c)
			speed := rs.speed
			ui.Segmented(c, &rs.speed, speedLabels...)
			if rs.speed != speed {
				v.schedule()
			}
		})
	})
}

// scrubber is the whole game as a segment per turn in its player's color, as long as its steps,
// with the current step marked. Clicking a turn jumps to its start.
func (*view) scrubber(c *ui.Context, sv *stepView, rs *replayState) {
	t := c.Theme()
	steps := sv.r.Steps
	n := float32(len(steps))
	type seg struct {
		turn, first, last int
		player            string
	}
	var segs []seg
	for i, s := range steps {
		if len(segs) == 0 || segs[len(segs)-1].turn != s.Turn {
			segs = append(segs, seg{turn: s.Turn, first: i, player: s.Player})
		}
		segs[len(segs)-1].last = i
	}
	bar := ui.Box(c).Height(26).Cursor(ui.CursorPointer).Label("Jump to a turn")
	if bar.Clicked() {
		if x, _, ok := bar.PointerPosition(); ok {
			at := int(x / bar.Bounds().W * n)
			for _, sg := range segs {
				if at >= sg.first && at <= sg.last {
					rs.jump(sg.first)
				}
			}
		}
	}
	bar.Draw(func(p *ui.Painter, r ui.Rect) {
		unit := r.W / n
		for _, sg := range segs {
			x0, x1 := r.X+float32(sg.first)*unit+1, r.X+float32(sg.last+1)*unit-1
			col := t.Border
			if sg.turn > 0 {
				col = playerColor(sv.m, sg.player)
			}
			if sg.first > rs.step {
				col = col.Alpha(0.35)
			}
			p.Fill(ui.Rect{X: x0, Y: r.Y + 4, W: max(x1-x0, 2), H: 7}, col, 3)
			if sg.turn > 0 && sg.turn%2 == 0 {
				label := fmt.Sprint(sg.turn)
				p.Text((x0+x1)/2-float32(len(label))*3, r.Y+14, label, 10, faintColor(t))
			}
		}
		x := r.X + (float32(rs.step)+0.5)*unit
		p.Fill(ui.Rect{X: x - 5, Y: r.Y, W: 10, H: 15}, t.Text, 4)
	})
}

// iconButton is a button with an icon at its text's size and color, before the text or, with
// after, behind it.
func iconButton(c *ui.Context, icon *ui.SVG, text string, after bool) ui.Element {
	return ui.Button(c, "").Children(func() {
		if !after {
			ui.Icon(c, icon)
		}
		if text != "" {
			ui.Text(c, text).SingleLine()
		}
		if after {
			ui.Icon(c, icon)
		}
	})
}

func turnOf(name string) string {
	if name == "You" {
		return "Your turn"
	}
	return possessive(name) + " turn"
}

// phaseWord names a step's part of the turn, as the sim's game screen does. An effect during an
// attack takes the name of the step it comes in.
func phaseWord(steps []replay.Step, i int) string {
	for i > 0 && steps[i].Kind == replay.KindBattleEffect {
		i--
	}
	s := steps[i]
	switch s.Kind {
	case replay.KindAttack:
		return "Attack Step"
	case replay.KindBlock:
		return "Blocker Step"
	case replay.KindCounter:
		return "Counter Step"
	case replay.KindDamage:
		return "Damage"
	case replay.KindTurnStart:
		return "Refresh · Draw · DON"
	case replay.KindPlay, replay.KindAttach, replay.KindEffect:
		return "Main"
	case replay.KindConcede:
		return "Conceded"
	case replay.KindDeckOut:
		return "Out of cards"
	}
	return "Setup"
}

// actionList is every step under a header per turn, the setup steps as one row, with the
// current step shaded and brought into view when it changes. A row jumps to its step.
func (v *view) actionList(c *ui.Context, sv *stepView, rs *replayState, moved bool) {
	t := c.Theme()
	ui.Column(c).Gap(2).Padding(0, 4, 0, 0).Children(func() {
		last := -1
		for i, s := range sv.r.Steps {
			if s.Turn == 0 {
				if i == 0 {
					v.setupRow(c, sv, rs, moved)
				}
				continue
			}
			if s.Turn != last {
				last = s.Turn
				ui.Text(c.Key(-i-1), fmt.Sprintf("Turn %d · %s", s.Turn, sv.who(s.Player))).FontSize(11).Bold().
					TextColor(t.TextMuted).Margin(8, 0, 2, 4)
			}
			v.stepRow(c, sv, rs, i, moved)
		}
	})
}

func (*view) setupRow(c *ui.Context, sv *stepView, rs *replayState, moved bool) {
	t := c.Theme()
	cur := sv.step.Turn == 0
	row := ui.Row(c.Key("setup")).Gap(8).Padding(6, 8).Radius(6).Label("Setup steps")
	if cur {
		row.Background(t.Surface).BorderStartWidth(2).BorderColor(t.Accent)
	}
	row.Children(func() {
		ui.Text(c, "Setup").FontSize(12).Bold()
		ui.Text(c, "opening hands, mulligans, life").FontSize(12).TextColor(t.TextMuted).SingleLine()
	})
	if cur && moved {
		row.ScrollIntoView()
	}
	if row.Clicked() {
		rs.jump(0)
	}
}

func (v *view) stepRow(c *ui.Context, sv *stepView, rs *replayState, i int, moved bool) {
	t := c.Theme()
	s := sv.r.Steps[i]
	row := ui.Row(c.Key(i)).Gap(8).Padding(5, 8).Radius(6).AlignItems(ui.Center)
	if i == rs.step {
		row.Background(t.Surface).BorderStartWidth(2).BorderColor(t.Accent)
	}
	row.Children(func() {
		ui.Box(c).Size(3, 18).Radius(2).Background(sv.color(s.Player))
		for j, code := range s.Cards {
			ui.Box(c.Key(j)).Children(func() { v.cardArt(c, code, 22, 31) })
		}
		ui.Column(c).Grow(1).Basis(0).Gap(1).Children(func() {
			ui.Text(c, s.Title).FontSize(12).Wrap()
			if s.Result != "" {
				ui.Text(c, s.Result).FontSize(11).TextColor(t.TextMuted).SingleLine()
			}
		})
	})
	if i == rs.step && moved {
		row.ScrollIntoView()
	}
	if row.Clicked() {
		rs.jump(i)
	}
}
