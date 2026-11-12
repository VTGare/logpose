package gui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/store"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func settingsSnapshot() *snapshot {
	s := testSnapshot(4)
	s.Stored = 93
	s.Cards = cards.NewDB()
	for i := range 2926 {
		s.Cards.Name(fmt.Sprintf("T-%04d", i), "Card")
	}
	return s
}

func testSettingsInfo() settingsInfo {
	return settingsInfo{
		SimDir: `E:\Games\OPTCG\Builds_Windows`, SimVersion: "1.44a", SimExe: true, Logs: 93,
		Fetched: now.Add(-25 * time.Hour),
		DBPath:  `C:\Users\you\AppData\Roaming\logpose\logpose.db`, DBFolder: `AppData\Roaming\logpose`, DBSize: 1 << 20,
		ArtDir:  `C:\Users\you\AppData\Roaming\logpose\art`,
		Players: []store.PlayerCount{{ID: me, Games: 93}, {ID: "Opp#2", Games: 9}, {ID: "Alt#3", Games: 2}, {ID: "Rare#4", Games: 1}},
	}
}

// settingsView is a view on Settings with the info the app would read.
func settingsView(info settingsInfo) (*view, *ui.Tester, *fakeActions) {
	v, tt := tester(settingsSnapshot())
	fa := &fakeActions{}
	v.act = fa
	v.page = pageSettings
	v.setSettingsInfo(info)
	return v, tt, fa
}

type settingsSpec struct {
	v  *view
	fa *fakeActions
	tt *ui.Tester
}

func settingsTester() settingsSpec {
	v, tt, fa := settingsView(testSettingsInfo())
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
		_, tt, _ = settingsView(testSettingsInfo())
		tt.Frame()
		_, ok = tt.Find("About card data")
		Expect(ok).To(BeTrue())
	})

	It("reads what it shows when it opens and when games change", func() {
		v, tt := tester(settingsSnapshot())
		fa := &fakeActions{}
		v.act = fa
		v.page = pageSettings
		tt.Frame()
		Expect(fa.settingsLoads).To(Equal(1))
		Expect(tt.HasText("Appearance")).To(BeFalse())
		v.setSettingsInfo(testSettingsInfo())
		tt.Frame()
		Expect(tt.HasText("Appearance")).To(BeTrue())
		Expect(fa.settingsLoads).To(Equal(1))
		v.setSnapshot(settingsSnapshot())
		tt.Frame()
		Expect(fa.settingsLoads).To(Equal(2))
		Expect(fa.usageLoads).To(Equal(1))
	})

	It("picks the name in the most games until the user picks names", func() {
		st := settingsTester()
		Expect(st.has("Alt#3")).To(BeTrue())
		Expect(st.has("Rare#4")).To(BeFalse())
		st.click("Pick automatically")
		st.click("Alt#3")
		st.click("Show all 4 names")
		Expect(st.has("Rare#4")).To(BeTrue())
		Expect(st.fa.calls).To(Equal([]string{store.SettingYou + "=" + me, store.SettingYou + "=" + me + "\nAlt#3"}))
	})

	It("keeps at least one name picked", func() {
		st := settingsTester()
		st.click("Pick automatically")
		st.click(me)
		Expect(st.fa.calls).To(Equal([]string{store.SettingYou + "=" + me}))
	})

	It("saves the theme, the colors and how the window closes", func() {
		st := settingsTester()
		st.click("Dark")
		st.click("Colorblind-safe colors")
		st.click("Quits logpose")
		Expect(st.fa.calls).To(Equal([]string{
			keyTheme + "=dark", keyColorblind + "=1", keyCloseQuits + "=1",
		}))
		Expect(st.fa.applied).To(HaveLen(3))
		Expect(st.fa.applied[2].closeQuits).To(BeTrue())
		var shown *ui.Theme
		ui.NewTester(func(c *ui.Context) {
			st.v.applyLook(c)
			shown = c.Theme()
		}, 100, 100).Frame()
		win, loss := resultColors(true)
		Expect(shown.Dark).To(BeTrue())
		Expect([]ui.Color{winColor(shown), lossColor(shown)}).To(Equal([]ui.Color{win, loss}))
	})

	It("starts with Windows, and only then offers to start hidden", func() {
		st := settingsTester()
		st.click("Start hidden in the tray")
		st.click("Start with Windows")
		st.click("Start hidden in the tray")
		Expect(st.fa.calls).To(Equal([]string{"login on", keyStartHidden + "=1"}))
	})

	Describe("card art", func() {
		artTester := func() settingsSpec {
			st := settingsTester()
			st.tt.SetSize(1180, 1100)
			st.tt.Frame()
			return st
		}
		used := arts.Usage{FullBytes: 141 << 20, FullCount: 1378, ThumbBytes: 117 << 20, ThumbCount: 2889}

		It("counts the folder when Settings opens, then shows its size", func() {
			st := artTester()
			Expect(st.fa.usageLoads).To(Equal(1))
			Expect(st.has("Counting…")).To(BeTrue())
			st.v.setArtUsage(used)
			Expect(st.has("258 MB on disk")).To(BeTrue())
			Expect(st.has("Thumbnails 117 MB · large images 141 MB of 150 MB")).To(BeTrue())
			Expect(st.fa.usageLoads).To(Equal(1))
		})

		It("shows the folder, clears it and says how much it freed", func() {
			st := artTester()
			st.click("Clear card art")
			Expect(st.fa.calls).To(BeEmpty())
			st.v.setArtUsage(used)
			st.click("Show the card art folder")
			st.click("Clear card art")
			Expect(st.fa.calls).To(Equal([]string{`open C:\Users\you\AppData\Roaming\logpose\art`, "clear art"}))
			st.v.settings.note(noteArt, "Cleared 258 MB.")
			st.v.setArtUsage(arts.Usage{})
			Expect(st.has("Cleared 258 MB.")).To(BeTrue())
			Expect(st.has("0 MB on disk")).To(BeTrue())
		})

		It("caps the large images at the size picked", func() {
			st := artTester()
			st.v.setArtUsage(used)
			st.click("300 MB")
			Expect(st.has("Thumbnails 117 MB · large images 141 MB of 300 MB")).To(BeTrue())
			Expect(st.fa.calls).To(Equal([]string{keyArtCap + "=300"}))
			Expect(st.fa.applied).To(HaveLen(1))
			Expect(st.fa.applied[0].artCap).To(Equal(300))
		})

		It("turns art downloads off and on", func() {
			st := artTester()
			st.click("Download card art")
			st.click("Download card art")
			Expect(st.fa.calls).To(Equal([]string{keyArtOff + "=1", keyArtOff + "="}))
			Expect(st.fa.applied).To(HaveLen(2))
			Expect([]bool{st.fa.applied[0].artOff, st.fa.applied[1].artOff}).To(Equal([]bool{true, false}))
		})

		It("offers only the switch without an art folder", func() {
			info := testSettingsInfo()
			info.ArtDir = ""
			_, tt, _ := settingsView(info)
			tt.SetSize(1180, 1100)
			tt.Frame()
			Expect(tt.HasText("Download card art")).To(BeTrue())
			Expect(tt.HasText("Counting…")).To(BeFalse())
			Expect(tt.HasText("Keep large images up to")).To(BeFalse())
		})
	})

	It("asks for the sim's folder when there's no sim and no games", func() {
		v, tt := tester(&snapshot{Loaded: true})
		fa := &fakeActions{}
		v.act = fa
		tt.Frame()
		Expect(tt.HasText("Where is OPTCGSim?")).To(BeTrue())
		Expect(tt.HasText("Looking for a running sim")).To(BeTrue())
		Expect(tt.Click("Choose folder…")).To(Succeed())
		Expect(fa.calls).To(Equal([]string{"choose sim"}))
	})
})
