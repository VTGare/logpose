package gui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/live"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const me = "Me#1"

var now = time.Date(2026, 11, 6, 20, 0, 0, 0, time.Local)

// testSnapshot is n games with Luffy against Kaido, alternating wins and losses, newest first.
func testSnapshot(n int) *snapshot {
	var games []store.Game
	for i := range n {
		loser := me
		if i%2 == 0 {
			loser = "Opp#2"
		}
		games = append(games, store.Game{
			Fingerprint: fmt.Sprint(i),
			PlayedAt:    now.Add(-time.Duration(i+1) * time.Hour),
			Turns:       12,
			First:       me,
			Lobby:       simlog.Lobby{Code: fmt.Sprintf("XNJ8M%02d", i)},
			End:         simlog.End{Kind: simlog.EndConcede, Loser: loser},
			Players: []store.GamePlayer{
				{ID: me, Name: "Me", Leader: "OP13-001"},
				{ID: "Opp#2", Name: "Opp", Leader: "OP17-058"},
			},
		})
	}
	db := cards.NewDB()
	db.Name("OP13-001", "Monkey D. Luffy")
	db.Name("OP17-058", "Kaido")
	ms := stats.Matches(games, me)
	return &snapshot{Loaded: true, SimFound: true, Matches: ms, Overview: stats.BuildOverview(ms, now), Cards: db, Now: now}
}

func tester(s *snapshot) (*view, *ui.Tester) {
	v := newView(nil)
	v.snap = s
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
		v, tt := tester(testSnapshot(10))
		v.open("decks")
		tt.Frame()
		Expect(tt.HasText("Turn order")).To(BeTrue())
		v.open("decks/list")
		Expect(v.page).To(Equal(pageDecks))
		Expect(v.decks.tab).To(Equal(tabList))
	})

	It("shows today, the week, each day played and every deck on the overview", func() {
		_, tt := tester(testSnapshot(10))
		for _, text := range []string{
			"Today", "Streak", "This week", "All time", "By day", "Your decks", "Monkey D. Luffy", "10 games · 1 h ago",
			"Facing most this week", "Kaido",
		} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("Turn order")).To(BeFalse())
	})

	It("narrows to games against a leader faced this week", func() {
		v, tt := tester(testSnapshot(10))
		Expect(tt.Click("Show games vs Kaido")).To(Succeed())
		Expect(v.filter.oppLeader).To(Equal("OP17-058"))
		tt.Frame()
		_, pill := tt.Find("vs Kaido")
		Expect(pill).To(BeTrue())
		Expect(tt.Click("Remove the filter vs Kaido")).To(Succeed())
		Expect(v.filter.oppLeader).To(BeEmpty())
	})

	It("lists the days played under their week", func() {
		s := testSnapshot(3)
		s.Matches[2].PlayedAt = now.AddDate(0, 0, -5)
		s.Matches[2].Result = stats.Win
		s.Overview = stats.BuildOverview(s.Matches, now)
		_, tt := tester(s)
		for _, text := range []string{"This week", "Today", "1–1", "Last week", "Sun Nov 1", "1–0"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("shows the current deck and its turn order split on the Decks page", func() {
		_, tt := tester(testSnapshot(10))
		Expect(tt.Click("Decks")).To(Succeed())
		for _, text := range []string{"Monkey D. Luffy", "Win rate", "50%", "5–5", "Turn order", "1st", "2nd"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("shows the game under the pointer on the Decks page's rolling chart", func() {
		v, tt := tester(testSnapshot(30))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.HasText("game 25")).To(BeFalse())
		v.leaderPoint = 5
		tt.Frame()
		Expect(tt.HasText("Nov 6 14:00 · game 25 · 50%, 10 of the last 20 won")).To(BeTrue(), fmt.Sprint(tt.Texts()))
	})

	It("writes a streak of one in the singular", func() {
		_, tt := tester(testSnapshot(1))
		Expect(tt.HasText("1 win in a row")).To(BeTrue())
	})

	It("says too few for a rate under the minimum games", func() {
		_, tt := tester(testSnapshot(3))
		Expect(tt.HasText("too few")).To(BeTrue())
	})

	It("lists matches on the Matches page", func() {
		v, tt := tester(testSnapshot(10))
		Expect(tt.Click("Matches")).To(Succeed())
		Expect(v.page).To(Equal(pageMatches))
		Expect(tt.HasText("10 games · 5–5")).To(BeTrue())
		Expect(tt.HasText("Kaido")).To(BeTrue())
		Expect(tt.HasText("Ranked")).To(BeTrue())
		Expect(tt.HasText("Today")).To(BeTrue())
	})

	It("shows a newly imported game on the Matches page", func() {
		v, tt := tester(testSnapshot(10))
		Expect(tt.Click("Matches")).To(Succeed())
		s := testSnapshot(11)
		s.Matches[0].Opp.Name = "Newcomer"
		v.snap = s
		tt.Frame()
		Expect(tt.HasText("11 games · 6–5")).To(BeTrue())
		Expect(tt.HasText("Newcomer")).To(BeTrue())
	})

	It("opens a leader's page from the overview", func() {
		s := testSnapshot(10)
		s.Matches[3].Me.Leader = "OP17-058"
		s.Overview = stats.BuildOverview(s.Matches, now)
		v, tt := tester(s)
		Expect(tt.HasText("Your decks")).To(BeTrue())
		Expect(tt.Click("Show Kaido")).To(Succeed())
		Expect(v.page).To(Equal(pageDecks))
		Expect(v.filter.deck).To(Equal("OP17-058"))
		Expect(tt.HasText("Results")).To(BeTrue())
	})

	It("shows the most played leader on the Decks page by default", func() {
		_, tt := tester(testSnapshot(10))
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.HasText("Monkey D. Luffy")).To(BeTrue())
		Expect(tt.HasText("Win rate")).To(BeTrue())
		Expect(tt.HasText("By day")).To(BeTrue())
	})

	It("filters the Decks page by game type", func() {
		s := testSnapshot(10)
		for i := range 4 {
			s.Matches[i].Ranked = false
		}
		v, tt := tester(s)
		Expect(tt.Click("Decks")).To(Succeed())
		Expect(tt.HasText("By day")).To(BeTrue())
		Expect(tt.HasText("All games")).To(BeTrue())
		v.filter.gameType = int(stats.CasualGames)
		tt.Frame()
		Expect(tt.HasText("Casual")).To(BeTrue())
		l, _ := v.leaderData()
		Expect(l.Record.Games()).To(Equal(4))
	})

	It("splits matchups with enough games at 50%, and counts the rest, which open the Matchups page", func() {
		s := testSnapshot(10)
		s.Matches[0].Opp.Leader = "OP09-042"
		v, tt := tester(s)
		Expect(tt.Click("Decks")).To(Succeed())
		for _, text := range []string{"Worst matchups", "Kaido", "4–5", "44%", "Rarely faced", "1 under 5 games"} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
		Expect(tt.HasText("Best matchups")).To(BeFalse())
		Expect(tt.Click("Show every matchup")).To(Succeed())
		Expect(v.page).To(Equal(pageMatchups))
	})

	It("invites a first game when the sim is there but nothing is stored", func() {
		_, tt := tester(&snapshot{Loaded: true, SimFound: true})
		Expect(tt.HasText("Play a game in the sim")).To(BeTrue())
	})

	It("shows the lobby while the sim waits for a game", func() {
		v, tt := tester(testSnapshot(10))
		v.live = live.State{Phase: live.Lobby, Lobby: "XABCDEF", Hosting: true}
		tt.Frame()
		Expect(tt.HasText("Match found")).To(BeTrue())
		Expect(tt.HasText("Ranked · XABCDEF")).To(BeTrue())
	})

	It("shows the history against the opponent in the sim", func() {
		v, tt := tester(testSnapshot(10))
		v.live = live.State{
			Phase: live.Playing, Lobby: "QWERTY", First: me,
			Players: []live.Player{
				{ID: me, Leader: "OP13-001", Local: true, Life: 4, Hand: 6},
				{ID: "Opp#2", Leader: "OP17-058", Life: 5, Hand: 5},
			},
		}
		tt.Frame()
		for _, text := range []string{
			"vs Opp", "With Monkey D. Luffy vs Kaido", "5–5 · 50%", "Going first · 5–5",
			"Against Opp", "4 life · 6 hand", "Casual · QWERTY",
		} {
			Expect(tt.HasText(text)).To(BeTrue(), text)
		}
	})

	It("says when the sim didn't log the opponent's leader", func() {
		v, tt := tester(testSnapshot(10))
		v.live = live.State{Phase: live.Playing, Players: []live.Player{
			{ID: me, Local: true}, {ID: "New#3"},
		}}
		tt.Frame()
		Expect(tt.HasText("Leader not in the sim's log")).To(BeTrue())
		Expect(tt.HasText("Against New")).To(BeFalse())
	})
})

var _ = Describe("sidebar", func() {
	It("snaps to the icon rail below its smallest width and stops at its largest", func() {
		Expect(snapSide(100)).To(BeEquivalentTo(sideRail))
		Expect(snapSide(140)).To(BeEquivalentTo(sideMin))
		Expect(snapSide(250)).To(BeEquivalentTo(250))
		Expect(snapSide(500)).To(BeEquivalentTo(sideMax))
		Expect(widthFrom("", sideDefault, snapSide)).To(BeEquivalentTo(sideDefault))
		Expect(widthFrom("90", sideDefault, snapSide)).To(BeEquivalentTo(sideRail))
	})

	It("folds to the rail and opens again when its edge is dragged a few DIPs at a time", func() {
		v, tt := tester(testSnapshot(4))
		tt.Frame()
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
		drag(sideDefault, 230)
		Expect(v.prefs.sideWidth).To(BeNumerically("~", 230, 3))
		drag(230, 90)
		Expect(v.prefs.sideWidth).To(BeEquivalentTo(sideRail))
		drag(sideRail, 200)
		Expect(v.prefs.sideWidth).To(BeNumerically(">=", sideMin))
	})

	It("folded, still goes to every page", func() {
		v, tt := tester(testSnapshot(4))
		v.prefs.sideWidth = sideRail
		tt.Frame()
		Expect(tt.Click("Matchups")).To(Succeed())
		tt.Frame()
		Expect(v.page).To(Equal(pageMatchups))
		Expect(tt.Click("Settings")).To(Succeed())
		tt.Frame()
		Expect(v.page).To(Equal(pageSettings))
	})
})
