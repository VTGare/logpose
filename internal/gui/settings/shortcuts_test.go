package settings

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/gui/guitest"

	"github.com/VTGare/logpose/internal/gui/page"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/shortcut"
)

const offerTitle = "Add logpose to the Start menu and desktop?"

var _ = Describe("shortcuts", func() {
	offer := func() (*ui.Tester, *fakeActions, func(string)) {
		v, tt := tester(guitest.Snapshot(10))
		fa := &fakeActions{}
		v.setActions(fa)
		v.setSystem(System{CanShortcut: true})
		tt.Frame()
		click := func(label string) {
			GinkgoHelper()
			Expect(tt.Click(label)).To(Succeed())
			tt.Frame()
		}
		Expect(tt.HasText(offerTitle)).To(BeTrue())
		return tt, fa, click
	}

	It("adds both shortcuts from the offer on the Overview and doesn't offer again", func() {
		tt, fa, click := offer()
		click("Add")
		Expect(fa.Calls).To(Equal([]string{"start menu shortcut true", "desktop shortcut true", page.KeyShortcutsOffered + "=1"}))
		Expect(tt.HasText(offerTitle)).To(BeFalse())
	})

	It("adds only the places left ticked", func() {
		_, fa, click := offer()
		click("Desktop")
		click("Add")
		Expect(fa.Calls).To(Equal([]string{"start menu shortcut true", page.KeyShortcutsOffered + "=1"}))
	})

	It("adds nothing on Not now", func() {
		_, fa, click := offer()
		click("Not now")
		Expect(fa.Calls).To(Equal([]string{page.KeyShortcutsOffered + "=1"}))
	})

	It("doesn't offer what Windows can't make, or what's already there", func() {
		v, tt := tester(guitest.Snapshot(10))
		tt.Frame()
		Expect(tt.HasText(offerTitle)).To(BeFalse())
		v.setSystem(System{CanShortcut: true, Shortcuts: [2]bool{shortcut.StartMenu: true, shortcut.Desktop: true}})
		tt.Frame()
		Expect(tt.HasText(offerTitle)).To(BeFalse())
	})

	It("switches each shortcut in Settings", func() {
		st := settingsTester()
		st.tt.SetSize(1180, 1100)
		Expect(st.has("Start with Windows")).To(BeTrue())
		Expect(st.has("Desktop shortcut")).To(BeFalse())
		st.v.setSystem(System{CanShortcut: true, Shortcuts: [2]bool{shortcut.StartMenu: true}})
		st.tt.Frame()
		st.click("Desktop shortcut")
		st.click("Start menu shortcut")
		Expect(st.fa.Calls).To(Equal([]string{
			"desktop shortcut true", page.KeyShortcutsOffered + "=1", "start menu shortcut false",
		}))
	})
})
