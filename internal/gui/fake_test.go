package gui

import (
	"fmt"
	"time"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/shortcut"
	"github.com/VTGare/logpose/internal/sim"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/store"
)

// fakeActions records what the view asks the app for.
type fakeActions struct {
	added     []store.Format
	dismissed []string
	loaded    []string
	types     []string
	notes     []string
	bookmarks []string
	replays   []string
	timers    []func()
	calls     []string
	// usageLoads counts LoadArtUsage, which entering Settings calls on its own.
	usageLoads int
	// settingsLoads counts LoadSettings, which Settings calls when it opens or games change.
	settingsLoads int
	// artLoads holds each LoadArts as "<code> <gen>". Showing a card calls it on its own.
	artLoads   []string
	builds     []store.Build
	deleted    []int64
	dismissals []store.Dismissal
	applied    []page.Prefs
	newDays    int
}

// SaveBuild gives a new build the ID 100 and up.
func (f *fakeActions) SaveBuild(b store.Build, saved func(id int64)) {
	f.builds = append(f.builds, b)
	if b.ID == 0 {
		b.ID = int64(99 + len(f.builds))
	}
	if saved != nil {
		saved(b.ID)
	}
}

func (f *fakeActions) DeleteBuild(id int64) { f.deleted = append(f.deleted, id) }

func (f *fakeActions) DismissSuggestion(d store.Dismissal) { f.dismissals = append(f.dismissals, d) }

func (f *fakeActions) SetBookmark(fingerprint string, on bool) {
	f.bookmarks = append(f.bookmarks, fmt.Sprintf("%s=%t", fingerprint, on))
}

func (f *fakeActions) AddFormat(name string, start time.Time) {
	f.added = append(f.added, store.Format{Name: name, Start: start})
}
func (*fakeActions) UpdateFormat(store.Format) {}
func (*fakeActions) DeleteFormat(int64)        {}
func (f *fakeActions) DismissSet(set string)   { f.dismissed = append(f.dismissed, set) }

func (f *fakeActions) OpenReplay(fingerprint string, _ *simlog.Game) {
	f.replays = append(f.replays, fingerprint)
}

func (f *fakeActions) ExportToSim(fingerprint, _ string, _ *simlog.Game) {
	f.calls = append(f.calls, "export "+fingerprint)
}

func (*fakeActions) ChooseFolder(string, string, func(string)) {}

func (f *fakeActions) NewDay() { f.newDays++ }

// After keeps the timer for a spec to fire with fire.
func (f *fakeActions) After(_ time.Duration, fn func()) { f.timers = append(f.timers, fn) }

func (f *fakeActions) fire() {
	timers := f.timers
	f.timers = nil
	for _, fn := range timers {
		fn()
	}
}

func (f *fakeActions) LoadTimeline(fingerprint, _, _ string) {
	f.loaded = append(f.loaded, fingerprint)
}

func (f *fakeActions) SetGameNote(fingerprint, note string) {
	f.notes = append(f.notes, fingerprint+"="+note)
}

func (f *fakeActions) SetGameType(fingerprint string, t store.GameType) {
	f.types = append(f.types, fingerprint+"="+fmt.Sprint(t))
}

func (f *fakeActions) SetSetting(key, value string) { f.calls = append(f.calls, key+"="+value) }
func (f *fakeActions) SetOpenAtLogin(on bool) {
	f.calls = append(f.calls, map[bool]string{true: "login on", false: "login off"}[on])
}

func (f *fakeActions) SetShortcut(place shortcut.Place, on bool) {
	name := map[shortcut.Place]string{shortcut.StartMenu: "start menu", shortcut.Desktop: "desktop"}[place]
	f.calls = append(f.calls, fmt.Sprintf("%s shortcut %t", name, on))
}
func (f *fakeActions) CheckUpdate()            { f.calls = append(f.calls, "check update") }
func (f *fakeActions) InstallUpdate()          { f.calls = append(f.calls, "install update") }
func (f *fakeActions) RestartToUpdate()        { f.calls = append(f.calls, "restart to update") }
func (f *fakeActions) ChooseSimFolder()        { f.calls = append(f.calls, "choose sim") }
func (f *fakeActions) FindSim()                { f.calls = append(f.calls, "find sim") }
func (f *fakeActions) OpenPath(path string)    { f.calls = append(f.calls, "open "+path) }
func (f *fakeActions) ShowFile(path string)    { f.calls = append(f.calls, "show "+path) }
func (f *fakeActions) ImportAgain()            { f.calls = append(f.calls, "import") }
func (f *fakeActions) RefreshCards()           { f.calls = append(f.calls, "refresh cards") }
func (f *fakeActions) Backup()                 { f.calls = append(f.calls, "backup") }
func (f *fakeActions) LoadArtUsage()           { f.usageLoads++ }
func (f *fakeActions) LoadSettings()           { f.settingsLoads++ }
func (f *fakeActions) ClearArt()               { f.calls = append(f.calls, "clear art") }
func (f *fakeActions) ApplyPrefs(p page.Prefs) { f.applied = append(f.applied, p) }

func (f *fakeActions) LoadSimArt(card cards.Card) {
	f.calls = append(f.calls, "load art "+card.Code)
}

func (f *fakeActions) LoadArts(card cards.Card, gen int) {
	f.artLoads = append(f.artLoads, fmt.Sprint(card.Code, " ", gen))
}
func (f *fakeActions) ScanSimArt() { f.calls = append(f.calls, "scan art") }
func (f *fakeActions) UseSimArt(card cards.Card, choice, _ sim.ArtChoice) {
	f.calls = append(f.calls, "use art "+card.Code+" "+choice.Key)
}
func (f *fakeActions) SaveArt(key string) { f.calls = append(f.calls, "save art "+key) }
func (f *fakeActions) CopyArt(key string) { f.calls = append(f.calls, "copy art "+key) }
func (f *fakeActions) OpenURL(url string) { f.calls = append(f.calls, "open url "+url) }
