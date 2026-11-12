package stats

import (
	"cmp"
	"slices"
)

type KeepSplit struct {
	Kept, Mulliganed Record
}

func (k *KeepSplit) add(mulliganed bool, r Result) {
	if mulliganed {
		k.Mulliganed.Add(r)
	} else {
		k.Kept.Add(r)
	}
}

// Mulligans is how the local player's games went by their mulligan, overall and by turn order,
// and by the opponent's.
type Mulligans struct {
	All, First, Second KeepSplit
	Opp                KeepSplit
	// Cards are the cards dealt in the first five, the most dealt first.
	Cards []DealtCard
}

// DealtCard is how the games went that a card was in the five the player was dealt, by whether
// they kept those five.
type DealtCard struct {
	Card           string
	Kept, SentBack Record
	// Games are the games Kept and SentBack count.
	Games []Match
}

// Dealt is the games the card was dealt in that have a result.
func (d DealtCard) Dealt() int { return d.Kept.Games() + d.SentBack.Games() }

// ForMulligans counts the matches by mulligan. A card counts once a game however many copies
// were dealt. Games whose log doesn't show the dealt hand count only in the splits.
func ForMulligans(ms []Match) Mulligans {
	var out Mulligans
	byCard := make(map[string]*DealtCard)
	for _, m := range ms {
		mul := m.Me.Mulliganed
		out.All.add(mul, m.Result)
		if m.TurnOrderKnown {
			if m.WentFirst {
				out.First.add(mul, m.Result)
			} else {
				out.Second.add(mul, m.Result)
			}
		}
		out.Opp.add(m.Opp.Mulliganed, m.Result)
		seen := make(map[string]bool)
		for _, c := range m.Me.HandBefore {
			if seen[c] {
				continue
			}
			seen[c] = true
			d := byCard[c]
			if d == nil {
				d = &DealtCard{Card: c}
				byCard[c] = d
			}
			if m.Result == Unknown {
				continue
			}
			d.Games = append(d.Games, m)
			if mul {
				d.SentBack.Add(m.Result)
			} else {
				d.Kept.Add(m.Result)
			}
		}
	}
	for _, d := range byCard {
		if d.Dealt() > 0 {
			out.Cards = append(out.Cards, *d)
		}
	}
	slices.SortFunc(out.Cards, func(a, b DealtCard) int {
		return cmp.Or(cmp.Compare(b.Dealt(), a.Dealt()), cmp.Compare(a.Card, b.Card))
	})
	return out
}
