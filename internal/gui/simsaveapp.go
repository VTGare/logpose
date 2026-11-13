package gui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/gui/kit"
	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/simsave"
)

func (a *app) simSavesDefault() string {
	return filepath.Join(filepath.Dir(a.opts.DBPath), "sim-saves")
}

func (a *app) simSavesDir() string {
	if dir := a.applied().SimSaves; dir != "" {
		return dir
	}
	return a.simSavesDefault()
}

func (x *appActions) ExportToSim(fingerprint, name string, g *simlog.Game) {
	go func() {
		path, res, err := x.a.exportToSim(x, fingerprint, name, g)
		if err != nil {
			x.a.opts.Log.Error("export to sim", "err", err)
			x.toast(ui.Toast{Title: "Couldn't export the match.", Description: err.Error(), Type: "error"})
			return
		}
		desc := "Open it with LoadState From File in the sim"
		if res.Left > 0 {
			desc = fmt.Sprintf("%s. The deck is %d %s short, since the log never named %s.", desc,
				res.Left, kit.Plural(res.Left, "card", "cards"), kit.Plural(res.Left, "it", "them"))
		}
		x.toast(ui.Toast{
			Title: "Exported to sim", Description: desc,
			Action: "Show", OnAction: func() { x.ShowFile(path) },
		})
	}()
}

func (a *app) exportToSim(x *appActions, fingerprint, name string, g *simlog.Game) (string, simsave.Result, error) {
	var res simsave.Result
	if g == nil {
		var err error
		if g, err = x.parseGame(fingerprint); err != nil {
			return "", res, err
		}
	}
	db := a.cardDB().DB
	r, err := replay.Build(g, func(code string) int { return db.Card(code).Power })
	if err != nil {
		return "", res, err
	}
	info := func(code string) (int, int) {
		c := db.Card(code)
		return c.Power, c.Life
	}
	lists := func(player, leader string) map[string]int {
		l, err := a.opts.Store.MostCopies(x.ctx, player, leader)
		if err != nil {
			a.opts.Log.Warn("read other deck lists", "err", err)
		}
		return l
	}
	data, res, err := simsave.Export(r, info, lists)
	if err != nil {
		return "", res, err
	}
	if res.Guessed > 0 || res.Left > 0 {
		a.opts.Log.Info("export to sim named unnamed deck cards", "game", fingerprint, "guessed", res.Guessed, "left out", res.Left)
	}
	dir := a.simSavesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", res, err
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", res, fmt.Errorf("write %s: %w", name, err)
	}
	return path, res, nil
}

func (x *appActions) ChooseFolder(title, start string, picked func(path string)) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: x.a.win, Title: title, DefaultPath: start, Directory: true})
		if err != nil || len(paths) == 0 {
			return
		}
		x.a.win.Update(func() { picked(paths[0]) })
	}()
}
