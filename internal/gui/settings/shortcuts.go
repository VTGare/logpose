package settings

import (
	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/shortcut"
)

var ShortcutNames = [...]string{shortcut.StartMenu: "Start menu", shortcut.Desktop: "Desktop"}

// ShortcutCard offers shortcuts once, on the Overview, until it's answered or both exist.
func (pg *Page) ShortcutCard(c *ui.Context, e *page.Env) {
	p := &pg.System
	if !p.CanShortcut || p.Installed || e.Prefs.ShortcutsOffered || p.Shortcuts[shortcut.StartMenu] && p.Shortcuts[shortcut.Desktop] {
		return
	}
	t := c.Theme()
	kit.Panel(c).Padding(14, 18).Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
				ui.Text(c, "Add logpose to the Start menu and desktop?").FontWeight(600)
				ui.Text(c, "So you can open it without finding the exe. You can change this in Settings.").
					FontSize(13).TextColor(t.TextMuted).Wrap()
			})
			for _, place := range shortcut.Places {
				if p.Shortcuts[place] {
					continue
				}
				on := !pg.offerSkip[place]
				ui.Checkbox(c, &on, ShortcutNames[place])
				pg.offerSkip[place] = !on
			}
			add := ui.PrimaryButton(c, "Add")
			add.Disabled(pg.offerSkip[shortcut.StartMenu] && pg.offerSkip[shortcut.Desktop])
			if add.Clicked() {
				for _, place := range shortcut.Places {
					if !p.Shortcuts[place] && !pg.offerSkip[place] {
						pg.setShortcut(place, true)
					}
				}
				answerShortcuts(e)
			}
			if ui.Button(c, "Not now").Clicked() {
				answerShortcuts(e)
			}
		})
	})
}

func (pg *Page) shortcutRows(c *ui.Context, e *page.Env) {
	if !pg.System.CanShortcut {
		return
	}
	for _, place := range shortcut.Places {
		divider(c)
		on := pg.System.Shortcuts[place]
		if switchRow(c, ShortcutNames[place]+" shortcut", "", &on, false) {
			pg.setShortcut(place, on)
			answerShortcuts(e)
		}
	}
}

func (pg *Page) setShortcut(place shortcut.Place, on bool) {
	pg.System.Shortcuts[place] = on
	pg.do(func(a Actions) { a.SetShortcut(place, on) })
}

func answerShortcuts(e *page.Env) {
	if e.Prefs.ShortcutsOffered {
		return
	}
	page.PrefShortcutsOffered.Set(e, true)
}
