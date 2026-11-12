package gui

import (
	"time"

	"github.com/egoist/mygo/ui"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

// choose clicks the items of the drawn menu shown along path, opening each submenu on the way.
func choose(tt *ui.Tester, path ...string) error {
	for _, label := range path {
		tt.Frame()
		if err := tt.Click(label); err != nil {
			return err
		}
	}
	tt.Frame()
	return nil
}

// shows reports each item the frame shows, for a drawn menu.
func shows(tt *ui.Tester, items ...string) {
	GinkgoHelper()
	tt.Frame()
	for _, item := range items {
		Expect(tt.HasText(item)).To(BeTrue(), item)
	}
}

var _ = Describe("right-click menus", func() {
	It("gives a game on the Overview the Matches row's menu", func() {
		v, tt := tester(testSnapshot(3))
		v.act = &fakeActions{}
		Expect(tt.RightClick("Open the match against Opp")).To(Succeed())
		shows(tt, "Open match", "Watch replay", "Game type", "Add note…", "Bookmark", "Filter to games vs Opp")
		Expect(choose(tt, "Open match")).To(Succeed())
		Expect(tt.HasText("Back to overview")).To(BeTrue())
		Expect(v.page).To(Equal(pageOverview))
	})

	It("gives a game on the Matchups page the same menu", func() {
		v, tt := tester(testSnapshot(3))
		v.page = pageMatchups
		tt.Frame()
		Expect(tt.RightClick("Open the match against Opp")).To(Succeed())
		shows(tt, "Open match", "Watch replay", "Filter to Monkey D. Luffy vs Kaido")
	})

	It("opens a deck on Matchups or its games in Matches", func() {
		v, tt := tester(testSnapshot(3))
		Expect(tt.RightClick("Show Monkey D. Luffy")).To(Succeed())
		shows(tt, "Open on Decks", "Open on Matchups", "Show in Matches", "Open in Cards", "View art")
		Expect(choose(tt, "Open on Matchups")).To(Succeed())
		Expect(v.page).To(Equal(pageMatchups))
		Expect(tt.Texts()).NotTo(ContainElement("All decks"))

		Expect(tt.RightClick("Show matchups with Monkey D. Luffy")).To(Succeed())
		Expect(choose(tt, "Show in Matches")).To(Succeed())
		Expect(v.page).To(Equal(pageMatches))
	})

	It("shows an opposing leader's games, or filters to them and stays", func() {
		v, tt := tester(testSnapshot(3))
		Expect(tt.RightClick("Show games vs Kaido")).To(Succeed())
		shows(tt, "Show in Matches", "Filter to games vs Kaido", "Open in Cards", "View art")
		Expect(choose(tt, "Filter to games vs Kaido")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("vs Kaido"))
		Expect(v.page).To(Equal(pageOverview))

		Expect(tt.Click("Remove the filter vs Kaido")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).NotTo(ContainElement("vs Kaido"))
		Expect(tt.RightClick("Show games vs Kaido")).To(Succeed())
		Expect(choose(tt, "Show in Matches")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("vs Kaido"))
		Expect(v.page).To(Equal(pageMatches))
	})

	It("puts both of the open matchup's links on a line under it", func() {
		v, tt := tester(testSnapshot(3))
		v.page = pageMatchups
		tt.Frame()
		Expect(tt.Click("Filter to games vs Kaido")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("vs Kaido"))
		Expect(v.page).To(Equal(pageMatchups))
		Expect(tt.Click("Show in Matches")).To(Succeed())
		tt.Frame()
		Expect(v.page).To(Equal(pageMatches))
		Expect(tt.Texts()).NotTo(ContainElement("All decks"))
	})

	It("opens a leader in Cards or its art over the page", func() {
		v, tt := tester(testSnapshot(3))
		Expect(tt.RightClick("Show games vs Kaido")).To(Succeed())
		Expect(choose(tt, "View art")).To(Succeed())
		Expect(tt.HasText("Close the viewer")).To(BeTrue())
		Expect(tt.HasText("OP17-058")).To(BeTrue())
		Expect(v.page).To(Equal(pageOverview))
		Expect(tt.Click("Close the viewer")).To(Succeed())

		Expect(tt.RightClick("Show games vs Kaido")).To(Succeed())
		Expect(choose(tt, "Open in Cards")).To(Succeed())
		Expect(v.page).To(Equal(pageCards))
		Expect(v.cardsUI.selected).To(Equal("OP17-058"))
		Expect(tt.HasText("Close the viewer")).To(BeFalse())
	})

	It("offers a card on a Decks tab its Cards page and its dealt hands", func() {
		v, tt := tester(handsSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		openDecks(v, tt, "cards")
		r, ok := tt.Find("Otama")
		Expect(ok).To(BeTrue())
		tt.RightClickAt(r.X+2, r.Y+r.H/2)
		shows(tt, "Open in Cards", "Show hands with Otama", "Art in the sim", "View art", "Save image…", "Copy image", "Open on Limitless")
		Expect(choose(tt, "Show hands with Otama")).To(Succeed())
		Expect(tt.HasText("Keep or mulligan")).To(BeTrue())
		Expect(tt.HasText("you were dealt Otama")).To(BeTrue())
	})

	It("gives a build's row in the builds table the build chip's menu", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.RightClick("Show Wano/Miracle")).To(Succeed())
		shows(tt, "Edit core", "Rename", "Delete build")
	})

	It("clears one pill or every filter from a pill's menu", func() {
		_, tt := tester(testSnapshot(3))
		Expect(tt.Click("Either turn")).To(Succeed())
		Expect(choose(tt, "Going first")).To(Succeed())
		Expect(tt.RightClick("Going first")).To(Succeed())
		shows(tt, "Clear turn order", "Clear all filters")
		Expect(choose(tt, "Clear turn order")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("Either turn"))

		Expect(tt.Click("All decks")).To(Succeed())
		Expect(choose(tt, "Monkey D. Luffy · 3 games")).To(Succeed())
		Expect(tt.Click("Either turn")).To(Succeed())
		Expect(choose(tt, "Going second")).To(Succeed())
		Expect(tt.Texts()).NotTo(ContainElements("Either turn", "All decks"))
		Expect(tt.RightClick("Going second")).To(Succeed())
		Expect(choose(tt, "Clear all filters")).To(Succeed())
		Expect(tt.Texts()).To(ContainElements("Either turn", "All decks"))
		Expect(tt.Texts()).NotTo(ContainElement("Clear"))
	})
})

var _ = Describe("the default format", func() {
	withFormats := func() *snapshot {
		s := testSnapshot(10)
		s.Formats = stats.Ranges([]store.Format{
			{ID: 1, Name: "Old format", Start: now.Add(-48 * time.Hour)},
			{ID: 2, Name: "New format", Start: now.Add(-4*time.Hour - 30*time.Minute)},
		})
		return s
	}

	It("starts on All formats until the user picks, and reads what they picked", func() {
		for stored, pill := range map[string]string{"": "All formats", startLatest: "New format", startAll: "All formats"} {
			v, tt := tester(withFormats())
			v.setPrefs(prefsFrom(map[string]string{keyStartFormat: stored}))
			tt.Frame()
			Expect(tt.Texts()).To(ContainElement(pill), stored)
			Expect(tt.HasText("Clear")).To(BeFalse(), stored)
		}
	})

	It("saves the choice under Start on without changing the format picked", func() {
		v, tt := tester(withFormats())
		fa := &fakeActions{}
		v.act = fa
		Expect(tt.Click("All formats")).To(Succeed())
		Expect(choose(tt, "Start on", "Latest format, now New format")).To(Succeed())
		Expect(fa.calls).To(Equal([]string{keyStartFormat + "=" + startLatest}))
		Expect(tt.Texts()).To(ContainElement("All formats"))
		Expect(tt.HasText("Clear")).To(BeTrue(), "All formats is away from the default now")
	})

	It("offers the same choice on the Formats page", func() {
		v, tt := tester(withFormats())
		fa := &fakeActions{}
		v.act = fa
		v.page = pageFormats
		tt.Frame()
		Expect(tt.HasText("Pages start on")).To(BeTrue())
		Expect(tt.Click("Latest format")).To(Succeed())
		tt.Frame()
		Expect(fa.calls).To(Equal([]string{keyStartFormat + "=" + startLatest}))
	})

	It("goes back to the default from Clear and from the format pill's menu", func() {
		v, tt := tester(withFormats())
		v.setPrefs(prefsFrom(map[string]string{keyStartFormat: startLatest}))
		tt.Frame()
		Expect(tt.Click("New format")).To(Succeed())
		Expect(choose(tt, "Old format · since Nov 4")).To(Succeed())
		Expect(tt.Click("Clear")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("New format"))
		Expect(tt.HasText("Clear")).To(BeFalse())

		Expect(tt.Click("New format")).To(Succeed())
		Expect(choose(tt, "All formats")).To(Succeed())
		Expect(tt.RightClick("All formats")).To(Succeed())
		shows(tt, "Back to New format", "Clear all filters")
		Expect(choose(tt, "Back to New format")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("New format"))
		Expect(tt.Texts()).NotTo(ContainElement("All formats"))
	})

	It("greys out Back at the default", func() {
		_, tt := tester(withFormats())
		Expect(tt.RightClick("All formats")).To(Succeed())
		shows(tt, "Back to All formats")
		Expect(choose(tt, "Back to All formats")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("All formats"))
	})

	It("counts the latest format picked by its ID as the default", func() {
		v, tt := tester(withFormats())
		v.setPrefs(prefsFrom(map[string]string{keyStartFormat: startLatest}))
		v.filter.format = 2
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("New format"))
		Expect(tt.HasText("Clear")).To(BeFalse())
		v.filter.format = 1
		tt.Frame()
		Expect(tt.HasText("Clear")).To(BeTrue())
	})

	It("counts the latest format and All formats as the same with no formats saved", func() {
		v, tt := tester(testSnapshot(3))
		v.setPrefs(prefsFrom(map[string]string{keyStartFormat: startAll}))
		v.filter.format = formatCurrent
		tt.Frame()
		Expect(tt.HasText("Clear")).To(BeFalse())
	})
})
