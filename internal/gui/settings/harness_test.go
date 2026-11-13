package settings

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/gui/guitest"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/shortcut"
)

// fakeActions records what the page asks the app for.
type fakeActions struct {
	guitest.EnvActions
	// usageLoads counts LoadArtUsage, which entering Settings calls on its own.
	usageLoads int
	// settingsLoads counts LoadSettings, which Settings calls when it opens or games change.
	settingsLoads int
}

func (f *fakeActions) call(what string) { f.Calls = append(f.Calls, what) }

func (f *fakeActions) SetOpenAtLogin(on bool) {
	f.call(map[bool]string{true: "login on", false: "login off"}[on])
}

func (f *fakeActions) SetShortcut(place shortcut.Place, on bool) {
	name := map[shortcut.Place]string{shortcut.StartMenu: "start menu", shortcut.Desktop: "desktop"}[place]
	f.call(fmt.Sprintf("%s shortcut %t", name, on))
}
func (f *fakeActions) CheckUpdate()         { f.call("check update") }
func (f *fakeActions) InstallUpdate()       { f.call("install update") }
func (f *fakeActions) RestartToUpdate()     { f.call("restart to update") }
func (f *fakeActions) ChooseSimFolder()     { f.call("choose sim") }
func (f *fakeActions) FindSim()             { f.call("find sim") }
func (f *fakeActions) OpenPath(path string) { f.call("open " + path) }
func (f *fakeActions) ShowFile(path string) { f.call("show " + path) }
func (f *fakeActions) ImportAgain()         { f.call("import") }
func (f *fakeActions) RefreshCards()        { f.call("refresh cards") }
func (f *fakeActions) Backup()              { f.call("backup") }
func (f *fakeActions) LoadArtUsage()        { f.usageLoads++ }
func (f *fakeActions) LoadSettings()        { f.settingsLoads++ }
func (f *fakeActions) ClearArt()            { f.call("clear art") }

// view is the Settings page in a host. It shows what the window shows of the page: the update
// box of the sidebar, then Settings itself, the shortcut card of the Overview on any other
// page, or the question about the sim's folder while there are no games and no sim.
type view struct {
	*guitest.Host
	settings Page
}

func (v *view) draw(c *ui.Context, e *page.Env) {
	v.settings.UpdateBox(c, e.Prefs.SideWidth > page.SideRail)
	switch {
	case len(e.Snap.Matches) == 0 && !e.Snap.SimFound && e.Page != page.Settings:
		v.settings.SetupPage(c)
	case e.Page == page.Settings:
		v.settings.Draw(c, e)
	default:
		v.settings.ShortcutCard(c, e)
	}
}

func tester(s *page.Snapshot) (*view, *ui.Tester) {
	v := &view{Host: &guitest.Host{}}
	_, tt := guitest.Start(v.Host, s, v.draw)
	return v, tt
}

func (v *view) setPrefs(p page.Prefs)        { v.SetPrefs(p) }
func (v *view) setSnapshot(s *page.Snapshot) { v.Snap = s }
func (v *view) setSystem(s System)           { v.settings.System = s }
func (v *view) setArtUsage(u arts.Usage)     { v.settings.Art = &u }
func (v *view) setSettingsInfo(info Info)    { v.settings.Info = &info }
func (v *view) setActions(f *fakeActions)    { v.Act, v.settings.Act = f, f }
