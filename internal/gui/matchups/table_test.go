package matchups

import (
	"fmt"

	"github.com/VTGare/logpose/internal/gui/guitest"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/egoist/mygo/ui"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func above(tt *ui.Tester, a, b string) bool {
	GinkgoHelper()
	_, ya := guitest.Center(tt, a)
	_, yb := guitest.Center(tt, b)
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
		act.Fire()
		Expect(act.Calls).To(Equal([]string{`tables={"matchups":{"sort":"rate","desc":true}}`}))
		Expect(tt.Click("Sort by Win rate")).To(Succeed())
		tt.Frame()
		act.Fire()
		Expect(act.Calls).To(ContainElement(`tables={"matchups":{"sort":"rate"}}`))
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
		x0, y0 := guitest.Center(tt, "Sort by Last faced")
		x1, y1 := guitest.Center(tt, "Sort by Games")
		guitest.Drag(tt, x0, y0, x1, y1)
		last, _ := guitest.Center(tt, "Sort by Last faced")
		games, _ := guitest.Center(tt, "Sort by Games")
		Expect(last).To(BeNumerically("<", games))
		act.Fire()
		Expect(act.Calls).To(ContainElement(ContainSubstring(`"order":["last","games","rate","first","second","squares"]`)))
	})

	It("resizes a column by its edge, undoes it on a double click, and resets everything from the menu", func() {
		_, tt, act := tableTester()
		w := headerWidth(tt, "Sort by Games")
		x, y := guitest.Center(tt, "Resize Games")
		guitest.Drag(tt, x, y, x+40, y)
		Expect(headerWidth(tt, "Sort by Games")).To(BeNumerically("~", w+40, 2))

		x, y = guitest.Center(tt, "Resize Games")
		tt.ClickAt(x, y)
		tt.ClickAt(x, y)
		tt.Frame()
		Expect(headerWidth(tt, "Sort by Games")).To(BeNumerically("~", w, 0.5))

		Expect(tt.Click("Sort by Games")).To(Succeed())
		guitest.Drag(tt, x, y, x+40, y)
		Expect(tt.RightClick("Sort by Games")).To(Succeed())
		Expect(guitest.Choose(tt, "Reset columns")).To(Succeed())
		tt.Frame()
		Expect(headerWidth(tt, "Sort by Games")).To(BeNumerically("~", w, 0.5))
		act.Fire()
		Expect(act.Calls[len(act.Calls)-1]).To(Equal(`tables={"matchups":{}}`))
	})

	It("keeps new columns at their place and ignores ones it doesn't know", func() {
		v, tt, _ := tableTester()
		games := headerWidth(tt, "Sort by Games")
		stored := func(width int) {
			v.setPrefs(page.PrefsFrom(map[string]string{page.KeyTables: fmt.Sprintf(`{"matchups":{"order":["last","gone"],"widths":{"gone":10,"last":%d}}}`, width)}))
			tt.Frame()
		}
		stored(90)
		last, _ := guitest.Center(tt, "Sort by Last faced")
		first, _ := guitest.Center(tt, "Sort by Games")
		Expect(last).To(BeNumerically("<", first))
		Expect(headerWidth(tt, "Sort by Games")).To(BeNumerically("~", games, 0.5))
		narrow := headerWidth(tt, "Sort by Last faced")
		stored(130)
		Expect(headerWidth(tt, "Sort by Last faced")).To(BeNumerically("~", narrow+40, 0.5))
	})
})
