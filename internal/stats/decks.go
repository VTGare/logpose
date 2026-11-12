package stats

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/deck"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/store"
)

// DeckSize is how many cards a deck has besides its leader.
const DeckSize = 50

type DeckGame struct {
	Match
	Facts deck.Facts
}

// DeckGames pairs matches with their facts. Matches without facts, from logs without RZ1
// moves, are left out. A list the log names only part of takes the cards of the nearest full
// list of the same leader that holds it.
func DeckGames(ms []Match, facts map[string]deck.Facts) []DeckGame {
	var out []DeckGame
	for _, m := range ms {
		if f, ok := facts[m.Fingerprint]; ok && len(f.List) > 0 {
			out = append(out, DeckGame{Match: m, Facts: f})
		}
	}
	// fill takes the first of two lists as near, so it reads them oldest first.
	slices.SortStableFunc(out, func(a, b DeckGame) int { return a.PlayedAt.Compare(b.PlayedAt) })
	for i := range out {
		if out[i].Facts.Named() < DeckSize {
			out[i].Facts.List = fill(out, i)
		}
	}
	slices.Reverse(out)
	return out
}

func fill(gs []DeckGame, i int) map[string]int {
	g := gs[i]
	best, gap := -1, time.Duration(0)
	for j, o := range gs {
		if o.Me.Leader != g.Me.Leader || o.Facts.Named() != DeckSize || !holds(o.Facts.List, g.Facts.List) {
			continue
		}
		d := o.PlayedAt.Sub(g.PlayedAt).Abs()
		if best < 0 || d < gap {
			best, gap = j, d
		}
	}
	if best < 0 {
		return g.Facts.List
	}
	return maps.Clone(gs[best].Facts.List)
}

func holds(list, part map[string]int) bool {
	for c, n := range part {
		if list[c] < n {
			return false
		}
	}
	return true
}

// Assignment is where each game of the builds' leaders goes.
type Assignment struct {
	Builds map[int64][]DeckGame
	// Conflicts are games that run every core card of two builds with cores of the same size.
	Conflicts []DeckGame

	in map[string]int64
}

// Assign puts each game in the build whose core its list runs in full. When several fit, the
// build with the bigger core is the more specific one. Games that fit none are left out.
func Assign(gs []DeckGame, builds []store.Build) Assignment {
	a := Assignment{Builds: make(map[int64][]DeckGame), in: make(map[string]int64)}
	for _, g := range gs {
		var fit []store.Build
		for _, b := range builds {
			if b.Leader == g.Me.Leader && runsAll(g.Facts.List, b.Core) {
				fit = append(fit, b)
			}
		}
		if len(fit) == 0 {
			continue
		}
		slices.SortFunc(fit, func(x, y store.Build) int { return cmp.Compare(len(y.Core), len(x.Core)) })
		if len(fit) > 1 && len(fit[0].Core) == len(fit[1].Core) {
			a.Conflicts = append(a.Conflicts, g)
			a.in[g.Fingerprint] = conflict
			continue
		}
		a.Builds[fit[0].ID] = append(a.Builds[fit[0].ID], g)
		a.in[g.Fingerprint] = fit[0].ID
	}
	return a
}

// conflict marks a game that fits two builds in Assignment.in. Stored builds count from 1.
const conflict = 0

// In is the build a game is in. ok is false for a game in no build, and a game that fits two
// builds is in none of them, with id 0.
func (a Assignment) In(fingerprint string) (id int64, ok bool) {
	id, ok = a.in[fingerprint]
	return id, ok
}

type BuildSplit struct {
	Assignment
	// Unbuilt are the games that fit no build.
	Unbuilt []DeckGame
	// Suggestions group the games no single build takes, the ones that fit two builds too.
	Suggestions []Suggestion
}

// Builds splits a leader's games into the builds. card looks up the searchers that name the
// suggestions.
func Builds(gs []DeckGame, builds []store.Build, dismissed [][]string, card func(code string) cards.Card) BuildSplit {
	s := BuildSplit{Assignment: Assign(gs, builds)}
	var rest []DeckGame
	for _, g := range gs {
		id, ok := s.In(g.Fingerprint)
		if !ok {
			s.Unbuilt = append(s.Unbuilt, g)
		}
		if !ok || id == conflict {
			rest = append(rest, g)
		}
	}
	s.Suggestions = suggest(rest, dismissed)
	for i := range s.Suggestions {
		s.Suggestions[i].Name = suggestionName(s.Suggestions[i], s.Suggestions, card)
	}
	return s
}

func runsAll(list map[string]int, core []string) bool {
	for _, c := range core {
		if list[c] == 0 {
			return false
		}
	}
	return true
}

// StapleShare is the share of a leader's games a card has to be in to be a staple, a card that
// can't tell the leader's builds apart.
const StapleShare = 0.95

func Staples(gs []DeckGame) []string { return staples(cardGames(gs), len(gs)) }

func staples(in map[string]int, games int) []string {
	var out []string
	for c, n := range in {
		if float64(n) >= StapleShare*float64(games) {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	return out
}

// cardGames counts the games that ran each card.
func cardGames(gs []DeckGame) map[string]int {
	in := make(map[string]int)
	for _, g := range gs {
		for c := range g.Facts.List {
			in[c]++
		}
	}
	return in
}

// unsaved stands in for the ID of a build not saved yet, since 0 is conflict.
const unsaved = -1

// BuildEdit is where a leader's games go when one build's core changes.
type BuildEdit struct {
	// Now holds the games the build has, After the ones it gets with the new core, and Others
	// the ones the other builds get then, by fingerprint.
	Now, After, Others map[string]bool
	Before, With       Record
	// Conflicts is how many games fit two builds with the new core.
	Conflicts int
	// Games counts the games that ran each card, and Staples are the cards nearly all ran.
	Games   map[string]int
	Staples []string
}

// EditBuild shows where the leader's games go with edited, one of builds with a changed core
// or a new build with ID 0.
func EditBuild(gs []DeckGame, builds []store.Build, edited store.Build) BuildEdit {
	e := BuildEdit{Now: make(map[string]bool), After: make(map[string]bool), Others: make(map[string]bool)}
	var others []store.Build
	for _, b := range builds {
		if b.ID != edited.ID {
			others = append(others, b)
		}
	}
	var now []DeckGame
	if edited.ID != 0 {
		now = Assign(gs, builds).Builds[edited.ID]
	}
	for _, g := range now {
		e.Now[g.Fingerprint] = true
	}
	e.Before = RecordOf(now)
	if edited.ID == 0 {
		edited.ID = unsaved
	}
	a := Assign(gs, append(others, edited))
	for id, games := range a.Builds {
		for _, g := range games {
			if id == edited.ID {
				e.After[g.Fingerprint] = true
			} else {
				e.Others[g.Fingerprint] = true
			}
		}
	}
	e.With = RecordOf(a.Builds[edited.ID])
	e.Conflicts = len(a.Conflicts)
	e.Games = cardGames(gs)
	e.Staples = staples(e.Games, len(gs))
	return e
}

// searcherShare is the share of the games a card was in that it has to have searched in to
// count as a searcher. It keeps out a card whose effect looked at the deck once.
const searcherShare = 0.25

func Searchers(gs []DeckGame) []string {
	in := make(map[string]int)
	searched := make(map[string]int)
	for _, g := range gs {
		for c := range g.Facts.List {
			in[c]++
		}
		seen := make(map[string]bool)
		for _, s := range g.Facts.Searches {
			if !seen[s.Source] {
				seen[s.Source] = true
				searched[s.Source]++
			}
		}
	}
	var out []string
	for c, n := range searched {
		if in[c] > 0 && float64(n) >= searcherShare*float64(in[c]) {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	return out
}

// Suggestion is a group of games whose lists run the same searchers.
type Suggestion struct {
	// Core is the searchers, sorted, and Key the same joined, which tells suggestions apart.
	Core []string
	Key  string
	// Name is what the searchers that tell the suggestion apart look for.
	Name  string
	Games []DeckGame
}

// suggest groups the games by the searchers their lists run. Games without searchers and the
// groups the user dismissed aren't suggested. The biggest group comes first, and of two as big
// the one played first.
func suggest(gs []DeckGame, dismissed [][]string) []Suggestion {
	searchers := Searchers(gs)
	byKey := make(map[string]*Suggestion)
	var out []*Suggestion
	for _, g := range slices.Backward(gs) {
		var core []string
		for _, c := range searchers {
			if g.Facts.List[c] > 0 {
				core = append(core, c)
			}
		}
		if len(core) == 0 {
			continue
		}
		key := strings.Join(core, ",")
		s, ok := byKey[key]
		if !ok {
			s = &Suggestion{Core: core, Key: key}
			byKey[key] = s
			out = append(out, s)
		}
		s.Games = append(s.Games, g)
	}
	var kept []Suggestion
	for _, s := range out {
		if !slices.ContainsFunc(dismissed, func(d []string) bool { return slices.Equal(d, s.Core) }) {
			slices.Reverse(s.Games)
			kept = append(kept, *s)
		}
	}
	slices.SortStableFunc(kept, func(a, b Suggestion) int { return cmp.Compare(len(b.Games), len(a.Games)) })
	return kept
}

// CardStats is how one card did in a build's games.
type CardStats struct {
	Card   string
	Latest bool
	// Games are the games the card was in the deck for.
	Games int
	// Opening is the games it was in the hand kept after the mulligan.
	Opening Record
	// OnCurve is the games it was in the hand by the player's first turn with as much DON!! as
	// its printed cost, OffCurve the games it wasn't, and EndedFirst the games that ended before
	// that turn or don't say who went first.
	OnCurve, OffCurve Record
	EndedFirst        int
	// Curve is false when FromOpening holds for the card's cost, and OnCurve and OffCurve only
	// repeat the opening hand.
	Curve bool
	// The games it was played the turn it could first be paid for or earlier, played later, only
	// put out by an effect, or never played. A game counts once, in the first that applies.
	PlayedOnCurve, PlayedLater, ByEffect, NeverPlayed Record
	// Shown is how often a searcher that has taken the card in any of the games showed it, and
	// Taken how often it was taken then.
	Shown, Taken int
}

// Cards returns the stats of every card in the games' lists, in the order of the latest list,
// then the cards no longer in it.
func Cards(gs []DeckGame, card func(code string) cards.Card) []CardStats {
	byCard := make(map[string]*CardStats)
	get := func(c string) *CardStats {
		if s, ok := byCard[c]; ok {
			return s
		}
		s := &CardStats{Card: c}
		byCard[c] = s
		return s
	}
	takes := searcherTakes(gs)
	for _, g := range gs {
		kept, _ := g.Me.Kept()
		for c := range g.Facts.List {
			get(c).add(g, kept, card(c))
		}
		for _, sr := range g.Facts.Searches {
			for _, c := range uniq(append([]string{sr.Taken}, sr.Passed...)) {
				if c == "" || !takes[sr.Source][c] {
					continue
				}
				s := get(c)
				s.Shown++
				if c == sr.Taken {
					s.Taken++
				}
			}
		}
	}
	var latest map[string]int
	if len(gs) > 0 {
		latest = gs[0].Facts.List
	}
	out := make([]CardStats, 0, len(byCard))
	for _, s := range byCard {
		if s.Games > 0 {
			s.Latest = latest[s.Card] > 0
			out = append(out, *s)
		}
	}
	sortCards(out, card)
	return out
}

func searcherTakes(gs []DeckGame) map[string]map[string]bool {
	takes := make(map[string]map[string]bool)
	for _, g := range gs {
		for _, s := range g.Facts.Searches {
			if s.Taken == "" {
				continue
			}
			if takes[s.Source] == nil {
				takes[s.Source] = make(map[string]bool)
			}
			takes[s.Source][s.Taken] = true
		}
	}
	return takes
}

func (s *CardStats) add(g DeckGame, kept []string, info cards.Card) {
	s.Games++
	s.Curve = !FromOpening(info.PaidCost())
	if slices.Contains(kept, s.Card) {
		s.Opening.Add(g.Result)
	}
	curve, ok := curveTurn(g, info.PaidCost())
	switch {
	case !ok:
		s.EndedFirst++
	case inHandBy(g, kept, s.Card, curve):
		s.OnCurve.Add(g.Result)
	default:
		s.OffCurve.Add(g.Result)
	}
	paid, effect := plays(g, s.Card, info.Type == cards.Event)
	switch {
	case paid > 0 && ok && paid <= curve:
		s.PlayedOnCurve.Add(g.Result)
	case paid > 0:
		s.PlayedLater.Add(g.Result)
	case effect:
		s.ByEffect.Add(g.Result)
	default:
		s.NeverPlayed.Add(g.Result)
	}
}

func sortCards(cs []CardStats, card func(code string) cards.Card) {
	slices.SortFunc(cs, func(a, b CardStats) int {
		if a.Latest != b.Latest {
			if a.Latest {
				return -1
			}
			return 1
		}
		ca, cb := card(a.Card), card(b.Card)
		return cmp.Or(
			cmp.Compare(eventRank(ca), eventRank(cb)),
			cmp.Compare(ca.Cost, cb.Cost),
			cmp.Compare(b.Games, a.Games),
			cmp.Compare(a.Card, b.Card),
		)
	})
}

func eventRank(c cards.Card) int {
	if c.Type == cards.Event {
		return 1
	}
	return 0
}

func uniq(cs []string) []string {
	out := slices.Clone(cs)
	slices.Sort(out)
	return slices.Compact(out)
}

func ownTurn(g DeckGame, t int) bool { return simlog.FirstPlayersTurn(t) == g.WentFirst }

func curveTurn(g DeckGame, cost int) (int, bool) {
	if !g.TurnOrderKnown {
		return 0, false
	}
	for t, d := range g.Facts.Don {
		if t > 0 && ownTurn(g, t) && d >= cost {
			return t, true
		}
	}
	return 0, false
}

func inHandBy(g DeckGame, kept []string, card string, turn int) bool {
	if slices.Contains(kept, card) {
		return true
	}
	for _, e := range g.Facts.Events {
		if e.Card == card && e.Turn <= turn && (e.Kind == deck.Drew || e.Kind == deck.FromLife) {
			return true
		}
	}
	return false
}

// plays returns the first turn the player paid for the card, 0 when never, and whether an
// effect put it out. An Event leaving the hand next to its own lines was played.
func plays(g DeckGame, card string, event bool) (paid int, effect bool) {
	for _, e := range g.Facts.Events {
		if e.Card != card {
			continue
		}
		switch {
		case e.Kind == deck.PaidPlay, e.Kind == deck.OwnLines && event:
			if paid == 0 {
				paid = e.Turn
			}
		case e.Kind == deck.EffectPlay:
			effect = true
		}
	}
	return paid, effect
}

// ListStats describes a list's cards, counted in copies.
type ListStats struct {
	// Curve is the copies at each cost from 0 to 10.
	Curve [11]int
	// Counters are the Characters' copies by counter value: 0, 1000 and 2000.
	Counters map[int]int
	Types    map[string]int
	Colors   map[cards.Color]int
	// Traits are the traits by copies, most first. A card with several traits counts for each.
	Traits []TraitCount
}

type TraitCount struct {
	Trait  string
	Copies int
}

func List(list map[string]int, card func(code string) cards.Card) ListStats {
	st := ListStats{Counters: make(map[int]int), Types: make(map[string]int), Colors: make(map[cards.Color]int)}
	traits := make(map[string]int)
	for c, n := range list {
		info := card(c)
		st.Curve[min(info.PaidCost(), 10)] += n
		if info.Type == cards.Character {
			st.Counters[info.Counter] += n
		}
		if info.Type != "" {
			st.Types[info.Type] += n
		}
		for _, col := range info.Colors {
			st.Colors[col] += n
		}
		for _, t := range info.Traits {
			traits[t] += n
		}
	}
	for t, n := range traits {
		st.Traits = append(st.Traits, TraitCount{t, n})
	}
	slices.SortFunc(st.Traits, func(a, b TraitCount) int {
		return cmp.Or(cmp.Compare(b.Copies, a.Copies), cmp.Compare(a.Trait, b.Trait))
	})
	return st
}

// Change is a list the build switched to, with the games played on it before the next change.
type Change struct {
	At time.Time
	// Swaps are the copies in (more than 0) and out (less than 0), by card. The first list has
	// none.
	Swaps map[string]int
	Games []DeckGame
}

// Changes returns the lists the games went through, oldest first, each with its games oldest
// first.
func Changes(gs []DeckGame) []Change {
	var out []Change
	for _, g := range slices.Backward(gs) {
		list := g.Facts.List
		if len(out) > 0 && maps.Equal(out[len(out)-1].Games[0].Facts.List, list) {
			last := &out[len(out)-1]
			last.Games = append(last.Games, g)
			continue
		}
		ch := Change{At: g.PlayedAt, Games: []DeckGame{g}}
		if len(out) > 0 {
			ch.Swaps = swaps(out[len(out)-1].Games[0].Facts.List, list)
		}
		out = append(out, ch)
	}
	return out
}

func swaps(from, to map[string]int) map[string]int {
	out := make(map[string]int)
	for c := range from {
		if d := to[c] - from[c]; d != 0 {
			out[c] = d
		}
	}
	for c, n := range to {
		if _, ok := from[c]; !ok {
			out[c] = n
		}
	}
	return out
}

// suggestionName is what the suggestion's searchers look for, such as "Land of Wano", or a
// searcher's name when its effect doesn't say. Searchers every suggestion runs are left out,
// since they don't tell the suggestions apart.
func suggestionName(s Suggestion, all []Suggestion, card func(code string) cards.Card) string {
	shared := func(c string) bool {
		return !slices.ContainsFunc(all, func(o Suggestion) bool { return !slices.Contains(o.Core, c) })
	}
	telling := slices.DeleteFunc(slices.Clone(s.Core), func(c string) bool { return len(all) > 1 && shared(c) })
	if len(telling) == 0 {
		telling = s.Core
	}
	var names []string
	for _, c := range telling {
		info := card(c)
		targets := cards.SearchTargets(info.Effect)
		if len(targets) == 0 {
			targets = []string{info.Name}
		}
		for _, t := range targets {
			if !slices.Contains(names, t) {
				names = append(names, t)
			}
		}
	}
	return strings.Join(names, " + ")
}
