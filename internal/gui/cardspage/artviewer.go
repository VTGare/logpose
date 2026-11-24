package cardspage

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/gui/kit"
	"github.com/VTGare/logpose/internal/gui/page"
)

const (
	// viewerCardMax is the height of the source images, which are 600×838.
	viewerCardMax = 838
	viewerSide    = 300
	viewerGap     = 36
	viewerMargin  = 32
)

// ArtViewer shows the viewed picture as large as the window allows, up to the size of its source,
// with the card's other pictures listed beside it.
func (pg *Page) ArtViewer(c *ui.Context, e *page.Env, card cards.Card) {
	s := &pg.State
	t := c.Theme()
	info, loaded := e.SimArt.For(e, card)
	ca, _ := e.Arts.For(e, card)
	entries := page.ArtEntries(e, card, ca, info, loaded, s.Viewed)
	viewed := pg.viewedKey(card, info, ca)
	at := max(slices.IndexFunc(entries, func(en page.ArtEntry) bool { return en.Key == viewed }), 0)
	entry := entries[at]

	ww, wh := c.Size()
	h := min(wh-2*viewerMargin, viewerCardMax)
	w := min(h/page.CardAspect, ww-viewerSide-viewerGap-2*viewerMargin)
	h = page.CardHeight(w)

	look := func(key string) { s.Viewed, s.Versions = key, "" }
	step := func(by int) func() {
		return func() { look(entries[(at+by+len(entries))%len(entries)].Key) }
	}
	ui.DialogBase(c, &s.Viewer, func(back, panel ui.Element) {
		back.Background(ui.RGBA(9, 9, 11, 0.95))
		panel.OnShortcut(0, ui.KeyUp, step(-1))
		panel.OnShortcut(0, ui.KeyDown, step(1))
		ui.Row(c).Gap(viewerGap).Children(func() {
			e.CardImage(c, page.ExactPrintRef(entry.Key), w).Radius(w*0.042).Shadow(0, 18, 60, 0, ui.RGBA(0, 0, 0, 0.7))
			ui.Column(c).Width(viewerSide).Height(h).Gap(14).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
						ui.Text(c, card.Name).FontSize(20).Bold().Wrap()
						ui.Text(c, card.Code+" · "+colorNames(card.Colors)+" "+card.Type).FontSize(13).TextColor(t.TextMuted)
					})
					if smallButton(c, "Close the viewer", kit.IconClose, false).Clicked() {
						s.Viewer = false
					}
				})
				ui.Scroll(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
					ui.Column(c).Gap(6).Children(func() {
						for i, en := range entries {
							if viewerEntry(c.Key(en.Key), e, en, i == at).Clicked() {
								look(en.Key)
							}
						}
					})
				})
				if loaded && !info.NoSim && viewerUse(c, entry).Clicked() {
					e.SimArt.Use(e, card, entry.Choice)
				}
				ui.Row(c).Gap(6).Children(func() { pg.artButtons(c, entry.Key) })
				ui.Text(c, "↑ ↓ other art · Esc close").FontSize(12).TextColor(kit.FaintColor(t))
			})
		})
	})
}

func viewerEntry(c *ui.Context, e *page.Env, en page.ArtEntry, viewed bool) ui.Element {
	t := c.Theme()
	row := ui.ButtonBase(c).Label("View "+en.Name).Padding(6).Radius(9).Gap(12).AlignItems(ui.Center).Justify(ui.Start).
		Background(ui.Transparent).Border(1, ui.Transparent)
	switch {
	case viewed:
		row.Background(t.Accent.Alpha(0.12)).Border(1, t.Accent)
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}
	return row.Children(func() {
		e.CardImage(c, page.ExactPrintRef(en.Key), 60)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, en.Name).FontSize(13.5).FontWeight(600)
			note, col := en.Note, t.TextMuted
			if en.InSim {
				note, col = "In the sim", page.SimGold
			}
			if note != "" {
				ui.Text(c, note).FontSize(12).TextColor(col).MaxLines(2).Ellipsis("…")
			}
		})
	})
}

func viewerUse(c *ui.Context, en page.ArtEntry) ui.Element {
	off := !en.Usable || en.InSim
	text, alpha := "Use in the sim", float32(1)
	switch {
	case en.InSim:
		text = "In the sim"
	case off:
		alpha = 0.4
	}
	b := ui.ButtonBase(c).Label("Use this art in the sim").Height(30).Padding(0, 11).Radius(7).Gap(7).AlignItems(ui.Center).
		AlignSelf(ui.Start).Background(ui.Transparent).Border(1, page.SimGold.Alpha(0.45*alpha)).Disabled(off)
	if b.Hovered() && !off {
		b.Background(c.Theme().SurfaceHover)
	}
	return b.Children(func() {
		ui.Icon(c, kit.IconSim).Size(14, 14).TextColor(page.SimGold.Alpha(alpha))
		ui.Text(c, text).FontSize(12.5).FontWeight(500).TextColor(page.SimGold.Alpha(alpha))
	})
}
