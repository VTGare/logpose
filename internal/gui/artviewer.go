package gui

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"
)

const (
	// viewerCardMax is the height of the source images, which are 600×838.
	viewerCardMax = 838
	viewerSide    = 300
	viewerGap     = 36
	viewerMargin  = 32
)

// artEntry is one picture the art viewer lists.
type artEntry struct {
	key, name, note string
	choice          sim.ArtChoice
	inSim, usable   bool
}

// artEntries lists a card's pictures in the Art tab's order: its arts, the user's file and the
// sim's own. An art stands for the version of it being viewed, or its head.
func (v *view) artEntries(card cards.Card, ca arts.CardArts, info simArtInfo, loaded bool) []artEntry {
	usable := loaded && !info.noSim && !info.busy
	var out []artEntry
	for _, art := range ca.Arts {
		ver := art.Head()
		if shown, ok := art.Find(v.cardsUI.viewed); ok {
			ver = shown
		}
		key := ver.Key.String()
		out = append(out, artEntry{
			key: key, name: art.Name(), note: ver.Product,
			choice: sim.ArtChoice{Kind: sim.ArtPrint, Key: key},
			inSim:  loaded && info.cur.Kind == sim.ArtPrint && art.Has(info.cur.Key),
			usable: usable && !v.prefs.artOff,
		})
	}
	if info.ownFile != "" {
		out = append(out, artEntry{
			key: ownFilePrefix + info.ownFile, name: "Your file", note: "A file you added to the sim",
			choice: sim.ArtChoice{Kind: sim.ArtFile}, inSim: info.cur.Kind == sim.ArtFile, usable: usable,
		})
	}
	if loaded && !info.noSim {
		out = append(out, artEntry{
			key: simOwnPrefix + card.Code, name: "Sim's own", note: "The sim's picture",
			choice: sim.ArtChoice{Kind: sim.ArtOwn}, inSim: info.cur.Kind == sim.ArtOwn, usable: usable,
		})
	}
	return out
}

// artViewer shows the viewed picture as large as the window allows, up to the size of its
// source, with the card's other pictures listed beside it.
func (v *view) artViewer(c *ui.Context, card cards.Card) {
	s := &v.cardsUI
	t := c.Theme()
	info, loaded := v.simArtFor(card)
	ca, _ := v.artsFor(card)
	entries := v.artEntries(card, ca, info, loaded)
	viewed := v.viewedKey(card, info, ca)
	at := max(slices.IndexFunc(entries, func(e artEntry) bool { return e.key == viewed }), 0)
	entry := entries[at]

	ww, wh := c.Size()
	h := min(wh-2*viewerMargin, viewerCardMax)
	w := min(h*5/7, ww-viewerSide-viewerGap-2*viewerMargin)
	h = w * 7 / 5

	look := func(key string) { s.viewed, s.versions = key, "" }
	step := func(by int) func() {
		return func() { look(entries[(at+by+len(entries))%len(entries)].key) }
	}
	ui.DialogBase(c, &s.viewer, func(back, panel ui.Element) {
		back.Background(ui.RGBA(9, 9, 11, 0.95))
		panel.OnShortcut(0, ui.KeyUp, step(-1))
		panel.OnShortcut(0, ui.KeyDown, step(1))
		ui.Row(c).Gap(viewerGap).Children(func() {
			v.printArt(c, entry.key, w, h).Radius(w*0.042).Shadow(0, 18, 60, 0, ui.RGBA(0, 0, 0, 0.7))
			ui.Column(c).Width(viewerSide).Height(h).Gap(14).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
						ui.Text(c, card.Name).FontSize(20).Bold().Wrap()
						ui.Text(c, card.Code+" · "+colorNames(card.Colors)+" "+card.Type).FontSize(13).TextColor(t.TextMuted)
					})
					if smallButton(c, "Close the viewer", iconClose, false).Clicked() {
						s.viewer = false
					}
				})
				ui.Scroll(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
					ui.Column(c).Gap(6).Children(func() {
						for i, e := range entries {
							if v.viewerEntry(c.Key(e.key), e, i == at).Clicked() {
								look(e.key)
							}
						}
					})
				})
				if loaded && !info.noSim && viewerUse(c, entry).Clicked() {
					v.useSimArt(card, entry.choice)
				}
				ui.Row(c).Gap(6).Children(func() { v.artButtons(c, entry.key) })
				ui.Text(c, "↑ ↓ other art · Esc close").FontSize(12).TextColor(faintColor(t))
			})
		})
	})
}

func (v *view) viewerEntry(c *ui.Context, e artEntry, viewed bool) ui.Element {
	t := c.Theme()
	row := ui.ButtonBase(c).Label("View "+e.name).Padding(6).Radius(9).Gap(12).AlignItems(ui.Center).Justify(ui.Start).
		Background(ui.Transparent).Border(1, ui.Transparent)
	switch {
	case viewed:
		row.Background(t.Accent.Alpha(0.12)).Border(1, t.Accent)
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}
	return row.Children(func() {
		v.printArt(c, e.key, 60, 84).Radius(4)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, e.name).FontSize(13.5).FontWeight(600)
			note, col := e.note, t.TextMuted
			if e.inSim {
				note, col = "In the sim", simGold
			}
			if note != "" {
				ui.Text(c, note).FontSize(12).TextColor(col).MaxLines(2).Ellipsis("…")
			}
		})
	})
}

// viewerUse is the button that puts the viewed picture in the sim, or says it's there.
func viewerUse(c *ui.Context, e artEntry) ui.Element {
	off := !e.usable || e.inSim
	text, alpha := "Use in the sim", float32(1)
	switch {
	case e.inSim:
		text = "In the sim"
	case off:
		alpha = 0.4
	}
	b := ui.ButtonBase(c).Label("Use this art in the sim").Height(30).Padding(0, 11).Radius(7).Gap(7).AlignItems(ui.Center).
		AlignSelf(ui.Start).Background(ui.Transparent).Border(1, simGold.Alpha(0.45*alpha)).Disabled(off)
	if b.Hovered() && !off {
		b.Background(c.Theme().SurfaceHover)
	}
	return b.Children(func() {
		ui.Icon(c, iconSim).Size(14, 14).TextColor(simGold.Alpha(alpha))
		ui.Text(c, text).FontSize(12.5).FontWeight(500).TextColor(simGold.Alpha(alpha))
	})
}
