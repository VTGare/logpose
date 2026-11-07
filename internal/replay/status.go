package replay

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/VTGare/logpose/internal/simlog"
)

type StatusKind int

const (
	// StatusPower adds N to the card's power.
	StatusPower StatusKind = iota + 1
	// StatusFrozen keeps the card rested through its owner's next refresh.
	StatusFrozen
	StatusNoRest
	StatusNoAttack
	StatusNegated
	StatusProtected
	StatusRush
	StatusUnblockable
	// StatusCost sets the card's cost to N.
	StatusCost
)

// Status is a lasting effect on a card, from a readable line.
type Status struct {
	Kind StatusKind
	N    int
	// Source is the code of the card whose effect it is, or "".
	Source string
	// Counter is set on power from a counter: a card discarded for it, or an event.
	Counter bool
	// EndsTurn is the turn it ends at the end of, or 0 when a line or a refresh ends it.
	EndsTurn int
	// ForAttack is set on statuses that end with the attack in progress.
	ForAttack bool
}

func (c *Card) has(k StatusKind) bool {
	return slices.ContainsFunc(c.Statuses, func(s Status) bool { return s.Kind == k })
}

// add and drop replace Statuses rather than change it, since board copies share it.
func (c *Card) add(s Status) { c.Statuses = append(slices.Clip(c.Statuses), s) }

func (c *Card) drop(gone func(Status) bool) {
	if !slices.ContainsFunc(c.Statuses, gone) {
		return
	}
	var out []Status
	for _, s := range c.Statuses {
		if !gone(s) {
			out = append(out, s)
		}
	}
	c.Statuses = out
}

// Duration is how long a phrase's status lasts.
type Duration int

const (
	// UntilLine lasts until a line ends it.
	UntilLine Duration = iota
	ThisTurn
	// NextTurn lasts to the end of the turn after the one it was given in.
	NextTurn
	// OpponentsNextTurn lasts to the end of the next turn of the effect owner's opponent.
	OpponentsNextTurn
	Combat
	// UntilRefresh lasts until the card's owner's next refresh.
	UntilRefresh
)

// Phrase names a readable effect phrase that sets a status.
type Phrase string

const (
	PhraseGrant      Phrase = "Grant X N until next turn end"
	PhraseGrantOpp   Phrase = "Grant X N until opponent's next turn end"
	PhraseBuffCombat Phrase = "Buff X N for the Combat"
	PhraseBuff       Phrase = "Buff X N"
	PhraseBuffSelf   Phrase = "Buff Self N"
	// PhraseBasePower reads as a power change: "Set Imu Base Power to 2000" takes Imu from 5000 to
	// 7000 in the "vs" line.
	PhraseBasePower   Phrase = "Set X Base Power to N"
	PhraseCounter     Phrase = "Counter N"
	PhraseNoRest      Phrase = "X can't be rested until next turn ends"
	PhraseFrozen      Phrase = "X will not Activate during next Refresh"
	PhraseNoAttack    Phrase = "X can't attack next turn"
	PhraseNullify     Phrase = "Nullify effects of X"
	PhraseProtect     Phrase = "Prevent Effect To X"
	PhraseRush        Phrase = "Give X Rush"
	PhraseUnblockable Phrase = "Grant Unblockable to X"
	PhraseCost        Phrase = "X Cost N"
	PhraseCostOpp     Phrase = "X Cost N until Opponent's Turn End"
)

// Durations are how long each phrase lasts. The ones the log doesn't spell out were tested
// against the power the sim prints for every battle across 93 logs: see replays.md.
var Durations = map[Phrase]Duration{
	PhraseGrant:       NextTurn,
	PhraseGrantOpp:    OpponentsNextTurn,
	PhraseBuffCombat:  Combat,
	PhraseBuff:        ThisTurn,
	PhraseBuffSelf:    ThisTurn,
	PhraseBasePower:   ThisTurn,
	PhraseCounter:     Combat,
	PhraseNoRest:      NextTurn,
	PhraseFrozen:      UntilRefresh,
	PhraseNoAttack:    NextTurn,
	PhraseNullify:     ThisTurn,
	PhraseProtect:     ThisTurn,
	PhraseRush:        ThisTurn,
	PhraseUnblockable: ThisTurn,
	PhraseCost:        ThisTurn,
	PhraseCostOpp:     OpponentsNextTurn,
}

// effect is a status a line gives, before its card is found.
type effect struct {
	phrase Phrase
	kind   StatusKind
	n      int
	target string
	// own is set when the effect's owner's copy is the likelier target.
	own bool
}

var (
	nText = `(-?\d+)`
	// effectTexts match the part of an effect line after "Source [CODE]: ", with card links
	// as "Name [CODE]".
	effectTexts = []struct {
		re     *regexp.Regexp
		phrase Phrase
		kind   StatusKind
	}{
		{regexp.MustCompile(`^Grant .*\[([A-Z0-9-]+)\] ` + nText + ` until next turn end$`), PhraseGrant, StatusPower},
		{regexp.MustCompile(`^Grant .*\[([A-Z0-9-]+)\] ` + nText + ` until opponent's next turn end$`), PhraseGrantOpp, StatusPower},
		{regexp.MustCompile(`^Buff .*\[([A-Z0-9-]+)\] ` + nText + ` for the Combat$`), PhraseBuffCombat, StatusPower},
		{regexp.MustCompile(`^Buff .*\[([A-Z0-9-]+)\] ` + nText + `$`), PhraseBuff, StatusPower},
		{regexp.MustCompile(`^Buff Self` + `()` + ` ` + nText + `$`), PhraseBuffSelf, StatusPower},
		{regexp.MustCompile(`^Set .*\[([A-Z0-9-]+)\] Base Power to ` + nText + `$`), PhraseBasePower, StatusPower},
		{regexp.MustCompile(`^Counter()` + ` ` + nText + `$`), PhraseCounter, StatusPower},
		{regexp.MustCompile(`^.*\[([A-Z0-9-]+)\] can't be rested until next turn ends()$`), PhraseNoRest, StatusNoRest},
		{regexp.MustCompile(`^.*\[([A-Z0-9-]+)\] will not Activate during next Refresh()$`), PhraseFrozen, StatusFrozen},
		{regexp.MustCompile(`^.*\[([A-Z0-9-]+)\] can't attack next turn()$`), PhraseNoAttack, StatusNoAttack},
		{regexp.MustCompile(`^Nullify effects of .*\[([A-Z0-9-]+)\]()$`), PhraseNullify, StatusNegated},
		{regexp.MustCompile(`^Prevent Effect To .*\[([A-Z0-9-]+)\]()$`), PhraseProtect, StatusProtected},
		{regexp.MustCompile(`^Give .*\[([A-Z0-9-]+)\] (?:Character )?Rush()$`), PhraseRush, StatusRush},
		{regexp.MustCompile(`^Grant Unblockable to .*\[([A-Z0-9-]+)\]()$`), PhraseUnblockable, StatusUnblockable},
		{regexp.MustCompile(`^.*\[([A-Z0-9-]+)\] Cost (\d+) until Opponent's Turn End$`), PhraseCostOpp, StatusCost},
		{regexp.MustCompile(`^.*\[([A-Z0-9-]+)\] Cost (\d+)$`), PhraseCost, StatusCost},
	}
	// endTexts are lines that end a status early.
	endTexts = []struct {
		re   *regexp.Regexp
		kind StatusKind
	}{
		{regexp.MustCompile(`\[([A-Z0-9-]+)\] is no longer blocked from attacking$`), StatusNoAttack},
		{regexp.MustCompile(`\[([A-Z0-9-]+)\] no longer nullified$`), StatusNegated},
		{regexp.MustCompile(`\[([A-Z0-9-]+)\] cost restored$`), StatusCost},
	}
	discardText = regexp.MustCompile(`^Discard .*\[([A-Z0-9-]+)\] for Counter (\d+)$`)
)

// parseEffect reads the status an effect line gives. Self buffs and counters have no target
// in the line: the source, and the defender of the attack, take them.
func parseEffect(rest string) (effect, bool) {
	for _, t := range effectTexts {
		m := t.re.FindStringSubmatch(rest)
		if m == nil {
			continue
		}
		e := effect{phrase: t.phrase, kind: t.kind, target: m[1]}
		e.n, _ = strconv.Atoi(m[2])
		switch t.kind {
		case StatusPower:
			e.own = e.n >= 0
		case StatusRush, StatusUnblockable, StatusProtected, StatusCost:
			e.own = true
		}
		return e, true
	}
	return effect{}, false
}

// statusLine applies what a readable line does to statuses: one an effect gives, a counter
// card discarded, or the line that ends one early.
func (b *builder) statusLine(step int, l simlog.Line) {
	owner := b.owner[l.Player]
	mine := b.board.Side(owner)
	if mine == nil {
		return
	}
	theirs := b.board.Side(3 - owner)
	for _, t := range endTexts {
		if m := t.re.FindStringSubmatch(l.Text); m != nil {
			for _, c := range append(mine.all(m[1]), theirs.all(m[1])...) {
				c.drop(func(s Status) bool { return s.Kind == t.kind })
			}
			return
		}
	}
	if m := discardText.FindStringSubmatch(l.Text); m != nil {
		n, _ := strconv.Atoi(m[2])
		b.giveDefender(step, Status{Kind: StatusPower, N: n, Source: m[1], Counter: true, ForAttack: true})
		return
	}
	source, isEffect := effectSource(l)
	if !isEffect {
		return
	}
	_, rest, _ := strings.Cut(l.Text, ": ")
	e, ok := parseEffect(rest)
	if !ok {
		return
	}
	st := Status{Kind: e.kind, N: e.n, Source: source}
	b.expire(&st, e.phrase, l.Turn, owner)
	if cur := b.steps[step]; cur.Kind == KindCounter && cur.Cards[0] == source && e.kind == StatusPower {
		st.Counter = true
	}
	switch e.phrase {
	case PhraseCounter:
		st.Counter = true
		b.giveDefender(step, st)
		return
	case PhraseBuffSelf:
		e.target = source
	}
	sides := []*Side{theirs, mine}
	if e.own {
		sides = []*Side{mine, theirs}
	}
	if c := b.statusTarget(step, e, sides); c != nil {
		c.add(st)
	}
}

// statusTarget finds the copy an effect's status goes to. A power effect during an attack goes
// to a card in it. "Can't attack" goes to a copy that doesn't have it yet, and an attack by
// that copy later shows the guess was wrong.
func (b *builder) statusTarget(step int, e effect, sides []*Side) *Card {
	switch e.kind {
	case StatusPower:
		if c := b.battleCard(step, e.target, sides...); c != nil {
			return c
		}
		return b.pickPower(e.target, e.n, sides...)
	case StatusNoAttack:
		g := guess{kind: guessStatus, status: StatusNoAttack}
		return b.choose(e.target, g, nil, func(c *Card) bool { return c.has(StatusNoAttack) }, sides...)
	}
	c := b.pick(e.target, false, sides...)
	if c != nil && c.Rested && e.kind == StatusFrozen {
		c = b.pick(e.target, true, sides...)
	}
	return c
}

// battleCard is the attacker or the defender of the attack in progress, when the code is
// theirs. Power effects during an attack go to the cards in it. When both have the code, the
// one on the first side is taken.
func (b *builder) battleCard(step int, code string, sides ...*Side) *Card {
	st := b.steps[step]
	if st.Battle == nil || st.Kind == KindDamage {
		return nil
	}
	for _, s := range sides {
		for _, id := range []int{b.attackerID, b.defenderID} {
			if c := s.byID(id); c != nil && id != 0 && c.Code == code {
				return c
			}
		}
	}
	return nil
}

// expire sets when a status from a phrase ends, from the turn it was given in.
func (b *builder) expire(st *Status, p Phrase, turn, owner int) {
	switch Durations[p] {
	case ThisTurn:
		st.EndsTurn = turn
	case NextTurn:
		st.EndsTurn = turn + 1
	case OpponentsNextTurn:
		st.EndsTurn = turn + 1
		if b.owner[b.g.TurnPlayer(turn)] != owner {
			st.EndsTurn = turn + 2
		}
	case Combat:
		st.ForAttack = true
	}
}

// giveDefender adds a counter to the card the attack in progress lands on.
func (b *builder) giveDefender(step int, st Status) {
	bt := b.steps[step].Battle
	if bt == nil {
		return
	}
	if s := b.board.Side(3 - bt.AttackerOwner); s != nil {
		if c := s.byID(b.defenderID); c != nil {
			c.add(st)
		}
	}
}

// endTurn ends the statuses that last until the end of the turn.
func (b *builder) endTurn(turn int) {
	for o := 1; o <= 2; o++ {
		for _, c := range b.board.Side(o).cards() {
			c.drop(func(s Status) bool { return s.EndsTurn != 0 && s.EndsTurn <= turn || s.ForAttack })
		}
	}
}

// endCombat ends the statuses that last for an attack.
func (b *builder) endCombat() {
	for o := 1; o <= 2; o++ {
		for _, c := range b.board.Side(o).cards() {
			c.drop(func(s Status) bool { return s.ForAttack })
		}
	}
}
