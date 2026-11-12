package gui

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/shortcut"
)

var shortcutNames = [...]string{shortcut.StartMenu: "Start menu", shortcut.Desktop: "Desktop"}

// shortcutCard offers shortcuts once, on the Overview, until it's answered or both exist.
func (v *view) shortcutCard(c *ui.Context) {
	p := &v.system
	if !p.canShortcut || p.installed || v.prefs.shortcutsOffered || p.shortcuts[shortcut.StartMenu] && p.shortcuts[shortcut.Desktop] {
		return
	}
	t := c.Theme()
	s := &v.settings
	panel(c).Padding(14, 18).Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
				ui.Text(c, "Add logpose to the Start menu and desktop?").FontWeight(600)
				ui.Text(c, "So you can open it without finding the exe. You can change this in Settings.").
					FontSize(13).TextColor(t.TextMuted).Wrap()
			})
			for _, place := range shortcut.Places {
				if p.shortcuts[place] {
					continue
				}
				on := !s.offerSkip[place]
				ui.Checkbox(c, &on, shortcutNames[place])
				s.offerSkip[place] = !on
			}
			add := ui.PrimaryButton(c, "Add")
			add.Disabled(s.offerSkip[shortcut.StartMenu] && s.offerSkip[shortcut.Desktop])
			if add.Clicked() {
				for _, place := range shortcut.Places {
					if !p.shortcuts[place] && !s.offerSkip[place] {
						v.setShortcut(place, true)
					}
				}
				v.answerShortcuts()
			}
			if ui.Button(c, "Not now").Clicked() {
				v.answerShortcuts()
			}
		})
	})
}

func (v *view) shortcutRows(c *ui.Context) {
	if !v.system.canShortcut {
		return
	}
	for _, place := range shortcut.Places {
		divider(c)
		on := v.system.shortcuts[place]
		if switchRow(c, shortcutNames[place]+" shortcut", "", &on, false) {
			v.setShortcut(place, on)
			v.answerShortcuts()
		}
	}
}

func (v *view) setShortcut(place shortcut.Place, on bool) {
	v.system.shortcuts[place] = on
	v.do(func(a actions) { a.SetShortcut(place, on) })
}

func (v *view) answerShortcuts() {
	if v.prefs.shortcutsOffered {
		return
	}
	prefShortcutsOffered.set(v, true)
}

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

func (a *app) readSystem() systemState {
	s := systemState{openAtLogin: mygo.App.OpenAtLogin(), installed: installed()}
	link, err := a.shortcutLink()
	if err != nil {
		return s
	}
	if _, err := shortcut.Path(shortcut.StartMenu, link.Name); err != nil {
		return s
	}
	s.canShortcut = true
	for _, place := range shortcut.Places {
		s.shortcuts[place] = shortcut.Exists(place, link.Name)
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
			x.toast(ui.Toast{Title: "Windows didn't take the " + shortcutNames[place] + " shortcut change.", Type: "error"})
		}
	}()
}
