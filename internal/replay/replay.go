// Package replay rebuilds the board of a stored game at every step, from its RZ1 moves and
// readable lines, and checks it against the log's own zone counts.
package replay

import (
	"errors"

	"github.com/VTGare/logpose/internal/simlog"
)

type Phase int

const (
	// PhaseSetup is before the first turn: shuffles, opening hands, mulligans and life.
	PhaseSetup Phase = iota
	// PhaseStart is a turn's refresh, draw and DON phases.
	PhaseStart
	PhaseMain
	// PhaseBattle is an attack, from its declaration to the effects after its damage.
	PhaseBattle
	// PhaseEnd is the game's end: a concession or a deck out.
	PhaseEnd
)

func (p Phase) String() string {
	switch p {
	case PhaseSetup:
		return "Setup"
	case PhaseStart:
		return "Start"
	case PhaseMain:
		return "Main"
	case PhaseBattle:
		return "Battle"
	default:
		return "End"
	}
}

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
	Turn  int
	Phase Phase
	Kind  Kind
	// Player is the ID of the player who acted.
	Player string
	// Title says what happened, with card names, such as "Monkey D. Luffy attacked Rocks D.
	// Xebec".
	Title string
	// Result is how an attack ended, such as "Hit for 1" or "Attack failed", or "".
	Result string
	// Cards are the cards the step is about: the card played, the attacker and its target, or
	// the source of an effect.
	Cards []string
	// Battle is set on the steps of an attack.
	Battle *Battle
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
	// AttackerOwner is the attacker's RZ1 owner.
	AttackerOwner int
	// AttackerID and DefenderID tell the attacking card and the card the attack lands on apart
	// from other copies, as Card.ID does, or are 0 when not known.
	AttackerID, DefenderID int
	// Blocker is the card that blocked, or "".
	Blocker string
	// AttackerPower and TargetPower are the powers in the sim's "vs" line, set from the
	// damage step on.
	AttackerPower, TargetPower int
	// Attack and Defense are the attacker's and the defending card's power at the step.
	Attack, Defense Power
}

// Defender is the card the attack lands on: the blocker, or else the target.
func (bt Battle) Defender() string {
	if bt.Blocker != "" {
		return bt.Blocker
	}
	return bt.Target
}

// Replay is a game as steps.
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
	names         map[string]string
}

var ErrNoMoves = errors.New("replay: the game has no RZ1 moves")

// Build replays a parsed game. power gives a card's printed power, or 0 when it isn't known;
// nil leaves powers to the "vs" lines alone. Games from sim versions without the RZ1 stream
// can't be replayed.
func Build(g *simlog.Game, power func(code string) int) (*Replay, error) {
	if !g.HasRZ() {
		return nil, ErrNoMoves
	}
	b := newBuilder(g, power)
	b.run()
	r := &Replay{
		Steps: b.steps, Issues: b.issues, Unverified: -1, PowerChecks: b.powerChecks,
		PowerCompared: b.powerCompared, names: g.CardNames,
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

// FirstOfTurn is the first step of the turn, or -1.
func (r *Replay) FirstOfTurn(turn int) int {
	for i, s := range r.Steps {
		if s.Turn == turn {
			return i
		}
	}
	return -1
}

// Name is a card's name from the log, or its code.
func (r *Replay) Name(code string) string {
	if n, ok := r.names[code]; ok {
		return n
	}
	return code
}
