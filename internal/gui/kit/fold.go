package kit

import (
	"github.com/egoist/mygo/ui"
)

// FoldHead is what a collapsible tile's title row shows. The summary, when there is one, takes
// the body's place while the tile is collapsed, and the tag shows only while it's open.
type FoldHead struct {
	Label, Tag, note, Summary string
}

// FoldTile is a tile whose title row collapses it to that row and its summary. toggle runs
// when the row is clicked.
func FoldTile(c *ui.Context, h FoldHead, folded bool, toggle, body func()) ui.Element {
	t := c.Theme()
	verb, icon := "Collapse ", IconChevronUp
	if folded {
		verb, icon = "Expand ", IconChevron
	}
	return Panel(c).Gap(8).Padding(14, 16).Children(func() {
		head := ui.ButtonBase(c).Label(verb+h.Label).Row().Gap(6).AlignItems(ui.Center).
			Padding(4, 8).Margin(-4, -8).Radius(6).Background(ui.Transparent).
			Transition(ui.ElementTransition{Duration: HoverTime, Colors: true})
		if head.Hovered() {
			head.Background(t.Surface.Alpha(0.5))
		}
		if head.Clicked() {
			toggle()
		}
		head.Children(func() {
			ui.Text(c, h.Label).FontSize(13).FontWeight(600).SingleLine()
			if h.Tag != "" && !folded {
				ui.Text(c, "· "+h.Tag).FontSize(12).TextColor(t.TextMuted).SingleLine()
			}
			switch {
			case folded && h.Summary != "":
				ui.Text(c, h.Summary).FontSize(12).TextColor(t.TextMuted).SingleLine().Grow(1).Basis(0).MinWidth(0).Padding(0, 0, 0, 6)
			case !folded && h.note != "":
				ui.Spacer(c)
				ui.Text(c, h.note).FontSize(12).TextColor(t.TextMuted).SingleLine()
			default:
				ui.Spacer(c)
			}
			ui.Icon(c, icon).FontSize(12).TextColor(FaintColor(t))
		})
		if !folded {
			body()
		}
	})
}
