package gui

import (
	"fmt"
	"slices"
	"time"

	"github.com/VTGare/logpose/internal/gui/guitest"

	"github.com/VTGare/logpose/internal/gui/overview"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/live"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func tester(s *page.Snapshot) (*view, *ui.Tester) {
	v := newView(nil)
	v.Snap = s
	v.Clock = func() time.Time { return guitest.Now }
	// A Tester's clock stands still. A transition would never end, and every settle would run
	// all 20 frames mygo allows. DisableAnimations shows each frame's changes at once.
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		v.build(c)
	}, 1180, 780)
	// The specs check what pages show, and frames of motion only slow them down.
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	return v, tt
}

var _ = Describe("view", func() {
	It("opens on any page -page names", func() {
		Expect(Pages()).To(ContainElements("overview", "decks", "decks/list", "decks/edit", "decks/build", "decks/nobuild", "matchups", "matches", "formats", "cards", "settings"))
		v, tt := tester(guitest.Snapshot(10))
		Expect(IsPage("cards/OP01-024")).To(BeTrue())
		Expect(IsPage("cards/")).To(BeFalse())
		Expect(IsPage("nowhere")).To(BeFalse())
		Expect(IsPage("settings/list")).To(BeFalse())
		Expect(IsPage("decks/nowhere")).To(BeFalse())
		v.open("decks")
		tt.Frame()
		Expect(tt.HasText("Turn order")).To(BeTrue())

		v, tt = tester(guitest.DeckSnapshot())
		v.open("decks/list")
		tt.Frame()
		Expect(tt.HasText("Suggested builds · 2")).To(BeTrue())
	})

	It("waits for the games to choose the first build or edit it from -page", func() {
		wano := store.Build{ID: 1, Leader: "OP13-001", Name: "Wano/Miracle", Core: []string{"W-1"}}
		v, tt := tester(&page.Snapshot{})
		v.open("decks/build")
		tt.Frame()
		v.setSnapshot(guitest.DeckSnapshot(wano))
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("· Wano/Miracle"))

		v, tt = tester(guitest.DeckSnapshot(wano))
		v.open("decks/edit")
		tt.Frame()
		Expect(tt.HasText("Edit Wano/Miracle")).To(BeTrue())
	})

	It("shows today, the week, each day played and every deck on the overview", func() {
		_, tt := tester(guitest.Snapshot(10))
		for _, text := range []string{
			"Today", "Streak", "This week", "All time", "By day", "Your decks", "Monkey D. Luffy", "10 games · 1 h ago",
			"Facing most this week", "Kaido",
		} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("Turn order")).To(BeFalse())
	})

	It("names the days by the clock, and asks for a frame at midnight", func() {
		v, tt := tester(guitest.Snapshot(10))
		f := &fakeActions{}
		v.setActions(f)
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("Today"))
		Expect(f.timers).To(HaveLen(1))
		v.Clock = func() time.Time { return guitest.Now.AddDate(0, 0, 1) }
		tt.Frame()
		Expect(tt.Texts()).To(ContainElements("Yesterday", "None yet"))
		Expect(f.timers).To(HaveLen(2))
	})

	It("puts today's record in the tray tooltip by the clock, and updates it at midnight", func() {
		v, tt := tester(guitest.Snapshot(3))
		f := &fakeActions{}
		v.setActions(f)
		tt.Frame()
		Expect(v.trayTip()).To(Equal("logpose · today 2–1"))
		v.Clock = func() time.Time { return guitest.Now.AddDate(0, 0, 1) }
		Expect(v.trayTip()).To(Equal("logpose"))
		f.fire()
		Expect(f.newDays).To(Equal(1))
	})

	It("narrows to games against a leader faced this week", func() {
		_, tt := tester(guitest.Snapshot(10))
		Expect(tt.Click("Show games vs Kaido")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).To(ContainElement("vs Kaido"))
		Expect(tt.Click("Remove the filter vs Kaido")).To(Succeed())
		tt.Frame()
		Expect(tt.Texts()).NotTo(ContainElement("vs Kaido"))
	})

	It("lists the days played under their week", func() {
		s := guitest.Snapshot(3)
		s.Matches[2].PlayedAt = guitest.Now.AddDate(0, 0, -5)
		s.Matches[2].Result = stats.Win
		s.Overview = stats.BuildOverview(s.Matches, guitest.Now)
		_, tt := tester(s)
		for _, text := range []string{"This week", "Today", "1–1", "Last week", "Sun Nov 1", "1–0"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("shows the current deck and its turn order split on the Decks page", func() {
		_, tt := tester(guitest.Snapshot(10))
		Expect(tt.Click("Decks")).To(Succeed())
		for _, text := range []string{"Monkey D. Luffy", "Win rate", "50%", "5–5", "Turn order", "1st", "2nd"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("shows the game under the pointer on the Decks page's rolling chart", func() {
		v, tt := tester(guitest.Snapshot(30))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.HasText("game 25")).To(BeFalse())
		v.decks.LeaderPoint.I = 5
		tt.Frame()
		Expect(tt.HasText("Nov 6 14:00 · game 25 · 50%, 10 of the last 20 won")).To(BeTrue(), fmt.Sprint(tt.Texts()))
	})

	It("says no change when this week's rate matches last week's", func() {
		o := stats.Overview{Week: stats.Record{Wins: 3, Losses: 2}, PrevWeek: stats.Record{Wins: 3, Losses: 2}}
		change, d := overview.WeekChange(o)
		Expect(change).To(Equal("no change"))
		Expect(d).To(BeZero())
	})

	It("writes a streak of one in the singular", func() {
		_, tt := tester(guitest.Snapshot(1))
		Expect(tt.HasText("1 win in a row")).To(BeTrue())
	})

	It("says too few for a rate under the minimum games", func() {
		_, tt := tester(guitest.Snapshot(3))
		Expect(tt.HasText("too few")).To(BeTrue())
	})

	It("lists matches on the Matches page", func() {
		v, tt := tester(guitest.Snapshot(10))
		Expect(tt.Click("Matches")).To(Succeed())
		Expect(v.Page).To(Equal(page.Matches))
		Expect(tt.HasText("10 games · 5–5")).To(BeTrue())
		Expect(tt.HasText("Kaido")).To(BeTrue())
		Expect(tt.HasText("Ranked")).To(BeTrue())
		Expect(tt.HasText("Today")).To(BeTrue())
	})

	It("shows a newly imported game on the Matches page", func() {
		v, tt := tester(guitest.Snapshot(10))
		Expect(tt.Click("Matches")).To(Succeed())
		s := guitest.Snapshot(11)
		s.Matches[0].Opp.Name = "Newcomer"
		v.Snap = s
		tt.Frame()
		Expect(tt.HasText("11 games · 6–5")).To(BeTrue())
		Expect(tt.HasText("Newcomer")).To(BeTrue())
	})

	It("opens a leader's page from the overview", func() {
		s := guitest.Snapshot(10)
		s.Matches[3].Me.Leader = "OP17-058"
		s.Overview = stats.BuildOverview(s.Matches, guitest.Now)
		v, tt := tester(s)
		Expect(tt.HasText("Your decks")).To(BeTrue())
		Expect(tt.Click("Show Kaido")).To(Succeed())
		tt.Frame()
		Expect(v.Page).To(Equal(page.Decks))
		Expect(tt.Texts()).To(ContainElements("OP17 · since Nov 6", "0–1 · 1 game"))
		Expect(tt.HasText("Results")).To(BeTrue())
	})

	It("shows the most played leader on the Decks page by default", func() {
		_, tt := tester(guitest.Snapshot(10))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.HasText("Monkey D. Luffy")).To(BeTrue())
		Expect(tt.HasText("Win rate")).To(BeTrue())
		Expect(tt.HasText("By day")).To(BeTrue())
	})

	It("filters the Decks page by game type", func() {
		s := guitest.Snapshot(10)
		for i := range 4 {
			s.Matches[i].Ranked = false
		}
		_, tt := tester(s)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("5–5 · 10 games"))
		Expect(tt.Click("All games")).To(Succeed())
		Expect(guitest.Choose(tt, "Casual")).To(Succeed())
		Expect(tt.Texts()).To(ContainElement("2–2 · 4 games"))
	})

	It("splits matchups with enough games at 50%, and counts the rest, which open the Matchups page", func() {
		s := guitest.Snapshot(10)
		s.Matches[0].Opp.Leader = "OP09-042"
		v, tt := tester(s)
		Expect(tt.Click("Decks")).To(Succeed())
		for _, text := range []string{"Worst matchups", "Kaido", "4–5", "44%", "Rarely faced", "1 under 5 games"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("Best matchups")).To(BeFalse())
		Expect(tt.Click("Show every matchup")).To(Succeed())
		Expect(v.Page).To(Equal(page.Matchups))
	})

	It("invites a first game when the sim is there but nothing is stored", func() {
		_, tt := tester(&page.Snapshot{Loaded: true, SimFound: true})
		Expect(tt.HasText("Play a game in the sim")).To(BeTrue())
	})

	It("shows the lobby while the sim waits for a game", func() {
		v, tt := tester(guitest.Snapshot(10))
		v.setLive(live.State{Phase: live.Lobby, Lobby: "XABCDEF", Hosting: true})
		tt.Frame()
		Expect(tt.HasText("Match found")).To(BeTrue())
		Expect(tt.HasText("Ranked · XABCDEF")).To(BeTrue())
	})

	It("shows the history against the opponent in the sim", func() {
		v, tt := tester(guitest.Snapshot(10))
		v.setLive(live.State{
			Phase: live.Playing, Lobby: "QWERTY", First: guitest.Me,
			Players: []live.Player{
				{ID: guitest.Me, Leader: "OP13-001", Local: true, Life: 4, Hand: 6},
				{ID: "Opp#2", Leader: "OP17-058", Life: 5, Hand: 5},
			},
		})
		tt.Frame()
		for _, text := range []string{
			"vs Opp", "With Monkey D. Luffy vs Kaido", "5–5 · 50%", "Going first · 5–5",
			"Against Opp", "4 life · 6 hand", "Casual · QWERTY",
		} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("says when the sim didn't log the opponent's leader", func() {
		v, tt := tester(guitest.Snapshot(10))
		v.setLive(live.State{Phase: live.Playing, Players: []live.Player{
			{ID: guitest.Me, Local: true}, {ID: "New#3"},
		}})
		tt.Frame()
		Expect(tt.HasText("Leader not in the sim's log")).To(BeTrue())
		Expect(tt.HasText("Against New")).To(BeFalse())
	})
})

var _ = Describe("sidebar", func() {
	It("snaps to the icon rail below its smallest width and stops at its largest", func() {
		Expect(page.SnapSide(100)).To(BeEquivalentTo(page.SideRail))
		Expect(page.SnapSide(140)).To(BeEquivalentTo(page.SideMin))
		Expect(page.SnapSide(250)).To(BeEquivalentTo(250))
		Expect(page.SnapSide(500)).To(BeEquivalentTo(page.SideMax))
		Expect(page.PrefsFrom(nil).SideWidth).To(BeEquivalentTo(page.SideDefault))
		Expect(page.PrefsFrom(map[string]string{page.KeySidebarWidth: "90"}).SideWidth).To(BeEquivalentTo(page.SideRail))
	})

	It("folds to the rail and opens again when its edge is dragged a few DIPs at a time", func() {
		v, tt := tester(guitest.Snapshot(4))
		fa := &fakeActions{}
		v.setActions(fa)
		tt.Frame()
		stored := func() float32 {
			GinkgoHelper()
			fa.fire()
			Expect(fa.calls).NotTo(BeEmpty())
			var w float32
			_, err := fmt.Sscanf(fa.calls[len(fa.calls)-1], page.KeySidebarWidth+"=%g", &w)
			Expect(err).NotTo(HaveOccurred())
			return w
		}
		drag := func(from, to float32) {
			GinkgoHelper()
			tt.Press(from, 400)
			tt.Frame()
			step := float32(3)
			if to < from {
				step = -3
			}
			for x := from; (step > 0 && x < to) || (step < 0 && x > to); x += step {
				tt.Move(x, 400)
			}
			tt.Release(to, 400)
			tt.Frame()
		}
		drag(page.SideDefault, 230)
		Expect(stored()).To(BeNumerically("~", 230, 3))
		Expect(tt.Texts()).To(ContainElement("Matchups"))
		drag(230, 90)
		Expect(stored()).To(BeEquivalentTo(page.SideRail))
		drag(page.SideRail, 200)
		Expect(stored()).To(BeNumerically(">=", page.SideMin))
	})

	It("lists the pages read after games, a line, Cards, then Formats and Settings at the bottom", func() {
		v, tt := tester(guitest.Snapshot(4))
		v.Page = page.Matches
		order := []string{"Overview", "Matches", "Decks", "Matchups", "Cards", "Formats", "Settings"}
		centers := func() []float32 {
			GinkgoHelper()
			tt.Frame()
			var ys []float32
			for _, label := range order {
				r, ok := tt.Find(label)
				Expect(ok).To(BeTrue(), label)
				ys = append(ys, r.Y+r.H/2)
			}
			return ys
		}
		open := centers()
		Expect(slices.IsSorted(open)).To(BeTrue())
		step := open[1] - open[0]
		Expect(open[4]-open[3]).To(BeNumerically(">", step+1), "a line sits between Matchups and Cards")
		Expect(open[6] - open[5]).To(BeNumerically("~", step, 0.5))
		Expect(open[5] - open[4]).To(BeNumerically(">", 4*step))

		v.setPrefs(page.PrefsFrom(map[string]string{page.KeySidebarWidth: "90"}))
		Expect(centers()).To(Equal(open))
	})

	It("folded, still goes to every page", func() {
		v, tt := tester(guitest.Snapshot(4))
		v.setPrefs(page.PrefsFrom(map[string]string{page.KeySidebarWidth: "90"}))
		tt.Frame()
		Expect(tt.Click("Matchups")).To(Succeed())
		tt.Frame()
		Expect(v.Page).To(Equal(page.Matchups))
		Expect(tt.Click("Settings")).To(Succeed())
		tt.Frame()
		Expect(v.Page).To(Equal(page.Settings))
	})
})
