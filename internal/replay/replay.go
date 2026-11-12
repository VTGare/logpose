// Package replay rebuilds the board of a stored game at every step, from its RZ1 moves and
// readable lines, and checks it against the log's own zone counts.
package replay

import (
	"errors"

	"github.com/VTGare/logpose/internal/simlog"
)

type Kind int

const (
	// KindLobby is the lines before any card moves. Steps only have it in games without
	// moves before the first turn.
	KindLobby Kind = iota
	// KindOpeningHand is a player's shuffle and first five cards.
	KindOpeningHand
	KindMulligan
	KindLife
	// KindTurnStart is the refresh, draw and DON of one turn.
	KindTurnStart
	// KindPlay is a card played from hand: a character, stage or event.
	KindPlay
	KindAttach
	// KindAttack is an attack's declaration and what the attacker's effects do then.
	KindAttack
	// KindBlock, KindCounter and KindDamage are the attack's blocker step, one counter card
	// each, and the damage from the "vs" line on.
	KindBlock
	KindCounter
	KindDamage
	// KindBattleEffect is an effect activated during an attack before its damage, from a card
	// other than the step's own: such as the defender's [On Your Opponent's Attack] effect.
	KindBattleEffect
	// KindEffect is an effect the turn's player activated on its own, such as a leader's.
	KindEffect
	KindConcede
	KindDeckOut
)

// Step is one action, with the board after it.
type Step struct {
	Turn   int
	Kind   Kind
	Player string
	// Cards are the cards the step is about: the card played, the attacker and its target, or
	// the source of an effect.
	Cards  []string
	Battle *Battle
	// Drew is the cards the turn's player drew in a KindTurnStart step. Don is the DON!! they
	// got in it, or the DON!! given in a KindAttach step.
	Drew []string
	Don  int
	// Counter is the power a KindCounter step gave, or 0 when the log doesn't say.
	Counter int
	// Outcome is how the attack ended, on its KindDamage step.
	Outcome []Outcome
	// Lethal is set on the last step of the attack that won the game.
	Lethal bool
	Effect *Effect
	// Lines are indexes into simlog.Game.Lines, and Moves into simlog.Game.Moves.
	Lines []int
	Moves []int
	Board Board
	// starter is the index of the line the step starts on, or -1.
	starter int
}

// Battle is an attack's cards and their power at a step.
type Battle struct {
	Attacker, Target string
	AttackerOwner    int
	// AttackerID and DefenderID tell the attacking card and the card the attack lands on apart
	// from other copies, as Card.ID does, or are 0 when not known.
	AttackerID, DefenderID int
	Blocker                string
	// AttackerPower and TargetPower are the powers in the sim's "vs" line, set from the
	// damage step on.
	AttackerPower, TargetPower int
	// Attack and Defense are the attacker's and the defending card's power at the step.
	Attack, Defense Power
	// Left is the defender as it was when it left the board in this step, from LeftSlot of its
	// characters, or nil.
	Left     *Card
	LeftSlot int
}

func (bt Battle) Defender() string {
	if bt.Blocker != "" {
		return bt.Blocker
	}
	return bt.Target
}

type Replay struct {
	Steps  []Step
	Issues []Issue
	// Unverified is the first step whose zone counts differ from the log's, or -1 when every
	// step matches.
	Unverified int
	// Guesses counts the lines that named a card with more than one copy in play in the state
	// the line acts on, and Corrected the guesses a later attack showed were another copy.
	// PowerGuesses and PowerCorrected count the same for power effects, which "vs" lines
	// correct.
	Guesses, Corrected           int
	PowerGuesses, PowerCorrected int
	// PowerChecks are the attackers' and defenders' worked-out powers that differ from the
	// sim's "vs" lines.
	PowerChecks []PowerCheck
	// PowerCompared counts the powers compared with "vs" lines.
	PowerCompared int
}

// Version goes up whenever Build gives different steps or boards for the same game, so facts
// read from them are read again.
const Version = 2

var ErrNoMoves = errors.New("replay: the game has no RZ1 moves")

// Build replays a parsed game. power gives a card's printed power, or 0 when it isn't known;
// nil leaves powers to the "vs" lines alone. Games from sim versions without the RZ1 stream
// can't be replayed.
func Build(g *simlog.Game, power func(code string) int, opts ...Option) (*Replay, error) {
	if !g.HasRZ() {
		return nil, ErrNoMoves
	}
	b := newBuilder(g, power)
	for _, o := range opts {
		o(b)
	}
	b.run()
	r := &Replay{
		Steps: b.steps, Issues: b.issues, Unverified: -1, PowerChecks: b.powerChecks,
		PowerCompared: b.powerCompared,
	}
	for k, gs := range b.guesses {
		flipped := b.flips[k] > 0
		switch gs.kind {
		case guessPower:
			r.PowerGuesses++
			if flipped {
				r.PowerCorrected++
			}
		default:
			r.Guesses++
			if flipped {
				r.Corrected++
			}
		}
	}
	for _, is := range b.issues {
		if is.Kind == IssueCount {
			r.Unverified = is.Step
			break
		}
	}
	return r, nil
}

// StepAtLine is the step that holds the readable line, or the first after it, such as for
// opening a key moment.
func (r *Replay) StepAtLine(line int) int {
	for i, s := range r.Steps {
		for _, l := range s.Lines {
			if l >= line {
				return i
			}
		}
	}
	return max(len(r.Steps)-1, 0)
}

// FirstOfTurn is -1 when the turn has no step.
func (r *Replay) FirstOfTurn(turn int) int {
	for i, s := range r.Steps {
		if s.Turn == turn {
			return i
		}
	}
	return -1
}
