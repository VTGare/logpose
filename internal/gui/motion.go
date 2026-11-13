package gui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The app's motion. mygo skips all of it when Windows asks for less motion.
const (
	pageEnterTime = 180 * time.Millisecond
	tileEnterTime = 240 * time.Millisecond
	tileStagger   = 30 * time.Millisecond
	// staggerMost caps the delays, so long lists don't drag.
	staggerMost = 8
	drawTime    = 500 * time.Millisecond
	barTime     = 300 * time.Millisecond
	hoverTime   = 120 * time.Millisecond
	slideTime   = 200 * time.Millisecond
	openTime    = 220 * time.Millisecond
)

// pageEnter fades a page in and lifts it 8 DIPs as it opens.
var pageEnter = ui.ElementTransition{Duration: pageEnterTime, Colors: true, Enter: &ui.Motion{Y: 8}}

// tileEnter brings the i-th of a list's tiles in a little after the one before it.
func tileEnter(i int) ui.ElementTransition {
	return ui.ElementTransition{
		Colors: true, Enter: &ui.Motion{Y: 8},
		Animation: &ui.Animation{Duration: tileEnterTime, Delay: time.Duration(min(i, staggerMost)) * tileStagger},
	}
}

// slide moves a list's rows to their new places when a filter reorders them.
var slide = ui.ElementTransition{Duration: slideTime, Position: true}

// unfold opens and closes a part of a list in place, as a matchup's row.
var unfold = ui.ElementTransition{
	Duration: openTime, Colors: true,
	Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true},
}

// lift raises el 2 DIPs with a deeper shadow while the pointer is over it, to show it opens
// something.
func lift(el ui.Element, radius float32) ui.Element {
	up := float32(0)
	if el.Hovered() {
		up = 1
	}
	at := el.AnimateTo("lift", up, ui.Animation{Duration: hoverTime})
	return el.Translate(0, -2*at).Radius(radius).Shadow(0, 2+6*at, 6+12*at, 0, ui.RGBA(0, 0, 0, 0.3+0.25*at))
}

// grow is a value that goes to target over d, from zero in a page's first frame, so bars grow
// when a page opens and follow the filters after.
func (e *env) grow(el ui.Element, key any, target float32, d time.Duration) float32 {
	if e.entered {
		el.Jump(key, 0)
	}
	return el.AnimateTo(key, target, ui.Animation{Duration: d})
}
