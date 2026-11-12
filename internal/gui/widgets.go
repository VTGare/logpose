package gui

import "github.com/egoist/mygo/ui"

func tile(c *ui.Context, label string, body func()) ui.Element {
	return titledTile(c, label, "", body)
}

// titledTile is a tile with a muted note at the right of its title, such as a count.
func titledTile(c *ui.Context, label, note string, body func()) ui.Element {
	return tagTile(c, label, "", note, body)
}

// tagTile is a titled tile whose title names the games it counts after a dot, such as
// "Matchups · Wano/Miracle".
func tagTile(c *ui.Context, label, tag, note string, body func()) ui.Element {
	t := c.Theme()
	return panel(c).Gap(8).Padding(14, 16).Children(func() {
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

// popover styles a popover's panel: the page's color with an edge and a deep shadow, so it
// stands over what it covers.
func popover(panel ui.Element, t *ui.Theme) ui.Element {
	return panel.Radius(10).Background(t.Background).Border(1, t.Border).Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.45))
}

// tinted marks a pill that's on with the accent.
func tinted(el ui.Element, t *ui.Theme) ui.Element {
	return el.Background(t.Accent.Alpha(0.18)).Border(1, t.Accent)
}

// panel is the raised surface tiles sit on: a step above the page, with a hairline edge and a
// soft shadow.
func panel(c *ui.Context) ui.Element {
	t := c.Theme()
	return ui.Column(c).Radius(12).Background(panelColor(t)).Border(1, t.Border.Alpha(0.6)).
		Shadow(0, 6, 20, 0, panelShade)
}

func iconButton(c *ui.Context, icon *ui.SVG, text string) ui.Element {
	return ui.Button(c, "").Children(func() {
		ui.Icon(c, icon)
		ui.Text(c, text).SingleLine()
	})
}
