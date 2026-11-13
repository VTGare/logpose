package gui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/gui/settings"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo"

	"github.com/VTGare/logpose/internal/importer"
	"github.com/VTGare/logpose/internal/sim"
	"github.com/VTGare/logpose/internal/store"
)

func (x *appActions) LoadSettings() {
	go func() {
		info, err := x.a.settingsInfo(x.ctx)
		if err != nil {
			x.a.opts.Log.Error("settings", "err", err)
			return
		}
		x.a.win.Update(func() { x.a.view.setSettingsInfo(info) })
	}()
}

func (a *app) settingsInfo(ctx context.Context) (settings.Info, error) {
	st := a.opts.Store
	players, err := st.PlayerCounts(ctx)
	if err != nil {
		return settings.Info{}, err
	}
	you, err := st.Setting(ctx, store.SettingYou)
	if err != nil {
		return settings.Info{}, err
	}
	chosen, err := st.Setting(ctx, keySimDirChosen)
	if err != nil {
		return settings.Info{}, err
	}
	l := a.cardDB()
	fetched := l.Fetched
	if !l.Checked.IsZero() {
		fetched = l.Checked
	}
	info := settings.Info{
		SimFlag:  a.opts.SimFlag,
		Fetched:  fetched,
		DBPath:   a.opts.DBPath,
		ArtDir:   a.opts.ArtDir,
		DBFolder: shortFolder(filepath.Dir(a.opts.DBPath)),
		DBSize:   st.Size(),
		Players:  players,
		You:      store.SplitNames(you),
	}
	if in := a.install(); in != nil {
		info.SimDir = in.Root
		info.SimChosen = chosen == in.Root
		logged, err := st.LatestSimVersion(ctx)
		if err != nil {
			return settings.Info{}, err
		}
		info.SimVersion = sim.Newer(sim.Version(), logged)
		info.SimExe = in.HasExe()
		info.Logs = in.LogCount()
	}
	return info, nil
}

// shortFolder is dir without the user's home folder in front, such as AppData\Roaming\logpose.
func shortFolder(dir string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return dir
	}
	if rel, err := filepath.Rel(home, dir); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return dir
}

func (x *appActions) SetSetting(key, value string) {
	x.run("save setting", func(ctx context.Context, st *store.Store) error { return st.SetSetting(ctx, key, value) })
}

func (x *appActions) ApplyPrefs(p page.Prefs) { x.a.applyPrefs(x.ctx, p) }

// applyPrefs makes the app follow the prefs it acts on, from what it followed before. A change
// to art downloads groups every card again, since pictures that couldn't be read while
// downloads were off can be now, and turning them on reads Bandai's card list if it is stale.
func (a *app) applyPrefs(ctx context.Context, p page.Prefs) {
	a.prefsMu.Lock()
	was := a.prefs
	a.prefs = p
	a.prefsMu.Unlock()
	web := a.web
	if p.ArtOff != was.ArtOff {
		if web != nil {
			web.SetOff(p.ArtOff)
			a.resetArtCache()
		}
		a.win.Update(func() { a.view.cards.ResetArts() })
		if !p.ArtOff {
			go func() { _ = a.refreshCatalog(ctx, false) }()
		}
	}
	if p.ArtCap != was.ArtCap && web != nil {
		web.SetLimit(int64(p.ArtCap) << 20)
		go func() {
			web.Trim()
			a.loadArtUsage()
		}()
	}
}

func (a *app) applied() page.Prefs {
	a.prefsMu.Lock()
	defer a.prefsMu.Unlock()
	return a.prefs
}

func (x *appActions) SetOpenAtLogin(on bool) {
	go func() {
		err := mygo.App.SetOpenAtLogin(on)
		if err != nil {
			x.a.opts.Log.Warn("start with Windows", "err", err)
		}
		now := mygo.App.OpenAtLogin()
		x.a.win.Update(func() {
			x.a.view.setOpenAtLogin(now)
			if err != nil {
				x.a.view.settings.Note(settings.NoteStartup, "Windows didn't take the change.")
			}
		})
	}()
}

func (x *appActions) note(part, text string) {
	x.a.win.Update(func() { x.a.view.settings.Note(part, text) })
}

func (x *appActions) ChooseSimFolder() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: x.a.win, Title: "Choose OPTCGSim's folder", Directory: true})
		if err != nil || len(paths) == 0 {
			return
		}
		in, err := sim.Open(paths[0])
		if err != nil {
			x.note(settings.NoteSim, "That folder isn't OPTCGSim's. Pick the one with OPTCGSim.exe in it.")
			return
		}
		x.note(settings.NoteSim, "")
		if err := x.a.setInstall(x.ctx, in, true); err != nil {
			x.a.opts.Log.Error("choose sim", "err", err)
		}
	}()
}

func (x *appActions) FindSim() {
	go func() {
		if err := forgetChosenSim(x.ctx, x.a.opts.Store); err != nil {
			x.a.opts.Log.Error("forget sim folder", "err", err)
			return
		}
		in, err := sim.Locate()
		if err != nil {
			x.note(settings.NoteSim, "The sim isn't running, so the chosen folder stays until it is.")
			x.a.refresh(x.ctx)
			return
		}
		if err := x.a.setInstall(x.ctx, in, false); err != nil {
			x.a.opts.Log.Error("find sim", "err", err)
		}
	}()
}

func (x *appActions) OpenPath(path string) {
	go func() {
		if err := mygo.Shell.OpenPath(path); err != nil {
			x.a.opts.Log.Warn("open folder", "err", err)
		}
	}()
}

func (*appActions) ShowFile(path string) { go mygo.Shell.ShowItemInFolder(path) }

func (x *appActions) ImportAgain() {
	in := x.a.install()
	if in == nil {
		return
	}
	go func() {
		x.note(settings.NoteLogs, "Reading every log…")
		r, err := importer.Dir(x.ctx, x.a.opts.Store, in.Logs())
		switch {
		case err != nil:
			x.a.opts.Log.Error("import again", "err", err)
			x.note(settings.NoteLogs, "Import stopped. The log file has the reason.")
		case r.New == 0:
			x.note(settings.NoteLogs, fmt.Sprintf("Read %d logs. Nothing was missing.", r.Files))
		default:
			x.note(settings.NoteLogs, fmt.Sprintf("Read %d logs and added %s.", r.Files, kit.Games(r.New)))
		}
		x.a.refresh(x.ctx)
	}()
}

func (x *appActions) RefreshCards() {
	go func() {
		x.note(settings.NoteCards, "Refreshing…")
		err := x.a.refreshCatalog(x.ctx, true)
		switch {
		case errors.Is(err, errRefreshing):
			x.note(settings.NoteCards, "Card data is already being checked.")
			return
		case err != nil:
			x.note(settings.NoteCards, "Couldn't reach the card sites. The cards you have stay.")
			return
		}
		x.note(settings.NoteCards, "Up to date.")
	}()
}

func (x *appActions) Backup() {
	go func() {
		name := "logpose-backup-" + time.Now().Format("2006-01-02") + ".db"
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent: x.a.win, Title: "Back up logpose", ButtonLabel: "Back up",
			DefaultPath: filepath.Join(filepath.Dir(x.a.opts.DBPath), name),
			Filters:     []mygo.FileFilter{{Name: "logpose database", Extensions: []string{"db"}}},
		})
		if err != nil || path == "" {
			return
		}
		if !strings.EqualFold(filepath.Ext(path), ".db") {
			path += ".db"
		}
		if err := x.a.opts.Store.Backup(x.ctx, path); err != nil {
			x.a.opts.Log.Error("backup", "err", err)
			x.note(settings.NoteData, "The backup failed. The log file has the reason.")
			return
		}
		x.note(settings.NoteData, "Saved "+filepath.Base(path)+".")
	}()
}

func (x *appActions) LoadArtUsage() { go x.a.loadArtUsage() }

func (a *app) loadArtUsage() {
	if a.web == nil {
		return
	}
	u := a.web.Usage()
	a.win.Update(func() { a.view.setArtUsage(u) })
}

func (x *appActions) ClearArt() {
	web := x.a.web
	if web == nil {
		return
	}
	go func() {
		freed, err := web.Clear()
		text := "Cleared " + kit.Megabytes(freed) + "."
		if err != nil {
			x.a.opts.Log.Error("clear card art", "err", err)
			text = "Some images couldn't be deleted. The log file has the reason."
		}
		x.note(settings.NoteArt, text)
		x.a.resetArtCache()
		x.a.loadArtUsage()
	}()
}
