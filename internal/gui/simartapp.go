package gui

import (
	"bytes"
	"errors"
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
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/sim"
)

func simArtInfo(st arts.SimStatus) page.SimArtInfo {
	if st.Err != nil {
		return page.SimArtInfo{Err: "Couldn't read the sim's art: " + st.Err.Error()}
	}
	return page.SimArtInfo{NoSim: st.NoSim, Cur: st.Cur, OwnFile: st.OwnFile}
}

func (x *appActions) LoadSimArt(card cards.Card) {
	go func() {
		info := simArtInfo(x.a.simArt.Inspect(x.ctx, card))
		x.a.win.Update(func() { x.a.view.SimArt.Set(card.Code, info) })
	}()
}

func (x *appActions) LoadArts(card cards.Card, gen int) {
	go x.a.gallery.Arts(x.ctx, card, func(ca arts.CardArts) {
		x.a.win.Update(func() { x.a.view.Arts.Set(gen, card.Code, ca) })
	})
}

func (x *appActions) ScanSimArt() {
	go func() {
		codes := x.a.simArt.Scan()
		x.a.win.Update(func() { x.a.view.SimArt.SetChanged(codes) })
	}()
}

func (x *appActions) UseSimArt(card cards.Card, choice, prev sim.ArtChoice) {
	go func() {
		st, err := x.a.simArt.Use(x.ctx, card, choice)
		if err != nil {
			x.a.opts.Log.Warn("sim art", "card", card.Code, "err", err)
		}
		info := simArtInfo(st)
		x.a.win.Update(func() { x.a.view.SimArt.Done(&x.a.view.Env, card, choice, info, prev, err) })
	}()
}

func (a *app) artImage(key string) (image.Image, error) {
	return a.simArt.Resolve(arts.ParseRef(key), arts.SizeFull, false, false)
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
			DefaultPath: filepath.Join(dir, "Pictures", arts.ParseRef(key).FileName()),
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

func (x *appActions) CopyText(text string, toast ui.Toast) {
	go func() {
		if err := mygo.Clipboard.Write(transfer.TextData(text)); err != nil {
			x.toast(ui.Toast{Title: "Couldn't copy to the clipboard.", Description: err.Error(), Type: "error"})
			return
		}
		if toast.Title != "" {
			x.toast(toast)
		}
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
	x.a.win.Update(func() { x.a.view.Toast = &t })
}
