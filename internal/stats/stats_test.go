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

	It("lists every deck played, latest first, with the deck of the latest game first", func() {
		o := stats.BuildOverview(stats.Matches(build(
			game{ago: time.Hour, mine: luffy, theirs: kid, won: true},
			game{ago: 2 * time.Hour, mine: kid, theirs: luffy, won: true},
			game{ago: 3 * time.Hour, mine: luffy, theirs: kid, won: false},
		), me), now)
		Expect(o.Deck).To(Equal(luffy))
		Expect(o.Total).To(Equal(stats.Record{Wins: 2, Losses: 1}))
		Expect(o.Decks).To(Equal([]stats.Deck{
			{Leader: luffy, Record: stats.Record{Wins: 1, Losses: 1}, Last: now.Add(-time.Hour)},
			{Leader: kid, Record: stats.Record{Wins: 1}, Last: now.Add(-2 * time.Hour)},
		}))
	})

	It("splits this week from the week before", func() {
		o := stats.BuildOverview(stats.Matches(build(
			game{ago: time.Hour, mine: luffy, won: true},
			game{ago: 6 * 24 * time.Hour, mine: luffy, won: false},
			game{ago: 8 * 24 * time.Hour, mine: luffy, won: true},
			game{ago: 20 * 24 * time.Hour, mine: luffy, won: true},
		), me), now)
		Expect(o.Week).To(Equal(stats.Record{Wins: 1, Losses: 1}))
		Expect(o.PrevWeek).To(Equal(stats.Record{Wins: 1}))
	})

	It("keeps the last 10 results, oldest first", func() {
		var gs []game
		for i := range 12 {
			gs = append(gs, game{ago: time.Duration(i+1) * time.Hour, mine: luffy, won: i == 0})
		}
		o := stats.BuildOverview(stats.Matches(build(gs...), me), now)
		Expect(o.Last).To(HaveLen(stats.LastGames))
		Expect(o.Last[9]).To(Equal(stats.Win))
		Expect(o.Last[0]).To(Equal(stats.Loss))
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

	It("rolls the win rate over the last 20 games with any leader once there are 20", func() {
		var gs []game
		for i := range 25 {
			mine := luffy
			if i%2 == 0 {
				mine = kid
			}
			gs = append(gs, game{ago: time.Duration(i+1) * time.Hour, mine: mine, won: i < 5})
		}
		o := stats.BuildOverview(stats.Matches(build(gs...), me), now)
		Expect(o.Rolling).To(HaveLen(6))
		Expect(o.Rolling[0].Rate).To(BeNumerically("~", 0))
		Expect(o.Rolling[5].Rate).To(BeNumerically("~", 0.25))
		Expect(o.Rolling[0].Game).To(Equal(20))
		Expect(o.Rolling[5].Game).To(Equal(25))
		Expect(o.Rolling[5].Wins).To(Equal(5))
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

var _ = Describe("ForLeader", func() {
	luffy, kid, buggy := "OP13-001", "OP17-058", "OP09-042"
	ms := stats.Matches(build(
		game{ago: time.Hour, mine: kid, theirs: buggy, won: true},
		game{ago: 2 * time.Hour, mine: luffy, theirs: buggy, won: true, first: true},
		game{ago: 3 * time.Hour, mine: luffy, theirs: kid},
		game{ago: 4 * time.Hour, mine: luffy, theirs: buggy},
	), me)

	It("only counts games with that leader", func() {
		l := stats.ForLeader(ms, luffy)
		Expect(l.Record).To(Equal(stats.Record{Wins: 1, Losses: 2}))
		Expect(l.TurnOrder.First).To(Equal(stats.Record{Wins: 1}))
		Expect(l.Recent).To(HaveLen(3))
		Expect(l.Last).To(BeTemporally("==", now.Add(-2*time.Hour)))
		Expect(l.First).To(BeTemporally("==", now.Add(-4*time.Hour)))
	})

	It("lists the leaders it faced, most played first", func() {
		l := stats.ForLeader(ms, luffy)
		Expect(l.Matchups).To(Equal([]stats.LeaderRecord{
			{Leader: buggy, Record: stats.Record{Wins: 1, Losses: 1}},
			{Leader: kid, Record: stats.Record{Losses: 1}},
		}))
	})

	It("is empty for a leader never played", func() {
		Expect(stats.ForLeader(ms, "OP13-079").Record.Games()).To(BeZero())
	})
})

var _ = Describe("Leaders", func() {
	It("lists every leader played, most played first", func() {
		ms := stats.Matches(build(
			game{mine: "OP17-058", won: true},
			game{mine: "OP13-001"},
			game{mine: "OP13-001", won: true},
		), me)
		Expect(stats.Leaders(ms)).To(Equal([]stats.LeaderRecord{
			{Leader: "OP13-001", Record: stats.Record{Wins: 1, Losses: 1}},
			{Leader: "OP17-058", Record: stats.Record{Wins: 1}},
		}))
	})
})

var _ = Describe("Filter", func() {
	ms := stats.Matches(build(
		game{mine: "OP13-001", lobby: "XNJ8M66", won: true},
		game{mine: "OP13-001", lobby: "7KQ2MP"},
		game{mine: "OP13-001"},
	), me)

	It("keeps ranked or casual games", func() {
		Expect(stats.Filter(ms, stats.AllGames)).To(HaveLen(3))
		Expect(stats.Filter(ms, stats.RankedGames)).To(HaveLen(1))
		Expect(stats.Filter(ms, stats.CasualGames)).To(HaveLen(2))
	})
})

var _ = Describe("SortMatchups", func() {
	lrs := []stats.LeaderRecord{
		{Leader: "a", Record: stats.Record{Wins: 2, Losses: 2}},
		{Leader: "b", Record: stats.Record{Wins: 6, Losses: 1}},
		{Leader: "c", Record: stats.Record{Wins: 1, Losses: 9}},
		{Leader: "d", Record: stats.Record{Wins: 1}},
	}
	order := func(out []stats.LeaderRecord) []string {
		var s []string
		for _, lr := range out {
			s = append(s, lr.Leader)
		}
		return s
	}

	It("sorts by games, best or worst rate", func() {
		Expect(order(stats.SortMatchups(lrs, stats.MostPlayed))).To(Equal([]string{"c", "b", "a", "d"}))
		Expect(order(stats.SortMatchups(lrs, stats.BestFirst))).To(Equal([]string{"d", "b", "a", "c"}))
		Expect(order(stats.SortMatchups(lrs, stats.WorstFirst))).To(Equal([]string{"c", "a", "b", "d"}))
	})

	It("splits off matchups under the minimum games", func() {
		enough, few := stats.SplitBySample(stats.SortMatchups(lrs, stats.MostPlayed))
		Expect(order(enough)).To(Equal([]string{"c", "b"}))
		Expect(order(few)).To(Equal([]string{"a", "d"}))
	})
})

var _ = Describe("VersusFor", func() {
	luffy, kaido := "OP13-001", "OP17-058"
	ms := stats.Matches(build(
		game{mine: luffy, theirs: kaido, won: true, first: true, opponent: "Bob#2"},
		game{mine: luffy, theirs: kaido, won: false},
		game{mine: luffy, theirs: kaido, won: false, first: true},
		game{mine: "OP01-001", theirs: kaido, won: true, opponent: "Bob#2"},
		game{mine: luffy, theirs: "OP05-060", won: true},
	), me)

	It("sums the matchup with turn order, and the opponent across leaders", func() {
		v := stats.VersusFor(ms, luffy, kaido, "Bob#2")
		Expect(v.Matchup).To(Equal(stats.Record{Wins: 1, Losses: 2}))
		Expect(v.TurnOrder.First).To(Equal(stats.Record{Wins: 1, Losses: 1}))
		Expect(v.TurnOrder.Second).To(Equal(stats.Record{Losses: 1}))
		Expect(v.Player).To(Equal(stats.Record{Wins: 2}))
	})

	It("has no matchup without the opponent's leader", func() {
		v := stats.VersusFor(ms, luffy, "", "Bob#2")
		Expect(v.Matchup.Games()).To(BeZero())
		Expect(v.Player.Games()).To(Equal(2))
	})
})
