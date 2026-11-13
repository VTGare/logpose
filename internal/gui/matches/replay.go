package matches

import (
	"time"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
)

// replayState is an open replay. Its game and steps are nil while the app builds them.
type replayState struct {
	Fingerprint string
	game        *simlog.Game
	r           *replay.Replay
	err         error
	// asked is set once the app was asked to build the replay.
	asked bool
	step  int
	// atLine is the log line the replay opens at once built, or -1 for its start.
	atLine  int
	playing bool
	// speed indexes replaySpeeds.
	speed int
	// moved is set when the step changes, so the action list and the log bring it into view
	// in the next frame only.
	moved bool
	// ticks counts the playback timers started, so a timer from before a pause or a jump
	// does nothing.
	ticks int
	// Glide is how long cards take to move to the current step, which is 0 after a jump of
	// more than one step.
	Glide time.Duration
	// pageBox and matBox are the page's and the mat area's bounds in the frame before.
	pageBox, matBox ui.Rect
	// selected is the card clicked on the mat, which the inspector shows, or 0 for the step's
	// own cards. tab is 0 for the action list and 1 for the game log.
	selected int
	tab      int
	// pile is the deck, trash or life clicked on the mat, which the inspector shows in place of
	// a card.
	pile pileRef
}

type pileRef struct {
	kind pileKind
	Mine bool
}

type pileKind int

const (
	pileNone pileKind = iota
	pileDeck
	pileTrash
	pileLife
)

type LoadedReplay struct {
	Fingerprint string
	Game        *simlog.Game
	Replay      *replay.Replay
	Err         error
}

// replaySpeeds are how long each step shows while the replay plays, how long cards take to move
// on a step, and the speed's name. Cards don't move at the fastest speed.
var replaySpeeds = []struct {
	tick, glide time.Duration
	label       string
}{
	{1200 * time.Millisecond, 250 * time.Millisecond, "1×"},
	{600 * time.Millisecond, 150 * time.Millisecond, "2×"},
	{300 * time.Millisecond, 0, "4×"},
}

func speedLabels() []string {
	out := make([]string, len(replaySpeeds))
	for i, s := range replaySpeeds {
		out[i] = s.label
	}
	return out
}

// glideTime is how long cards take to move on a one-step change. A step taken by hand glides as
// at the slowest speed.
func glideTime(playing bool, speed int) time.Duration {
	if !playing {
		speed = 0
	}
	return replaySpeeds[speed].glide
}

// openReplay shows the match's replay, at the step of a log line or, with -1, from the start.
func (pg *Page) openReplay(e *page.Env, m stats.Match, line int) {
	if rs := pg.Detail.Replay; rs != nil && rs.Fingerprint == m.Fingerprint {
		return
	}
	if pg.Detail.Fingerprint != m.Fingerprint {
		pg.OpenMatch(e, m)
	}
	pg.Detail.Replay = &replayState{Fingerprint: m.Fingerprint, atLine: line}
	pg.buildReplay()
}

func (pg *Page) buildReplay() {
	rs, l := pg.Detail.Replay, pg.Detail.timeline
	if rs == nil || rs.asked || l == nil || l.Fingerprint != rs.Fingerprint {
		return
	}
	rs.asked = true
	if l.Err != nil {
		rs.err = l.Err
		return
	}
	pg.do(func(a Actions) { a.OpenReplay(rs.Fingerprint, l.Game) })
}

func (pg *Page) closeReplay() { pg.Detail.Replay = nil }

func (pg *Page) SetReplay(l LoadedReplay) {
	rs := pg.Detail.Replay
	if rs == nil || rs.Fingerprint != l.Fingerprint {
		return
	}
	rs.game, rs.r, rs.err = l.Game, l.Replay, l.Err
	if rs.r != nil && rs.atLine >= 0 {
		rs.jump(rs.r.StepAtLine(rs.atLine))
	}
}

func (rs *replayState) ready() bool { return rs != nil && rs.r != nil && len(rs.r.Steps) > 0 }

func (rs *replayState) jump(step int) {
	step = min(max(step, 0), len(rs.r.Steps)-1)
	if step == rs.step {
		return
	}
	rs.Glide = 0
	if step == rs.step+1 || step == rs.step-1 {
		rs.Glide = glideTime(rs.playing, rs.speed)
	}
	rs.step, rs.moved = step, true
}

func (rs *replayState) cur() replay.Step { return rs.r.Steps[rs.step] }

func (rs *replayState) restartTurn() { rs.jump(rs.r.FirstOfTurn(rs.cur().Turn)) }

func (rs *replayState) prevTurn() {
	turn := rs.cur().Turn
	if turn == 0 {
		rs.jump(0)
		return
	}
	rs.jump(max(rs.r.FirstOfTurn(turn-1), 0))
}

func (rs *replayState) lastTurn() int { return rs.r.Steps[len(rs.r.Steps)-1].Turn }

func (rs *replayState) nextTurn() {
	turn := rs.cur().Turn
	for i := rs.step + 1; i < len(rs.r.Steps); i++ {
		if rs.r.Steps[i].Turn != turn {
			rs.jump(i)
			return
		}
	}
	rs.jump(len(rs.r.Steps) - 1)
}

func (pg *Page) ReplayKeys(c *ui.Context, e *page.Env) {
	rs := pg.Detail.Replay
	if !rs.ready() {
		return
	}
	c.OnShortcut(0, ui.KeyLeft, func() { rs.jump(rs.step - 1) })
	c.OnShortcut(0, ui.KeyRight, func() { rs.jump(rs.step + 1) })
	c.OnShortcut(0, ui.KeyPageUp, rs.prevTurn)
	c.OnShortcut(0, ui.KeyPageDown, rs.nextTurn)
	c.OnShortcut(0, ui.KeyHome, func() { rs.jump(0) })
	c.OnShortcut(0, ui.KeyEnd, func() { rs.jump(len(rs.r.Steps) - 1) })
	c.OnShortcut(0, ui.KeySpace, func() { pg.togglePlay(e) })
}

func (pg *Page) togglePlay(e *page.Env) {
	rs := pg.Detail.Replay
	rs.playing = !rs.playing
	if rs.playing && rs.step == len(rs.r.Steps)-1 {
		rs.jump(0)
	}
	pg.schedule(e)
}

func (pg *Page) schedule(e *page.Env) {
	rs := pg.Detail.Replay
	rs.ticks++
	if !rs.playing {
		return
	}
	tick := rs.ticks
	pg.do(func(a Actions) {
		a.After(replaySpeeds[rs.speed].tick, func() {
			if pg.Detail.Replay != rs || !rs.playing || rs.ticks != tick {
				return
			}
			rs.jump(rs.step + 1)
			if rs.step == len(rs.r.Steps)-1 {
				rs.playing = false
			}
			pg.schedule(e)
		})
	})
}
