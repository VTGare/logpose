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
		Expect(v.detail.fingerprint).NotTo(BeEmpty())
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
		Expect(v.filter.deck).To(Equal("OP13-001"))

		Expect(tt.RightClick("Show matchups with Monkey D. Luffy")).To(Succeed())
		Expect(choose(tt, "Show in Matches")).To(Succeed())
		Expect(v.page).To(Equal(pageMatches))
	})

	It("shows an opposing leader's games, or filters to them and stays", func() {
		v, tt := tester(testSnapshot(3))
		Expect(tt.RightClick("Show games vs Kaido")).To(Succeed())
		shows(tt, "Show in Matches", "Filter to games vs Kaido", "Open in Cards", "View art")
		Expect(choose(tt, "Filter to games vs Kaido")).To(Succeed())
		Expect(v.filter.oppLeader).To(Equal("OP17-058"))
		Expect(v.page).To(Equal(pageOverview))

		v.filter.oppLeader = ""
		Expect(tt.RightClick("Show games vs Kaido")).To(Succeed())
		Expect(choose(tt, "Show in Matches")).To(Succeed())
		Expect(v.filter.oppLeader).To(Equal("OP17-058"))
		Expect(v.page).To(Equal(pageMatches))
	})

	It("puts both of the open matchup's links on a line under it", func() {
		v, tt := tester(testSnapshot(3))
		v.page = pageMatchups
		tt.Frame()
		Expect(tt.Click("Filter to games vs Kaido")).To(Succeed())
		Expect(v.filter.oppLeader).To(Equal("OP17-058"))
		Expect(v.page).To(Equal(pageMatchups))
		Expect(tt.Click("Show in Matches")).To(Succeed())
		Expect(v.page).To(Equal(pageMatches))
		Expect(v.filter.deck).To(Equal("OP13-001"))
	})

	It("opens a leader in Cards or its art over the page", func() {
		v, tt := tester(testSnapshot(3))
		Expect(tt.RightClick("Show games vs Kaido")).To(Succeed())
		Expect(choose(tt, "View art")).To(Succeed())
		Expect(v.cardsUI.viewer).To(BeTrue())
		Expect(v.cardsUI.selected).To(Equal("OP17-058"))
		Expect(tt.HasText("Close the viewer")).To(BeTrue())
		Expect(v.page).To(Equal(pageOverview))
		Expect(tt.Click("Close the viewer")).To(Succeed())

		Expect(tt.RightClick("Show games vs Kaido")).To(Succeed())
		Expect(choose(tt, "Open in Cards")).To(Succeed())
		Expect(v.page).To(Equal(pageCards))
		Expect(v.cardsUI.selected).To(Equal("OP17-058"))
		Expect(v.cardsUI.viewer).To(BeFalse())
	})

	It("offers a card on a Decks tab its Cards page and its dealt hands", func() {
		v, tt := tester(deckSnapshot())
		Expect(tt.Click("Decks")).To(Succeed())
		v.decks.tab = tabCards
		tt.Frame()
		r, ok := tt.Find("Otama")
		Expect(ok).To(BeTrue())
		tt.RightClickAt(r.X+2, r.Y+r.H/2)
		shows(tt, "Open in Cards", "Show hands with Otama", "Art in the sim", "View art", "Save image…", "Copy image", "Open on Limitless")
		Expect(choose(tt, "Show hands with Otama")).To(Succeed())
		Expect(v.decks.tab).To(Equal(tabHands))
		Expect(v.decks.handsCard).To(Equal("OTA"))
	})

	It("gives a build's row in the builds table the build chip's menu", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		_, tt := tester(deckSnapshot(wano))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.RightClick("Show Wano/Miracle")).To(Succeed())
		shows(tt, "Edit core", "Rename", "Delete build")
	})

	It("clears one pill or every filter from a pill's menu", func() {
		v, tt := tester(testSnapshot(3))
		v.filter.turn = stats.GoingFirst
		tt.Frame()
		Expect(tt.RightClick("Going first")).To(Succeed())
		shows(tt, "Clear turn order", "Clear all filters")
		Expect(choose(tt, "Clear turn order")).To(Succeed())
		Expect(v.filter.turn).To(Equal(stats.EitherTurn))

		v.filter.turn, v.filter.deck = stats.GoingSecond, "OP13-001"
		tt.Frame()
		Expect(tt.RightClick("Going second")).To(Succeed())
		Expect(choose(tt, "Clear all filters")).To(Succeed())
		Expect(v.filter).To(Equal(v.cleared()))
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
		Expect(startUnpicked).To(Equal(formatAll))
		v, tt := tester(withFormats())
		Expect(v.scoped().matches).To(HaveLen(10))
		Expect(tt.HasText("Clear")).To(BeFalse())
		Expect(prefsFrom(nil).startFormat).To(Equal(startUnpicked))
		Expect(prefsFrom(map[string]string{store.SettingStartFormat: store.StartLatestFormat}).startFormat).To(Equal(formatCurrent))
		Expect(prefsFrom(map[string]string{store.SettingStartFormat: store.StartAllFormats}).startFormat).To(Equal(formatAll))
	})

	It("saves the choice under Start on without changing the format picked", func() {
		v, tt := tester(withFormats())
		fa := &fakeActions{}
		v.act = fa
		Expect(tt.Click("All formats")).To(Succeed())
		Expect(choose(tt, "Start on", "Latest format, now New format")).To(Succeed())
		Expect(v.prefs.startFormat).To(Equal(formatCurrent))
		Expect(fa.calls).To(Equal([]string{store.SettingStartFormat + "=" + store.StartLatestFormat}))
		Expect(v.filter.format).To(Equal(formatAll))
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
		Expect(v.prefs.startFormat).To(Equal(formatCurrent))
		Expect(fa.calls).To(Equal([]string{store.SettingStartFormat + "=" + store.StartLatestFormat}))
	})

	It("goes back to the default from Clear and from the format pill's menu", func() {
		v, tt := tester(withFormats())
		v.prefs.startFormat = formatCurrent
		v.filter.format = 1
		tt.Frame()
		Expect(tt.Click("Clear")).To(Succeed())
		Expect(v.filter.format).To(Equal(formatCurrent))
		tt.Frame()
		Expect(tt.HasText("Clear")).To(BeFalse())

		v.filter.format = formatAll
		tt.Frame()
		Expect(tt.RightClick("All formats")).To(Succeed())
		shows(tt, "Back to New format", "Clear all filters")
		Expect(choose(tt, "Back to New format")).To(Succeed())
		Expect(v.filter.format).To(Equal(formatCurrent))
	})

	It("greys out Back at the default", func() {
		v, tt := tester(withFormats())
		Expect(tt.RightClick("All formats")).To(Succeed())
		shows(tt, "Back to All formats")
		Expect(choose(tt, "Back to All formats")).To(Succeed())
		Expect(v.filter.format).To(Equal(formatAll))
		v.filter.format = 2
		Expect(v.atStartFormat()).To(BeFalse())
		v.prefs.startFormat = formatCurrent
		Expect(v.atStartFormat()).To(BeTrue(), "the latest format picked by its ID is the default")
	})

	It("counts the latest format and All formats as the same with no formats saved", func() {
		v, tt := tester(testSnapshot(3))
		v.filter.format = formatCurrent
		tt.Frame()
		Expect(tt.HasText("Clear")).To(BeFalse())
	})
})
