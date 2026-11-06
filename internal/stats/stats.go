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
	// Game is the point's game number among those the chart covers, counting from 1, and Wins the wins among
	// the RollingWindow games up to it.
	Game int
	Wins int
}

// RollingWindow is how many games the rolling win rate averages.
const RollingWindow = 20

// Overview is the local player's play across every leader: the session, the week and their
// decks.
type Overview struct {
	// Deck is the leader of the local player's latest game.
	Deck  string
	Total Record
	// Today is the results of today's games, oldest first.
	Today       []Result
	TodayRecord Record
	TodayTime   time.Duration
	// Last is the results of the latest games, at most LastGames, oldest first.
	Last []Result
	// Streak is the length of the latest run of wins or losses, and StreakResult which.
	Streak       int
	StreakResult Result
	// Week is the last 7 days and PrevWeek the 7 before them.
	Week, PrevWeek Record
	// Rolling is the win rate over the last RollingWindow games with any leader, oldest first.
	Rolling []Point
	// Decks is every leader the local player played, latest first.
	Decks []Deck
	// FacedThisWeek is the opponents' leaders of the last 7 days, most played first.
	FacedThisWeek []LeaderRecord
	Recent        []Match
}

// Deck is one of the local player's leaders, in short.
type Deck struct {
	Leader string
	Record Record
	// Last is when the latest game with it ended.
	Last time.Time
}

// LastGames is how many results Overview.Last holds.
const LastGames = 10

// Leader is everything about one of the local player's leaders.
type Leader struct {
	Leader    string
	Record    Record
	TurnOrder TurnOrder
	// Rolling is the win rate over the last RollingWindow games with it, oldest first. It
	// starts once there are that many games.
	Rolling []Point
	// Matchups is the opponents' leaders faced with it, most played first.
	Matchups []LeaderRecord
	Recent   []Match
	// First and Last are when the first and the latest game with it ended.
	First, Last time.Time
}

// ForLeader summarizes the matches, given newest first, played with one leader.
func ForLeader(ms []Match, leader string) Leader {
	l := Leader{Leader: leader}
	var games []Match
	for _, m := range ms {
		if m.Me.Leader != leader {
			continue
		}
		games = append(games, m)
		l.Record.Add(m.Result)
		l.TurnOrder.Add(m)
	}
	if len(games) == 0 {
		return l
	}
	l.Rolling = rolling(games)
	l.Matchups = byLeader(games, func(m Match) (string, bool) { return m.Opp.Leader, true })
	l.Recent = games[:min(len(games), 10)]
	l.Last, l.First = games[0].PlayedAt, games[len(games)-1].PlayedAt
	return l
}

// Leaders is every leader the local player played, most played first.
func Leaders(ms []Match) []LeaderRecord {
	return byLeader(ms, func(m Match) (string, bool) { return m.Me.Leader, true })
}

// BuildOverview summarizes matches given newest first.
func BuildOverview(ms []Match, now time.Time) Overview {
	var o Overview
	if len(ms) == 0 {
		return o
	}
	o.Deck = ms[0].Me.Leader
	weekAgo := now.AddDate(0, 0, -7)
	twoWeeksAgo := now.AddDate(0, 0, -14)
	decks := make(map[string]int)
	for _, m := range ms {
		o.Total.Add(m.Result)
		switch {
		case m.PlayedAt.After(weekAgo):
			o.Week.Add(m.Result)
		case m.PlayedAt.After(twoWeeksAgo):
			o.PrevWeek.Add(m.Result)
		}
		i, seen := decks[m.Me.Leader]
		if !seen {
			i = len(o.Decks)
			decks[m.Me.Leader] = i
			o.Decks = append(o.Decks, Deck{Leader: m.Me.Leader, Last: m.PlayedAt})
		}
		o.Decks[i].Record.Add(m.Result)
	}
	for i := min(len(ms), LastGames) - 1; i >= 0; i-- {
		o.Last = append(o.Last, ms[i].Result)
	}
	o.Rolling = rolling(ms)
	o.today(ms, now)
	o.Streak, o.StreakResult = streak(ms)
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

func rolling(games []Match) []Point {
	var known []Match
	for i := len(games) - 1; i >= 0; i-- {
		if games[i].Result != Unknown {
			known = append(known, games[i])
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
			out = append(out, Point{At: m.PlayedAt, Rate: float64(wins) / RollingWindow, Game: i + 1, Wins: wins})
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

type GameType int

const (
	AllGames GameType = iota
	RankedGames
	CasualGames
)

// Filter keeps the matches of one game type, in order.
func Filter(ms []Match, t GameType) []Match {
	if t == AllGames {
		return ms
	}
	out := make([]Match, 0, len(ms))
	for _, m := range ms {
		if m.Ranked == (t == RankedGames) {
			out = append(out, m)
		}
	}
	return out
}

// Scope is which matches the pages show: a game type, one deck or all of them, and a start.
type Scope struct {
	Type GameType
	// Deck is the local player's leader, or "" for every deck.
	Deck string
	// Since is the earliest time a match can end, or zero for all time.
	Since time.Time
	// Until is when matches stop counting, or zero for no end.
	Until time.Time
}

// Apply keeps the matches in the scope, in order.
func (s Scope) Apply(ms []Match) []Match {
	if s == (Scope{}) {
		return ms
	}
	out := make([]Match, 0, len(ms))
	for _, m := range Filter(ms, s.Type) {
		if s.Deck != "" && m.Me.Leader != s.Deck || !s.Since.IsZero() && m.PlayedAt.Before(s.Since) ||
			!s.Until.IsZero() && !m.PlayedAt.Before(s.Until) {
			continue
		}
		out = append(out, m)
	}
	return out
}

type MatchupOrder int

const (
	MostPlayed MatchupOrder = iota
	BestFirst
	WorstFirst
)

// SortMatchups orders leader records. Best and worst go by raw win rate, with more games first
// among equal rates.
func SortMatchups(lrs []LeaderRecord, by MatchupOrder) []LeaderRecord {
	out := slices.Clone(lrs)
	raw := func(r Record) float64 {
		if r.Games() == 0 {
			return 0
		}
		return float64(r.Wins) / float64(r.Games())
	}
	slices.SortStableFunc(out, func(a, b LeaderRecord) int {
		ra, rb := raw(a.Record), raw(b.Record)
		switch {
		case by == BestFirst && ra != rb:
			return cmpFloat(rb, ra)
		case by == WorstFirst && ra != rb:
			return cmpFloat(ra, rb)
		default:
			return b.Record.Games() - a.Record.Games()
		}
	})
	return out
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// SplitBySample splits leader records into those with at least MinGames games and the rest,
// keeping their order.
func SplitBySample(lrs []LeaderRecord) (enough, few []LeaderRecord) {
	for _, lr := range lrs {
		if lr.Record.Games() >= MinGames {
			enough = append(enough, lr)
		} else {
			few = append(few, lr)
		}
	}
	return enough, few
}

// Versus is the local player's history for a game about to be played.
type Versus struct {
	// Matchup is with the local player's leader against the opponent's.
	Matchup   Record
	TurnOrder TurnOrder
	// Player is against the same opponent, with any leaders.
	Player Record
}

// VersusFor sums the matches against an opponent. An empty leader leaves Matchup and TurnOrder
// empty.
func VersusFor(ms []Match, mine, theirs, opp string) Versus {
	var v Versus
	for _, m := range ms {
		if opp != "" && m.Opp.ID == opp {
			v.Player.Add(m.Result)
		}
		if mine == "" || theirs == "" || m.Me.Leader != mine || m.Opp.Leader != theirs {
			continue
		}
		v.Matchup.Add(m.Result)
		v.TurnOrder.Add(m)
	}
	return v
}
