package gui

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// tableTester opens the Matchups page with fake actions, so layout changes are stored.
func tableTester() (*view, *ui.Tester, *fakeActions) {
	v, tt := tester(matchupsSnapshot())
	act := &fakeActions{}
	v.act = act
	Expect(tt.Click("Matchups")).To(Succeed())
	return v, tt, act
}

func center(tt *ui.Tester, label string) (float32, float32) {
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

var _ = Describe("Tables", func() {
	It("sorts by a column on a click, turns it around on the next, and stores the sort", func() {
		v, tt, act := tableTester()
		Expect(tt.Click("Sort by Win rate")).To(Succeed())
		tt.Frame()
		lay := v.prefs.tables["matchups"]
		Expect(lay.Sort).To(Equal("rate"))
		Expect(lay.Desc).To(BeTrue())
		Expect(tt.Click("Sort by Win rate")).To(Succeed())
		tt.Frame()
		Expect(lay.Desc).To(BeFalse())
		act.fire()
		Expect(act.calls).To(ContainElement(ContainSubstring(`tables={"matchups":{"sort":"rate"}}`)))
	})

	It("sorts names from A and numbers from the most", func() {
		v, tt, _ := tableTester()
		Expect(tt.Click("Sort by 2 leaders faced")).To(Succeed())
		tt.Frame()
		Expect(v.prefs.tables["matchups"].Desc).To(BeFalse())
	})

	It("moves a column dragged onto another", func() {
		v, tt, _ := tableTester()
		x0, y0 := center(tt, "Sort by Last faced")
		x1, y1 := center(tt, "Sort by Games")
		drag(tt, x0, y0, x1, y1)
		Expect(v.prefs.tables["matchups"].Order).To(HaveExactElements("last", "games", "rate", "first", "second", "squares"))
		Expect(v.matchupTable(2).cols[1].id).To(Equal("last"))
	})

	It("resizes a column by its edge, undoes it on a double click, and resets everything from the menu", func() {
		v, tt, _ := tableTester()
		x, y := center(tt, "Resize Games")
		drag(tt, x, y, x+40, y)
		lay := v.prefs.tables["matchups"]
		Expect(lay.Widths["games"]).To(BeNumerically("~", muGamesW+40, 2))

		x, y = center(tt, "Resize Games")
		tt.ClickAt(x, y)
		tt.ClickAt(x, y)
		tt.Frame()
		Expect(lay.Widths).NotTo(HaveKey("games"))

		Expect(tt.Click("Sort by Games")).To(Succeed())
		tt.Frame()
		Expect(tt.RightClick("Sort by Games")).To(Succeed())
		Expect(choose(tt, "Reset columns")).To(Succeed())
		tt.Frame()
		Expect(*v.prefs.tables["matchups"]).To(Equal(tableLayout{}))
	})

	It("keeps new columns at their place and ignores ones it doesn't know", func() {
		v := newView(nil)
		v.prefs.tables = tablesFrom(`{"t":{"order":["c","gone"],"widths":{"c":90,"gone":10,"a":5}}}`)
		tv := v.arrangeTable("t", 8, []tableCol{{id: "a"}, {id: "b", width: 50}, {id: "c", width: 60}}, "", false)
		Expect(tv.cols).To(HaveLen(3))
		Expect([]string{tv.cols[0].id, tv.cols[1].id, tv.cols[2].id}).To(Equal([]string{"a", "c", "b"}))
		Expect(tv.cols[0].width).To(BeZero())
		Expect(tv.cols[1].width).To(BeNumerically("==", 90))
	})

	It("sorts rows with too few games last either way", func() {
		type row struct {
			name string
			r    stats.Record
		}
		rows := []row{{"few", stats.Record{Wins: 1}}, {"low", stats.Record{Wins: 1, Losses: 4}}, {"high", stats.Record{Wins: 4, Losses: 1}}}
		by := map[string]func(a, b row) int{"rate": func(a, b row) int {
			x, _ := a.r.Rate()
			y, _ := b.r.Rate()
			return int((x - y) * 100)
		}}
		few := map[string]func(row) bool{"rate": func(r row) bool { return r.r.Games() < stats.MinGames }}
		names := func() []string { return []string{rows[0].name, rows[1].name, rows[2].name} }
		sortRows(rows, "rate", true, by, few, nil)
		Expect(names()).To(Equal([]string{"high", "low", "few"}))
		sortRows(rows, "rate", false, by, few, nil)
		Expect(names()).To(Equal([]string{"low", "high", "few"}))
	})
})

var _ = Describe("Collapsible tiles", func() {
	It("collapses a tile to its summary, stores it, and opens it again", func() {
		v, tt := handsTester()
		act := &fakeActions{}
		v.act = act
		Expect(tt.Click("Collapse Keep or mulligan")).To(Succeed())
		tt.Frame()
		Expect(v.prefs.folded).To(HaveKey("hands.keep"))
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
		Expect(foldedFrom("a,,b")).To(Equal(map[string]bool{"a": true, "b": true}))
		Expect(foldedFrom("")).To(BeEmpty())
	})
})
