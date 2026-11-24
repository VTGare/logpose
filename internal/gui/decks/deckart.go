package decks

import (
	"fmt"
	"maps"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/gui/kit"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/sim"
)

const (
	// deckArtTwoColMin is the narrowest grid width that fits two card boxes side by side.
	deckArtTwoColMin = 740
	// deckArtWideMin is the fewest printings that span a full row instead of half a row.
	deckArtWideMin = 5
)

type deckArtPick struct {
	leader, title string
	list          map[string]int
	versionsCard  string
	versionsHead  string
}

type deckArtItem struct {
	card    cards.Card
	copies  int
	info    page.SimArtInfo
	loaded  bool
	ca      arts.CardArts
	entries []page.ArtEntry
	wide    bool
}

func (pg *Page) openDeckArt(leader, title string, list map[string]int) {
	pg.artPick = &deckArtPick{leader: leader, title: title, list: maps.Clone(list)}
}

func (pg *Page) deckArtPage(c *ui.Context, e *page.Env) {
	t := c.Theme()
	ap := pg.artPick
	leaderName := e.Snap.Card(ap.leader).Name

	items, changed, noSim := loadDeckArtItems(e, ap)

	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		back := ui.Row(c).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer).Label("Back to " + leaderName)
		if back.Clicked() {
			pg.artPick = nil
			return
		}
		back.Children(func() {
			ui.Icon(c, kit.IconPrevStep).TextColor(t.TextMuted)
			ui.Text(c, leaderName).TextColor(t.TextMuted)
		})
		ui.Text(c, "/").TextColor(kit.FaintColor(t))
		ui.Text(c, "Deck art · "+ap.title).Bold()
		if !noSim {
			ui.Text(c, fmt.Sprintf("%d of %d changed in the sim", changed, len(items))).FontSize(13).TextColor(t.TextMuted)
		}
	})
	if pg.artPick == nil {
		return
	}
	if noSim {
		page.NoSimBanner(c, e)
	} else if e.Prefs.ArtOff {
		ui.Text(c, "Card art downloads are off in Settings, so only the sim's own art and your own file can be used.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
	}

	grid := kit.Panel(c).Padding(16).Gap(12)
	grid.Children(func() {
		twoCols := pg.artGridW == 0 || pg.artGridW >= deckArtTwoColMin
		if !twoCols {
			for i := range items {
				pg.deckArtBox(c.Key(items[i].card.Code), e, &items[i], false)
			}
			return
		}
		var left *deckArtItem
		flushLeft := func(key string) {
			if left == nil {
				return
			}
			ui.Row(c.Key("row-" + key)).Gap(12).AlignItems(ui.Stretch).Children(func() {
				pg.deckArtBox(c.Key(left.card.Code), e, left, true)
				ui.Box(c).Grow(1).Basis(0)
			})
			left = nil
		}
		for i := range items {
			it := &items[i]
			if it.wide {
				flushLeft(it.card.Code)
				pg.deckArtBox(c.Key(it.card.Code), e, it, false)
				continue
			}
			if left == nil {
				left = it
				continue
			}
			ui.Row(c.Key("row-" + left.card.Code)).Gap(12).AlignItems(ui.Stretch).Children(func() {
				pg.deckArtBox(c.Key(left.card.Code), e, left, true)
				pg.deckArtBox(c.Key(it.card.Code), e, it, true)
			})
			left = nil
		}
		flushLeft("end")
	})
	if b := grid.Bounds(); b.W > 0 && b.W != pg.artGridW {
		pg.artGridW = b.W
		c.Invalidate()
	}
}

func loadDeckArtItems(e *page.Env, ap *deckArtPick) (items []deckArtItem, changed int, noSim bool) {
	order := page.DeckListOrder(e, ap.list)
	items = make([]deckArtItem, 0, 1+len(order))
	add := func(code string, copies int) {
		card := e.Snap.Card(code)
		info, loaded := e.SimArt.For(e, card)
		ca, _ := e.Arts.For(e, card)
		entries := page.ArtEntries(e, card, ca, info, loaded, "")
		prints := len(ca.Arts)
		if ca.Pending > 0 {
			prints++
		}
		if loaded && info.NoSim {
			noSim = true
		} else if loaded && info.Cur.Kind != sim.ArtOwn {
			changed++
		}
		items = append(items, deckArtItem{
			card: card, copies: copies, info: info, loaded: loaded,
			ca: ca, entries: entries, wide: prints >= deckArtWideMin,
		})
	}
	if ap.leader != "" {
		add(ap.leader, 0)
	}
	for _, code := range order {
		add(code, ap.list[code])
	}
	return items, changed, noSim
}

func (pg *Page) deckArtBox(c *ui.Context, e *page.Env, it *deckArtItem, half bool) {
	t := c.Theme()
	box := ui.Column(c).Gap(10).Padding(12, 14).Radius(9).Background(t.Background).Border(1, t.Border.Alpha(0.6))
	if half {
		box.Grow(1).Basis(0).MinWidth(0)
	}
	box.Children(func() {
		deckArtHeader(c, it)
		ui.Row(c).Gap(10).Wrap().AlignItems(ui.Start).Children(func() {
			pending := it.ca.Pending > 0
			for _, en := range it.entries {
				if en.Art == nil && pending {
					page.PendingColumn(c, e, it.card, it.ca.Pending, true)
					pending = false
				}
				pg.deckArtEntry(c, e, it, en)
			}
			if pending {
				page.PendingColumn(c, e, it.card, it.ca.Pending, true)
			}
		})
		if it.info.Err != "" {
			ui.Text(c, it.info.Err).FontSize(12).TextColor(kit.LossColor(t)).Wrap()
		}
	})
}

func deckArtHeader(c *ui.Context, it *deckArtItem) {
	t := c.Theme()
	card := it.card
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Grow(1).Basis(0).MinWidth(0).Children(func() {
			ui.Text(c, card.Name).FontSize(13.5).FontWeight(600).MaxLines(1).Ellipsis("…")
			meta := card.Code + " · Leader"
			if it.copies > 0 {
				meta = fmt.Sprintf("%s · ×%d", card.Code, it.copies)
			}
			ui.Text(c, meta).FontSize(12).TextColor(kit.FaintColor(t)).SingleLine()
		})
		label, on := simArtStatus(it.ca, it.info, it.loaded)
		if label == "" {
			return
		}
		if on {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, kit.IconSim).Size(13, 13).TextColor(page.SimGold)
				ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(page.SimGold).SingleLine()
			})
			return
		}
		ui.Text(c, label).FontSize(12).TextColor(kit.FaintColor(t)).SingleLine()
	})
}

func (pg *Page) deckArtEntry(c *ui.Context, e *page.Env, it *deckArtItem, en page.ArtEntry) {
	ap := pg.artPick
	card := it.card
	if en.Art == nil {
		_, look, use := page.ArtColumn(c, e, page.ArtColumnOpts{
			Key: en.Key, Name: en.Name, Subject: card.Name, Versions: 1, InSim: en.InSim, Usable: en.Usable, Compact: true,
		})
		if look {
			e.Nav.ViewArtAt(card.Code, en.Key)
		}
		if use {
			e.SimArt.Use(e, card, en.Choice)
		}
		return
	}
	art := *en.Art
	head := art.Head().Key.String()
	col, look, use := page.ArtColumn(c, e, page.ArtColumnOpts{
		Key: head, Name: en.Name, Subject: card.Name, Versions: len(art.Versions), InSim: en.InSim, Usable: en.Usable, Compact: true,
	})
	if look {
		if len(art.Versions) > 1 {
			ap.versionsCard, ap.versionsHead = card.Code, head
		} else {
			e.Nav.ViewArtAt(card.Code, en.Key)
		}
	}
	if use {
		e.SimArt.Use(e, card, en.Choice)
	}
	if len(art.Versions) <= 1 {
		return
	}
	open := ap.versionsCard == card.Code && ap.versionsHead == head
	page.VersionPopover(c, e, col, art, it.info, en.Usable, "", &open, func(k string) {
		ap.versionsCard, ap.versionsHead = "", ""
		e.Nav.ViewArtAt(card.Code, k)
	}, func(ch sim.ArtChoice) {
		e.SimArt.Use(e, card, ch)
	})
	if !open && ap.versionsCard == card.Code && ap.versionsHead == head {
		ap.versionsCard, ap.versionsHead = "", ""
	}
}

func simArtStatus(ca arts.CardArts, info page.SimArtInfo, loaded bool) (label string, changed bool) {
	if !loaded || info.NoSim {
		return "", false
	}
	switch info.Cur.Kind {
	case sim.ArtOwn:
		return "Sim's own", false
	case sim.ArtFile:
		return "Your file", true
	default:
		if art, _, ok := ca.Find(info.Cur.Key); ok {
			return art.Name(), true
		}
		return "Other art", true
	}
}
