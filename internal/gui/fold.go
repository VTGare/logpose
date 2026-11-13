package gui

import (
	"github.com/egoist/mygo/ui"
)

// foldHead is what a collapsible tile's title row shows. The summary, when there is one, takes
// the body's place while the tile is collapsed, and the tag shows only while it's open.
type foldHead struct {
	label, tag, note, summary string
}

// foldTile is a tile whose title row collapses it to that row and its summary. It's kept
// collapsed across restarts by id.
func (e *env) foldTile(c *ui.Context, id string, h foldHead, body func()) ui.Element {
	t := c.Theme()
	folded := e.prefs.folded[id]
	verb, icon := "Collapse ", iconChevronUp
	if folded {
		verb, icon = "Expand ", iconChevron
	}
	return panel(c).Gap(8).Padding(14, 16).Children(func() {
		head := ui.ButtonBase(c).Label(verb+h.label).Row().Gap(6).AlignItems(ui.Center).
			Padding(4, 8).Margin(-4, -8).Radius(6).Background(ui.Transparent).
			Transition(ui.ElementTransition{Duration: hoverTime, Colors: true})
		if head.Hovered() {
			head.Background(t.Surface.Alpha(0.5))
		}
		if head.Clicked() {
			e.setFolded(id, !folded)
		}
		head.Children(func() {
			ui.Text(c, h.label).FontSize(13).FontWeight(600).SingleLine()
			if h.tag != "" && !folded {
				ui.Text(c, "· "+h.tag).FontSize(12).TextColor(t.TextMuted).SingleLine()
			}
			switch {
			case folded && h.summary != "":
				ui.Text(c, h.summary).FontSize(12).TextColor(t.TextMuted).SingleLine().Grow(1).Basis(0).MinWidth(0).Padding(0, 0, 0, 6)
			case !folded && h.note != "":
				ui.Spacer(c)
				ui.Text(c, h.note).FontSize(12).TextColor(t.TextMuted).SingleLine()
			default:
				ui.Spacer(c)
			}
			ui.Icon(c, icon).FontSize(12).TextColor(faintColor(t))
		})
		if !folded {
			body()
		}
	})
}

func (e *env) setFolded(id string, folded bool) {
	if folded {
		e.prefs.folded[id] = true
	} else {
		delete(e.prefs.folded, id)
	}
	e.savePref(prefFolded)
}
