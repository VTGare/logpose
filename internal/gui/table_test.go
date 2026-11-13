package gui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// tableTester opens the Matchups page with fake actions, so layout changes are stored.
func tableTester() (*view, *ui.Tester, *fakeActions) {
	v, tt := tester(matchupsSnapshot())
	act := &fakeActions{}
	v.setActions(act)
	Expect(tt.Click("Matchups")).To(Succeed())
	return v, tt, act
}

func center(tt *ui.Tester, label string) (float32, float32) {
	GinkgoHelper()
	r, ok := tt.Find(label)
	Expect(ok).To(BeTrue(), label)
	return r.X + r.W/2, r.Y + r.H/2
}

// drag presses at one point and moves to another in steps, a frame each, then lets go.
func drag(tt *ui.Tester, x0, y0, x1, y1 float32) {
	tt.Press(x0, y0)
	tt.Frame()
	for i := 1; i <= 8; i++ {
		f := float32(i) / 8
		tt.Move(x0+(x1-x0)*f, y0+(y1-y0)*f)
		tt.Frame()
	}
	tt.Release(x1, y1)
	tt.Frame()
}

func above(tt *ui.Tester, a, b string) bool {
	GinkgoHelper()
	_, ya := center(tt, a)
	_, yb := center(tt, b)
	return ya < yb
}

func headerWidth(tt *ui.Tester, label string) float32 {
	GinkgoHelper()
	r, ok := tt.Find(label)
	Expect(ok).To(BeTrue(), label)
	return r.W
}

const (
	kaidoRow = "Show the matchup vs Kaido OP17"
	buggyRow = "Show the matchup vs Buggy OP09"
)

var _ = Describe("Tables", func() {
	It("sorts by a column on a click, turns it around on the next, and stores the sort", func() {
		_, tt, act := tableTester()
		Expect(tt.Click("Sort by Win rate")).To(Succeed())
		tt.Frame()
		act.fire()
		Expect(act.calls).To(Equal([]string{`tables={"matchups":{"sort":"rate","desc":true}}`}))
		Expect(tt.Click("Sort by Win rate")).To(Succeed())
		tt.Frame()
		act.fire()
		Expect(act.calls).To(ContainElement(`tables={"matchups":{"sort":"rate"}}`))
	})

	It("sorts rows with too few games last either way", func() {
		_, tt, _ := tableTester()
		for range 2 {
			Expect(tt.Click("Sort by Win rate")).To(Succeed())
			tt.Frame()
			Expect(above(tt, kaidoRow, buggyRow)).To(BeTrue())
		}
	})

	It("sorts names from A and numbers from the most", func() {
		_, tt, _ := tableTester()
		Expect(tt.Click("Sort by 2 leaders faced")).To(Succeed())
		tt.Frame()
		Expect(above(tt, buggyRow, kaidoRow)).To(BeTrue())
		Expect(tt.Click("Sort by Games")).To(Succeed())
		tt.Frame()
		Expect(above(tt, kaidoRow, buggyRow)).To(BeTrue())
	})

	It("moves a column dragged onto another, and stores the order", func() {
		_, tt, act := tableTester()
		x0, y0 := center(tt, "Sort by Last faced")
		x1, y1 := center(tt, "Sort by Games")
		drag(tt, x0, y0, x1, y1)
		last, _ := center(tt, "Sort by Last faced")
		games, _ := center(tt, "Sort by Games")
		Expect(last).To(BeNumerically("<", games))
		act.fire()
		Expect(act.calls).To(ContainElement(ContainSubstring(`"order":["last","games","rate","first","second","squares"]`)))
	})

	It("resizes a column by its edge, undoes it on a double click, and resets everything from the menu", func() {
		_, tt, act := tableTester()
		w := headerWidth(tt, "Sort by Games")
		x, y := center(tt, "Resize Games")
		drag(tt, x, y, x+40, y)
		Expect(headerWidth(tt, "Sort by Games")).To(BeNumerically("~", w+40, 2))

		x, y = center(tt, "Resize Games")
		tt.ClickAt(x, y)
		tt.ClickAt(x, y)
		tt.Frame()
		Expect(headerWidth(tt, "Sort by Games")).To(BeNumerically("~", w, 0.5))

		Expect(tt.Click("Sort by Games")).To(Succeed())
		drag(tt, x, y, x+40, y)
		Expect(tt.RightClick("Sort by Games")).To(Succeed())
		Expect(choose(tt, "Reset columns")).To(Succeed())
		tt.Frame()
		Expect(headerWidth(tt, "Sort by Games")).To(BeNumerically("~", w, 0.5))
		act.fire()
		Expect(act.calls[len(act.calls)-1]).To(Equal(`tables={"matchups":{}}`))
	})

	It("keeps new columns at their place and ignores ones it doesn't know", func() {
		v, tt, _ := tableTester()
		games := headerWidth(tt, "Sort by Games")
		stored := func(width int) {
			v.setPrefs(prefsFrom(map[string]string{keyTables: fmt.Sprintf(`{"matchups":{"order":["last","gone"],"widths":{"gone":10,"last":%d}}}`, width)}))
			tt.Frame()
		}
		stored(90)
		last, _ := center(tt, "Sort by Last faced")
		first, _ := center(tt, "Sort by Games")
		Expect(last).To(BeNumerically("<", first))
		Expect(headerWidth(tt, "Sort by Games")).To(BeNumerically("~", games, 0.5))
		narrow := headerWidth(tt, "Sort by Last faced")
		stored(130)
		Expect(headerWidth(tt, "Sort by Last faced")).To(BeNumerically("~", narrow+40, 0.5))
	})
})

var _ = Describe("Collapsible tiles", func() {
	It("collapses a tile to its summary, stores it, and opens it again", func() {
		v, tt := handsTester()
		act := &fakeActions{}
		v.setActions(act)
		Expect(tt.Click("Collapse Keep or mulligan")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Kept 100% (5–0) · mulliganed 0% (0–5)")).To(BeTrue())
		Expect(tt.HasText("Kept the first five")).To(BeFalse())
		Expect(act.calls).To(ContainElement("folded_tiles=hands.keep"))

		Expect(tt.Click("Expand Keep or mulligan")).To(Succeed())
		tt.Frame()
		Expect(tt.HasText("Kept the first five")).To(BeTrue())
		Expect(tt.HasText("Kept 100% (5–0) · mulliganed 0% (0–5)")).To(BeFalse())
		Expect(act.calls).To(ContainElement("folded_tiles="))
	})

	It("reads the collapsed tiles back", func() {
		v, tt := handsTester()
		v.setPrefs(prefsFrom(map[string]string{keyFolded: "a,,hands.keep"}))
		tt.Frame()
		Expect(tt.HasText("Kept 100% (5–0) · mulliganed 0% (0–5)")).To(BeTrue())
		Expect(tt.HasText("Kept the first five")).To(BeFalse())
	})
})
