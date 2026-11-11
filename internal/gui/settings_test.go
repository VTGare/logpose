package gui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/shortcut"
	"github.com/VTGare/logpose/internal/sim"
	"github.com/VTGare/logpose/internal/store"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (f *fakeActions) SetSetting(key, value string) { f.calls = append(f.calls, key+"="+value) }
func (f *fakeActions) SetOpenAtLogin(on bool) {
	f.calls = append(f.calls, map[bool]string{true: "login on", false: "login off"}[on])
}

func (f *fakeActions) SetShortcut(place shortcut.Place, on bool) {
	f.calls = append(f.calls, fmt.Sprintf("shortcut %d %t", place, on))
}
func (f *fakeActions) CheckUpdate()         { f.calls = append(f.calls, "check update") }
func (f *fakeActions) InstallUpdate()       { f.calls = append(f.calls, "install update") }
func (f *fakeActions) RestartToUpdate()     { f.calls = append(f.calls, "restart to update") }
func (f *fakeActions) ChooseSimFolder()     { f.calls = append(f.calls, "choose sim") }
func (f *fakeActions) FindSim()             { f.calls = append(f.calls, "find sim") }
func (f *fakeActions) OpenPath(path string) { f.calls = append(f.calls, "open "+path) }
func (f *fakeActions) ShowFile(path string) { f.calls = append(f.calls, "show "+path) }
func (f *fakeActions) ImportAgain()         { f.calls = append(f.calls, "import") }
func (f *fakeActions) RefreshCards()        { f.calls = append(f.calls, "refresh cards") }
func (f *fakeActions) Backup()              { f.calls = append(f.calls, "backup") }
func (f *fakeActions) LoadArtUsage()        { f.usageLoads++ }
func (f *fakeActions) ClearArt()            { f.calls = append(f.calls, "clear art") }
func (f *fakeActions) SetArtCap(mb int)     { f.calls = append(f.calls, fmt.Sprintf("art cap %d", mb)) }

func (f *fakeActions) SetArtDownloads(on bool) {
	f.calls = append(f.calls, map[bool]string{true: "art downloads on", false: "art downloads off"}[on])
}

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

func settingsSnapshot() *snapshot {
	s := testSnapshot(4)
	s.Settings = settingsInfo{
		SimDir: `E:\Games\OPTCG\Builds_Windows`, SimVersion: "1.44a", SimExe: true, Logs: 93,
		Cards: 2926, Fetched: now.Add(-25 * time.Hour),
		DBPath: `C:\Users\you\AppData\Roaming\logpose\logpose.db`, DBFolder: `AppData\Roaming\logpose`, DBSize: 1 << 20, Games: 93,
		ArtDir:  `C:\Users\you\AppData\Roaming\logpose\art`,
		Players: []store.PlayerCount{{ID: me, Games: 93}, {ID: "Opp#2", Games: 9}, {ID: "Alt#3", Games: 2}, {ID: "Rare#4", Games: 1}},
	}
	return s
}

type settingsSpec struct {
	v  *view
	fa *fakeActions
	tt *ui.Tester
}

func settingsTester() settingsSpec {
	v, tt := tester(settingsSnapshot())
	fa := &fakeActions{}
	v.act = fa
	v.page = pageSettings
	tt.Frame()
	return settingsSpec{v, fa, tt}
}

func (s settingsSpec) has(text string) bool {
	s.tt.Frame()
	return s.tt.HasText(text)
}

func (s settingsSpec) click(label string) {
	GinkgoHelper()
	Expect(s.tt.Click(label)).To(Succeed())
	s.tt.Frame()
}

var _ = Describe("settings", func() {
	It("shows what logpose found, and the buttons call the app", func() {
		st := settingsTester()
		for _, text := range []string{
			"Version 1.44a", "93 in AutoSaved", "2,926 cards", "Refreshed yesterday at 19:00", "93 games",
			`1.0 MB in AppData\Roaming\logpose`,
		} {
			Expect(st.has(text)).To(BeTrue(), text)
		}
		st.click("Import every log again")
		st.click("Refresh card data")
		st.click("Back up the database")
		st.click("Show the database in its folder")
		st.click("Change the sim's folder")
		Expect(st.fa.calls).To(Equal([]string{
			"import", "refresh cards", "backup", `show C:\Users\you\AppData\Roaming\logpose\logpose.db`, "choose sim",
		}))
	})

	It("explains where card data comes from", func() {
		_, tt := tester(settingsSnapshot())
		_, ok := tt.Find("About card data")
		Expect(ok).To(BeFalse())
		v, tt := tester(settingsSnapshot())
		v.page = pageSettings
		tt.Frame()
		_, ok = tt.Find("About card data")
		Expect(ok).To(BeTrue())
	})

	It("picks the name in the most games until the user picks names", func() {
		st := settingsTester()
		Expect(st.has("Alt#3")).To(BeTrue())
		Expect(st.has("Rare#4")).To(BeFalse())
		st.click("Pick automatically")
		Expect(st.v.snap.Settings.You).To(Equal([]string{me}))
		st.click("Alt#3")
		Expect(st.v.snap.Settings.You).To(Equal([]string{me, "Alt#3"}))
		st.click("Show all 4 names")
		Expect(st.has("Rare#4")).To(BeTrue())
		Expect(st.fa.calls).To(Equal([]string{store.SettingYou + "=" + me, store.SettingYou + "=" + me + "\nAlt#3"}))
	})

	It("keeps at least one name picked", func() {
		st := settingsTester()
		st.click("Pick automatically")
		st.click(me)
		Expect(st.v.snap.Settings.You).To(Equal([]string{me}))
		Expect(st.fa.calls).To(HaveLen(1))
	})

	It("saves the theme, the colors and how the window closes", func() {
		st := settingsTester()
		st.click("Dark")
		st.click("Colorblind-safe colors")
		st.click("Quits logpose")
		Expect(st.v.prefs).To(Equal(prefs{theme: "dark", colorblind: true, closeQuits: true, sideWidth: sideDefault, startFormat: startUnpicked}))
		Expect(st.fa.calls).To(Equal([]string{
			store.SettingTheme + "=dark", store.SettingColorblind + "=1", store.SettingCloseQuits + "=1",
		}))
		win, _ := resultColors(true)
		Expect(winColor).To(Equal(win))
		st.v.prefs.colorblind = false
	})

	It("starts with Windows, and only then offers to start hidden", func() {
		st := settingsTester()
		st.click("Start hidden in the tray")
		Expect(st.v.prefs.startHidden).To(BeFalse())
		st.click("Start with Windows")
		st.click("Start hidden in the tray")
		Expect(st.v.prefs.startHidden).To(BeTrue())
		Expect(st.fa.calls).To(Equal([]string{"login on", store.SettingStartHidden + "=1"}))
	})

	Describe("card art", func() {
		artTester := func() settingsSpec {
			st := settingsTester()
			st.tt.SetSize(1180, 1100)
			st.tt.Frame()
			return st
		}
		used := &artUsage{FullBytes: 141 << 20, FullCount: 1378, ThumbBytes: 117 << 20, ThumbCount: 2889}

		It("counts the folder when Settings opens, then shows its size", func() {
			st := artTester()
			Expect(st.fa.usageLoads).To(Equal(1))
			Expect(st.has("Counting…")).To(BeTrue())
			st.v.settings.art = used
			Expect(st.has("258 MB on disk")).To(BeTrue())
			Expect(st.has("Thumbnails 117 MB · large images 141 MB of 150 MB")).To(BeTrue())
			Expect(st.fa.usageLoads).To(Equal(1))
		})

		It("shows the folder, clears it and says how much it freed", func() {
			st := artTester()
			st.click("Clear card art")
			Expect(st.fa.calls).To(BeEmpty())
			st.v.settings.art = used
			st.click("Show the card art folder")
			st.click("Clear card art")
			Expect(st.fa.calls).To(Equal([]string{`open C:\Users\you\AppData\Roaming\logpose\art`, "clear art"}))
			st.v.settings.note(noteArt, "Cleared 258 MB.")
			st.v.settings.art = &artUsage{}
			Expect(st.has("Cleared 258 MB.")).To(BeTrue())
			Expect(st.has("0 MB on disk")).To(BeTrue())
		})

		It("caps the large images at the size picked", func() {
			st := artTester()
			st.v.settings.art = used
			st.click("300 MB")
			Expect(st.v.prefs.artCap).To(Equal(300))
			Expect(st.has("Thumbnails 117 MB · large images 141 MB of 300 MB")).To(BeTrue())
			Expect(st.fa.calls).To(Equal([]string{"art cap 300"}))
		})

		It("turns art downloads off and on", func() {
			st := artTester()
			st.click("Download card art")
			Expect(st.v.prefs.artOff).To(BeTrue())
			st.click("Download card art")
			Expect(st.v.prefs.artOff).To(BeFalse())
			Expect(st.fa.calls).To(Equal([]string{"art downloads off", "art downloads on"}))
		})

		It("offers only the switch without an art folder", func() {
			s := settingsSnapshot()
			s.Settings.ArtDir = ""
			v, tt := tester(s)
			fa := &fakeActions{}
			v.act = fa
			v.page = pageSettings
			tt.SetSize(1180, 1100)
			tt.Frame()
			Expect(tt.HasText("Download card art")).To(BeTrue())
			Expect(tt.HasText("Counting…")).To(BeFalse())
			Expect(tt.HasText("Keep large images up to")).To(BeFalse())
			Expect(fa.usageLoads).To(BeZero())
		})
	})

	It("asks for the sim's folder when there's no sim and no games", func() {
		v, tt := tester(&snapshot{Loaded: true, Now: now})
		fa := &fakeActions{}
		v.act = fa
		tt.Frame()
		Expect(tt.HasText("Where is OPTCGSim?")).To(BeTrue())
		Expect(tt.HasText("Looking for a running sim")).To(BeTrue())
		Expect(tt.Click("Choose folder…")).To(Succeed())
		Expect(fa.calls).To(Equal([]string{"choose sim"}))
	})
})
