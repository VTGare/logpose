package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The app's motion. mygo skips all of it when Windows asks for less motion.
const (
	pageEnterTime = 140 * time.Millisecond
	tileEnterTime = 240 * time.Millisecond
	tileStagger   = 30 * time.Millisecond
	// staggerMost caps the delays, so long lists don't drag.
	staggerMost = 8
	DrawTime    = 500 * time.Millisecond
	BarTime     = 300 * time.Millisecond
	HoverTime   = 120 * time.Millisecond
	slideTime   = 200 * time.Millisecond
	openTime    = 220 * time.Millisecond
	closeTime   = 150 * time.Millisecond
	swapTime    = 300 * time.Millisecond
	pressTime   = 100 * time.Millisecond
)

// PageEnter fades a page in.
var PageEnter = ui.ElementTransition{Duration: pageEnterTime, Colors: true, Enter: &ui.Motion{}}

// TileEnter brings the i-th of a list's tiles in a little after the one before it.
func TileEnter(i int) ui.ElementTransition {
	return ui.ElementTransition{
		Colors: true, Enter: &ui.Motion{Y: 8},
		Animation: &ui.Animation{Duration: tileEnterTime, Delay: time.Duration(min(i, staggerMost)) * tileStagger},
	}
}

// Slide moves a list's rows to their new places when a filter reorders them.
var Slide = ui.ElementTransition{Duration: slideTime, Position: true}

// Unfold opens and closes a part of a list in place, as a matchup's row.
var Unfold = ui.ElementTransition{
	Duration: openTime, ExitDuration: closeTime, Colors: true,
	Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true},
}

// IconCross crosses one icon over into another, as Play into Pause.
var IconCross = ui.ElementTransition{
	Duration: swapTime, Ease: ui.CubicBezier(0.2, 0, 0, 1),
	Enter: &ui.Motion{Scale: 0.25, Blur: 4}, Exit: &ui.Motion{Scale: 0.25, Blur: 4},
}

// PressScale shrinks a button a little while it's pressed.
func PressScale(b ui.Element) ui.Element {
	down := float32(0)
	if b.Pressed() {
		down = 1
	}
	at := b.AnimateTo("press", down, ui.Animation{Duration: pressTime})
	s := 1 - 0.04*at
	return b.Scale(s, s)
}

// Lift raises el 2 DIPs with a deeper shadow while the pointer is over it, to show it opens
// something.
func Lift(el ui.Element, radius float32) ui.Element {
	up := float32(0)
	if el.Hovered() {
		up = 1
	}
	at := el.AnimateTo("lift", up, ui.Animation{Duration: HoverTime})
	return el.Translate(0, -2*at).Radius(radius).Shadow(0, 2+6*at, 6+12*at, 0, ui.RGBA(0, 0, 0, 0.3+0.25*at))
}
