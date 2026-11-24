package decks

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/gui/guitest"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/sim"
	"github.com/VTGare/logpose/internal/store"
)

func deckArtSnapshot() *page.Snapshot {
	wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano", Core: []string{"OTA"}}
	s := guitest.DeckSnapshot(wano)
	lib := &cards.Library{Cards: map[string]cards.Card{
		"OP13-001": {
			Code: "OP13-001", Name: "Monkey D. Luffy", Type: cards.Leader,
			Prints: []cards.Print{
				{Key: "OP13-001", Set: "OP13", Artist: "Artist A"},
				{Key: "OP13-001_p1", Set: "OP13", Kind: "aa", Artist: "Artist B"},
			},
		},
		"OTA": {
			Code: "OTA", Name: "Otama", Type: cards.Character, Cost: 1,
			Prints: []cards.Print{
				{Key: "OTA", Set: "OP01", Artist: "Artist A"},
				{Key: "OTA_p1", Set: "OP01", Kind: "aa", Artist: "Artist B"},
			},
		},
		"GARP": {
			Code: "GARP", Name: "Garp", Type: cards.Character, Cost: 2,
			Prints: []cards.Print{
				{Key: "GARP", Set: "OP02", Artist: "Artist A"},
			},
		},
		"W-1": {
			Code: "W-1", Name: "Nami", Type: cards.Character, Cost: 1,
			Prints: []cards.Print{
				{Key: "W-1", Set: "OP01", Artist: "A1"},
				{Key: "W-1_p1", Set: "OP01", Kind: "aa", Artist: "A2"},
				{Key: "W-1_p2", Set: "OP01", Kind: "fa", Artist: "A3"},
				{Key: "W-1_p3", Set: "OP05", Rarity: "Special Card", Artist: "A4"},
				{Key: "W-1_p4", Set: "PRB01", Kind: "manga", Artist: "A5"},
			},
		},
		"OLD-1": {
			Code: "OLD-1", Name: "Going Merry", Type: cards.Stage, Cost: 2,
			Prints: []cards.Print{
				{Key: "OLD-1", Set: "OP01", Artist: "Artist A"},
			},
		},
	}}
	lib.Apply(s.Cards)
	return s
}

var _ = Describe("Deck art picker", func() {
	openArt := func() (*view, *ui.Tester, *fakeActions) {
		GinkgoHelper()
		v, tt := tester(deckArtSnapshot())
		act := &fakeActions{}
		v.setActions(act)
		tt.Frame()
		guitest.Click(tt, "Wano")
		guitest.Click(tt, "List")
		guitest.Click(tt, "Customize art")
		return v, tt, act
	}

	It("opens from Decks List, shows the deck's cards and changes the sim's art with Undo", func() {
		v, tt, act := openArt()
		v.SimArt.Set("OP13-001", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP13-001_p1"}})
		v.SimArt.Set("OTA", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		v.SimArt.Set("GARP", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		v.SimArt.Set("W-1", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtPrint, Key: "W-1_p4"}})
		tt.Frame()

		for _, text := range []string{
			"Deck art · Latest list", "2 of 4 changed in the sim",
			"Monkey D. Luffy", "OP13-001 · Leader",
			"Otama", "OTA · ×4", "Nami", "W-1 · ×4", "Garp", "GARP · ×4",
		} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}

		guitest.Click(tt, "Use Otama Alt art in the sim")
		Expect(act.Calls).To(ContainElement("use art OTA OTA_p1"))

		alt := sim.ArtChoice{Kind: sim.ArtPrint, Key: "OTA_p1"}
		v.SimArt.Done(&v.Env, v.Snap.Card("OTA"), alt, page.SimArtInfo{Cur: alt}, sim.ArtChoice{Kind: sim.ArtOwn}, nil)
		tt.Frame()

		Expect(tt.HasText("3 of 4 changed in the sim")).To(BeTrue())
		Expect(tt.HasText("The sim now shows Otama's Alt art.")).To(BeTrue())
		guitest.Click(tt, "Undo")
		Expect(act.Calls).To(ContainElement("use art OTA "))

		guitest.Click(tt, "Back to Monkey D. Luffy")
		Expect(tt.HasText("Deck art · Latest list")).To(BeFalse())
		Expect(tt.HasText("Copy for sim")).To(BeTrue())
	})

	It("opens the art viewer on the clicked printing", func() {
		v, tt, act := openArt()
		v.SimArt.Set("OP13-001", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		v.SimArt.Set("OTA", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		v.SimArt.Set("GARP", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		v.SimArt.Set("W-1", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		tt.Frame()

		guitest.Click(tt, "Show Otama Alt art")
		Expect(guitest.Labelled(tt, "Close the viewer")).To(BeTrue())
		Expect(tt.HasText("OTA ·")).To(BeTrue())
		guitest.Click(tt, "Save image…")
		Expect(act.Calls).To(ContainElement("save art OTA_p1"))
	})

	It("opens the version popover for multi-stamp arts and lets you pick or view one", func() {
		v, tt, act := openArt()
		v.SimArt.Set("GARP", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		ca := arts.CardArts{Arts: []arts.Art{
			{Versions: []arts.Version{{Key: arts.Key{Source: arts.Limitless, Print: "GARP"}, Name: "Base", Short: "Base"}}},
			{Versions: []arts.Version{
				{Key: arts.Key{Source: arts.Bandai, Print: "GARP_p1"}, Name: "Regional", Short: "Offline finalist", Product: "Offline Regional 2025"},
				{Key: arts.Key{Source: arts.Bandai, Print: "GARP_p2"}, Name: "Regional", Short: "Online champion", Product: "Online Regional 2025"},
			}},
		}}
		var code string
		var gen int
		Expect(act.ArtLoads).NotTo(BeEmpty())
		_, err := fmt.Sscan(act.ArtLoads[len(act.ArtLoads)-1], &code, &gen)
		Expect(err).NotTo(HaveOccurred())
		v.Arts.Set(gen, "GARP", ca)
		tt.Frame()

		guitest.Click(tt, "Show Garp Regional, 2 versions")
		Expect(tt.HasText("Regional · 2 versions")).To(BeTrue())
		guitest.Click(tt, "Use Online champion, Online Regional 2025 in the sim")
		Expect(act.Calls).To(ContainElement("use art GARP GARP_p2_bd"))
		guitest.Click(tt, "Show Offline finalist, Offline Regional 2025")
		Expect(guitest.Labelled(tt, "Close the viewer")).To(BeTrue())
		Expect(tt.HasText("GARP ·")).To(BeTrue())
		guitest.Click(tt, "Save image…")
		Expect(act.Calls).To(ContainElement("save art GARP_p1_bd"))
	})

	It("opens an older list from the version rail right-click menu", func() {
		s := deckArtSnapshot()
		latest := map[string]int{"OTA": 4, "GARP": 4, "W-1": 4}
		older := map[string]int{"OTA": 4, "GARP": 4, "OLD-1": 4}
		for id, list := range map[string]map[string]int{"0": latest, "2": latest, "4": latest, "6": older, "8": older} {
			g := s.Decks[id]
			g.Facts.List = list
			s.Decks[id] = g
		}
		v, tt := tester(s)
		v.setActions(&fakeActions{})
		tt.Frame()
		guitest.Click(tt, "Wano")
		guitest.Click(tt, "List")

		Expect(tt.RightClick("Nov 6 11:00")).To(Succeed())
		tt.Frame()
		item, ok := tt.AccessByRole(ui.RoleMenuItem, "Customize art")
		Expect(ok).To(BeTrue())
		tt.ClickAt(item.Bounds.X+item.Bounds.W/2, item.Bounds.Y+item.Bounds.H/2)
		tt.Frame()

		Expect(tt.HasText("Deck art · Nov 6, 11:00")).To(BeTrue())
		Expect(tt.HasText("Going Merry")).To(BeTrue())
		Expect(tt.HasText("OLD-1 · ×4")).To(BeTrue())
	})

	It("shows the no-sim warning banner when OPTCGSim isn't found", func() {
		v, tt, act := openArt()
		v.SimArt.Set("OP13-001", page.SimArtInfo{NoSim: true})
		v.SimArt.Set("OTA", page.SimArtInfo{NoSim: true})
		v.SimArt.Set("GARP", page.SimArtInfo{NoSim: true})
		v.SimArt.Set("W-1", page.SimArtInfo{NoSim: true})
		tt.Frame()

		Expect(tt.HasText("logpose can't find the sim, so it can't change the sim's art.")).To(BeTrue())
		Expect(tt.HasText("changed in the sim")).To(BeFalse())
		guitest.Click(tt, "Choose the sim's folder")
		Expect(act.Calls).To(ContainElement("choose sim"))
	})

	It("lays out two columns when wide, spans 5-printing cards full width, and collapses to one column when narrow", func() {
		v, tt, _ := openArt()
		v.SimArt.Set("OP13-001", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		v.SimArt.Set("OTA", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		v.SimArt.Set("GARP", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		v.SimArt.Set("W-1", page.SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}})
		tt.Frame()
		tt.Frame()

		luffy, ok := tt.Find("OP13-001 · Leader")
		Expect(ok).To(BeTrue())
		nami, ok := tt.Find("W-1 · ×4")
		Expect(ok).To(BeTrue())
		otama, ok := tt.Find("OTA · ×4")
		Expect(ok).To(BeTrue())
		garp, ok := tt.Find("GARP · ×4")
		Expect(ok).To(BeTrue())
		// 5-printing Nami flushes Leader onto its own row, spans full width, and Otama/Garp pair below it.
		Expect(nami.Y).To(BeNumerically(">", luffy.Y+40))
		Expect(otama.Y).To(BeNumerically(">", nami.Y+40))
		Expect(garp.X).To(BeNumerically(">", otama.X+200))
		Expect(garp.Y).To(BeNumerically("~", otama.Y, 4))

		tt.SetSize(820, 780)
		tt.Frame()
		tt.Frame()
		otamaNarrow, ok := tt.Find("OTA · ×4")
		Expect(ok).To(BeTrue())
		garpNarrow, ok := tt.Find("GARP · ×4")
		Expect(ok).To(BeTrue())
		Expect(garpNarrow.X).To(BeNumerically("~", otamaNarrow.X, 40))
		Expect(garpNarrow.Y).To(BeNumerically(">", otamaNarrow.Y+40))
	})

	It("closes the picker on leader change, ShowHands, Open, and re-entering Decks", func() {
		v, tt, _ := openArt()
		Expect(tt.HasText("Deck art · Latest list")).To(BeTrue())

		v.decks.ShowHands("OTA")
		tt.Frame()
		Expect(tt.HasText("Deck art · Latest list")).To(BeFalse())

		guitest.Click(tt, "List")
		guitest.Click(tt, "Customize art")
		Expect(tt.HasText("Deck art · Latest list")).To(BeTrue())

		v.decks.Open(&v.Env, "list")
		tt.Frame()
		Expect(tt.HasText("Deck art · Latest list")).To(BeFalse())

		guitest.Click(tt, "Customize art")
		Expect(tt.HasText("Deck art · Latest list")).To(BeTrue())

		// Leaving Decks and returning clears the picker via e.Entered.
		v.Page = page.Overview
		tt.Frame()
		v.Page = page.Decks
		tt.Frame()
		Expect(tt.HasText("Deck art · Latest list")).To(BeFalse())

		guitest.Click(tt, "Customize art")
		Expect(tt.HasText("Deck art · Latest list")).To(BeTrue())
		v.decks.artPick.leader = "OTHER"
		tt.Frame()
		Expect(tt.HasText("Deck art · Latest list")).To(BeFalse())
	})
})
