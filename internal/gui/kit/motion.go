package kit

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
	DrawTime    = 500 * time.Millisecond
	BarTime     = 300 * time.Millisecond
	HoverTime   = 120 * time.Millisecond
	slideTime   = 200 * time.Millisecond
	openTime    = 220 * time.Millisecond
)

// PageEnter fades a page in and lifts it 8 DIPs as it opens.
var PageEnter = ui.ElementTransition{Duration: pageEnterTime, Colors: true, Enter: &ui.Motion{Y: 8}}

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
	Duration: openTime, Colors: true,
	Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true},
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
