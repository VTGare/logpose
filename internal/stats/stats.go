package stats

import (
	"math"
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

// LobbySizes counts the games played in each lobby.
func LobbySizes(games []store.Game) map[string]int {
	sizes := make(map[string]int)
	for _, g := range games {
		if g.Lobby.Code != "" {
			sizes[g.Lobby.Code]++
		}
	}
	return sizes
}

// IsRanked guesses whether a game was ranked, from the games in each lobby. The sim logs
// nothing that tells a ranked game from a friendly in the same room. A ranked match is one game
// in its own lobby, while friendlies usually play several, so a lobby with more than one game
// counts as friendly.
func IsRanked(g store.Game, lobbySizes map[string]int) bool {
	return g.Lobby.InRoom() && lobbySizes[g.Lobby.Code] == 1
}

// Matches turns stored games into matches for the local player, keeping the store's order.
// me is the user's names. A game with more than one of them is from the first one's side.
// Games none of them are in are left out.
func Matches(games []store.Game, me ...string) []Match {
	out := make([]Match, 0, len(games))
	sizes := LobbySizes(games)
	for _, g := range games {
		var mine store.GamePlayer
		ok := false
		for _, id := range me {
			if mine, ok = g.Player(id); ok {
				break
			}
		}
		if !ok {
			continue
		}
		m := Match{Game: g, Me: mine, Ranked: IsRanked(g, sizes)}
		// Only room lobbies can be ranked, whatever the user set.
		if g.Lobby.InRoom() && g.Type != store.TypeGuessed {
			m.Ranked = g.Type == store.TypeRanked
		}
		for _, p := range g.Players {
			if p.ID != mine.ID {
				m.Opp = p
			}
		}
		switch g.End.Loser {
		case "":
			m.Result = Unknown
		case mine.ID:
			m.Result = Loss
		default:
			m.Result = Win
		}
		m.TurnOrderKnown = g.First != ""
		m.WentFirst = g.First == mine.ID
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

// likelyZ is the normal quantile for an 80% two-sided range.
const likelyZ = 1.2816

// LikelyRange is where the true win rate likely is, from 0 to 1: the 80% Wilson score
// interval. Unlike the plain normal range, it stays inside 0 and 1 and isn't zero wide for a
// record like 2–0. With no games it's the whole range.
func (r Record) LikelyRange() (lo, hi float64) {
	n := float64(r.Games())
	if n == 0 {
		return 0, 1
	}
	p := float64(r.Wins) / n
	z2 := likelyZ * likelyZ
	d := 1 + z2/n
	mid := (p + z2/(2*n)) / d
	half := likelyZ * math.Sqrt(p*(1-p)/n+z2/(4*n*n)) / d
	return max(mid-half, 0), min(mid+half, 1)
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

// Overview is the local player's play across every leader: today, the week, each day played and
// their decks.
type Overview struct {
	// Deck is the leader of the local player's latest game.
	Deck  string
	Total Record
	// Today is the results of today's games, oldest first.
	Today       []Result
	TodayRecord Record
	TodayTime   time.Duration
	// Streak is the length of the latest run of wins or losses, and StreakResult which.
	Streak       int
	StreakResult Result
	// Week is the calendar week so far, from Monday, and PrevWeek the whole week before it.
	Week, PrevWeek Record
	// Days is every day played, latest first.
	Days []Day
	// Decks is every leader the local player played, latest first.
	Decks []Deck
	// FacedThisWeek is the opponents' leaders of the calendar week, most played first.
	FacedThisWeek []LeaderRecord
}

// Day is the games of one day, oldest first.
type Day struct {
	// Date is the day's midnight.
	Date    time.Time
	Matches []Match
	Record  Record
	// Played estimates the time spent playing.
	Played time.Duration
}

// Deck is one of the local player's leaders, in short.
type Deck struct {
	Leader string
	Record Record
	// Last is when the latest game with it ended.
	Last time.Time
}

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

// Matchup is the local player's games with one of their leaders against one opposing leader.
type Matchup struct {
	// Leader is the opponent's leader.
	Leader    string
	Record    Record
	TurnOrder TurnOrder
	// Games are newest first.
	Games []Match
}

// Matchups groups the matches, given newest first, played with one leader by the opponent's
// leader, most played first.
func Matchups(ms []Match, leader string) []Matchup {
	idx := make(map[string]int)
	var out []Matchup
	for _, m := range ms {
		if m.Me.Leader != leader || m.Opp.Leader == "" {
			continue
		}
		i, seen := idx[m.Opp.Leader]
		if !seen {
			i = len(out)
			idx[m.Opp.Leader] = i
			out = append(out, Matchup{Leader: m.Opp.Leader})
		}
		mu := &out[i]
		mu.Record.Add(m.Result)
		mu.TurnOrder.Add(m)
		mu.Games = append(mu.Games, m)
	}
	slices.SortStableFunc(out, func(a, b Matchup) int { return b.Record.Games() - a.Record.Games() })
	return out
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
	week := WeekStart(now)
	prevWeek := week.AddDate(0, 0, -7)
	decks := make(map[string]int)
	for _, m := range ms {
		o.Total.Add(m.Result)
		switch {
		case !m.PlayedAt.Before(week):
			o.Week.Add(m.Result)
		case !m.PlayedAt.Before(prevWeek):
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
	o.Days = days(ms)
	if d := o.Days[0]; d.Date.Equal(dayStart(now)) {
		for _, m := range d.Matches {
			o.Today = append(o.Today, m.Result)
		}
		o.TodayRecord, o.TodayTime = d.Record, d.Played
	}
	o.Streak, o.StreakResult = streak(ms)
	o.FacedThisWeek = byLeader(ms, func(m Match) (string, bool) { return m.Opp.Leader, !m.PlayedAt.Before(week) })
	return o
}

func dayStart(t time.Time) time.Time {
	y, mo, d := t.Date()
	return time.Date(y, mo, d, 0, 0, 0, 0, t.Location())
}

// WeekStart is the Monday midnight that starts t's week.
func WeekStart(t time.Time) time.Time {
	day := dayStart(t)
	return day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
}

// days groups matches given newest first by the day they ended on.
func days(ms []Match) []Day {
	var out []Day
	for i := len(ms) - 1; i >= 0; i-- {
		m := ms[i]
		date := dayStart(m.PlayedAt)
		if len(out) == 0 || !out[len(out)-1].Date.Equal(date) {
			out = append(out, Day{Date: date})
		}
		d := &out[len(out)-1]
		d.Matches = append(d.Matches, m)
		d.Record.Add(m.Result)
	}
	for i := range out {
		out[i].Played = played(out[i].Matches)
	}
	slices.Reverse(out)
	return out
}

// played adds up the time between games ended less than breakGap apart, and estimates the
// length of the first game after each break from its turns.
func played(ms []Match) time.Duration {
	length := func(m Match) time.Duration { return time.Duration(m.Turns) * approxTurn }
	total := length(ms[0])
	for i := 1; i < len(ms); i++ {
		if gap := ms[i].PlayedAt.Sub(ms[i-1].PlayedAt); gap < breakGap {
			total += gap
		} else {
			total += length(ms[i])
		}
	}
	return total
}

// breakGap is the longest time between two games' ends that still counts as playing.
const breakGap = 45 * time.Minute

// approxTurn estimates how long a turn takes, for the length of a game after a break. A log only
// has the time the game ended.
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

// TurnOrderFilter keeps games by who went first. GoingFirst and GoingSecond leave out games
// whose log doesn't say.
type TurnOrderFilter int

const (
	EitherTurn TurnOrderFilter = iota
	GoingFirst
	GoingSecond
)

func (f TurnOrderFilter) keeps(m Match) bool {
	return f == EitherTurn || m.TurnOrderKnown && m.WentFirst == (f == GoingFirst)
}

// Scope is which matches the pages show: a game type, one deck or all of them, a turn order and
// a start.
type Scope struct {
	Type GameType
	Turn TurnOrderFilter
	// Deck is the local player's leader, or "" for every deck.
	Deck string
	// Opp is the opponent's player ID and OppLeader their leader, or "" for any.
	Opp, OppLeader string
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
		if s.Deck != "" && m.Me.Leader != s.Deck || s.Opp != "" && m.Opp.ID != s.Opp ||
			s.OppLeader != "" && m.Opp.Leader != s.OppLeader || !s.Turn.keeps(m) ||
			!s.Since.IsZero() && m.PlayedAt.Before(s.Since) || !s.Until.IsZero() && !m.PlayedAt.Before(s.Until) {
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
