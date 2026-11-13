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

// artEntry is one picture a card offers. art is nil for the user's file and the sim's own.
type artEntry struct {
	key, name, note string
	art             *arts.Art
	choice          sim.ArtChoice
	inSim, usable   bool
}

// artEntries lists a card's pictures in the Art tab's order: its arts, the user's file and the
// sim's own. An art's key is the version of it being viewed, or its head, and its choice puts
// that version in the sim. The viewer, the Art tab, the art menu and the tab's count all read
// this list.
func (pg *cardsPage) artEntries(e *env, card cards.Card, ca arts.CardArts, info simArtInfo, loaded bool) []artEntry {
	usable := loaded && !info.noSim && !info.busy
	var out []artEntry
	for i, art := range ca.Arts {
		ver := art.Head()
		if shown, ok := art.Find(pg.state.viewed); ok {
			ver = shown
		}
		key := ver.Key.String()
		out = append(out, artEntry{
			key: key, name: art.Name(), note: ver.Product, art: &ca.Arts[i],
			choice: sim.ArtChoice{Kind: sim.ArtPrint, Key: key},
			inSim:  loaded && info.cur.Kind == sim.ArtPrint && art.Has(info.cur.Key),
			usable: usable && !e.prefs.artOff,
		})
	}
	if info.ownFile != "" {
		out = append(out, artEntry{
			key: fileRef(info.ownFile), name: "Your file", note: "A file you added to the sim",
			choice: sim.ArtChoice{Kind: sim.ArtFile}, inSim: info.cur.Kind == sim.ArtFile, usable: usable,
		})
	}
	if loaded && !info.noSim {
		out = append(out, artEntry{
			key: simOwnRef(card.Code), name: "Sim's own", note: "The sim's picture",
			choice: sim.ArtChoice{Kind: sim.ArtOwn}, inSim: info.cur.Kind == sim.ArtOwn, usable: usable,
		})
	}
	return out
}

// artViewer shows the viewed picture as large as the window allows, up to the size of its
// source, with the card's other pictures listed beside it.
func (pg *cardsPage) artViewer(c *ui.Context, e *env, card cards.Card) {
	s := &pg.state
	t := c.Theme()
	info, loaded := pg.simArtFor(card)
	ca, _ := pg.artsFor(card)
	entries := pg.artEntries(e, card, ca, info, loaded)
	viewed := pg.viewedKey(card, info, ca)
	at := max(slices.IndexFunc(entries, func(en artEntry) bool { return en.key == viewed }), 0)
	entry := entries[at]

	ww, wh := c.Size()
	h := min(wh-2*viewerMargin, viewerCardMax)
	w := min(h/cardAspect, ww-viewerSide-viewerGap-2*viewerMargin)
	h = cardHeight(w)

	look := func(key string) { s.viewed, s.versions = key, "" }
	step := func(by int) func() {
		return func() { look(entries[(at+by+len(entries))%len(entries)].key) }
	}
	ui.DialogBase(c, &s.viewer, func(back, panel ui.Element) {
		back.Background(ui.RGBA(9, 9, 11, 0.95))
		panel.OnShortcut(0, ui.KeyUp, step(-1))
		panel.OnShortcut(0, ui.KeyDown, step(1))
		ui.Row(c).Gap(viewerGap).Children(func() {
			e.cardImage(c, entry.key, w).Radius(w*0.042).Shadow(0, 18, 60, 0, ui.RGBA(0, 0, 0, 0.7))
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
						for i, en := range entries {
							if viewerEntry(c.Key(en.key), e, en, i == at).Clicked() {
								look(en.key)
							}
						}
					})
				})
				if loaded && !info.noSim && viewerUse(c, entry).Clicked() {
					pg.useSimArt(card, entry.choice)
				}
				ui.Row(c).Gap(6).Children(func() { pg.artButtons(c, entry.key) })
				ui.Text(c, "↑ ↓ other art · Esc close").FontSize(12).TextColor(faintColor(t))
			})
		})
	})
}

func viewerEntry(c *ui.Context, e *env, en artEntry, viewed bool) ui.Element {
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
		e.cardImage(c, en.key, 60)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, en.name).FontSize(13.5).FontWeight(600)
			note, col := en.note, t.TextMuted
			if en.inSim {
				note, col = "In the sim", simGold
			}
			if note != "" {
				ui.Text(c, note).FontSize(12).TextColor(col).MaxLines(2).Ellipsis("…")
			}
		})
	})
}

// viewerUse is the button that puts the viewed picture in the sim, or says it's there.
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
		AlignSelf(ui.Start).Background(ui.Transparent).Border(1, simGold.Alpha(0.45*alpha)).Disabled(off)
	if b.Hovered() && !off {
		b.Background(c.Theme().SurfaceHover)
	}
	return b.Children(func() {
		ui.Icon(c, iconSim).Size(14, 14).TextColor(simGold.Alpha(alpha))
		ui.Text(c, text).FontSize(12.5).FontWeight(500).TextColor(simGold.Alpha(alpha))
	})
}
