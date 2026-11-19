package matches

import (
	"time"

	"github.com/VTGare/logpose/internal/replay"

	"github.com/egoist/mygo/ui"
)

// cardEase lifts a card off gently and sets it down with a long slow end, as a hand places it.
// A plain ease-out starts at full speed, which reads as a jump on long moves.
var cardEase = ui.CubicBezier(0.3, 0, 0.2, 1)

// motion is how things move to the step. After a jump they move in a nanosecond, so they're
// there at once but every Match is still kept for the next step.
func (sv *stepView) motion() ui.Animation {
	return ui.Animation{Duration: max(sv.glideFor, time.Nanosecond), Ease: cardEase}
}

// Glide moves a card from where the frame before showed it, when the step just moved by one.
func (sv *stepView) Glide(el ui.Element, id int) ui.Element {
	if id == 0 {
		return el
	}
	return el.Transition(sv.glide(cardMatch(id), nil))
}

func (sv *stepView) glide(match any, enter *ui.Motion) ui.ElementTransition {
	a := sv.motion()
	// No Exit, since mygo leaves an exit's copy behind even when a Match takes the card elsewhere.
	return ui.ElementTransition{Animation: &a, Position: true, Match: match, Enter: enter}
}

// ease moves a value of el, under key, to target over the step's glide, or puts it there at once
// after a jump.
func (sv *stepView) ease(el ui.Element, key any, target float32) float32 {
	if sv.glideFor == 0 {
		return el.Jump(key, target)
	}
	return el.AnimateTo(key, target, sv.motion())
}

func lerp(a, b, t float32) float32 { return a + (b-a)*t }

// pulse is how far along a one-off effect of d is, from 0 as the step shows to 1, and 1 at once
// after a jump or when the desktop asks for less motion.
func (sv *stepView) pulse(c *ui.Context, d time.Duration) float32 {
	if sv.glideFor == 0 || c.Preferences().ReduceMotion {
		return 1
	}
	t := float32(c.Now().Sub(sv.rs.shownAt)) / float32(d)
	if t >= 1 {
		return 1
	}
	c.AnimationFrame()
	return max(t, 0)
}

// hit reports whether the step is an attack's damage that took life or a card.
func (sv *stepView) hit() bool {
	if sv.step.Kind != replay.KindDamage {
		return false
	}
	for _, o := range sv.step.Outcome {
		if o.Kind == replay.OutcomeHit || o.Kind == replay.OutcomeKO {
			return true
		}
	}
	return false
}
