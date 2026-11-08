package gui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	// ArtDir is the card art folder, empty when there's none.
	ArtDir string
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
		ArtDir:  a.opts.ArtDir,
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
	// openAtLogin and shortcuts are read from Windows, not the store. canShortcut is false where
	// shortcuts can't be made.
	openAtLogin      bool
	shortcuts        [2]bool
	canShortcut      bool
	shortcutsOffered bool
	sideWidth        float32
	panelWidth       float32
	artOff           bool
	// artCap is the MB of large card images kept, one of artCaps.
	artCap int
}

// artCaps are the sizes in MB the large card images can be capped at.
var artCaps = []int{100, 150, 300, 600}

const artCapDefault = webFullCap >> 20

func prefsFrom(set map[string]string) prefs {
	return prefs{
		theme:            set[store.SettingTheme],
		colorblind:       set[store.SettingColorblind] == "1",
		closeQuits:       set[store.SettingCloseQuits] == "1",
		startHidden:      set[store.SettingStartHidden] == "1",
		shortcutsOffered: set[store.SettingShortcutsOffered] == "1",
		sideWidth:        widthFrom(set[store.SettingSidebarWidth], sideDefault, snapSide),
		panelWidth:       widthFrom(set[store.SettingCardPanelWidth], panelDefault, snapPanel),
		artOff:           set[store.SettingArtOff] == "1",
		artCap:           artCapFrom(set[store.SettingArtCap]),
	}
}

func artCapFrom(s string) int {
	mb, err := strconv.Atoi(s)
	if err != nil || !slices.Contains(artCaps, mb) {
		return artCapDefault
	}
	return mb
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
		err := x.a.refreshCatalog(x.ctx, true)
		switch {
		case errors.Is(err, errRefreshing):
			x.note(noteCards, "Card data is already being checked.")
			return
		case err != nil:
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

func (x *appActions) LoadArtUsage() {
	web := x.a.web
	if web == nil {
		return
	}
	go func() {
		u := web.usage()
		x.a.win.Update(func() { x.a.view.settings.art = &u })
	}()
}

func (x *appActions) ClearArt() {
	web := x.a.web
	if web == nil {
		return
	}
	go func() {
		freed, err := web.clear()
		text := "Cleared " + megabytes(freed) + "."
		if err != nil {
			x.a.opts.Log.Error("clear card art", "err", err)
			text = "Some images couldn't be deleted. The log file has the reason."
		}
		x.a.win.Update(func() {
			x.a.view.settings.note(noteArt, text)
			x.a.view.art = newArtCache(x.a.install(), web, x.a.win.Update)
		})
		x.LoadArtUsage()
	}()
}

func (x *appActions) SetArtCap(mb int) {
	web := x.a.web
	go func() {
		if err := x.a.opts.Store.SetSetting(x.ctx, store.SettingArtCap, strconv.Itoa(mb)); err != nil {
			x.a.opts.Log.Error("save art cap", "err", err)
		}
		if web == nil {
			return
		}
		web.setLimit(int64(mb) << 20)
		web.trim()
		x.LoadArtUsage()
	}()
}

// SetArtDownloads also groups every card again, since pictures that couldn't be read while
// downloads were off can be now. Turning them on reads Bandai's card list if it is stale.
func (x *appActions) SetArtDownloads(on bool) {
	web := x.a.web
	if web != nil {
		web.setOff(!on)
		x.a.view.art = newArtCache(x.a.install(), web, x.a.win.Update)
	}
	x.a.view.resetArts()
	go func() {
		if err := x.a.opts.Store.SetSetting(x.ctx, store.SettingArtOff, flag(!on)); err != nil {
			x.a.opts.Log.Error("save art downloads", "err", err)
		}
		if on {
			_ = x.a.refreshCatalog(x.ctx, false)
		}
	}()
}
