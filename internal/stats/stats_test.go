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
	sets     []string
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
			Sets:        g.sets,
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

	It("takes the game type the user set over the guess, in room lobbies only", func() {
		games := build(
			game{mine: "OP13-001", theirs: "OP17-058", lobby: "XNJ8M66"},
			game{mine: "OP13-001", theirs: "OP17-058", lobby: "XAAAA11"},
			game{mine: "OP13-001", theirs: "OP17-058", lobby: "XAAAA11"},
			game{mine: "OP13-001", theirs: "OP17-058", lobby: "7KQ2MP"},
		)
		games[0].Type = store.TypeFriendly
		games[1].Type = store.TypeRanked
		games[3].Type = store.TypeRanked
		ms := stats.Matches(games, me)
		Expect(ms[0].Ranked).To(BeFalse())
		Expect(ms[1].Ranked).To(BeTrue())
		Expect(ms[2].Ranked).To(BeFalse())
		Expect(ms[3].Ranked).To(BeFalse())
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

	DescribeTable("gives the 80% Wilson range as the likely win rate",
		func(r stats.Record, lo, hi float64) {
			gotLo, gotHi := r.LikelyRange()
			Expect(gotLo).To(BeNumerically("~", lo, 0.0005))
			Expect(gotHi).To(BeNumerically("~", hi, 0.0005))
		},
		Entry("10–4", stats.Record{Wins: 10, Losses: 4}, 0.5437, 0.8399),
		Entry("5–5", stats.Record{Wins: 5, Losses: 5}, 0.3122, 0.6878),
		Entry("2–0 stays under 100%", stats.Record{Wins: 2}, 0.5491, 1.0),
		Entry("0–1 stays over 0%", stats.Record{Losses: 1}, 0.0, 0.6216),
		Entry("no games", stats.Record{}, 0.0, 1.0),
	)

	It("narrows the likely range as games add up at the same rate", func() {
		lo1, hi1 := stats.Record{Wins: 1, Losses: 1}.LikelyRange()
		lo2, hi2 := stats.Record{Wins: 10, Losses: 10}.LikelyRange()
		Expect(hi2 - lo2).To(BeNumerically("<", hi1-lo1))
		Expect(lo2).To(BeNumerically("<", 0.5))
		Expect(hi2).To(BeNumerically(">", 0.5))
	})
})

var _ = Describe("Matchups", func() {
	luffy, kaido, buggy := "OP13-001", "OP17-058", "OP09-042"
	ms := stats.Matches(build(
		game{ago: time.Hour, mine: luffy, theirs: buggy, won: true, first: true},
		game{ago: 2 * time.Hour, mine: luffy, theirs: kaido},
		game{ago: 3 * time.Hour, mine: "OP01-001", theirs: buggy, won: true},
		game{ago: 4 * time.Hour, mine: luffy, theirs: kaido, won: true, first: true},
		game{ago: 5 * time.Hour, mine: luffy, theirs: kaido, unknown: true},
		game{ago: 6 * time.Hour, mine: luffy, theirs: ""},
	), me)

	It("groups one leader's games by the opponent's leader, most played first", func() {
		mus := stats.Matchups(ms, luffy)
		Expect(mus).To(HaveLen(2))
		Expect(mus[0].Leader).To(Equal(kaido))
		Expect(mus[0].Record).To(Equal(stats.Record{Wins: 1, Losses: 1}))
		Expect(mus[0].TurnOrder.First).To(Equal(stats.Record{Wins: 1}))
		Expect(mus[0].TurnOrder.Second).To(Equal(stats.Record{Losses: 1}))
		Expect(mus[1].Leader).To(Equal(buggy))
		Expect(mus[1].Record).To(Equal(stats.Record{Wins: 1}))
	})

	It("keeps each matchup's games newest first, unknown results included", func() {
		games := stats.Matchups(ms, luffy)[0].Games
		Expect(games).To(HaveLen(3))
		Expect(games[0].PlayedAt).To(BeTemporally("==", now.Add(-2*time.Hour)))
		Expect(games[2].Result).To(Equal(stats.Unknown))
	})

	It("is empty for a leader never played", func() {
		Expect(stats.Matchups(ms, "OP13-079")).To(BeEmpty())
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

	It("splits the calendar week from Monday from the week before", func() {
		o := stats.BuildOverview(stats.Matches(build(
			game{ago: time.Hour, mine: luffy, won: true},
			game{ago: 4 * 24 * time.Hour, mine: luffy, won: false},
			game{ago: 6 * 24 * time.Hour, mine: luffy, won: true},
			game{ago: 11 * 24 * time.Hour, mine: luffy, won: false},
			game{ago: 20 * 24 * time.Hour, mine: luffy, won: true},
		), me), now)
		Expect(now.Weekday()).To(Equal(time.Friday))
		Expect(o.Week).To(Equal(stats.Record{Wins: 1, Losses: 1}))
		Expect(o.PrevWeek).To(Equal(stats.Record{Wins: 1, Losses: 1}))
	})

	It("starts the week on Monday", func() {
		Expect(stats.WeekStart(now)).To(Equal(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)))
		sunday := time.Date(2026, 11, 8, 23, 0, 0, 0, time.UTC)
		Expect(stats.WeekStart(sunday)).To(Equal(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)))
	})

	It("groups games by day, latest day first and each day's games oldest first", func() {
		o := stats.BuildOverview(stats.Matches(build(
			game{ago: time.Hour, mine: luffy, won: true},
			game{ago: 90 * time.Minute, mine: luffy, won: false},
			game{ago: 4 * time.Hour, mine: luffy, won: true},
			game{ago: 30 * time.Hour, mine: luffy, won: true},
		), me), now)
		Expect(o.Days).To(HaveLen(2))
		today := o.Days[0]
		Expect(today.Date).To(Equal(time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)))
		Expect(today.Record).To(Equal(stats.Record{Wins: 2, Losses: 1}))
		Expect(today.Matches[0].PlayedAt).To(Equal(now.Add(-4 * time.Hour)))
		// The break after the first game counts as the length of the next one, 10 turns.
		Expect(today.Played).To(Equal(2*10*70*time.Second + 30*time.Minute))
		Expect(o.Days[1].Record).To(Equal(stats.Record{Wins: 1}))
	})

	It("counts today's games oldest first", func() {
		o := stats.BuildOverview(stats.Matches(build(
			game{ago: time.Hour, mine: luffy, won: true},
			game{ago: 2 * time.Hour, mine: luffy, won: false},
			game{ago: 30 * time.Hour, mine: luffy, won: true},
		), me), now)
		Expect(o.Today).To(Equal([]stats.Result{stats.Loss, stats.Win}))
		Expect(o.TodayRecord).To(Equal(stats.Record{Wins: 1, Losses: 1}))
		Expect(o.TodayTime).To(Equal(2 * 10 * 70 * time.Second))
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

var _ = Describe("Scope", func() {
	luffy, kid := "OP13-001", "OP17-058"
	ms := stats.Matches(build(
		game{ago: time.Hour, mine: luffy, lobby: "XABCDEF", won: true},
		game{ago: 2 * time.Hour, mine: kid, lobby: "QWERTY"},
		game{ago: 10 * 24 * time.Hour, mine: luffy, lobby: "XABCDEG"},
	), me)

	It("keeps everything when empty", func() {
		Expect(stats.Scope{}.Apply(ms)).To(HaveLen(3))
	})

	It("combines game type, deck and start", func() {
		Expect(stats.Scope{Type: stats.RankedGames}.Apply(ms)).To(HaveLen(2))
		Expect(stats.Scope{Deck: luffy}.Apply(ms)).To(HaveLen(2))
		Expect(stats.Scope{Opp: ms[0].Opp.ID}.Apply(ms)).To(ContainElement(ms[0]))
		Expect(stats.Scope{OppLeader: "OP01-001"}.Apply(ms)).To(BeEmpty())
		got := stats.Scope{Type: stats.RankedGames, Deck: luffy, Since: now.AddDate(0, 0, -7)}.Apply(ms)
		Expect(got).To(HaveLen(1))
		Expect(got[0].Result).To(Equal(stats.Win))
	})

	It("keeps games by turn order and leaves out games that don't say who went first", func() {
		ms := stats.Matches(build(
			game{ago: time.Hour, mine: luffy, first: true},
			game{ago: 2 * time.Hour, mine: luffy},
			game{ago: 3 * time.Hour, mine: luffy, noFirst: true},
		), me)
		Expect(stats.Scope{Turn: stats.EitherTurn}.Apply(ms)).To(HaveLen(3))
		first := stats.Scope{Turn: stats.GoingFirst}.Apply(ms)
		Expect(first).To(HaveLen(1))
		Expect(first[0].Fingerprint).To(Equal("0"))
		second := stats.Scope{Turn: stats.GoingSecond, Deck: luffy}.Apply(ms)
		Expect(second).To(HaveLen(1))
		Expect(second[0].Fingerprint).To(Equal("1"))
	})
})

var _ = Describe("ranked", func() {
	It("counts a game alone in its room lobby as ranked, and rematches in one as friendlies", func() {
		ms := stats.Matches(build(
			game{ago: time.Hour, mine: "A", lobby: "XAAAAAA"},
			game{ago: 2 * time.Hour, mine: "A", lobby: "XBBBBBB"},
			game{ago: 3 * time.Hour, mine: "A", lobby: "XBBBBBB"},
			game{ago: 4 * time.Hour, mine: "A", lobby: "QWERTY"},
		), me)
		Expect([]bool{ms[0].Ranked, ms[1].Ranked, ms[2].Ranked, ms[3].Ranked}).To(Equal([]bool{true, false, false, false}))
	})
})
