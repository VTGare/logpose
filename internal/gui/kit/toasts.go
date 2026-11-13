package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Toasts draws the window's Toasts with the theme's look plus a close button, which the
// theme's own Toasts don't have. Call it once per frame.
func Toasts(c *ui.Context) {
	t := c.Theme()
	fill, text := t.Inverse, t.InverseText
	if fill == ui.Transparent {
		fill = t.Text
	}
	if text == ui.Transparent {
		text = t.Background
	}
	ui.ToastViewportBase(c, func(viewport ui.Element, shown []ui.Toast) {
		viewport.Bottom(t.Space(6)).AlignItems(ui.Center).Gap(t.Space(2))
		for _, ts := range shown {
			p := ui.ToastBase(c, ts)
			box := p.Root.Row().AlignItems(ui.Center).Gap(t.Space(3)).Padding(t.Space(2.5), t.Space(2.5), t.Space(2.5), t.Space(4)).
				Radius(t.Space(2)).Background(fill).TextColor(text).Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, 0.25))
			const fade = 200 * time.Millisecond
			if left := p.Left(); left > 0 && left < fade {
				box.Opacity(float32(left) / float32(fade))
				c.AnimationFrame()
			}
			box.Children(func() {
				if ts.Description == "" {
					ui.Text(c, ts.Title)
				} else {
					ui.Column(c).Shrink(1).Gap(t.Space(0.5)).Children(func() {
						ui.Text(c, ts.Title).FontWeight(600)
						ui.Text(c, ts.Description).TextColor(text.Alpha(0.75))
					})
				}
				if ts.Action != "" {
					b := p.ActionButton().Padding(t.Space(1), t.Space(2.5)).Radius(t.Radius).TextColor(t.Accent.Mix(text, 0.35)).FontWeight(600)
					if b.Hovered() {
						b.Background(text.Alpha(0.12))
					}
					b.Children(func() { ui.Text(c, ts.Action) })
				}
				x := p.CloseButton().Label("Close").Size(24, 24).Radius(t.Radius).Center().TextColor(text.Alpha(0.6))
				if x.Hovered() {
					x.Background(text.Alpha(0.12)).TextColor(text)
				}
				x.Children(func() { ui.Icon(c, IconClose).FontSize(14) })
			})
		}
	})
}
