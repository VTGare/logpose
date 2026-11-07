package gui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
)

// replayState is an open replay. Its game and steps are nil while the app builds them.
type replayState struct {
	fingerprint string
	game        *simlog.Game
	r           *replay.Replay
	err         error
	step        int
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
	// glide is how long cards take to move to the current step, which is 0 after a jump of
	// more than one step.
	glide time.Duration
	// pageBox and matBox are the page's and the mat area's bounds in the frame before.
	pageBox, matBox ui.Rect
	// selected is the card clicked on the mat, which the inspector shows, or 0 for the step's
	// own cards. tab is 0 for the action list and 1 for the game log.
	selected int
	tab      int
}

// loadedReplay is a game's replay as the app builds it off the main thread.
type loadedReplay struct {
	Fingerprint string
	Game        *simlog.Game
	Replay      *replay.Replay
	Err         error
}

var (
	replaySpeeds = []time.Duration{1200 * time.Millisecond, 600 * time.Millisecond, 300 * time.Millisecond}
	speedLabels  = []string{"1×", "2×", "4×"}
)

// openReplay shows the match's replay, at the step of a log line or, with -1, from the start.
func (v *view) openReplay(m stats.Match, line int) {
	if rs := v.detail.replay; rs != nil && rs.fingerprint == m.Fingerprint {
		return
	}
	if v.detail.fingerprint != m.Fingerprint {
		v.openMatch(m)
	}
	v.detail.replay = &replayState{fingerprint: m.Fingerprint, atLine: line}
	v.act.OpenReplay(m.Fingerprint)
}

func (v *view) closeReplay() { v.detail.replay = nil }

// setReplay shows a replay once built, unless another one opened meanwhile.
func (v *view) setReplay(l loadedReplay) {
	rs := v.detail.replay
	if rs == nil || rs.fingerprint != l.Fingerprint {
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
	rs.glide = 0
	if step == rs.step+1 || step == rs.step-1 {
		rs.glide = glideTime(rs.playing, rs.speed)
	}
	rs.step, rs.moved = step, true
}

func (rs *replayState) cur() replay.Step { return rs.r.Steps[rs.step] }

func (rs *replayState) restartTurn() { rs.jump(rs.r.FirstOfTurn(rs.cur().Turn)) }

// prevTurn goes to the first step of the turn before, or to the start from the first turn.
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

// replayKeys steps with the arrow keys, goes to the turn before or after with Page Up and
// Page Down, to the start and end with Home and End, and plays or pauses with Space.
func (v *view) replayKeys(c *ui.Context) {
	rs := v.detail.replay
	if !rs.ready() {
		return
	}
	c.OnShortcut(0, ui.KeyLeft, func() { rs.jump(rs.step - 1) })
	c.OnShortcut(0, ui.KeyRight, func() { rs.jump(rs.step + 1) })
	c.OnShortcut(0, ui.KeyPageUp, rs.prevTurn)
	c.OnShortcut(0, ui.KeyPageDown, rs.nextTurn)
	c.OnShortcut(0, ui.KeyHome, func() { rs.jump(0) })
	c.OnShortcut(0, ui.KeyEnd, func() { rs.jump(len(rs.r.Steps) - 1) })
	c.OnShortcut(0, ui.KeySpace, func() { v.togglePlay() })
}

func (v *view) togglePlay() {
	rs := v.detail.replay
	rs.playing = !rs.playing
	if rs.playing && rs.step == len(rs.r.Steps)-1 {
		rs.jump(0)
	}
	v.schedule()
}

// schedule starts the timer for the next step while playing.
func (v *view) schedule() {
	rs := v.detail.replay
	rs.ticks++
	if !rs.playing {
		return
	}
	tick := rs.ticks
	v.act.After(replaySpeeds[rs.speed], func() {
		if v.detail.replay != rs || !rs.playing || rs.ticks != tick {
			return
		}
		rs.jump(rs.step + 1)
		if rs.step == len(rs.r.Steps)-1 {
			rs.playing = false
		}
		v.schedule()
	})
}
