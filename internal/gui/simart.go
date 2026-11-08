package gui

import (
	"bytes"
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

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"
)

// Art keys for pictures that aren't printings: the sim's own picture of a card, and a file of
// the user's by its path.
const (
	simOwnPrefix  = "sim:"
	ownFilePrefix = "file:"
)

// The panel shows a printing by its index in the card's printings, or one of these.
const (
	viewOwnFile = -1
	viewSimOwn  = -2
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
	// toast is a message for the next frame to show.
	toast *ui.Toast
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
		t.Title = "The sim now shows " + card.Name + "'s " + choiceName(card, choice) + "."
	}
	t.Description = "Restart the sim to see it."
	if undo != nil {
		back := *undo
		t.Action = "Undo"
		t.OnAction = func() { v.useSimArt(card, back) }
	}
	v.simArt.toast = &t
}

// choiceName names a choice in a sentence: "Alt art", "Base art", "own file" or "own art".
func choiceName(card cards.Card, choice sim.ArtChoice) string {
	switch choice.Kind {
	case sim.ArtFile:
		return "own file"
	case sim.ArtOwn:
		return "own art"
	}
	for _, p := range card.Prints {
		if p.Key != choice.Key {
			continue
		}
		name := printName(card, p)
		if !strings.HasSuffix(name, " art") {
			name += " art"
		}
		return name
	}
	return "art"
}

// showToast adds the message an action left for this frame.
func (v *view) showToast(c *ui.Context) {
	if t := v.simArt.toast; t != nil {
		v.simArt.toast = nil
		c.AddToast(*t)
	}
}

// printsOf is the card's printings, or its code alone when Limitless has none for it.
func printsOf(card cards.Card) []cards.Print {
	if len(card.Prints) > 0 {
		return card.Prints
	}
	return []cards.Print{{Key: card.Code}}
}

// viewedKey is the art key of the picture the panel shows large.
func (v *view) viewedKey(card cards.Card, info simArtInfo) string {
	switch i := v.cardsUI.print; {
	case i == viewOwnFile && info.ownFile != "":
		return ownFilePrefix + info.ownFile
	case i == viewSimOwn:
		return simOwnPrefix + card.Code
	case i >= 0 && i < len(printsOf(card)):
		return printsOf(card)[i].Key
	}
	v.cardsUI.print = 0
	return printsOf(card)[0].Key
}

// artChoices is the card's pictures with a dot under each. A picture shows large on a click,
// and its dot puts it in the sim.
func (v *view) artChoices(c *ui.Context, card cards.Card, info simArtInfo, loaded bool) {
	s := &v.cardsUI
	usable := loaded && !info.noSim && !info.busy
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			for i, p := range printsOf(card) {
				on := loaded && info.cur.Kind == sim.ArtPrint && info.cur.Key == p.Key
				look, use := v.artColumn(c, p.Key, printName(card, p), s.print == i, on, usable)
				if look {
					s.print = i
				}
				if use {
					v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtPrint, Key: p.Key})
				}
			}
			if info.ownFile != "" {
				on := info.cur.Kind == sim.ArtFile
				look, use := v.artColumn(c, ownFilePrefix+info.ownFile, "Your file", s.print == viewOwnFile, on, usable)
				if look {
					s.print = viewOwnFile
				}
				if use {
					v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtFile})
				}
			}
			if loaded && !info.noSim {
				on := info.cur.Kind == sim.ArtOwn
				look, use := v.artColumn(c, simOwnPrefix+card.Code, "Sim's own", s.print == viewSimOwn, on, usable)
				if look {
					s.print = viewSimOwn
				}
				if use {
					v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtOwn})
				}
			}
		})
		v.artNote(c, info)
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
	case info.cur.Kind == sim.ArtFile:
		ui.Text(c, "The sim shows a file you added yourself. If you pick other art, logpose keeps your file "+
			"and it stays here to pick again.").FontSize(12).TextColor(t.TextMuted).Wrap()
	default:
		ui.Text(c, "Click a picture to look at it. Pick a dot to use that art in the sim.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
	}
}

// artColumn is one picture with its name and dot. It reports a click on the picture and one
// on the dot.
func (v *view) artColumn(c *ui.Context, key, name string, viewed, on, usable bool) (look, use bool) {
	t := c.Theme()
	const w = 60
	ui.Column(c.Key(key)).Gap(4).Width(w).AlignItems(ui.Center).Children(func() {
		pic := ui.ButtonBase(c).Label("Show " + name).Background(ui.Transparent).Padding(0).FocusRing(false)
		look = pic.Clicked()
		pic.Children(func() { v.printArt(c, key, w, w*7/5) })
		bar := ui.Box(c).Height(3).Width(w).Radius(2)
		col := t.TextMuted
		if viewed {
			bar.Background(t.Accent)
			col = t.Text
		}
		ui.Text(c, name).FontSize(11).TextColor(col).MaxLines(1)
		label := "Use " + name + " in the sim"
		if on {
			label = name + " is in the sim"
		}
		dot := ui.ButtonBase(c).Label(label).Background(ui.Transparent).Padding(1, 4).Radius(6).Gap(5).
			AlignItems(ui.Center).Disabled(!usable || on)
		use = dot.Clicked()
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
	})
	return look, use
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
// picture, and the card's page on Limitless.
func (v *view) cardMenu(card cards.Card) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		info, loaded := v.simArtFor(card)
		usable := loaded && !info.noSim && !info.busy
		m.Submenu("Art in the sim", func(m *ui.Menu) {
			for _, p := range printsOf(card) {
				on := info.cur.Kind == sim.ArtPrint && info.cur.Key == p.Key
				if m.Item(menuPrintName(card, p)).Radio(on).Disabled(!usable).Chosen() {
					v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtPrint, Key: p.Key})
				}
			}
			if info.ownFile != "" && m.Item("Your file").Radio(info.cur.Kind == sim.ArtFile).Disabled(!usable).Chosen() {
				v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtFile})
			}
			m.Separator()
			if m.Item("The sim's own").Radio(loaded && info.cur.Kind == sim.ArtOwn).Disabled(!usable).Chosen() {
				v.useSimArt(card, sim.ArtChoice{Kind: sim.ArtOwn})
			}
		}).Disabled(loaded && info.noSim)
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

// menuPrintName adds the set to every printing but the base card's, since a menu has no
// pictures to tell two of a kind apart.
func menuPrintName(card cards.Card, p cards.Print) string {
	name := printName(card, p)
	if p.Key != card.Code && p.Set != "" {
		return name + ", " + p.Set
	}
	return name
}

// The app's side of the sim's art.

// artMatchMax is how far apart, as a mean squared difference of 0–255 channels on a 16×22
// grid, two images of the same printing may be. The same printing from two sites measured
// under 500, and the closest different printings over 1,600.
const artMatchMax = 800

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

func (a *app) webImages() *webArt {
	a.simArtMu.Lock()
	defer a.simArtMu.Unlock()
	if a.web == nil {
		a.web = newWebArt(a.opts.ArtDir)
	}
	return a.web
}

// readSimArt is what the sim shows for the card. A file of the user's is compared with the
// card's printings once, so one that is a printing shows as that printing.
func (a *app) readSimArt(card cards.Card) simArtInfo {
	art := a.simArt()
	if art == nil {
		return simArtInfo{noSim: true}
	}
	cur, err := art.Current(card.Code)
	if err != nil {
		return simArtInfo{err: "Couldn't read the sim's art: " + err.Error()}
	}
	if cur.Kind == sim.ArtFile && a.firstMatch(card.Code) {
		if key := a.matchPrint(art.Install.Override(card.Code), card); key != "" {
			if err := art.Matched(card.Code, key); err == nil {
				cur = sim.ArtChoice{Kind: sim.ArtPrint, Key: key}
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

// matchPrint is the printing the image at path shows, or "".
func (a *app) matchPrint(path string, card cards.Card) string {
	web := a.webImages()
	if web == nil || path == "" {
		return ""
	}
	img, err := decode(path)
	if err != nil {
		return ""
	}
	want := artGrid(img)
	for _, p := range printsOf(card) {
		other, err := web.image(p.Key)
		if err == nil && gridDistance(want, artGrid(other)) <= artMatchMax {
			return p.Key
		}
	}
	return ""
}

// artGrid samples an image on a 16×22 grid, about a card's shape.
func artGrid(img image.Image) []float64 {
	b := img.Bounds()
	out := make([]float64, 0, 16*22*3)
	for y := range 22 {
		for x := range 16 {
			r, g, bl, _ := img.At(b.Min.X+(x*2+1)*b.Dx()/32, b.Min.Y+(y*2+1)*b.Dy()/44).RGBA()
			out = append(out, float64(r>>8), float64(g>>8), float64(bl>>8))
		}
	}
	return out
}

func gridDistance(a, b []float64) float64 {
	sum := 0.0
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return sum / float64(len(a))
}

func (x *appActions) LoadSimArt(card cards.Card) {
	go func() {
		info := x.a.readSimArt(card)
		x.a.win.Update(func() { x.a.view.setSimArt(card.Code, info) })
	}()
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
		info := x.a.readSimArt(card)
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
	web := a.webImages()
	if web == nil {
		return errNoWebArt
	}
	img, err := web.image(choice.Key)
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
	web := a.webImages()
	if web == nil {
		return nil, errNoWebArt
	}
	return web.image(key)
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
	x.a.win.Update(func() { x.a.view.simArt.toast = &t })
}
