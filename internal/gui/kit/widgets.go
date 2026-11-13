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
	return ui.Button(c, "").Children(func() {
		ui.Icon(c, icon)
		ui.Text(c, text).SingleLine()
	})
}
