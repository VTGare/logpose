package matches

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
)

type stepView struct {
	m        stats.Match
	g        *simlog.Game
	r        *replay.Replay
	index    int
	step     replay.Step
	me, them *replay.Side
	rs       *replayState
	// glideFor is how long cards that moved take to get to their new place, or 0 when they
	// jump there.
	glideFor time.Duration
	// moved is set in the first frame of a step.
	moved bool
}

func newStepView(m stats.Match, rs *replayState) *stepView {
	sv := &stepView{m: m, g: rs.game, r: rs.r, index: rs.step, step: rs.cur(), rs: rs, glideFor: rs.Glide}
	b := &sv.step.Board
	sv.me, sv.them = b.Side(1), b.Side(2)
	if sv.them.Player == m.Me.ID {
		sv.me, sv.them = sv.them, sv.me
	}
	return sv
}

func (sv *stepView) side(id string) page.Side { return page.SideOf(sv.m, id) }

func (sv *stepView) seat(mine bool) page.Side { return page.Side{Mine: mine, Opp: sv.m.Opp.Name} }

// sameBattleBefore reports whether the step before showed this step's attack, as a counter
// step does after the attack.
func (sv *stepView) sameBattleBefore() bool {
	if sv.index == 0 {
		return false
	}
	a, b := sv.r.Steps[sv.index-1].Battle, sv.step.Battle
	return a != nil && b != nil && a.AttackerID == b.AttackerID && a.DefenderID == b.DefenderID
}

// attacker is the page.Side of the step's attack. The step has to have a battle.
func (sv *stepView) attacker() page.Side { return sv.seat(sv.me.Owner == sv.step.Battle.AttackerOwner) }

func (pg *Page) replayPage(c *ui.Context, e *page.Env, m stats.Match) {
	rs := pg.Detail.Replay
	if !rs.ready() {
		pg.replayWaiting(c, rs)
		return
	}
	sv := newStepView(m, rs)
	moved := rs.moved
	sv.moved = moved
	rs.moved = false
	if moved {
		rs.shownAt = c.Now()
	}
	page := ui.Row(c).Fill().Padding(14).Gap(14).AlignItems(ui.Stretch)
	side := min(max(rs.pageBox.W*0.2, 250), 340)
	page.Children(func() {
		hands := pg.replayLeft(c, e, sv, side)
		var center ui.Element
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).MinHeight(0).Gap(12).AlignItems(ui.Center).Children(func() {
			center = ui.Column(c).Grow(1).Basis(0).MinHeight(0).AlignSelf(ui.Stretch).AlignItems(ui.Center).Justify(ui.Center)
			center.Children(func() { playmat(c, e, fitMat(rs.matBox), sv) })
			// The bar is as wide as the mat, so it stays under it as the window changes.
			ui.Box(c).Width(fitMat(rs.matBox).W).Children(func() { pg.controls(c, e, sv) })
		})
		replayRight(c, e, sv, side, moved)
		// The hands are built after the mat, so cards on their way from the mat to a hand pass
		// over it. Each takes the place its stand-in had in the frame before.
		pb := page.Bounds()
		for i, s := range []*replay.Side{sv.them, sv.me} {
			b := hands[i].Bounds()
			if b.W == 0 {
				continue
			}
			ui.Box(c.Key(handKey(i))).Absolute().Left(b.X - pb.X).Top(b.Y - pb.Y).Children(func() {
				fan(c, e, sv, s.Hand, b, fanWidth(side), i == 0)
			})
		}
		rs.measure(c, pb, center.Bounds())
	})
}

type handKey int

func fanWidth(side float32) float32 { return side * 0.19 }

func (rs *replayState) measure(c *ui.Context, page, mat ui.Rect) {
	if page.W == rs.pageBox.W && page.H == rs.pageBox.H && mat.W == rs.matBox.W && mat.H == rs.matBox.H {
		return
	}
	rs.pageBox, rs.matBox = page, mat
	c.Invalidate()
}

func (pg *Page) replayWaiting(c *ui.Context, rs *replayState) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
		if ui.Button(c, "Back to match").Clicked() {
			pg.closeReplay()
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

// replayLeft returns the stand-ins for the opponent's hand and the user's, which take the room
// the fans need.
func (pg *Page) replayLeft(c *ui.Context, e *page.Env, sv *stepView, side float32) (hands [2]ui.Element) {
	t := c.Theme()
	fw := fanWidth(side)
	stand := func(s *replay.Side) ui.Element {
		return ui.Box(c).Size(side, shapeFan(len(s.Hand), side, fw).h)
	}
	ui.Column(c).Width(side).Gap(8).Children(func() {
		back := ui.Row(c).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer).Label("Back to match")
		if back.Clicked() {
			pg.closeReplay()
		}
		back.Children(func() {
			ui.Icon(c, kit.IconPrevStep).TextColor(t.TextMuted)
			ui.Text(c, "Back to match").TextColor(t.TextMuted)
		})
		replaySideName(c, e, sv, sv.them)
		hands[0] = stand(sv.them)
		kit.Panel(c).Grow(1).Basis(0).Padding(12).Clip().Children(func() { inspector(c, e, sv, side-24) })
		hands[1] = stand(sv.me)
		replaySideName(c, e, sv, sv.me)
	})
	return hands
}

func replaySideName(c *ui.Context, e *page.Env, sv *stepView, s *replay.Side) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(8, 8).Radius(4).Background(sv.side(s.Player).Color())
		ui.Text(c, sv.side(s.Player).Name()).Bold().FontSize(13)
		ui.Text(c, fmt.Sprintf("%s · %d in hand", e.Snap.Card(s.Leader.Code).Name, len(s.Hand))).FontSize(12).TextColor(t.TextMuted).SingleLine()
	})
}

// deckBefore is the cards under the top of the deck in the step shown before, the opponent's
// or the user's. The top card shows on the mat already, and its Match brings it.
func (sv *stepView) deckBefore(theirs bool) map[int]bool {
	if sv.glideFor == 0 {
		return nil
	}
	b := &sv.r.Steps[sv.rs.from].Board
	s := b.Side(sv.me.Owner)
	if theirs {
		s = b.Side(sv.them.Owner)
	}
	d := s.Deck()
	in := make(map[int]bool, len(d))
	for _, c := range d[:max(len(d)-1, 0)] {
		in[c.ID] = true
	}
	return in
}

// deckSide indexes replayState.deckAt.
func deckSide(sv *stepView, s *replay.Side) int {
	if s == sv.me {
		return 1
	}
	return 0
}

// fanShape is how a hand of n cards fans out in width: how far apart the cards sit, how much
// each turns and drops, and the room the fan takes.
type fanShape struct {
	stepX, turn, drop, curve, reach, h float32
}

func shapeFan(n int, width, fw float32) fanShape {
	fh := page.CardHeight(fw)
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
	return fanShape{stepX: stepX, turn: turn, drop: drop, curve: curve, reach: reach, h: fh + curve + 2*reach}
}

// fan is a hand as overlapping cards turned a little apart, at at in the window. A card that
// comes from elsewhere on the mat flies from there, and any other new card from the deck.
func fan(c *ui.Context, e *page.Env, sv *stepView, hand []replay.Card, at ui.Rect, fw float32, top bool) ui.Element {
	n := len(hand)
	sh := shapeFan(n, at.W, fw)
	deck := sv.rs.deckAt[1]
	if top {
		deck = sv.rs.deckAt[0]
	}
	fromDeck := sv.deckBefore(top)
	return ui.Box(c).Size(at.W, sh.h).Children(func() {
		top0 := sh.reach
		if top {
			top0 += sh.curve
		}
		for i, card := range hand {
			off := float32(i) - float32(n-1)/2
			angle, dy := off*sh.turn, off*off*sh.drop
			if top {
				angle, dy = -angle, -dy
			}
			x, y := 4+float32(i)*sh.stepX, top0+dy
			box := ui.Box(c.Key(cardMatch(card.ID))).Absolute().Left(x).Top(y)
			var enter *ui.Motion
			if fromDeck[card.ID] {
				enter = &ui.Motion{X: deck.X - at.X - x, Y: deck.Y - at.Y - y, Opacity: 1}
			}
			box.Transition(sv.glide(cardMatch(card.ID), enter))
			box.Rotate(sv.ease(box, "turn", angle))
			box.Children(func() { e.Card(c, card.Code, fw) })
		}
	})
}

func gameLog(c *ui.Context, e *page.Env, sv *stepView, moved bool) {
	t := c.Theme()
	ui.Column(c).Gap(2).Padding(8).Children(func() {
		for i, s := range sv.r.Steps[:sv.index+1] {
			cur := i == sv.index
			last := -1
			for _, li := range s.Lines {
				if sv.g.Lines[li].Kind != simlog.LinePrintout {
					last = li
				}
			}
			for _, li := range s.Lines {
				l := sv.g.Lines[li]
				if l.Kind == simlog.LinePrintout {
					continue
				}
				row := ui.Box(c.Key(li)).Padding(2, 6).Radius(4)
				text := t.TextMuted
				if cur {
					row.Background(t.Accent.Alpha(0.16)).BorderStartWidth(2).BorderColor(t.Accent)
					text = t.Text
				}
				row.Children(func() { cardText(c, e, 12, logPieces(sv, l, text)) })
				if cur && moved && li == last {
					row.ScrollIntoView()
				}
			}
		}
	})
}

func replayRight(c *ui.Context, e *page.Env, sv *stepView, side float32, moved bool) {
	rs := sv.rs
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
				if p := sv.g.TurnPlayer(s.Turn); p != "" {
					ui.Text(c, "· "+sv.side(p).Possessive()+" turn").FontSize(12).FontWeight(600).TextColor(sv.side(p).Color())
				}
				ui.Spacer(c)
				ui.Text(c, fmt.Sprintf("Step %d of %d", rs.step+1, len(rs.r.Steps))).FontSize(12).TextColor(kit.FaintColor(t))
			})
			ui.Text(c, phaseWord(sv.r.Steps, sv.index)).FontSize(22).FontWeight(700)
			if title := stepTitle(sv.g, &s, t.TextMuted); len(title) > 0 {
				cardText(c, e, 12, title)
			}
			if result := stepResult(sv.g, &s); result != "" {
				ui.Text(c, result).FontSize(13).Bold().TextColor(kit.CardGold).Wrap()
			}
		})
		ui.Segmented(c, &rs.tab, "Actions", "Game log").Label("Show")
		ui.Scroll(c).Grow(1).Basis(0).Children(func() {
			if rs.tab == 1 {
				gameLog(c, e, sv, moved)
				return
			}
			actionList(c, e, sv, moved)
		})
	})
}

func (pg *Page) controls(c *ui.Context, e *page.Env, sv *stepView) {
	rs := sv.rs
	t := c.Theme()
	s := sv.step
	round := func(icon *ui.SVG, label, tip string, size float32, primary, off bool) ui.Element {
		key := label
		if primary {
			key = "play"
		}
		b := ui.ButtonBase(c.Key(key)).Size(size, size).Radius(size / 2).Center().Label(label).Tooltip(tip).Disabled(off)
		col := t.Text
		if primary {
			b.Background(t.Accent)
			col = t.AccentText
		}
		is := size * 0.45
		b.Children(func() {
			ui.Box(c).Size(is, is).Children(func() {
				ui.Box(c.Key(label)).Absolute().Fill().Center().Transition(kit.IconCross).Children(func() {
					ui.Icon(c, icon).Size(is, is).TextColor(col)
				})
			})
		})
		return kit.PressScale(b)
	}
	ui.Row(c).Gap(16).AlignItems(ui.Center).Padding(8, 14).Radius(29).Background(t.Surface).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			if round(kit.IconPrevTurn, "Previous turn", "Previous turn (Page Up)", 36, false, s.Turn == 0).Clicked() {
				rs.prevTurn()
			}
			if round(kit.IconPrevStep, "Previous step", "Previous step (Left)", 36, false, rs.step == 0).Clicked() {
				rs.jump(rs.step - 1)
			}
			icon, label := kit.IconPlay, "Play"
			if rs.playing {
				icon, label = kit.IconPause, "Pause"
			}
			if round(icon, label, "Space", 42, true, false).Clicked() {
				pg.togglePlay(e)
			}
			if round(kit.IconNextStep, "Next step", "Next step (Right)", 36, false, rs.step == len(rs.r.Steps)-1).Clicked() {
				rs.jump(rs.step + 1)
			}
			if round(kit.IconNextTurn, "Next turn", "Next turn (Page Down)", 36, false, s.Turn == rs.lastTurn()).Clicked() {
				rs.nextTurn()
			}
		})
		ui.Box(c).Grow(1).Basis(0).MinWidth(80).Children(func() { scrubber(c, sv) })
		restart := ui.ButtonBase(c).Label("Restart this turn").Tooltip("Restart this turn").Padding(4, 6).Radius(6).
			Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, kit.IconRestart).TextColor(t.TextMuted)
					ui.Text(c, "Restart turn").FontSize(12).TextColor(t.TextMuted).SingleLine()
				})
			})
		if restart.Clicked() {
			rs.restartTurn()
		}
		speed := rs.speed
		speedPills(c, &rs.speed, speedLabels())
		if rs.speed != speed {
			pg.schedule(e)
		}
	})
}

// speedPills is a segmented control drawn as pills, so it sits inside the round controls bar.
func speedPills(c *ui.Context, selected *int, labels []string) {
	t := c.Theme()
	p := ui.SegmentedBase(c, selected, len(labels))
	p.Track.Label("Speed").Padding(2).Gap(2).Radius(99).Background(t.Surface).Border(1, t.Border)
	p.Track.Children(func() {
		for i, label := range labels {
			s := p.Segment(i).Padding(4, 12).Radius(99).Background(ui.Transparent)
			switch {
			case i == *selected:
				s.Background(t.SurfacePressed).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.12))
			case s.Hovered():
				s.Background(t.SurfaceHover)
			}
			s.Children(func() { ui.Text(c, label).SingleLine() })
		}
	})
}

func scrubber(c *ui.Context, sv *stepView) {
	rs := sv.rs
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
			segs = append(segs, seg{turn: s.Turn, first: i, player: sv.g.TurnPlayer(s.Turn)})
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
				col = sv.side(sg.player).Color()
			}
			if sg.first > rs.step {
				col = col.Alpha(0.35)
			}
			p.Fill(ui.Rect{X: x0, Y: r.Y + 4, W: max(x1-x0, 2), H: 7}, col, 3)
			if sg.turn > 0 && sg.turn%2 == 0 {
				label := fmt.Sprint(sg.turn)
				p.Text((x0+x1)/2-float32(len(label))*3, r.Y+14, label, 10, kit.FaintColor(t))
			}
		}
		x := r.X + (float32(rs.step)+0.5)*unit
		p.Fill(ui.Rect{X: x - 5, Y: r.Y, W: 10, H: 15}, t.Text, 4)
	})
}

func actionList(c *ui.Context, e *page.Env, sv *stepView, moved bool) {
	t := c.Theme()
	ui.Column(c).Gap(2).Padding(0, 4, 0, 0).Children(func() {
		last := -1
		for i, s := range sv.r.Steps {
			if s.Turn == 0 {
				if i == 0 {
					setupRow(c, sv, moved)
				}
				continue
			}
			if s.Turn != last {
				last = s.Turn
				ui.Text(c.Key(-i-1), fmt.Sprintf("Turn %d · %s", s.Turn, sv.side(s.Player).Name())).FontSize(11).Bold().
					TextColor(t.TextMuted).Margin(8, 0, 2, 4)
			}
			stepRow(c, e, sv, i, moved)
		}
	})
}

func setupRow(c *ui.Context, sv *stepView, moved bool) {
	rs := sv.rs
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

func stepRow(c *ui.Context, e *page.Env, sv *stepView, i int, moved bool) {
	rs := sv.rs
	t := c.Theme()
	s := sv.r.Steps[i]
	row := ui.Row(c.Key(i)).Gap(8).Padding(5, 8).Radius(6).AlignItems(ui.Center)
	if i == rs.step {
		row.Background(t.Surface).BorderStartWidth(2).BorderColor(t.Accent)
	}
	row.Children(func() {
		ui.Box(c).Size(3, 18).Radius(2).Background(sv.side(s.Player).Color())
		for j, code := range s.Cards {
			ui.Box(c.Key(j)).Children(func() { e.Card(c, code, 22) })
		}
		ui.Column(c).Grow(1).Basis(0).Gap(1).Children(func() {
			cardText(c, e, 12, stepTitle(sv.g, &s, t.Text))
			if result := stepResult(sv.g, &s); result != "" {
				ui.Text(c, result).FontSize(11).TextColor(t.TextMuted).SingleLine()
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
