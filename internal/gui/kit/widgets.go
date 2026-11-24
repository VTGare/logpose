package kit

import "github.com/egoist/mygo/ui"

func Tile(c *ui.Context, label string, body func()) ui.Element {
	return TitledTile(c, label, "", body)
}

// TitledTile is a tile with a muted note at the right of its title, such as a count.
func TitledTile(c *ui.Context, label, note string, body func()) ui.Element {
	return TagTile(c, label, "", note, body)
}

// TagTile is a titled tile whose title names the games it counts after a dot, such as
// "Matchups · Wano/Miracle".
func TagTile(c *ui.Context, label, tag, note string, body func()) ui.Element {
	t := c.Theme()
	return Panel(c).Gap(8).Padding(14, 16).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(13).FontWeight(600)
			if tag != "" {
				ui.Text(c, "· "+tag).FontSize(12).TextColor(t.TextMuted).SingleLine()
			}
			if note != "" {
				ui.Spacer(c)
				ui.Text(c, note).FontSize(12).TextColor(t.TextMuted)
			}
		})
		body()
	})
}

// Popover styles a Popover's panel: the page's color with an edge and a deep shadow, so it
// stands over what it covers.
func Popover(panel ui.Element, t *ui.Theme) ui.Element {
	return panel.Radius(10).Background(t.Background).Border(1, t.Border).Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.45))
}

// Tinted marks a pill that's on with the accent.
func Tinted(el ui.Element, t *ui.Theme) ui.Element {
	return el.Background(t.Accent.Alpha(0.18)).Border(1, t.Accent)
}

// Panel is the raised surface tiles sit on: a step above the page, with a hairline edge and a
// soft shadow.
func Panel(c *ui.Context) ui.Element {
	t := c.Theme()
	return ui.Column(c).Radius(12).Background(PanelColor(t)).Border(1, t.Border.Alpha(0.6)).
		Shadow(0, 6, 20, 0, PanelShade)
}

func IconButton(c *ui.Context, icon *ui.SVG, text string) ui.Element {
	return PressScale(ui.Button(c, "")).Children(func() {
		ui.Icon(c, icon)
		ui.Text(c, text).SingleLine()
	})
}

// LinkButton is text in the accent color, with an optional leading icon.
func LinkButton(c *ui.Context, icon *ui.SVG, text string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(text).Background(ui.Transparent).Padding(0).Gap(6).AlignItems(ui.Center)
	col := t.Accent
	if b.Hovered() {
		col = t.AccentHover
	}
	return b.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(14, 14).TextColor(col)
		}
		ui.Text(c, text).FontSize(13).TextColor(col).SingleLine()
	})
}

// BackLink is a muted back arrow and text that reports a click.
func BackLink(c *ui.Context, text, label string) bool {
	t := c.Theme()
	b := ui.Row(c).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer).Label(label)
	b.Children(func() {
		ui.Icon(c, IconPrevStep).TextColor(t.TextMuted)
		ui.Text(c, text).TextColor(t.TextMuted)
	})
	return b.Clicked()
}

// UnderlineTabs draws tabs with an accent underline on the active one.
func UnderlineTabs(c *ui.Context, active *int, labels []string) {
	t := c.Theme()
	tabs := ui.TabsBase(c, active, len(labels))
	tabs.List.Gap(4).Children(func() {
		for i, label := range labels {
			tab := tabs.Tab(i).Column().Gap(6).Padding(7, 4, 0, 4)
			col, bar, weight := t.TextMuted, ui.Transparent, 400
			switch {
			case i == *active:
				col, bar, weight = t.Text, t.Accent, 500
			case tab.Hovered():
				col = t.Text
			}
			tab.Children(func() {
				ui.Text(c, label).FontSize(13).FontWeight(weight).TextColor(col).Padding(0, 8)
				ui.Box(c).Height(2).Radius(1).Background(bar).AlignSelf(ui.Stretch)
			})
		}
	})
}

// FoldHead is what a collapsible tile's title row shows. The summary, when there is one, takes
// the body's place while the tile is collapsed, and the tag shows only while it's open.
type FoldHead struct {
	Label, Tag, Summary string
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
			if folded && h.Summary != "" {
				ui.Text(c, h.Summary).FontSize(12).TextColor(t.TextMuted).SingleLine().Grow(1).Basis(0).MinWidth(0).Padding(0, 0, 0, 6)
			} else {
				ui.Spacer(c)
			}
			ui.Icon(c, icon).FontSize(12).TextColor(FaintColor(t))
		})
		if !folded {
			body()
		}
	})
}
