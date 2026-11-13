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

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"
)

func (a *app) simArt() *sim.SimArt {
	in := a.install()
	if in == nil {
		return nil
	}
	a.simArtMu.Lock()
	defer a.simArtMu.Unlock()
	if a.simArtObj == nil || a.simArtObj.Install.Root != in.Root {
		a.simArtObj = &sim.SimArt{Install: *in, Dir: a.opts.SimArtDir}
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
		x.a.win.Update(func() { x.a.view.cards.setSimArt(card.Code, info) })
	}()
}

func (x *appActions) LoadArts(card cards.Card, gen int) {
	go x.a.gallery.Arts(x.ctx, card, func(ca arts.CardArts) {
		x.a.win.Update(func() { x.a.view.cards.setArts(gen, card.Code, ca) })
	})
}

func (x *appActions) ScanSimArt() {
	go func() {
		var codes []string
		if in := x.a.install(); in != nil {
			codes = in.Overrides()
		}
		x.a.win.Update(func() { x.a.view.cards.setSimArtChanged(codes) })
	}()
}

func (x *appActions) UseSimArt(card cards.Card, choice, prev sim.ArtChoice) {
	go func() {
		err := x.a.useSimArt(card, choice)
		info := x.a.readSimArt(x.ctx, card)
		if err != nil {
			x.a.opts.Log.Warn("sim art", "card", card.Code, "err", err)
		}
		x.a.win.Update(func() { x.a.view.cards.simArtDone(&x.a.view.env, card, choice, info, prev, err) })
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
		return arts.ErrNoPicture
	}
	img, err := artRef{refPrint, choice.Key}.read(nil, a.web, artLarge, false)
	if err != nil {
		return fmt.Errorf("download %s: %w", choice.Key, err)
	}
	return art.UsePrint(card.Code, choice.Key, img)
}

func (a *app) artImage(key string) (image.Image, error) {
	return parseArtRef(key).read(a.install(), a.web, artLarge, false)
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
			DefaultPath: filepath.Join(dir, "Pictures", parseArtRef(key).fileName()),
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
	x.a.win.Update(func() { x.a.view.queueToast(t) })
}
