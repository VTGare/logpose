package stats_test

import (
	"fmt"
	"time"

	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const me = "Me#1"

var now = time.Date(2026, 11, 6, 20, 0, 0, 0, time.UTC)

type game struct {
	ago      time.Duration
	mine     string
	theirs   string
	won      bool
	first    bool
	unknown  bool
	lobby    string
	noFirst  bool
	opponent string
}

func build(gs ...game) []store.Game {
	out := make([]store.Game, len(gs))
	for i, g := range gs {
		opp := g.opponent
		if opp == "" {
			opp = fmt.Sprintf("Opp#%d", i)
		}
		sg := store.Game{
			Fingerprint: fmt.Sprint(i),
			PlayedAt:    now.Add(-g.ago),
			Turns:       10,
			Lobby:       simlog.Lobby{Code: g.lobby},
			Players: []store.GamePlayer{
				{ID: me, Leader: g.mine},
				{ID: opp, Leader: g.theirs},
			},
		}
		switch {
		case g.unknown:
		case g.won:
			sg.End = simlog.End{Kind: simlog.EndConcede, Loser: opp}
		default:
			sg.End = simlog.End{Kind: simlog.EndLethal, Loser: me}
		}
		if !g.noFirst {
			sg.First = opp
			if g.first {
				sg.First = me
			}
		}
		out[i] = sg
	}
	return out
}

var _ = Describe("Matches", func() {
	It("reads each game from the local player's side", func() {
		ms := stats.Matches(build(
			game{mine: "OP13-001", theirs: "OP17-058", won: true, first: true, lobby: "XNJ8M66"},
			game{mine: "OP13-001", theirs: "OP09-042", lobby: "7KQ2MP"},
			game{mine: "OP13-001", theirs: "OP09-042", unknown: true, noFirst: true},
		), me)
		Expect(ms).To(HaveLen(3))
		Expect(ms[0].Result).To(Equal(stats.Win))
		Expect(ms[0].WentFirst).To(BeTrue())
		Expect(ms[0].Ranked).To(BeTrue())
		Expect(ms[0].Opp.Leader).To(Equal("OP17-058"))
		Expect(ms[1].Result).To(Equal(stats.Loss))
		Expect(ms[1].Ranked).To(BeFalse())
		Expect(ms[2].Result).To(Equal(stats.Unknown))
		Expect(ms[2].TurnOrderKnown).To(BeFalse())
	})

	It("leaves out games without the local player", func() {
		games := build(game{mine: "OP13-001", theirs: "OP17-058"})
		Expect(stats.Matches(games, "Someone#9")).To(BeEmpty())
	})
})

var _ = Describe("Record", func() {
	It("has no rate below the minimum games", func() {
		r := stats.Record{Wins: 3, Losses: 1}
		_, ok := r.Rate()
		Expect(ok).To(BeFalse())
		r.Add(stats.Win)
		rate, ok := r.Rate()
		Expect(ok).To(BeTrue())
		Expect(rate).To(BeNumerically("~", 0.8))
	})
})

var _ = Describe("BuildOverview", func() {
	luffy, kid := "OP13-001", "OP17-058"

	It("is about the deck of the latest game", func() {
		o := stats.BuildOverview(stats.Matches(build(
			game{ago: time.Hour, mine: luffy, theirs: kid, won: true, first: true},
			game{ago: 2 * time.Hour, mine: luffy, theirs: kid, won: false, first: false},
			game{ago: 3 * time.Hour, mine: kid, theirs: luffy, won: true},
		), me), now)
		Expect(o.Deck).To(Equal(luffy))
		Expect(o.Record).To(Equal(stats.Record{Wins: 1, Losses: 1}))
		Expect(o.TurnOrder.First).To(Equal(stats.Record{Wins: 1}))
		Expect(o.TurnOrder.Second).To(Equal(stats.Record{Losses: 1}))
		Expect(o.OtherLeaders).To(Equal([]stats.LeaderRecord{{Leader: kid, Record: stats.Record{Wins: 1}}}))
	})

	It("counts today's games oldest first", func() {
		o := stats.BuildOverview(stats.Matches(build(
			game{ago: time.Hour, mine: luffy, won: true},
			game{ago: 2 * time.Hour, mine: luffy, won: false},
			game{ago: 30 * time.Hour, mine: luffy, won: true},
		), me), now)
		Expect(o.Today).To(Equal([]stats.Result{stats.Loss, stats.Win}))
		Expect(o.TodayRecord).To(Equal(stats.Record{Wins: 1, Losses: 1}))
		Expect(o.TodayTime).To(BeNumerically(">", time.Hour))
	})

	It("finds the latest streak", func() {
		o := stats.BuildOverview(stats.Matches(build(
			game{ago: 1, mine: luffy, won: true},
			game{ago: 2, mine: luffy, unknown: true},
			game{ago: 3, mine: luffy, won: true},
			game{ago: 4, mine: luffy, won: false},
		), me), now)
		Expect(o.Streak).To(Equal(2))
		Expect(o.StreakResult).To(Equal(stats.Win))
	})

	It("rolls the win rate over the last 20 games once there are 20", func() {
		var gs []game
		for i := range 25 {
			gs = append(gs, game{ago: time.Duration(i+1) * time.Hour, mine: luffy, won: i < 5})
		}
		o := stats.BuildOverview(stats.Matches(build(gs...), me), now)
		Expect(o.Rolling).To(HaveLen(6))
		Expect(o.Rolling[0].Rate).To(BeNumerically("~", 0))
		Expect(o.Rolling[5].Rate).To(BeNumerically("~", 0.25))
	})

	It("lists this week's opposing leaders, most played first", func() {
		o := stats.BuildOverview(stats.Matches(build(
			game{ago: time.Hour, mine: luffy, theirs: "OP09-042", won: true},
			game{ago: 2 * time.Hour, mine: luffy, theirs: kid},
			game{ago: 3 * time.Hour, mine: luffy, theirs: kid, won: true},
			game{ago: 10 * 24 * time.Hour, mine: luffy, theirs: "OP13-079"},
		), me), now)
		Expect(o.FacedThisWeek).To(HaveLen(2))
		Expect(o.FacedThisWeek[0].Leader).To(Equal(kid))
		Expect(o.FacedThisWeek[0].Record).To(Equal(stats.Record{Wins: 1, Losses: 1}))
	})

	It("is empty without games", func() {
		Expect(stats.BuildOverview(nil, now).Deck).To(BeEmpty())
	})
})
