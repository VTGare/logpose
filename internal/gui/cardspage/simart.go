package cardspage

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/gui/kit"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/sim"
)

// viewedKey is the art key of the picture the panel shows large, falling back to the first art.
func (pg *Page) viewedKey(card cards.Card, info page.SimArtInfo, ca arts.CardArts) string {
	switch k := pg.State.Viewed; {
	case k == "":
	case k == page.SimOwnRef(card.Code):
		return k
	case info.OwnFile != "" && k == page.FileRef(info.OwnFile):
		return k
	default:
		if _, ver, ok := ca.Find(k); ok {
			return ver.Key.String()
		}
	}
	return ca.Arts[0].Head().Key.String()
}

func (pg *Page) artChoices(c *ui.Context, e *page.Env, card cards.Card, info page.SimArtInfo, loaded bool) {
	s := &pg.State
	ca, _ := e.Arts.For(e, card)
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(10).Wrap().AlignItems(ui.Start).Children(func() {
			pending := ca.Pending > 0
			for _, en := range page.ArtEntries(e, card, ca, info, loaded, s.Viewed) {
				if en.Art == nil && pending {
					page.PendingColumn(c, e, card, ca.Pending, false)
					pending = false
				}
				if en.Art == nil {
					_, look, use := page.ArtColumn(c, e, page.ArtColumnOpts{
						Key: en.Key, Name: en.Name, Versions: 1, Viewed: s.Viewed == en.Key, InSim: en.InSim, Usable: en.Usable,
					})
					if look {
						s.Viewed = en.Key
					}
					if use {
						e.SimArt.Use(e, card, en.Choice)
					}
					continue
				}
				art := *en.Art
				key := art.Head().Key.String()
				col, look, use := page.ArtColumn(c, e, page.ArtColumnOpts{
					Key: key, Name: en.Name, Versions: len(art.Versions), Viewed: art.Has(s.Viewed), InSim: en.InSim, Usable: en.Usable,
				})
				if look {
					s.Viewed = key
					if len(art.Versions) > 1 {
						s.Versions = key
					}
				}
				if use {
					e.SimArt.Use(e, card, en.Choice)
				}
				if len(art.Versions) > 1 {
					open := s.Versions == key
					page.VersionPopover(c, e, col, art, info, en.Usable, s.Viewed, &open, func(k string) {
						s.Viewed = k
					}, func(ch sim.ArtChoice) {
						e.SimArt.Use(e, card, ch)
					})
					if !open && s.Versions == key {
						s.Versions = ""
					}
				}
			}
			if pending {
				page.PendingColumn(c, e, card, ca.Pending, false)
			}
		})
		artNote(c, e, info)
	})
}

func artNote(c *ui.Context, e *page.Env, info page.SimArtInfo) {
	t := c.Theme()
	switch {
	case info.NoSim:
		page.NoSimBanner(c, e)
	case info.Err != "":
		ui.Text(c, info.Err).FontSize(12).TextColor(kit.LossColor(t)).Wrap()
	case e.Prefs.ArtOff:
		ui.Text(c, "Card art downloads are off in Settings, so only the sim's own art and your own file can be used.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
	case info.Cur.Kind == sim.ArtFile:
		ui.Text(c, "The sim shows a file you added yourself. If you pick other art, logpose keeps your file "+
			"and it stays here to pick again.").FontSize(12).TextColor(t.TextMuted).Wrap()
	default:
		ui.Text(c, "Click a picture to look at it. Pick a dot to use that art in the sim.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
	}
}

func smallButton(c *ui.Context, label string, icon *ui.SVG, text bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Height(28).Padding(0, 9).Radius(7).Gap(6).AlignItems(ui.Center).
		Background(ui.Transparent).Border(1, t.Border)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Tooltip(label)
	kit.PressScale(b)
	b.Children(func() {
		ui.Icon(c, icon).Size(14, 14)
		if text {
			ui.Text(c, label).FontSize(12.5).FontWeight(500)
		}
	})
	return b
}

func (pg *Page) artButtons(c *ui.Context, key string) {
	if smallButton(c, "Save image…", kit.IconSave, true).Clicked() {
		pg.do(func(a Actions) { a.SaveArt(key) })
	}
	if smallButton(c, "Copy image", kit.IconCopy, false).Clicked() {
		pg.do(func(a Actions) { a.CopyArt(key) })
	}
}

func (pg *Page) CardMenu(e *page.Env, card cards.Card) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		if e.Page != page.Cards {
			if m.Item("Open in Cards").Chosen() {
				e.Nav.OpenInCards(card.Code)
			}
			if e.Nav.ShowsHands() && m.Item("Show hands with "+card.Name).Chosen() {
				e.Nav.ShowHands(card.Code)
			}
			m.Separator()
		}
		info, loaded := e.SimArt.For(e, card)
		m.Submenu("Art in the sim", func(m *ui.Menu) { pg.artMenu(e, m, card, info, loaded) }).Disabled(loaded && info.NoSim)
		key := shownArtKey(e, card, info, loaded)
		if m.Item("View art").Chosen() {
			if key != card.Code {
				e.Nav.ViewArtAt(card.Code, key)
			} else {
				e.Nav.ViewArt(card.Code)
			}
		}
		if m.Item("Save image…").Chosen() {
			pg.do(func(a Actions) { a.SaveArt(key) })
		}
		if m.Item("Copy image").Chosen() {
			pg.do(func(a Actions) { a.CopyArt(key) })
		}
		m.Separator()
		if m.Item("Open on Limitless").Chosen() {
			pg.do(func(a Actions) { a.OpenURL(cards.LimitlessBase + "/cards/" + card.Code) })
		}
	}
}

// shownArtKey is the art reference visible on the right-clicked card: the sim's override off the
// Cards page when mirroring is on, otherwise the base code.
func shownArtKey(e *page.Env, card cards.Card, info page.SimArtInfo, loaded bool) string {
	if e.Page == page.Cards || e.Prefs.SimArtOff || !loaded || info.NoSim {
		return card.Code
	}
	switch info.Cur.Kind {
	case sim.ArtPrint:
		if info.Cur.Key != "" {
			return info.Cur.Key
		}
	case sim.ArtFile:
		if info.OwnFile != "" {
			return page.FileRef(info.OwnFile)
		}
	}
	return card.Code
}

func (pg *Page) artMenu(e *page.Env, m *ui.Menu, card cards.Card, info page.SimArtInfo, loaded bool) {
	ca, _ := e.Arts.For(e, card)
	cur := ""
	if loaded && info.Cur.Kind == sim.ArtPrint {
		cur = info.Cur.Key
	}
	own := false
	for _, en := range page.ArtEntries(e, card, ca, info, loaded, pg.State.Viewed) {
		switch {
		case en.Choice.Kind == sim.ArtOwn:
			m.Separator()
			own = true
			if m.Item("The sim's own").Radio(en.InSim).Disabled(!en.Usable).Chosen() {
				e.SimArt.Use(e, card, en.Choice)
			}
		case en.Art == nil:
			if m.Item(en.Name).Radio(en.InSim).Disabled(!en.Usable).Chosen() {
				e.SimArt.Use(e, card, en.Choice)
			}
		case len(en.Art.Versions) == 1:
			if m.Item(menuName(card, en.Art.Head())).Radio(en.InSim).Disabled(!en.Usable).Chosen() {
				e.SimArt.Use(e, card, en.Choice)
			}
		default:
			art := *en.Art
			m.Submenu(fmt.Sprintf("%s · %d versions", art.Name(), len(art.Versions)), func(m *ui.Menu) {
				shown, inArt := art.Find(cur)
				for _, ver := range art.Versions {
					key := ver.Key.String()
					on := inArt && shown.Key == ver.Key
					label := ver.Product
					if label == "" {
						label = ver.Name
					}
					if m.Item(label).Radio(on).Disabled(!en.Usable).Chosen() {
						e.SimArt.Use(e, card, sim.ArtChoice{Kind: sim.ArtPrint, Key: key})
					}
				}
			}).Disabled(!en.Usable)
		}
	}
	if !own {
		// Without the sim's state, the entry shows but can't be picked.
		m.Separator()
		m.Item("The sim's own").Disabled(true)
	}
}

// menuName adds the product to non-base printings so duplicate kinds can be told apart without pictures.
func menuName(card cards.Card, ver arts.Version) string {
	if ver.Key.String() == card.Code || ver.Product == "" {
		return ver.Name
	}
	return ver.Name + ", " + ver.Product
}
