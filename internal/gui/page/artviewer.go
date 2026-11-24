package page

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/gui/kit"
)

const (
	// viewerCardMax is the height of the source images, which are 600×838.
	viewerCardMax = 838
	viewerSide    = 300
	viewerGap     = 36
	viewerMargin  = 32
)

// ArtViewer holds the state of the full-size card art modal.
type ArtViewer struct {
	open   bool
	code   string
	viewed string
}

func (v *ArtViewer) Close() { v.open = false }

func (e *Env) ViewArt(code string) { e.Viewer = ArtViewer{open: true, code: code} }

func (e *Env) ViewArtAt(code, key string) {
	e.Viewer = ArtViewer{open: true, code: code, viewed: key}
}

// Draw shows the viewed picture as large as the window allows, up to the size of its source,
// with the card's other pictures listed beside it.
func (v *ArtViewer) Draw(c *ui.Context, e *Env) {
	if !v.open || v.code == "" || e.Snap.Cards == nil {
		return
	}
	card := e.Snap.Cards.Card(v.code)
	t := c.Theme()
	info, loaded := e.SimArt.For(e, card)
	ca, _ := e.Arts.For(e, card)
	entries := artEntries(e, card, ca, info, loaded, v.viewed)
	viewed := ViewedArtKey(card, info, ca, v.viewed)
	at := max(slices.IndexFunc(entries, func(en artEntry) bool { return en.key == viewed }), 0)
	entry := entries[at]

	ww, wh := c.Size()
	h := min(wh-2*viewerMargin, viewerCardMax)
	w := min(h/CardAspect, ww-viewerSide-viewerGap-2*viewerMargin)
	h = CardHeight(w)

	look := func(key string) { v.viewed = key }
	step := func(by int) func() {
		return func() { look(entries[(at+by+len(entries))%len(entries)].key) }
	}
	ui.DialogBase(c, &v.open, func(back, panel ui.Element) {
		back.Background(ui.RGBA(9, 9, 11, 0.95))
		panel.OnShortcut(0, ui.KeyUp, step(-1))
		panel.OnShortcut(0, ui.KeyDown, step(1))
		ui.Row(c).Gap(viewerGap).Children(func() {
			e.CardImage(c, arts.ExactPrintRef(entry.key).String(), w).Radius(w*0.042).Shadow(0, 18, 60, 0, ui.RGBA(0, 0, 0, 0.7))
			ui.Column(c).Width(viewerSide).Height(h).Gap(14).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
						ui.Text(c, card.Name).FontSize(20).Bold().Wrap()
						ui.Text(c, card.Code+" · "+ColorNames(card.Colors)+" "+card.Type).FontSize(13).TextColor(t.TextMuted)
					})
					if SmallButton(c, "Close the viewer", kit.IconClose, false).Clicked() {
						v.open = false
					}
				})
				ui.Scroll(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
					ui.Column(c).Gap(6).Children(func() {
						for i, en := range entries {
							if viewerEntry(c.Key(en.key), e, en, i == at).Clicked() {
								look(en.key)
							}
						}
					})
				})
				if loaded && !info.NoSim && viewerUse(c, entry).Clicked() {
					e.SimArt.Use(e, card, entry.choice)
				}
				ui.Row(c).Gap(6).Children(func() { ArtButtons(c, e, entry.key) })
				ui.Text(c, "↑ ↓ other art · Esc close").FontSize(12).TextColor(kit.FaintColor(t))
			})
		})
	})
}

func viewerEntry(c *ui.Context, e *Env, en artEntry, viewed bool) ui.Element {
	t := c.Theme()
	row := ui.ButtonBase(c).Label("View "+en.name).Padding(6).Radius(9).Gap(12).AlignItems(ui.Center).Justify(ui.Start).
		Background(ui.Transparent).Border(1, ui.Transparent)
	switch {
	case viewed:
		row.Background(t.Accent.Alpha(0.12)).Border(1, t.Accent)
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}
	return row.Children(func() {
		e.CardImage(c, arts.ExactPrintRef(en.key).String(), 60)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, en.name).FontSize(13.5).FontWeight(600)
			note, col := en.note, t.TextMuted
			if en.inSim {
				note, col = "In the sim", SimGold
			}
			if note != "" {
				ui.Text(c, note).FontSize(12).TextColor(col).MaxLines(2).Ellipsis("…")
			}
		})
	})
}

func viewerUse(c *ui.Context, en artEntry) ui.Element {
	off := !en.usable || en.inSim
	text, alpha := "Use in the sim", float32(1)
	switch {
	case en.inSim:
		text = "In the sim"
	case off:
		alpha = 0.4
	}
	b := ui.ButtonBase(c).Label("Use this art in the sim").Height(30).Padding(0, 11).Radius(7).Gap(7).AlignItems(ui.Center).
		AlignSelf(ui.Start).Background(ui.Transparent).Border(1, SimGold.Alpha(0.45*alpha)).Disabled(off)
	if b.Hovered() && !off {
		b.Background(c.Theme().SurfaceHover)
	}
	return b.Children(func() {
		ui.Icon(c, kit.IconSim).Size(14, 14).TextColor(SimGold.Alpha(alpha))
		ui.Text(c, text).FontSize(12.5).FontWeight(500).TextColor(SimGold.Alpha(alpha))
	})
}

func ColorNames(cols []cards.Color) string {
	names := make([]string, len(cols))
	for i, col := range cols {
		names[i] = string(col)
	}
	return strings.Join(names, "/")
}
