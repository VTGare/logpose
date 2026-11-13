package gui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/gui/guitest"

	"github.com/VTGare/logpose/internal/gui/page"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

var _ = Describe("right-click menus", func() {
	It("gives a game on the Overview the Matches row's menu", func() {
		v, tt := tester(guitest.Snapshot(3))
		v.setActions(&fakeActions{})
		Expect(tt.RightClick("Open the match against Opp")).To(Succeed())
		guitest.Shows(tt, "Open match", "Watch replay", "Game type", "Add note…", "Bookmark", "Filter to games vs Opp")
		Expect(guitest.Choose(tt, "Open match")).To(Succeed())
		Expect(tt.HasText("Back to overview")).To(BeTrue())
		Expect(v.Page).To(Equal(page.Overview))
	})

	It("gives a game on the Matchups page the same menu", func() {
		v, tt := tester(guitest.Snapshot(3))
		v.Page = page.Matchups
		tt.Frame()
		Expect(tt.RightClick("Open the match against Opp")).To(Succeed())
		guitest.Shows(tt, "Open match", "Watch replay", "Filter to Monkey D. Luffy vs Kaido")
	})

	It("opens a deck on Matchups or its games in Matches", func() {
		v, tt := tester(guitest.Snapshot(3))
		Expect(tt.RightClick("Show Monkey D. Luffy")).To(Succeed())
		guitest.Shows(tt, "Open on Decks", "Open on Matchups", "Show in Matches", "Open in Cards", "View art")
		Expect(guitest.Choose(tt, "Open on Matchups")).To(Succeed())
		Expect(v.Page).To(Equal(page.Matchups))
		Expect(tt.Texts()).NotTo(ContainElement("All decks"))

		Expect(tt.RightClick("Show matchups with Monkey D. Luffy")).To(Succeed())
		Expect(guitest.Choose(tt, "Show in Matches")).To(Succeed())
		Expect(v.Page).To(Equal(page.Matches))
	})

	It("shows an opposing leader's games, or filters to them and stays", func() {
		v, tt := tester(guitest.Snapshot(3))
		Expect(tt.RightClick("Show games vs Kaido")).To(Succeed())
		guitest.Shows(tt, "Show in Matches", "Filter to games vs Kaido", "Open in Cards", "View art")
		Expect(guitest.Choose(tt, "Filter to games vs Kaido")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("vs Kaido"))
		Expect(v.Page).To(Equal(page.Overview))

		Expect(tt.Click("Remove the filter vs Kaido")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).NotTo(ContainElement("vs Kaido"))
		Expect(tt.RightClick("Show games vs Kaido")).To(Succeed())
		Expect(guitest.Choose(tt, "Show in Matches")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("vs Kaido"))
		Expect(v.Page).To(Equal(page.Matches))
	})

	It("puts both of the open matchup's links on a line under it", func() {
		v, tt := tester(guitest.Snapshot(3))
		v.Page = page.Matchups
		tt.Frame()
		Expect(tt.Click("Filter to games vs Kaido")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("vs Kaido"))
		Expect(v.Page).To(Equal(page.Matchups))
		Expect(tt.Click("Show in Matches")).To(Succeed())
		tt.Frame()
		Expect(v.Page).To(Equal(page.Matches))
		Expect(tt.Texts()).NotTo(ContainElement("All decks"))
	})

	It("opens a leader in Cards or its art over the page", func() {
		v, tt := tester(guitest.Snapshot(3))
		Expect(tt.RightClick("Show games vs Kaido")).To(Succeed())
		Expect(guitest.Choose(tt, "View art")).To(Succeed())
		Expect(tt.HasText("Close the viewer")).To(BeTrue())
		Expect(tt.HasText("OP17-058")).To(BeTrue())
		Expect(v.Page).To(Equal(page.Overview))
		Expect(tt.Click("Close the viewer")).To(Succeed())

		Expect(tt.RightClick("Show games vs Kaido")).To(Succeed())
		Expect(guitest.Choose(tt, "Open in Cards")).To(Succeed())
		Expect(v.Page).To(Equal(page.Cards))
		Expect(v.cards.State.Selected).To(Equal("OP17-058"))
		Expect(tt.HasText("Close the viewer")).To(BeFalse())
	})

	It("offers a card on a Decks tab its Cards page and its dealt hands", func() {
		v, tt := tester(guitest.HandsSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		openDecks(v, tt, "cards")
		r, ok := tt.Find("Otama")
		Expect(ok).To(BeTrue())
		tt.RightClickAt(r.X+2, r.Y+r.H/2)
		guitest.Shows(tt, "Open in Cards", "Show hands with Otama", "Art in the sim", "View art", "Save image…", "Copy image", "Open on Limitless")
		Expect(guitest.Choose(tt, "Show hands with Otama")).To(Succeed())
		Expect(tt.HasText("Keep or mulligan")).To(BeTrue())
		Expect(tt.HasText("you were dealt Otama")).To(BeTrue())
	})

	It("gives a build's row in the builds table the build chip's menu", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		_, tt := tester(guitest.DeckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.RightClick("Show Wano/Miracle")).To(Succeed())
		guitest.Shows(tt, "Edit core", "Rename", "Delete build")
	})

	It("clears one pill or every filter from a pill's menu", func() {
		_, tt := tester(guitest.Snapshot(3))
		Expect(tt.Click("Either turn")).To(Succeed())
		Expect(guitest.Choose(tt, "Going first")).To(Succeed())
		Expect(tt.RightClick("Going first")).To(Succeed())
		guitest.Shows(tt, "Clear turn order", "Clear all filters")
		Expect(guitest.Choose(tt, "Clear turn order")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("Either turn"))

		Expect(tt.Click("All decks")).To(Succeed())
		Expect(guitest.Choose(tt, "Monkey D. Luffy · 3 games")).To(Succeed())
		Expect(tt.Click("Either turn")).To(Succeed())
		Expect(guitest.Choose(tt, "Going second")).To(Succeed())
		Expect(tt.Texts()).NotTo(ContainElements("Either turn", "All decks"))
		Expect(tt.RightClick("Going second")).To(Succeed())
		Expect(guitest.Choose(tt, "Clear all filters")).To(Succeed())
		Expect(tt.Texts()).To(ContainElements("Either turn", "All decks"))
		Expect(tt.Texts()).NotTo(ContainElement("Clear"))
	})
})

var _ = Describe("the default format", func() {
	withFormats := func() *page.Snapshot {
		s := guitest.Snapshot(10)
		s.Formats = stats.Ranges([]store.Format{
			{ID: 1, Name: "Old format", Start: guitest.Now.Add(-48 * time.Hour)},
			{ID: 2, Name: "New format", Start: guitest.Now.Add(-4*time.Hour - 30*time.Minute)},
		})
		return s
	}

	It("starts on All formats until the user picks, and reads what they picked", func() {
		for stored, pill := range map[string]string{"": "All formats", page.StartLatest: "New format", page.StartAll: "All formats"} {
			v, tt := tester(withFormats())
			v.setPrefs(page.PrefsFrom(map[string]string{page.KeyStartFormat: stored}))
			tt.Frame()
			Expect(tt.Texts()).To(ContainElement(pill), stored)
			Expect(tt.HasText("Clear")).To(BeFalse(), stored)
		}
	})

	It("saves the choice under Start on without changing the format picked", func() {
		v, tt := tester(withFormats())
		fa := &fakeActions{}
		v.setActions(fa)
		Expect(tt.Click("All formats")).To(Succeed())
		Expect(guitest.Choose(tt, "Start on", "Latest format, now New format")).To(Succeed())
		Expect(fa.calls).To(Equal([]string{page.KeyStartFormat + "=" + page.StartLatest}))
		Expect(tt.Texts()).To(ContainElement("All formats"))
		Expect(tt.HasText("Clear")).To(BeTrue(), "All formats is away from the default now")
	})

	It("offers the same choice on the Formats page", func() {
		v, tt := tester(withFormats())
		fa := &fakeActions{}
		v.setActions(fa)
		v.Page = page.Formats
		tt.Frame()
		Expect(tt.HasText("Pages start on")).To(BeTrue())
		Expect(tt.Click("Latest format")).To(Succeed())
		tt.Frame()
		Expect(fa.calls).To(Equal([]string{page.KeyStartFormat + "=" + page.StartLatest}))
	})

	It("goes back to the default from Clear and from the format pill's menu", func() {
		v, tt := tester(withFormats())
		v.setPrefs(page.PrefsFrom(map[string]string{page.KeyStartFormat: page.StartLatest}))
		tt.Frame()
		Expect(tt.Click("New format")).To(Succeed())
		Expect(guitest.Choose(tt, "Old format · since Nov 4")).To(Succeed())
		Expect(tt.Click("Clear")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("New format"))
		Expect(tt.HasText("Clear")).To(BeFalse())

		Expect(tt.Click("New format")).To(Succeed())
		Expect(guitest.Choose(tt, "All formats")).To(Succeed())
		Expect(tt.RightClick("All formats")).To(Succeed())
		guitest.Shows(tt, "Back to New format", "Clear all filters")
		Expect(guitest.Choose(tt, "Back to New format")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("New format"))
		Expect(tt.Texts()).NotTo(ContainElement("All formats"))
	})

	It("greys out Back at the default", func() {
		_, tt := tester(withFormats())
		Expect(tt.RightClick("All formats")).To(Succeed())
		guitest.Shows(tt, "Back to All formats")
		Expect(guitest.Choose(tt, "Back to All formats")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("All formats"))
	})

	It("counts the latest format picked by its ID as the default", func() {
		v, tt := tester(withFormats())
		v.setPrefs(page.PrefsFrom(map[string]string{page.KeyStartFormat: page.StartLatest}))
		v.Filter.Format = 2
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("New format"))
		Expect(tt.HasText("Clear")).To(BeFalse())
		v.Filter.Format = 1
		tt.Frame()
		Expect(tt.HasText("Clear")).To(BeTrue())
	})

	It("counts the latest format and All formats as the same with no formats saved", func() {
		v, tt := tester(guitest.Snapshot(3))
		v.setPrefs(page.PrefsFrom(map[string]string{page.KeyStartFormat: page.StartAll}))
		v.Filter.Format = page.FormatCurrent
		tt.Frame()
		Expect(tt.HasText("Clear")).To(BeFalse())
	})
})

// openDecks opens a Decks tab the way -page does, since the sidebar's Cards hides the tab's.
func openDecks(v *view, tt *ui.Tester, tab string) {
	v.open("decks/" + tab)
	tt.Frame()
}
