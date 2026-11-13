package gui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	// updateFirstCheck waits for the app to settle after it starts.
	updateFirstCheck = 10 * time.Second
	updateEvery      = 6 * time.Hour
)

// releasesURL is where each version's release notes are, by tag.
const releasesURL = "https://github.com/VTGare/logpose/releases/tag/v"

// updater holds the newer version found, between the check and the install.
type updater struct {
	mu    sync.Mutex
	state updateState
	found *mygo.Update
}

func Version() string { return mygo.App.Version() }

// installed reports whether logpose runs from the folder its installer made, which holds the
// uninstaller next to the exe.
func installed() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(filepath.Dir(exe), "Uninstall.exe"))
	return err == nil
}

// startUpdates fills in the update state, says so when this is the first start of a new
// version, and checks for newer ones now and then.
func (a *app) startUpdates(ctx context.Context, set map[string]string) {
	u := &a.upd
	u.state.current = Version()
	u.state.can = !a.opts.Dev && mygo.Updater.Enabled()
	if ms, err := strconv.ParseInt(set[keyUpdateChecked], 10, 64); err == nil {
		u.state.checked = time.UnixMilli(ms)
	}
	a.view.settings.setUpdate(u.state)
	if last := set[keyLastVersion]; last != u.state.current && !a.opts.Dev {
		if last != "" {
			ver := u.state.current
			a.view.queueToast(ui.Toast{
				Title: "Updated to " + ver, Timeout: 10 * time.Second,
				Action: "What's new", OnAction: func() { a.view.do(func(x actions) { x.OpenURL(releasesURL + ver) }) },
			})
		}
		go func() {
			if err := a.opts.Store.SetSetting(ctx, keyLastVersion, u.state.current); err != nil {
				a.opts.Log.Warn("save version", "err", err)
			}
		}()
	}
	if !u.state.can {
		return
	}
	go func() {
		wait := updateFirstCheck
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			wait = updateEvery
			a.checkUpdate(ctx)
		}
	}()
}

func (a *app) setUpdate(change func(s *updateState)) {
	u := &a.upd
	u.mu.Lock()
	change(&u.state)
	s := u.state
	u.mu.Unlock()
	a.win.Update(func() { a.view.settings.setUpdate(s) })
}

// checkUpdate asks for a newer version and downloads it, unless the user downloads updates
// themselves.
func (a *app) checkUpdate(ctx context.Context) {
	u := &a.upd
	u.mu.Lock()
	busy := u.state.busy()
	u.mu.Unlock()
	if busy {
		return
	}
	a.setUpdate(func(s *updateState) { s.phase = updateChecking })
	up, err := mygo.Updater.Check(ctx)
	if errors.Is(err, context.Canceled) {
		return
	}
	now := time.Now()
	if err == nil {
		if serr := a.opts.Store.SetSetting(ctx, keyUpdateChecked, strconv.FormatInt(now.UnixMilli(), 10)); serr != nil {
			a.opts.Log.Warn("save update check", "err", serr)
		}
	}
	u.mu.Lock()
	u.found = up
	u.mu.Unlock()
	a.setUpdate(func(s *updateState) {
		switch {
		case err != nil:
			s.phase, s.err = updateFailed, "Couldn't check for updates: "+err.Error()
		case up == nil:
			s.phase, s.checked = updateIdle, now
		default:
			s.phase, s.checked, s.next, s.notes = updateFound, now, up.Version, up.Notes
		}
	})
	if err != nil {
		a.opts.Log.Warn("update check", "err", err)
		return
	}
	if up != nil && !a.applied().updateManual {
		a.installUpdate(ctx)
	}
}

// installUpdate downloads the version found and puts it in place of this one. The running app
// carries on, and the new version runs once it starts again.
func (a *app) installUpdate(ctx context.Context) {
	u := &a.upd
	u.mu.Lock()
	up := u.found
	u.mu.Unlock()
	if up == nil {
		return
	}
	a.setUpdate(func(s *updateState) { s.phase, s.done = updateDownloading, 0 })
	shown := -1
	err := up.Install(ctx, func(got, total int64) {
		if total <= 0 {
			return
		}
		if pct := int(got * 100 / total); pct != shown {
			shown = pct
			a.setUpdate(func(s *updateState) { s.done = float32(pct) / 100 })
		}
	})
	if errors.Is(err, context.Canceled) {
		return
	}
	a.setUpdate(func(s *updateState) {
		if err != nil {
			s.phase, s.err = updateFailed, "Couldn't install "+s.next+": "+err.Error()
			return
		}
		s.phase = updateReady
	})
	if err != nil {
		a.opts.Log.Warn("update install", "err", err)
	}
}

func (x *appActions) CheckUpdate()   { go x.a.checkUpdate(x.ctx) }
func (x *appActions) InstallUpdate() { go x.a.installUpdate(x.ctx) }

// RestartToUpdate quits and starts the installed new version.
func (x *appActions) RestartToUpdate() {
	x.a.quitting = true
	mygo.App.Relaunch()
}
