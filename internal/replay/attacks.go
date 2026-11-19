package replay

import (
	"strings"

	"github.com/VTGare/logpose/internal/simlog"
)

type AnswerKind int

const (
	AnswerBlock AnswerKind = iota + 1
	// AnswerCounter is a card discarded for its counter, and AnswerEvent a counter event.
	AnswerCounter
	AnswerEvent
	// AnswerEffect is power from an effect of the defender's, such as a leader's.
	AnswerEffect
)

// Answer is one thing the defender did against an attack. Power is what it added, or 0 for a
// block.
type Answer struct {
	Kind  AnswerKind
	Card  string
	Power int
}

// Action is a card the attacking player played, or whose effect they used, between two
// attacks. Effects are the effect lines that act on other cards, after "Card [CODE]: ".
type Action struct {
	Card    string
	Played  bool
	Effects []string
}

// TurnAttack is one attack of a turn and what came of it.
type TurnAttack struct {
	Attacker, Target, Blocker string
	// Power and Defense are the attacker's and the defending card's power in the sim's "vs"
	// line, after counters. An attack the log ends on has no "vs" line, so they are the powers
	// worked out when it was declared.
	Power, Defense int
	// DonActive is the attacking player's active DON!! when the attack was declared.
	DonActive int
	Answers   []Answer
	// Outcome is OutcomeHit, OutcomeKO or OutcomeFailed, or 0 when the log has no damage step.
	Outcome OutcomeKind
	Lethal  bool
	// Life is the defending player's life after the attack.
	Life int
	// Before is what the attacking player did since the attack before.
	Before []Action
}

// Attacks is every attack the turn's player made in turn t, in order.
func (r *Replay) Attacks(g *simlog.Game, t int) []TurnAttack {
	ar := attackReader{g: g, player: g.TurnPlayer(t)}
	for _, s := range r.Steps {
		if s.Turn == t {
			ar.step(s)
		}
	}
	return ar.out
}

type attackReader struct {
	g      *simlog.Game
	player string
	out    []TurnAttack
	// pending is what the player did since the last attack.
	pending []Action
}

func (ar *attackReader) step(s Step) {
	if s.Kind == KindAttack {
		ar.attack(s)
		return
	}
	if len(ar.out) == 0 {
		return
	}
	a := &ar.out[len(ar.out)-1]
	if s.Lethal {
		a.Lethal = true
	}
	switch s.Kind {
	case KindBlock:
		if len(s.Cards) > 0 {
			a.Blocker = s.Cards[0]
			a.Answers = append(a.Answers, Answer{Kind: AnswerBlock, Card: s.Cards[0]})
		}
	case KindCounter:
		if len(s.Cards) > 0 {
			a.Answers = append(a.Answers, ar.counter(s))
		}
	case KindBattleEffect:
		if n := effectPower(s); n > 0 && s.Player != ar.player {
			a.Answers = append(a.Answers, Answer{Kind: AnswerEffect, Card: s.Cards[0], Power: n})
		}
	case KindDamage:
		damage(a, s)
	case KindPlay, KindEffect:
		ar.action(s)
	}
}

func (ar *attackReader) attack(s Step) {
	if s.Player != ar.player || s.Battle == nil {
		return
	}
	b := s.Battle
	ar.out = append(ar.out, TurnAttack{
		Attacker: b.Attacker, Target: b.Target, Power: b.Attack.Value, Defense: b.Defense.Value,
		DonActive: s.Board.Sides[b.AttackerOwner-1].DonActive(), Lethal: s.Lethal,
		Life: len(s.Board.Sides[2-b.AttackerOwner].Life), Before: ar.pending,
	})
	ar.pending = nil
}

func (ar *attackReader) counter(s Step) Answer {
	k := AnswerEvent
	for _, li := range s.Lines {
		if ar.g.Lines[li].Kind == simlog.LineCounterDiscard {
			k = AnswerCounter
		}
	}
	return Answer{Kind: k, Card: s.Cards[0], Power: s.Counter}
}

func effectPower(s Step) int {
	if s.Effect == nil || len(s.Cards) == 0 {
		return 0
	}
	n := 0
	for _, d := range s.Effect.Did {
		if d.Kind == DidPower {
			n += d.N
		}
	}
	return n
}

func damage(a *TurnAttack, s Step) {
	if s.Battle == nil {
		return
	}
	a.Power, a.Defense = s.Battle.AttackerPower, s.Battle.TargetPower
	for _, o := range s.Outcome {
		if a.Outcome == 0 || o.Kind == OutcomeHit {
			a.Outcome = o.Kind
		}
	}
	a.Life = len(s.Board.Sides[2-s.Battle.AttackerOwner].Life)
}

func (ar *attackReader) action(s Step) {
	if s.Player != ar.player || len(s.Cards) == 0 {
		return
	}
	act := Action{Card: s.Cards[0], Played: s.Kind == KindPlay, Effects: actsOn(ar.g, s)}
	if act.Played || len(act.Effects) > 0 {
		ar.pending = append(ar.pending, act)
	}
}

// actsOn is the step's effect lines from its card that name another card, without the ones
// that only count DON!! or cards or attach DON!!.
func actsOn(g *simlog.Game, s Step) []string {
	var out []string
	for _, li := range s.Lines {
		l := g.Lines[li]
		if l.Kind != simlog.LineEffect || l.Source != s.Cards[0] {
			continue
		}
		rest := l.EffectText()
		if _, ok := counted(rest); ok || strings.HasPrefix(rest, "Attach ") {
			continue
		}
		for _, c := range l.Cards[1:] {
			if c != l.Source {
				out = append(out, rest)
				break
			}
		}
	}
	return out
}
