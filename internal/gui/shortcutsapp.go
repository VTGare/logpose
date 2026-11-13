package gui

import (
	"os"
	"path/filepath"

	"github.com/VTGare/logpose/internal/gui/settings"

	"github.com/egoist/mygo"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/shortcut"
)

// shortcutLink starts this exe as it was started, so -db and -sim carry over. A test copy's
// shortcuts are named apart from the real ones.
func (a *app) shortcutLink() (shortcut.Link, error) {
	exe, err := os.Executable()
	if err != nil {
		return shortcut.Link{}, err
	}
	name := "logpose"
	if a.opts.Dev {
		name = "logpose (dev)"
	}
	return shortcut.Link{
		Name: name, Target: exe, Args: os.Args[1:], Dir: filepath.Dir(exe),
		Description: "Stats for your OPTCG Sim games",
	}, nil
}

func (a *app) readSystem() settings.System {
	s := settings.System{OpenAtLogin: mygo.App.OpenAtLogin(), Installed: installed()}
	link, err := a.shortcutLink()
	if err != nil {
		return s
	}
	if _, err := shortcut.Path(shortcut.StartMenu, link.Name); err != nil {
		return s
	}
	s.CanShortcut = true
	for _, place := range shortcut.Places {
		s.Shortcuts[place] = shortcut.Exists(place, link.Name)
	}
	return s
}

func (x *appActions) SetShortcut(place shortcut.Place, on bool) {
	go func() {
		link, err := x.a.shortcutLink()
		if err == nil {
			if on {
				err = shortcut.Create(place, link)
			} else {
				err = shortcut.Remove(place, link.Name)
			}
		}
		if err != nil {
			x.a.opts.Log.Warn("shortcut", "place", place, "on", on, "err", err)
		}
		now := shortcut.Exists(place, link.Name)
		x.a.win.Update(func() { x.a.view.setShortcutMade(place, now) })
		if err != nil {
			x.toast(ui.Toast{Title: "Windows didn't take the " + settings.ShortcutNames[place] + " shortcut change.", Type: "error"})
		}
	}()
}
