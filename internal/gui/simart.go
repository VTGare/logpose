package gui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"
)

// Art keys for pictures that aren't printings: the sim's own picture of a card, and a file of
// the user's by its path.
const (
	simOwnPrefix  = "sim:"
	ownFilePrefix = "file:"
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
func (v *view) simArtFor(card cards.Card) (simArtInfo, bool) {
	s := &v.simArt
	if info, ok := s.cards[card.Code]; ok {
		return info, true
	}
	if s.loading == nil {
		s.loading = make(map[string]bool)
	}
	if !s.loading[card.Code] {
		s.loading[card.Code] = true
		v.do(func(a actions) { a.LoadSimArt(card) })
	}
	return simArtInfo{}, false
}

// setSimArt stores what the sim shows for a card.
func (v *view) setSimArt(code string, info simArtInfo) {
	s := &v.simArt
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

// setSimArtChanged stores the cards the sim shows art of their own for.
func (v *view) setSimArtChanged(codes []string) {
	v.simArt.changed = make(map[string]bool, len(codes))
	for _, code := range codes {
		v.simArt.changed[code] = true
	}
	v.simArt.scanned = true
}

// resetSimArt forgets what was read, for another sim folder.
func (v *view) resetSimArt() { v.simArt = simArtState{} }

// useSimArt makes the sim show choice for the card, and says so when it's done.
func (v *view) useSimArt(card cards.Card, choice sim.ArtChoice) {
	info, ok := v.simArtFor(card)
	if !ok || info.noSim || info.busy || info.cur == choice {
		return
	}
	info.busy, info.err = true, ""
	v.setSimArt(card.Code, info)
	prev := info.cur
	v.do(func(a actions) { a.UseSimArt(card, choice, prev) })
}

// simArtDone shows the result of UseSimArt. undo is the change that takes it back, or nil.
func (v *view) simArtDone(card cards.Card, choice sim.ArtChoice, info simArtInfo, undo *sim.ArtChoice, err error) {
	v.setSimArt(card.Code, info)
	if err != nil {
		info.err = "Couldn't change the sim's art: " + err.Error()
		v.setSimArt(card.Code, info)
		return
	}
	t := ui.Toast{ID: "sim-art", Timeout: simArtToastTime}
	switch choice.Kind {
	case sim.ArtOwn:
		t.Title = "The sim shows its own art for " + card.Name + " again."
	default:
		ca, _ := v.artsFor(card)
		t.Title = "The sim now shows " + card.Name + "'s " + choiceName(ca, choice) + "."
	}
	t.Description = "Restart the sim to see it."
	if undo != nil {
		back := *undo
		t.Action = "Undo"
		t.OnAction = func() { v.useSimArt(card, back) }
	}
	v.toast = &t
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

// showToast adds the message an action left for this frame.
func (v *view) showToast(c *ui.Context) {
	if t := v.toast; t != nil {
		v.toast = nil
		c.AddToast(*t)
	}
}

// artsFor is the card's arts, Limitless's alone until the gallery's first answer. It starts
// grouping them the first time.
func (v *view) artsFor(card cards.Card) (arts.CardArts, bool) {
	s := &v.arts
	if ca, ok := s.cards[card.Code]; ok {
		return ca, true
	}
	if s.loading == nil {
		s.loading = make(map[string]bool)
	}
	if !s.loading[card.Code] {
		s.loading[card.Code] = true
		gen := s.gen
		v.do(func(a actions) { a.LoadArts(card, gen) })
	}
	return arts.LimitlessArts(card), false
}

// setArts stores a step of a card's grouping. A step from before the last reset is dropped.
func (v *view) setArts(gen int, code string, ca arts.CardArts) {
	s := &v.arts
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
func (v *view) resetArts() { v.arts = artsState{gen: v.arts.gen + 1} }

// artsState is the grouping of each card the Cards page has looked at.
type artsState struct {
	// gen goes up on each reset, so a grouping that started before one is dropped.
	gen     int
	cards   map[string]arts.CardArts
	loading map[string]bool
}

// viewedKey is the art key of the picture the panel shows large: the one picked, or the
// first art's when that is gone.
func (v *view) viewedKey(card cards.Card, info simArtInfo, ca arts.CardArts) string {
	switch k := v.cardsUI.viewed; {
	case k == "":
	case k == simOwnPrefix+card.Code:
		return k
	case info.ownFile != "" && k == ownFilePrefix+info.ownFile:
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
func (v *view) artChoices(c *ui.Context, card cards.Card, info simArtInfo, loaded bool) {
	s := &v.cardsUI
	ca, _ := v.artsFor(card)
	usable := loaded && !info.noSim && !info.busy
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(10).Wrap().AlignItems(ui.Start).Children(func() {
			for _, art := range ca.Arts {
				key := art.Head().Key.String()
				on := loaded && info.cur.Kind == sim.ArtPrint && art.Has(info.cur.Key)
				col, look, use := v.artColumn(c, key, art.Name(), len(art.Versions), art.Has(s.viewed), on, usable && !v.prefs.artOff)
				if look {
					s.viewed = key
					if len(art.Versions) > 1 {
						s.versions = key
					}
				}
				if use {
					v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtPrint, Key: key})
				}
				if len(art.Versions) > 1 {
					v.versionList(c, col, card, art, info, usable && !v.prefs.artOff)
				}
			}
			if ca.Pending > 0 {
				pendingColumn(c, card, ca.Pending)
			}
			if info.ownFile != "" {
				key := ownFilePrefix + info.ownFile
				on := info.cur.Kind == sim.ArtFile
				_, look, use := v.artColumn(c, key, "Your file", 1, s.viewed == key, on, usable)
				if look {
					s.viewed = key
				}
				if use {
					v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtFile})
				}
			}
			if loaded && !info.noSim {
				key := simOwnPrefix + card.Code
				on := info.cur.Kind == sim.ArtOwn
				_, look, use := v.artColumn(c, key, "Sim's own", 1, s.viewed == key, on, usable)
				if look {
					s.viewed = key
				}
				if use {
					v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtOwn})
				}
			}
		})
		v.artNote(c, info)
	})
}

// versionList opens under an art's column while it is picked, with a row per version: its
// picture, which stamp it is, its product and a dot of its own. A row's picture shows large
// on a click, so its stamp can be seen.
func (v *view) versionList(c *ui.Context, anchor ui.Element, card cards.Card, art arts.Art, info simArtInfo, usable bool) {
	s := &v.cardsUI
	t := c.Theme()
	head := art.Head().Key.String()
	open := s.versions == head
	ui.PopoverBase(c, anchor, &open, func(panel ui.Element) {
		panel.Margin(6, 0, 0, 0).Padding(12, 14).Radius(10).Background(t.Background).Border(1, t.Border).Width(430).
			Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.45))
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
						v.printArt(c, key, 30, 42)
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
						v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtPrint, Key: key})
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
func pendingColumn(c *ui.Context, card cards.Card, n int) {
	t := c.Theme()
	const w = 60
	ui.Column(c.Key("pending")).Gap(4).Width(w).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(w, w*7/5).Radius(6).Background(colorsOf(card)[0].Card.Alpha(0.35))
		// Room for the bar under an art's picture, so this name lines up with theirs.
		ui.Box(c).Height(3)
		ui.Text(c, fmt.Sprintf("Checking %d more", n)).FontSize(11).TextColor(t.TextMuted).Wrap().TextAlign(ui.Center)
	})
}

// artNote says how the dots work, or why they can't be used.
func (v *view) artNote(c *ui.Context, info simArtInfo) {
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
					v.do(func(a actions) { a.ChooseSimFolder() })
				}
			})
		})
	case info.err != "":
		ui.Text(c, info.err).FontSize(12).TextColor(lossColor).Wrap()
	case v.prefs.artOff:
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
func (v *view) artColumn(c *ui.Context, key, name string, versions int, viewed, on, usable bool) (col ui.Element, look, use bool) {
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
			v.printArt(c, key, w, w*7/5)
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

// artButtons save or copy the picture the panel shows large.
func (v *view) artButtons(c *ui.Context, key string) {
	if smallButton(c, "Save image…", iconSave, true).Clicked() {
		v.do(func(a actions) { a.SaveArt(key) })
	}
	if smallButton(c, "Copy image", iconCopy, false).Clicked() {
		v.do(func(a actions) { a.CopyArt(key) })
	}
}

// cardMenu is a card's right-click menu: which art the sim shows, saving and copying the
// picture, and the card's page on Limitless. An art printed with several stamps has a submenu
// of them.
func (v *view) cardMenu(card cards.Card) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		info, loaded := v.simArtFor(card)
		m.Submenu("Art in the sim", func(m *ui.Menu) { v.artMenu(m, card, info, loaded) }).Disabled(loaded && info.noSim)
		m.Separator()
		if m.Item("Save image…").Chosen() {
			v.do(func(a actions) { a.SaveArt(card.Code) })
		}
		if m.Item("Copy image").Chosen() {
			v.do(func(a actions) { a.CopyArt(card.Code) })
		}
		m.Separator()
		if m.Item("Open on Limitless").Chosen() {
			v.do(func(a actions) { a.OpenURL(cards.LimitlessBase + "/cards/" + card.Code) })
		}
	}
}

// artMenu lists the card's arts to put in the sim, then the user's file and the sim's own.
func (v *view) artMenu(m *ui.Menu, card cards.Card, info simArtInfo, loaded bool) {
	ca, _ := v.artsFor(card)
	usable := loaded && !info.noSim && !info.busy
	off := !usable || v.prefs.artOff
	cur := ""
	if loaded && info.cur.Kind == sim.ArtPrint {
		cur = info.cur.Key
	}
	for _, art := range ca.Arts {
		if len(art.Versions) == 1 {
			if m.Item(menuName(card, art.Head())).Radio(art.Has(cur)).Disabled(off).Chosen() {
				v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtPrint, Key: art.Head().Key.String()})
			}
			continue
		}
		m.Submenu(fmt.Sprintf("%s · %d versions", art.Name(), len(art.Versions)), func(m *ui.Menu) {
			shown, inArt := art.Find(cur)
			for _, ver := range art.Versions {
				key := ver.Key.String()
				on := inArt && shown.Key == ver.Key
				label := ver.Product
				if label == "" {
					label = ver.Name
				}
				if m.Item(label).Radio(on).Disabled(off).Chosen() {
					v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtPrint, Key: key})
				}
			}
		}).Disabled(off)
	}
	if info.ownFile != "" && m.Item("Your file").Radio(info.cur.Kind == sim.ArtFile).Disabled(!usable).Chosen() {
		v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtFile})
	}
	m.Separator()
	if m.Item("The sim's own").Radio(loaded && info.cur.Kind == sim.ArtOwn).Disabled(!usable).Chosen() {
		v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtOwn})
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

// The app's side of the sim's art.

func (a *app) simArt() *sim.SimArt {
	in := a.install()
	if in == nil {
		return nil
	}
	a.simArtMu.Lock()
	defer a.simArtMu.Unlock()
	if a.simArtObj == nil || a.simArtObj.Install.Root != in.Root {
		a.simArtObj = &sim.SimArt{Install: *in, Dir: filepath.Join(filepath.Dir(a.opts.DBPath), "sim-art")}
	}
	return a.simArtObj
}

// readSimArt is what the sim shows for the card. A file of the user's is compared with the
// card's printings once, so one that is a printing shows as that printing.
func (a *app) readSimArt(ctx context.Context, card cards.Card) simArtInfo {
	art := a.simArt()
	if art == nil {
		return simArtInfo{noSim: true}
	}
	cur, err := art.Current(card.Code)
	if err != nil {
		return simArtInfo{err: "Couldn't read the sim's art: " + err.Error()}
	}
	if cur.Kind == sim.ArtFile && a.firstMatch(card.Code) {
		if img, err := decode(art.Install.Override(card.Code)); err == nil {
			if key, ok := a.gallery.Match(ctx, card, img); ok && art.Matched(card.Code, key.String()) == nil {
				cur = sim.ArtChoice{Kind: sim.ArtPrint, Key: key.String()}
			}
		}
	}
	return simArtInfo{cur: cur, ownFile: art.OwnFile(card.Code)}
}

// firstMatch reports whether the card's file hasn't been compared yet this run.
func (a *app) firstMatch(code string) bool {
	a.simArtMu.Lock()
	defer a.simArtMu.Unlock()
	if a.matched == nil {
		a.matched = make(map[string]bool)
	}
	first := !a.matched[code]
	a.matched[code] = true
	return first
}

func (x *appActions) LoadSimArt(card cards.Card) {
	go func() {
		info := x.a.readSimArt(x.ctx, card)
		x.a.win.Update(func() { x.a.view.setSimArt(card.Code, info) })
	}()
}

func (x *appActions) LoadArts(card cards.Card, gen int) {
	go x.a.gallery.Arts(x.ctx, card, func(ca arts.CardArts) {
		x.a.win.Update(func() { x.a.view.setArts(gen, card.Code, ca) })
	})
}

func (x *appActions) ScanSimArt() {
	go func() {
		var codes []string
		if in := x.a.install(); in != nil {
			codes = in.Overrides()
		}
		x.a.win.Update(func() { x.a.view.setSimArtChanged(codes) })
	}()
}

func (x *appActions) UseSimArt(card cards.Card, choice, prev sim.ArtChoice) {
	go func() {
		err := x.a.useSimArt(card, choice)
		info := x.a.readSimArt(x.ctx, card)
		if err != nil {
			x.a.opts.Log.Warn("sim art", "card", card.Code, "err", err)
		}
		undo := &prev
		x.a.win.Update(func() { x.a.view.simArtDone(card, choice, info, undo, err) })
	}()
}

func (a *app) useSimArt(card cards.Card, choice sim.ArtChoice) error {
	art := a.simArt()
	if art == nil {
		return sim.ErrNotFound
	}
	switch choice.Kind {
	case sim.ArtOwn:
		return art.UseOwn(card.Code)
	case sim.ArtFile:
		return art.UseFile(card.Code)
	}
	if a.web == nil {
		return errNoWebArt
	}
	img, err := a.web.full(choice.Key)
	if err != nil {
		return fmt.Errorf("download %s: %w", choice.Key, err)
	}
	return art.UsePrint(card.Code, choice.Key, img)
}

// artImage is the full picture an art key names.
func (a *app) artImage(key string) (image.Image, error) {
	if code, ok := strings.CutPrefix(key, simOwnPrefix); ok {
		in := a.install()
		if in == nil {
			return nil, sim.ErrNotFound
		}
		return decode(in.CardImage(code, false))
	}
	if path, ok := strings.CutPrefix(key, ownFilePrefix); ok {
		return decode(path)
	}
	if a.web == nil {
		return nil, errNoWebArt
	}
	return a.web.full(key)
}

// artFileName is the name a saved picture gets: the printing's key, or the card's code.
func artFileName(key string) string {
	switch {
	case strings.HasPrefix(key, simOwnPrefix):
		return strings.TrimPrefix(key, simOwnPrefix) + "_sim.png"
	case strings.HasPrefix(key, ownFilePrefix):
		base := filepath.Base(strings.TrimPrefix(key, ownFilePrefix))
		return strings.TrimSuffix(base, filepath.Ext(base)) + ".png"
	}
	return key + ".png"
}

func (x *appActions) SaveArt(key string) {
	go func() {
		img, err := x.a.artImage(key)
		if err != nil {
			x.toast(ui.Toast{Title: "Couldn't get that picture.", Description: err.Error(), Type: "error"})
			return
		}
		dir, _ := os.UserHomeDir()
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent: x.a.win, Title: "Save the card's picture", ButtonLabel: "Save",
			DefaultPath: filepath.Join(dir, "Pictures", artFileName(key)),
			Filters:     []mygo.FileFilter{{Name: "PNG image", Extensions: []string{"png"}}},
		})
		if err != nil || path == "" {
			return
		}
		if !strings.EqualFold(filepath.Ext(path), ".png") {
			path += ".png"
		}
		if err := savePNG(path, img); err != nil {
			x.toast(ui.Toast{Title: "Couldn't save the picture.", Description: err.Error(), Type: "error"})
			return
		}
		x.toast(ui.Toast{Title: "Saved " + filepath.Base(path) + ".", Action: "Show", OnAction: func() { x.ShowFile(path) }})
	}()
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return errors.Join(png.Encode(f, img), f.Close())
}

func (x *appActions) CopyArt(key string) {
	go func() {
		img, err := x.a.artImage(key)
		var buf bytes.Buffer
		if err == nil {
			err = png.Encode(&buf, img)
		}
		if err == nil {
			err = mygo.Clipboard.Write(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, buf.Bytes()))))
		}
		if err != nil {
			x.toast(ui.Toast{Title: "Couldn't copy the picture.", Description: err.Error(), Type: "error"})
			return
		}
		x.toast(ui.Toast{Title: "Copied the picture."})
	}()
}

func (x *appActions) OpenURL(url string) {
	go func() {
		if err := mygo.Shell.OpenExternal(url); err != nil {
			x.a.opts.Log.Warn("open url", "err", err)
		}
	}()
}

func (x *appActions) toast(t ui.Toast) {
	x.a.win.Update(func() { x.a.view.toast = &t })
}
