package gui

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/shortcut"
	"github.com/VTGare/logpose/internal/store"
)

const offerTitle = "Add logpose to the Start menu and desktop?"

var _ = Describe("shortcuts", func() {
	offer := func() (*view, *fakeActions, func(string)) {
		v, tt := tester(testSnapshot(10))
		fa := &fakeActions{}
		v.act = fa
		v.prefs.canShortcut = true
		tt.Frame()
		click := func(label string) {
			GinkgoHelper()
			Expect(tt.Click(label)).To(Succeed())
			tt.Frame()
		}
		Expect(tt.HasText(offerTitle)).To(BeTrue())
		return v, fa, click
	}

	It("adds both shortcuts from the offer on the Overview and doesn't offer again", func() {
		v, fa, click := offer()
		click("Add")
		Expect(fa.calls).To(Equal([]string{"shortcut 0 true", "shortcut 1 true", store.SettingShortcutsOffered + "=1"}))
		Expect(v.prefs.shortcutsOffered).To(BeTrue())
	})

	It("adds only the places left ticked", func() {
		_, fa, click := offer()
		click("Desktop")
		click("Add")
		Expect(fa.calls).To(Equal([]string{"shortcut 0 true", store.SettingShortcutsOffered + "=1"}))
	})

	It("adds nothing on Not now", func() {
		_, fa, click := offer()
		click("Not now")
		Expect(fa.calls).To(Equal([]string{store.SettingShortcutsOffered + "=1"}))
	})

	It("doesn't offer what Windows can't make, or what's already there", func() {
		v, tt := tester(testSnapshot(10))
		tt.Frame()
		Expect(tt.HasText(offerTitle)).To(BeFalse())
		v.prefs.canShortcut = true
		v.prefs.shortcuts = [2]bool{shortcut.StartMenu: true, shortcut.Desktop: true}
		tt.Frame()
		Expect(tt.HasText(offerTitle)).To(BeFalse())
	})

	It("switches each shortcut in Settings", func() {
		st := settingsTester()
		st.tt.SetSize(1180, 1100)
		Expect(st.has("Start with Windows")).To(BeTrue())
		Expect(st.has("Desktop shortcut")).To(BeFalse())
		st.v.prefs.canShortcut = true
		st.v.prefs.shortcuts[shortcut.StartMenu] = true
		st.tt.Frame()
		st.click("Desktop shortcut")
		st.click("Start menu shortcut")
		Expect(st.fa.calls).To(Equal([]string{
			"shortcut 1 true", store.SettingShortcutsOffered + "=1", "shortcut 0 false",
		}))
	})
})
