package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/VTGare/logpose/internal/importer"
	"github.com/VTGare/logpose/internal/sim"
	"github.com/VTGare/logpose/internal/store"
)

// settingsInfo is what the Settings page shows about the sim, the logs, the card data and the
// database.
type settingsInfo struct {
	// SimDir is empty when the sim isn't found.
	SimDir     string
	SimChosen  bool
	SimFlag    bool
	SimVersion string
	SimExe     bool
	Logs       int

	Cards   int
	Fetched time.Time

	DBPath string
	// DBFolder is the database's folder, short for the page.
	DBFolder string
	DBSize   int64
	Games    int

	Players []store.PlayerCount
	// You is the names the user picked, empty when logpose picks the name in the most games.
	You []string
}

func (a *app) settingsInfo(ctx context.Context, set map[string]string, games, cardCount int) (settingsInfo, error) {
	players, err := a.opts.Store.PlayerCounts(ctx)
	if err != nil {
		return settingsInfo{}, err
	}
	a.cardsMu.Lock()
	fetched := a.fetched
	if !a.checked.IsZero() {
		fetched = a.checked
	}
	a.cardsMu.Unlock()
	info := settingsInfo{
		SimFlag: a.opts.SimFlag,
		Cards:   cardCount,
		Fetched: fetched,
		DBPath:  a.opts.DBPath,
		Games:   games,
		Players: players,
		You:     store.SplitNames(set[store.SettingYou]),
	}
	if in := a.install(); in != nil {
		info.SimDir = in.Root
		info.SimChosen = set[store.SettingSimDirChosen] == in.Root
		info.SimVersion = sim.Version()
		info.SimExe = in.HasExe()
		info.Logs = in.LogCount()
	}
	info.DBFolder = shortFolder(filepath.Dir(a.opts.DBPath))
	for _, p := range []string{a.opts.DBPath, a.opts.DBPath + "-wal"} {
		if fi, err := os.Stat(p); err == nil {
			info.DBSize += fi.Size()
		}
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

// prefs are the settings the window applies itself. The view owns them, and saves each change.
type prefs struct {
	theme       string
	colorblind  bool
	closeQuits  bool
	startHidden bool
	// openAtLogin is read from Windows, not the store.
	openAtLogin bool
	sideWidth   float32
	panelWidth  float32
}

func prefsFrom(set map[string]string) prefs {
	return prefs{
		theme:       set[store.SettingTheme],
		colorblind:  set[store.SettingColorblind] == "1",
		closeQuits:  set[store.SettingCloseQuits] == "1",
		startHidden: set[store.SettingStartHidden] == "1",
		sideWidth:   widthFrom(set[store.SettingSidebarWidth], sideDefault, snapSide),
		panelWidth:  widthFrom(set[store.SettingCardPanelWidth], panelDefault, snapPanel),
	}
}

func widthFrom(s string, def float32, snap func(float32) float32) float32 {
	w, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return def
	}
	return snap(float32(w))
}

func flag(on bool) string {
	if on {
		return "1"
	}
	return ""
}

func (x *appActions) SetSetting(key, value string) {
	if key == store.SettingCloseQuits {
		x.a.closeQuits.Store(value == "1")
	}
	x.run("save setting", func(ctx context.Context, st *store.Store) error { return st.SetSetting(ctx, key, value) })
}

func (x *appActions) SetOpenAtLogin(on bool) {
	go func() {
		err := mygo.App.SetOpenAtLogin(on)
		if err != nil {
			x.a.opts.Log.Warn("start with Windows", "err", err)
		}
		now := mygo.App.OpenAtLogin()
		x.a.win.Update(func() {
			x.a.view.prefs.openAtLogin = now
			if err != nil {
				x.a.view.settings.note(noteStartup, "Windows didn't take the change.")
			}
		})
	}()
}

// note shows text on a part of the Settings page.
func (x *appActions) note(part, text string) {
	x.a.win.Update(func() { x.a.view.settings.note(part, text) })
}

func (x *appActions) ChooseSimFolder() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: x.a.win, Title: "Choose OPTCGSim's folder", Directory: true})
		if err != nil || len(paths) == 0 {
			return
		}
		in, err := sim.Open(paths[0])
		if err != nil {
			x.note(noteSim, "That folder isn't OPTCGSim's. Pick the one with OPTCGSim.exe in it.")
			return
		}
		x.note(noteSim, "")
		if err := x.a.setInstall(x.ctx, in, true); err != nil {
			x.a.opts.Log.Error("choose sim", "err", err)
		}
	}()
}

// FindSim forgets the folder the user chose and finds the sim again.
func (x *appActions) FindSim() {
	go func() {
		if err := x.a.opts.Store.SetSetting(x.ctx, store.SettingSimDirChosen, ""); err != nil {
			x.a.opts.Log.Error("forget sim folder", "err", err)
			return
		}
		in, err := sim.Locate()
		if err != nil {
			x.note(noteSim, "The sim isn't running, so the chosen folder stays until it is.")
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
		x.note(noteLogs, "Reading every log…")
		r, err := importer.Dir(x.ctx, x.a.opts.Store, in.Logs())
		if err == nil {
			_, err = x.a.opts.Store.Reparse(x.ctx)
		}
		switch {
		case err != nil:
			x.a.opts.Log.Error("import again", "err", err)
			x.note(noteLogs, "Import stopped. The log file has the reason.")
		case r.New == 0:
			x.note(noteLogs, fmt.Sprintf("Read %d logs. Nothing was missing.", r.Files))
		default:
			x.note(noteLogs, fmt.Sprintf("Read %d logs and added %s.", r.Files, games(r.New)))
		}
		x.a.refresh(x.ctx)
	}()
}

func (x *appActions) RefreshCards() {
	go func() {
		x.note(noteCards, "Refreshing…")
		if err := x.a.refreshCatalog(x.ctx, true); err != nil {
			x.note(noteCards, "Couldn't reach the card sites. The cards you have stay.")
			return
		}
		x.note(noteCards, "Up to date.")
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
			x.note(noteData, "The backup failed. The log file has the reason.")
			return
		}
		x.note(noteData, "Saved "+filepath.Base(path)+".")
	}()
}
