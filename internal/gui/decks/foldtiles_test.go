package decks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/gui/page"
)

var _ = Describe("Collapsible tiles", func() {
	It("collapses a tile to its summary, stores it, and opens it again", func() {
		v, tt := handsTester()
		act := &fakeActions{}
		v.setActions(act)
		Expect(tt.Click("Collapse Keep or mulligan")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Kept 100% (5–0) · mulliganed 0% (0–5)")).To(BeTrue())
		Expect(tt.HasText("Kept the first five")).To(BeFalse())
		Expect(act.Calls).To(ContainElement("folded_tiles=hands.keep"))

		Expect(tt.Click("Expand Keep or mulligan")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Kept the first five")).To(BeTrue())
		Expect(tt.HasText("Kept 100% (5–0) · mulliganed 0% (0–5)")).To(BeFalse())
		Expect(act.Calls).To(ContainElement("folded_tiles="))
	})

	It("reads the collapsed tiles back", func() {
		v, tt := handsTester()
		v.setPrefs(page.PrefsFrom(map[string]string{page.KeyFolded: "a,,hands.keep"}))
		tt.Frame()
		Expect(tt.HasText("Kept 100% (5–0) · mulliganed 0% (0–5)")).To(BeTrue())
		Expect(tt.HasText("Kept the first five")).To(BeFalse())
	})
})
