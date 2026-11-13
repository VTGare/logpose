package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"
)

// simArtToastTime is how long the message after a change shows, with its Undo.
const simArtToastTime = 10 * time.Second

// simArtState is what the sim shows for each card the Cards page has looked at.
type simArtState struct {
	cards   map[string]simArtInfo
	loading map[string]bool
	// changed holds the cards the sim shows art of their own for.
	changed map[string]bool
	scanned bool
}

type simArtInfo struct {
	noSim bool
	cur   sim.ArtChoice
	// ownFile is the user's own file for the card, when there's one that isn't a printing.
	ownFile string
	busy    bool
	err     string
}

// simArtFor is what the sim shows for the card. It starts reading it the first time.
func (pg *cardsPage) simArtFor(card cards.Card) (simArtInfo, bool) {
	s := &pg.simArt
	if info, ok := s.cards[card.Code]; ok {
		return info, true
	}
	if s.loading == nil {
		s.loading = make(map[string]bool)
	}
	if !s.loading[card.Code] {
		s.loading[card.Code] = true
		pg.do(func(a cardActions) { a.LoadSimArt(card) })
	}
	return simArtInfo{}, false
}

func (pg *cardsPage) setSimArt(code string, info simArtInfo) {
	s := &pg.simArt
	if s.cards == nil {
		s.cards = make(map[string]simArtInfo)
	}
	s.cards[code] = info
	delete(s.loading, code)
	if s.changed == nil {
		s.changed = make(map[string]bool)
	}
	s.changed[code] = info.cur.Kind != sim.ArtOwn
}

func (pg *cardsPage) setSimArtChanged(codes []string) {
	pg.simArt.changed = make(map[string]bool, len(codes))
	for _, code := range codes {
		pg.simArt.changed[code] = true
	}
	pg.simArt.scanned = true
}

// resetSimArt forgets what was read, for another sim folder.
func (pg *cardsPage) resetSimArt() { pg.simArt = simArtState{} }

func (pg *cardsPage) useSimArt(card cards.Card, choice sim.ArtChoice) {
	info, ok := pg.simArtFor(card)
	if !ok || info.noSim || info.busy || info.cur == choice {
		return
	}
	info.busy, info.err = true, ""
	pg.setSimArt(card.Code, info)
	prev := info.cur
	pg.do(func(a cardActions) { a.UseSimArt(card, choice, prev) })
}

// simArtDone shows the result of UseSimArt. undo is the change that takes it back.
func (pg *cardsPage) simArtDone(e *env, card cards.Card, choice sim.ArtChoice, info simArtInfo, undo sim.ArtChoice, err error) {
	if err != nil {
		info.err = "Couldn't change the sim's art: " + err.Error()
	}
	pg.setSimArt(card.Code, info)
	if err != nil {
		return
	}
	t := ui.Toast{ID: "sim-art", Timeout: simArtToastTime}
	switch choice.Kind {
	case sim.ArtOwn:
		t.Title = "The sim shows its own art for " + card.Name + " again."
	default:
		ca, _ := pg.artsFor(card)
		t.Title = "The sim now shows " + card.Name + "'s " + choiceName(ca, choice) + "."
	}
	t.Description = "Restart the sim to see it."
	t.Action = "Undo"
	t.OnAction = func() { pg.useSimArt(card, undo) }
	e.toast = &t
}

// choiceName names a choice in a sentence: "Alt art", "Base art", "own file" or "own art".
func choiceName(ca arts.CardArts, choice sim.ArtChoice) string {
	switch choice.Kind {
	case sim.ArtFile:
		return "own file"
	case sim.ArtOwn:
		return "own art"
	}
	art, _, ok := ca.Find(choice.Key)
	if !ok {
		return "art"
	}
	name := art.Name()
	if !strings.HasSuffix(name, " art") {
		name += " art"
	}
	return name
}

// artsFor is the card's arts, Limitless's alone until the gallery's first answer. It starts
// grouping them the first time.
func (pg *cardsPage) artsFor(card cards.Card) (arts.CardArts, bool) {
	s := &pg.arts
	if ca, ok := s.cards[card.Code]; ok {
		return ca, true
	}
	if s.loading == nil {
		s.loading = make(map[string]bool)
	}
	if !s.loading[card.Code] {
		s.loading[card.Code] = true
		gen := s.gen
		pg.do(func(a cardActions) { a.LoadArts(card, gen) })
	}
	return arts.LimitlessArts(card), false
}

// setArts stores a step of a card's grouping. A step from before the last reset is dropped.
func (pg *cardsPage) setArts(gen int, code string, ca arts.CardArts) {
	s := &pg.arts
	if gen != s.gen {
		return
	}
	if s.cards == nil {
		s.cards = make(map[string]arts.CardArts)
	}
	s.cards[code] = ca
}

// resetArts forgets every card's grouping, after the card data, Bandai's list or the art
// download switch changed.
func (pg *cardsPage) resetArts() { pg.arts = artsState{gen: pg.arts.gen + 1} }

// artsState is the grouping of each card the Cards page has looked at.
type artsState struct {
	// gen goes up on each reset, so a grouping that started before one is dropped.
	gen     int
	cards   map[string]arts.CardArts
	loading map[string]bool
}

// viewedKey is the art key of the picture the panel shows large: the one picked, or the
// first art's when that is gone.
func (pg *cardsPage) viewedKey(card cards.Card, info simArtInfo, ca arts.CardArts) string {
	switch k := pg.state.viewed; {
	case k == "":
	case k == simOwnRef(card.Code):
		return k
	case info.ownFile != "" && k == fileRef(info.ownFile):
		return k
	default:
		if _, ver, ok := ca.Find(k); ok {
			return ver.Key.String()
		}
	}
	return ca.Arts[0].Head().Key.String()
}

// artChoices is the card's arts with a dot under each. A picture shows large on a click, and
// its dot puts it in the sim. An art printed with several stamps lists them on a click.
func (pg *cardsPage) artChoices(c *ui.Context, e *env, card cards.Card, info simArtInfo, loaded bool) {
	s := &pg.state
	ca, _ := pg.artsFor(card)
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(10).Wrap().AlignItems(ui.Start).Children(func() {
			pending := ca.Pending > 0
			for _, en := range pg.artEntries(e, card, ca, info, loaded) {
				if en.art == nil && pending {
					pendingColumn(c, e, card, ca.Pending)
					pending = false
				}
				if en.art == nil {
					_, look, use := artColumn(c, e, en.key, en.name, 1, s.viewed == en.key, en.inSim, en.usable)
					if look {
						s.viewed = en.key
					}
					if use {
						pg.useSimArt(card, en.choice)
					}
					continue
				}
				art := *en.art
				key := art.Head().Key.String()
				col, look, use := artColumn(c, e, key, en.name, len(art.Versions), art.Has(s.viewed), en.inSim, en.usable)
				if look {
					s.viewed = key
					if len(art.Versions) > 1 {
						s.versions = key
					}
				}
				if use {
					pg.useSimArt(card, en.choice)
				}
				if len(art.Versions) > 1 {
					pg.versionList(c, e, col, card, art, info, en.usable)
				}
			}
			if pending {
				pendingColumn(c, e, card, ca.Pending)
			}
		})
		pg.artNote(c, e, info)
	})
}

// versionList opens under an art's column while it is picked, with a row per version: its
// picture, which stamp it is, its product and a dot of its own. A row's picture shows large
// on a click, so its stamp can be seen.
func (pg *cardsPage) versionList(c *ui.Context, e *env, anchor ui.Element, card cards.Card, art arts.Art, info simArtInfo, usable bool) {
	s := &pg.state
	t := c.Theme()
	head := art.Head().Key.String()
	open := s.versions == head
	ui.PopoverBase(c, anchor, &open, func(panel ui.Element) {
		popover(panel.Margin(6, 0, 0, 0).Padding(12, 14).Width(430), t)
		ui.Column(c).Gap(8).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, fmt.Sprintf("%s · %d versions", art.Name(), len(art.Versions))).FontSize(13).FontWeight(600)
				ui.Text(c, "The same art, stamped for each printing.").FontSize(12).TextColor(t.TextMuted)
			})
			shown, inArt := art.Find(info.cur.Key)
			inArt = inArt && info.cur.Kind == sim.ArtPrint
			for _, ver := range art.Versions {
				key := ver.Key.String()
				on := inArt && shown.Key == ver.Key
				ui.Row(c.Key(key)).Gap(10).AlignItems(ui.Center).Children(func() {
					row := ui.ButtonBase(c).Label("Show " + ver.Short + ", " + ver.Product).Background(ui.Transparent).Padding(2).Radius(6).
						Gap(10).AlignItems(ui.Center).Grow(1).Basis(0).MinWidth(0)
					if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					if row.Clicked() {
						s.viewed = key
					}
					row.Children(func() {
						e.cardImage(c, key, 30).Radius(6)
						ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
							col := t.Text
							if s.viewed == key {
								col = t.Accent
							}
							ui.Text(c, ver.Short).FontSize(13).FontWeight(500).TextColor(col).MaxLines(1).Ellipsis("…")
							ui.Text(c, ver.Product).FontSize(12).TextColor(t.TextMuted).MaxLines(2).Ellipsis("…")
						})
					})
					if useDot(c, ver.Short+", "+ver.Product, on, usable) {
						pg.useSimArt(card, sim.ArtChoice{Kind: sim.ArtPrint, Key: key})
					}
				})
			}
		})
	})
	if !open && s.versions == head {
		s.versions = ""
	}
}

// pendingColumn stands for the printings whose pictures are still being compared. They often
// turn out to be one art, so it is one column.
func pendingColumn(c *ui.Context, e *env, card cards.Card, n int) {
	t := c.Theme()
	const w = 60
	ui.Column(c.Key("pending")).Gap(4).Width(w).AlignItems(ui.Center).Children(func() {
		e.cardPlaceholder(c, card.Code, w, cardHeight(w)).Radius(6)
		// Room for the bar under an art's picture, so this name lines up with theirs.
		ui.Box(c).Height(3)
		ui.Text(c, fmt.Sprintf("Checking %d more", n)).FontSize(11).TextColor(t.TextMuted).Wrap().TextAlign(ui.Center)
	})
}

func (pg *cardsPage) artNote(c *ui.Context, e *env, info simArtInfo) {
	t := c.Theme()
	switch {
	case info.noSim:
		ui.Row(c).Gap(10).Padding(10, 12).Radius(10).Background(warnColor.Alpha(0.10)).Border(1, warnColor.Alpha(0.35)).
			AlignItems(ui.Start).Children(func() {
			ui.Icon(c, iconWarn).Size(16, 16).TextColor(warnColor)
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, "logpose can't find the sim, so it can't change the sim's art.").FontSize(13).Wrap()
				if ui.ButtonBase(c).Label("Choose the sim's folder").Background(ui.Transparent).Padding(0).
					Children(func() { ui.Text(c, "Choose the sim's folder").FontSize(13).TextColor(t.Accent) }).Clicked() {
					pg.do(func(a cardActions) { a.ChooseSimFolder() })
				}
			})
		})
	case info.err != "":
		ui.Text(c, info.err).FontSize(12).TextColor(lossColor(t)).Wrap()
	case e.prefs.artOff:
		ui.Text(c, "Card art downloads are off in Settings, so only the sim's own art and your own file can be used.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
	case info.cur.Kind == sim.ArtFile:
		ui.Text(c, "The sim shows a file you added yourself. If you pick other art, logpose keeps your file "+
			"and it stays here to pick again.").FontSize(12).TextColor(t.TextMuted).Wrap()
	default:
		ui.Text(c, "Click a picture to look at it. Pick a dot to use that art in the sim.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
	}
}

// artColumn is one picture with its name and dot, and a count on the picture when the art has
// several versions. It reports a click on the picture and one on the dot.
func artColumn(c *ui.Context, e *env, key, name string, versions int, viewed, on, usable bool) (col ui.Element, look, use bool) {
	t := c.Theme()
	const w = 60
	col = ui.Column(c.Key(key)).Gap(4).MinWidth(w).AlignItems(ui.Center).Children(func() {
		label := "Show " + name
		if versions > 1 {
			label = fmt.Sprintf("Show %s, %d versions", name, versions)
		}
		pic := ui.ButtonBase(c).Label(label).Background(ui.Transparent).Padding(0).FocusRing(false)
		look = pic.Clicked()
		pic.Children(func() {
			e.cardImage(c, key, w).Radius(6)
			if versions > 1 {
				ui.Box(c).Absolute().Top(-6).Right(-6).Size(20, 20).Radius(10).Background(ui.RGBA(20, 20, 22, 0.92)).
					Border(1, ui.RGBA(255, 255, 255, 0.3)).Center().Children(func() {
					ui.Text(c, fmt.Sprint(versions)).FontSize(11).FontWeight(600).TextColor(ui.Hex("#FFFFFF"))
				})
			}
		})
		bar := ui.Box(c).Height(3).Width(w).Radius(2)
		tc := t.TextMuted
		if viewed {
			bar.Background(t.Accent)
			tc = t.Text
		}
		ui.Text(c, name).FontSize(11).TextColor(tc).MaxLines(1)
		use = useDot(c, name, on, usable)
	})
	return col, look, use
}

// useDot is the dot that puts a picture in the sim, or says it's there. It reports a click.
func useDot(c *ui.Context, name string, on, usable bool) bool {
	t := c.Theme()
	label := "Use " + name + " in the sim"
	if on {
		label = name + " is in the sim"
	}
	dot := ui.ButtonBase(c).Label(label).Background(ui.Transparent).Padding(1, 4).Radius(6).Gap(5).
		AlignItems(ui.Center).Disabled(!usable || on)
	clicked := dot.Clicked()
	if dot.Hovered() && usable && !on {
		dot.Background(t.SurfaceHover)
	}
	dot.Children(func() {
		ring := t.TextMuted
		text, tc := "Use", t.TextMuted
		if on {
			ring, text, tc = simGold, "In sim", simGold
		}
		alpha := float32(1)
		if !usable && !on {
			alpha = 0.4
		}
		ui.Box(c).Size(14, 14).Radius(7).Border(1.5, ring.Alpha(alpha)).Justify(ui.Center).AlignItems(ui.Center).
			Children(func() {
				if on {
					ui.Box(c).Size(7, 7).Radius(3.5).Background(simGold)
				}
			})
		ui.Text(c, text).FontSize(11).TextColor(tc.Alpha(alpha)).FontWeight(map[bool]int{true: 600, false: 400}[on])
	})
	return clicked
}

// simGold marks the art the sim shows.
var simGold = ui.Hex("#E8B83A")

func smallButton(c *ui.Context, label string, icon *ui.SVG, text bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Height(28).Padding(0, 9).Radius(7).Gap(6).AlignItems(ui.Center).
		Background(ui.Transparent).Border(1, t.Border)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Tooltip(label)
	b.Children(func() {
		ui.Icon(c, icon).Size(14, 14)
		if text {
			ui.Text(c, label).FontSize(12.5).FontWeight(500)
		}
	})
	return b
}

func (pg *cardsPage) artButtons(c *ui.Context, key string) {
	if smallButton(c, "Save image…", iconSave, true).Clicked() {
		pg.do(func(a cardActions) { a.SaveArt(key) })
	}
	if smallButton(c, "Copy image", iconCopy, false).Clicked() {
		pg.do(func(a cardActions) { a.CopyArt(key) })
	}
}

// cardMenu is a card's right-click menu: off the Cards page a way there first, then which art
// the sim shows, saving and copying the picture, and the card's page on Limitless. An art
// printed with several stamps has a submenu of them.
func (pg *cardsPage) cardMenu(e *env, card cards.Card) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		if e.page != pageCards {
			if m.Item("Open in Cards").Chosen() {
				e.nav.openInCards(card.Code)
			}
			if e.nav.showsHands() && m.Item("Show hands with "+card.Name).Chosen() {
				e.nav.showHands(card.Code)
			}
			m.Separator()
		}
		info, loaded := pg.simArtFor(card)
		m.Submenu("Art in the sim", func(m *ui.Menu) { pg.artMenu(e, m, card, info, loaded) }).Disabled(loaded && info.noSim)
		if m.Item("View art").Chosen() {
			e.nav.viewArt(card.Code)
		}
		if m.Item("Save image…").Chosen() {
			pg.do(func(a cardActions) { a.SaveArt(card.Code) })
		}
		if m.Item("Copy image").Chosen() {
			pg.do(func(a cardActions) { a.CopyArt(card.Code) })
		}
		m.Separator()
		if m.Item("Open on Limitless").Chosen() {
			pg.do(func(a cardActions) { a.OpenURL(cards.LimitlessBase + "/cards/" + card.Code) })
		}
	}
}

// artMenu lists the card's arts to put in the sim, then the user's file and the sim's own.
func (pg *cardsPage) artMenu(e *env, m *ui.Menu, card cards.Card, info simArtInfo, loaded bool) {
	ca, _ := pg.artsFor(card)
	cur := ""
	if loaded && info.cur.Kind == sim.ArtPrint {
		cur = info.cur.Key
	}
	own := false
	for _, en := range pg.artEntries(e, card, ca, info, loaded) {
		switch {
		case en.choice.Kind == sim.ArtOwn:
			m.Separator()
			own = true
			if m.Item("The sim's own").Radio(en.inSim).Disabled(!en.usable).Chosen() {
				pg.useSimArt(card, en.choice)
			}
		case en.art == nil:
			if m.Item(en.name).Radio(en.inSim).Disabled(!en.usable).Chosen() {
				pg.useSimArt(card, en.choice)
			}
		case len(en.art.Versions) == 1:
			if m.Item(menuName(card, en.art.Head())).Radio(en.inSim).Disabled(!en.usable).Chosen() {
				pg.useSimArt(card, en.choice)
			}
		default:
			art := *en.art
			m.Submenu(fmt.Sprintf("%s · %d versions", art.Name(), len(art.Versions)), func(m *ui.Menu) {
				shown, inArt := art.Find(cur)
				for _, ver := range art.Versions {
					key := ver.Key.String()
					on := inArt && shown.Key == ver.Key
					label := ver.Product
					if label == "" {
						label = ver.Name
					}
					if m.Item(label).Radio(on).Disabled(!en.usable).Chosen() {
						pg.useSimArt(card, sim.ArtChoice{Kind: sim.ArtPrint, Key: key})
					}
				}
			}).Disabled(!en.usable)
		}
	}
	if !own {
		// Without the sim's state, the entry shows but can't be picked.
		m.Separator()
		m.Item("The sim's own").Disabled(true)
	}
}

// menuName adds the product to every version but the base card's, since a menu has no
// pictures to tell two of a kind apart.
func menuName(card cards.Card, ver arts.Version) string {
	if ver.Key.String() == card.Code || ver.Product == "" {
		return ver.Name
	}
	return ver.Name + ", " + ver.Product
}
