package stats

import (
	"slices"
	"time"

	"github.com/VTGare/logpose/internal/store"
)

// MinGames is the fewest games a rate is shown for. Below it the UI shows "too few".
const MinGames = 5

type Result int

const (
	Unknown Result = iota
	Win
	Loss
)

// Match is a stored game from the local player's side.
type Match struct {
	store.Game
	Me, Opp   store.GamePlayer
	Result    Result
	WentFirst bool
	// TurnOrderKnown is false for logs that don't say who went first.
	TurnOrderKnown bool
	Ranked         bool
}

// Matches turns stored games into matches for the local player, keeping the store's order.
// Games the local player isn't in are left out.
func Matches(games []store.Game, me string) []Match {
	out := make([]Match, 0, len(games))
	for _, g := range games {
		mine, ok := g.Player(me)
		if !ok {
			continue
		}
		m := Match{Game: g, Me: mine, Ranked: g.Lobby.OPBounty()}
		for _, p := range g.Players {
			if p.ID != me {
				m.Opp = p
			}
		}
		switch g.End.Loser {
		case "":
			m.Result = Unknown
		case me:
			m.Result = Loss
		default:
			m.Result = Win
		}
		m.TurnOrderKnown = g.First != ""
		m.WentFirst = g.First == me
		out = append(out, m)
	}
	return out
}

type Record struct {
	Wins, Losses int
}

func (r *Record) Add(res Result) {
	switch res {
	case Win:
		r.Wins++
	case Loss:
		r.Losses++
	}
}

func (r Record) Games() int { return r.Wins + r.Losses }

// Rate is the win rate from 0 to 1. ok is false below MinGames.
func (r Record) Rate() (rate float64, ok bool) {
	if r.Games() < MinGames {
		return 0, false
	}
	return float64(r.Wins) / float64(r.Games()), true
}

type TurnOrder struct {
	First, Second Record
}

func (t *TurnOrder) Add(m Match) {
	if !m.TurnOrderKnown {
		return
	}
	if m.WentFirst {
		t.First.Add(m.Result)
	} else {
		t.Second.Add(m.Result)
	}
}

type LeaderRecord struct {
	Leader string
	Record Record
}

type Point struct {
	At   time.Time
	Rate float64
}

// RollingWindow is how many games the rolling win rate averages.
const RollingWindow = 20

type Overview struct {
	// Deck is the leader of the local player's latest game, which the overview is about.
	Deck      string
	Record    Record
	TurnOrder TurnOrder
	// Rolling is the win rate over the last RollingWindow games with Deck, oldest first. It
	// starts once there are that many games.
	Rolling []Point
	// Today is the results of today's games, oldest first.
	Today       []Result
	TodayRecord Record
	TodayTime   time.Duration
	// Streak is the length of the latest run of wins or losses, and StreakResult which.
	Streak       int
	StreakResult Result
	OtherLeaders []LeaderRecord
	// FacedThisWeek is the opponents' leaders of the last 7 days, most played first.
	FacedThisWeek []LeaderRecord
	Recent        []Match
}

// BuildOverview summarizes matches given newest first.
func BuildOverview(ms []Match, now time.Time) Overview {
	var o Overview
	if len(ms) == 0 {
		return o
	}
	o.Deck = ms[0].Me.Leader
	var deck []Match
	for _, m := range ms {
		if m.Me.Leader == o.Deck {
			deck = append(deck, m)
			o.Record.Add(m.Result)
			o.TurnOrder.Add(m)
		}
	}
	o.Rolling = rolling(deck)
	o.today(ms, now)
	o.Streak, o.StreakResult = streak(ms)
	o.OtherLeaders = byLeader(ms, func(m Match) (string, bool) { return m.Me.Leader, m.Me.Leader != o.Deck })
	weekAgo := now.AddDate(0, 0, -7)
	o.FacedThisWeek = byLeader(ms, func(m Match) (string, bool) { return m.Opp.Leader, m.PlayedAt.After(weekAgo) })
	o.Recent = ms[:min(len(ms), 8)]
	return o
}

func (o *Overview) today(ms []Match, now time.Time) {
	y, mo, d := now.Date()
	start := time.Date(y, mo, d, 0, 0, 0, 0, now.Location())
	var first, last time.Time
	for i := len(ms) - 1; i >= 0; i-- {
		m := ms[i]
		if m.PlayedAt.Before(start) {
			continue
		}
		o.Today = append(o.Today, m.Result)
		o.TodayRecord.Add(m.Result)
		if first.IsZero() {
			first = m.PlayedAt.Add(-time.Duration(m.Turns) * approxTurn)
		}
		last = m.PlayedAt
	}
	if !first.IsZero() {
		o.TodayTime = last.Sub(first)
	}
}

// approxTurn estimates how long a turn takes, to start the day's play time before the first
// game ended. A log only has the time the game ended.
const approxTurn = 70 * time.Second

func streak(ms []Match) (int, Result) {
	n, res := 0, Unknown
	for _, m := range ms {
		if m.Result == Unknown {
			continue
		}
		if res == Unknown {
			res = m.Result
		}
		if m.Result != res {
			break
		}
		n++
	}
	return n, res
}

func rolling(deck []Match) []Point {
	var known []Match
	for i := len(deck) - 1; i >= 0; i-- {
		if deck[i].Result != Unknown {
			known = append(known, deck[i])
		}
	}
	var out []Point
	wins := 0
	for i, m := range known {
		if m.Result == Win {
			wins++
		}
		if i >= RollingWindow && known[i-RollingWindow].Result == Win {
			wins--
		}
		if i >= RollingWindow-1 {
			out = append(out, Point{At: m.PlayedAt, Rate: float64(wins) / RollingWindow})
		}
	}
	return out
}

func byLeader(ms []Match, pick func(Match) (string, bool)) []LeaderRecord {
	idx := make(map[string]int)
	var out []LeaderRecord
	for _, m := range ms {
		leader, ok := pick(m)
		if !ok || leader == "" {
			continue
		}
		i, seen := idx[leader]
		if !seen {
			i = len(out)
			idx[leader] = i
			out = append(out, LeaderRecord{Leader: leader})
		}
		out[i].Record.Add(m.Result)
	}
	slices.SortStableFunc(out, func(a, b LeaderRecord) int { return b.Record.Games() - a.Record.Games() })
	return out
}
