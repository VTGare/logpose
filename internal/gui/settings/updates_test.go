package settings

import (
	"time"

	"github.com/VTGare/logpose/internal/gui/guitest"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/egoist/mygo/ui"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const testNotes = `- Tiles fold away, and remember it.
- Tables sort by any column,
  and columns resize.

A **wrapped** paragraph.`

var _ = Describe("updates", func() {
	sidebar := func(u UpdateState) (*view, *ui.Tester, *fakeActions) {
		v, tt := tester(guitest.Snapshot(4))
		fa := &fakeActions{}
		v.setActions(fa)
		v.settings.SetUpdate(u)
		tt.Frame()
		return v, tt, fa
	}
	ready := UpdateState{Can: true, Current: "0.1.0", Phase: UpdateReady, Next: "0.1.1", Notes: testNotes}

	It("shows nothing in the sidebar while up to date", func() {
		_, tt, _ := sidebar(UpdateState{Can: true, Current: "0.1.0"})
		Expect(tt.HasText("Restart")).To(BeFalse())
		Expect(tt.HasText("What's new")).To(BeFalse())
	})

	It("offers a restart once the next version is in place", func() {
		_, tt, fa := sidebar(ready)
		Expect(tt.HasText("0.1.1 is ready")).To(BeTrue())
		Expect(tt.Click("Restart")).To(Succeed())
		tt.Frame()
		Expect(fa.Calls).To(Equal([]string{"restart to update"}))
	})

	It("shows the release notes from the sidebar", func() {
		_, tt, _ := sidebar(ready)
		Expect(tt.Click("What's new in 0.1.1")).To(Succeed())
		tt.Frame()
		for _, text := range []string{"logpose 0.1.1", "You have 0.1.0.", "Tiles fold away, and remember it.", "Tables sort by any column, and columns resize.", "A wrapped paragraph."} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("downloads a version found when the user downloads updates themselves", func() {
		_, tt, fa := sidebar(UpdateState{Can: true, Current: "0.1.0", Phase: UpdateFound, Next: "0.1.1"})
		Expect(tt.HasText("0.1.1 is out")).To(BeTrue())
		Expect(tt.Click("Get it")).To(Succeed())
		tt.Frame()
		Expect(fa.Calls).To(Equal([]string{"install update"}))
		Expect(tt.HasText("Getting 0.1.1 · 0%")).To(BeTrue())
		Expect(tt.HasText("Get it")).To(BeFalse())
	})

	It("shows an icon on the rail", func() {
		v, tt, fa := sidebar(ready)
		v.setPrefs(page.PrefsFrom(map[string]string{page.KeySidebarWidth: "90"}))
		tt.Frame()
		Expect(tt.Click("0.1.1 is ready")).To(Succeed())
		tt.Frame()
		Expect(fa.Calls).To(Equal([]string{"restart to update"}))
	})

	It("shows the version and checks from Settings, and stores the download choice", func() {
		st := settingsTester()
		st.v.settings.SetUpdate(UpdateState{Can: true, Current: "0.1.0", Checked: guitest.Now.Add(-25 * time.Hour)})
		Expect(st.has("Version 0.1.0")).To(BeTrue())
		Expect(st.has("Up to date · checked yesterday at 19:00")).To(BeTrue())
		st.click("Check now")
		Expect(st.has("Checking…")).To(BeTrue())
		st.click("Download updates by itself")
		Expect(st.fa.Calls).To(Equal([]string{"check update", page.KeyUpdateManual + "=1"}))
	})

	It("says when this copy can't update itself", func() {
		st := settingsTester()
		st.v.settings.SetUpdate(UpdateState{Current: "0.1.0"})
		Expect(st.has("Only logpose installed with its installer updates itself")).To(BeTrue())
		Expect(st.has("Check now")).To(BeFalse())
	})

	It("doesn't offer shortcuts to a copy its installer put in place", func() {
		v, tt := tester(guitest.Snapshot(10))
		v.setSystem(System{CanShortcut: true, Installed: true})
		tt.Frame()
		Expect(tt.HasText(offerTitle)).To(BeFalse())
	})
})
