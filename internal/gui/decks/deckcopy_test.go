package decks

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/gui/guitest"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/store"
)

func copyDeckSnapshot(lists map[string]map[string]int) *page.Snapshot {
	wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano", Core: []string{"OTA"}}
	s := guitest.DeckSnapshot(wano)
	lib := &cards.Library{Cards: map[string]cards.Card{
		"OTA":   {Code: "OTA", Name: "Otama", Type: cards.Character, Cost: 1},
		"GARP":  {Code: "GARP", Name: "Garp", Type: cards.Character, Cost: 2},
		"EVT-1": {Code: "EVT-1", Name: "Radical Beam", Type: cards.Event, Cost: 1},
		"STG-1": {Code: "STG-1", Name: "Thousand Sunny", Type: cards.Stage, Cost: 1},
		"OLD-1": {Code: "OLD-1", Name: "Going Merry", Type: cards.Stage, Cost: 2},
	}}
	lib.Apply(s.Cards)
	for id, list := range lists {
		g := s.Decks[id]
		g.Facts.List = list
		s.Decks[id] = g
	}
	return s
}

var _ = Describe("Copy for sim on Decks", func() {
	It("copies a full 50-card list in decklist order with the full-deck toast", func() {
		full := map[string]int{"OTA": 4, "GARP": 20, "EVT-1": 16, "STG-1": 10}
		lists := make(map[string]map[string]int)
		for i := 0; i < 10; i += 2 {
			lists[fmt.Sprint(i)] = full
		}
		v, tt := tester(copyDeckSnapshot(lists))
		act := &fakeActions{}
		v.setActions(act)
		tt.Frame()
		Expect(tt.Click("Wano")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()

		Expect(tt.HasText("wasn't named in the log")).To(BeFalse())
		Expect(tt.Click("Copy for sim")).To(Succeed())
		tt.Frame()

		Expect(act.Copied).To(HaveLen(1))
		Expect(act.Copied[0].Text).To(Equal("1xOP13-001\r\n4xOTA\r\n20xGARP\r\n16xEVT-1\r\n10xSTG-1\r\n"))
		Expect(act.Copied[0].Toast.Title).To(Equal("Copied decklist for the sim."))
		Expect(act.Copied[0].Toast.Description).To(Equal("50 cards · Monkey D. Luffy"))
	})

	It("shows the partial banner and partial toast for a 49-card list", func() {
		partial := map[string]int{"OTA": 4, "GARP": 20, "EVT-1": 16, "STG-1": 9}
		lists := make(map[string]map[string]int)
		for i := 0; i < 10; i += 2 {
			lists[fmt.Sprint(i)] = partial
		}
		v, tt := tester(copyDeckSnapshot(lists))
		act := &fakeActions{}
		v.setActions(act)
		tt.Frame()
		Expect(tt.Click("Wano")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()

		Expect(tt.HasText("49 of 50 cards · 1 card wasn't named in the log.")).To(BeTrue())
		Expect(tt.Click("Copy for sim")).To(Succeed())
		tt.Frame()

		Expect(act.Copied).To(HaveLen(1))
		Expect(act.Copied[0].Text).To(Equal("1xOP13-001\r\n4xOTA\r\n20xGARP\r\n16xEVT-1\r\n9xSTG-1\r\n"))
		Expect(act.Copied[0].Toast.Title).To(Equal("Copied 49 of 50 cards for the sim."))
		Expect(act.Copied[0].Toast.Description).To(Equal("Monkey D. Luffy · 1 card wasn't named in the log."))
	})

	It("copies an older version from the version rail menu instead of the latest list", func() {
		latest := map[string]int{"OTA": 4, "GARP": 20, "EVT-1": 16, "STG-1": 10}
		older := map[string]int{"OTA": 4, "GARP": 20, "EVT-1": 16, "OLD-1": 10}
		lists := map[string]map[string]int{
			"0": latest,
			"2": latest,
			"4": latest,
			"6": older,
			"8": older,
		}
		v, tt := tester(copyDeckSnapshot(lists))
		act := &fakeActions{}
		v.setActions(act)
		tt.Frame()
		Expect(tt.Click("Wano")).To(Succeed())
		Expect(tt.Click("List")).To(Succeed())
		tt.Frame()

		Expect(tt.RightClick("Nov 6 11:00")).To(Succeed())
		tt.Frame()
		item, ok := tt.AccessByRole(ui.RoleMenuItem, "Copy for sim")
		Expect(ok).To(BeTrue(), "popover menu should expose Copy for sim")
		tt.ClickAt(item.Bounds.X+item.Bounds.W/2, item.Bounds.Y+item.Bounds.H/2)
		tt.Frame()

		Expect(act.Copied).To(HaveLen(1))
		Expect(act.Copied[0].Text).To(Equal("1xOP13-001\r\n4xOTA\r\n20xGARP\r\n16xEVT-1\r\n10xOLD-1\r\n"))
		Expect(act.Copied[0].Toast.Title).To(Equal("Copied decklist for the sim."))
		Expect(act.Copied[0].Toast.Description).To(Equal("50 cards · Monkey D. Luffy"))
	})
})
