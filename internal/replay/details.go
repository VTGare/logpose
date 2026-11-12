package replay

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/VTGare/logpose/internal/simlog"
)

type OutcomeKind int

const (
	OutcomeFailed OutcomeKind = iota + 1
	// OutcomeKO is Card K.O.'d.
	OutcomeKO
	// OutcomeHit is a hit for N damage.
	OutcomeHit
)

// Outcome is one thing that happened at an attack's damage.
type Outcome struct {
	Kind OutcomeKind
	Card string
	N    int
}

type CountKind int

const (
	// CountOther is a counted effect logpose has no words for. Verb and Object are the sim's.
	CountOther CountKind = iota
	CountDonActive
	CountDonRested
	CountDonReturned
	CountDonAdded
	CountRestedDonAdded
	CountCardsDrawn
)

// Count is an effect the sim writes once per card or DON, added up.
type Count struct {
	Kind         CountKind
	N            int
	Verb, Object string
}

type DidKind int

const (
	// DidText is an effect line logpose doesn't add up, in Text.
	DidText DidKind = iota + 1
	DidCount
	// DidPower is N power given to Card, added up for the card.
	DidPower
	// DidTrash is Card trashed to pay for the effect.
	DidTrash
)

// Did is one thing an effect did.
type Did struct {
	Kind DidKind
	// Text is the effect as the line writes it after "Card [CODE]: ", with card links as
	// "Name [CODE]".
	Text  string
	Count Count
	N     int
	Card  string
}

// Effect is what the card of a KindEffect or KindBattleEffect step did.
type Effect struct {
	// Head is how the step's first effect line names its card, as "Name [CODE]". It is set on
	// KindEffect steps with effect lines.
	Head string
	// Did is, on a KindEffect step, the first thing the effect did. On a KindBattleEffect step
	// it is everything, each thing once with its counts and power added up, in the order of the
	// lines.
	Did []Did
}

// countText is an effect the sim writes once per card or DON, such as "Activate 1 Don" five
// times for five DON.
var countText = regexp.MustCompile(`^(Activate|Rest|Minus|Draw|Trash) (\d+) (Don|Rested Don|Card|Cards from Deck|Remaining Cards)$`)

var countKinds = map[string]CountKind{
	"Activate Don": CountDonActive, "Rest Don": CountDonRested, "Minus Don": CountDonReturned,
	"Draw Don": CountDonAdded, "Draw Rested Don": CountRestedDonAdded, "Draw Card": CountCardsDrawn,
}

// counted reads a counted effect. "Rest Don [Don]" is one DON.
func counted(rest string) (Count, bool) {
	if rest == "Rest Don [Don]" {
		return Count{Kind: CountDonRested, N: 1, Verb: "Rest", Object: "Don"}, true
	}
	m := countText.FindStringSubmatch(rest)
	if m == nil {
		return Count{}, false
	}
	n, _ := strconv.Atoi(m[2])
	return Count{Kind: countKinds[m[1]+" "+m[3]], N: n, Verb: m[1], Object: m[3]}, true
}

func (c Count) same(o Count) bool { return c.Verb == o.Verb && c.Object == o.Object }

// detail fills in what a step's lines and moves say happened in it.
func (b *builder) detail(s *Step) {
	switch s.Kind {
	case KindTurnStart:
		b.turnStart(s)
	case KindAttach:
		s.Don = b.g.Lines[s.starter].N
	case KindCounter:
		s.Counter = b.counter(s)
	case KindDamage:
		s.Outcome = b.outcome(s)
	case KindEffect:
		s.Effect = b.effectDid(s)
	case KindBattleEffect:
		s.Effect = b.battleEffectDid(s)
	}
}

// turnStart counts the turn player's draws and DON, up to the first move after the DON.
func (b *builder) turnStart(s *Step) {
	owner := b.owner[s.Player]
	for _, mi := range s.Moves {
		m := b.g.Moves[mi]
		if m.Owner != owner {
			continue
		}
		switch {
		case m.From == simlog.ZoneDeck && m.To == simlog.ZoneHand:
			s.Drew = append(s.Drew, m.Card)
		case m.From == simlog.ZoneDonDeck && m.To == simlog.ZoneDonField:
			s.Don++
		}
		if s.Don > 0 && m.From != simlog.ZoneDonDeck {
			break
		}
	}
}

func (b *builder) outcome(s *Step) []Outcome {
	var out []Outcome
	for _, li := range s.Lines {
		l := b.g.Lines[li]
		switch {
		case l.Kind == simlog.LineAttackFails:
			out = append(out, Outcome{Kind: OutcomeFailed})
		case l.Kind == simlog.LineKO && len(l.Cards) == 1:
			out = append(out, Outcome{Kind: OutcomeKO, Card: l.Cards[0]})
		case l.Kind == simlog.LineHit:
			out = append(out, Outcome{Kind: OutcomeHit, N: l.N})
		}
	}
	return out
}

// counter is the power a counter step gave: the counter of a discarded card, or else the
// power its effect gave. It is 0 when the log doesn't say.
func (b *builder) counter(s *Step) int {
	for _, li := range s.Lines {
		if l := b.g.Lines[li]; l.Kind == simlog.LineCounterDiscard {
			return l.N
		}
	}
	n := 0
	for _, l := range b.sourceLines(s) {
		if e, ok := parseEffect(l); ok && e.kind == StatusPower {
			n += e.n
		}
	}
	return max(n, 0)
}

// sourceLines are the step's lines from the effect of its first card.
func (b *builder) sourceLines(s *Step) []simlog.Line {
	var out []simlog.Line
	for _, li := range s.Lines {
		l := b.g.Lines[li]
		if l.Source != "" && l.Source == s.Cards[0] {
			out = append(out, l)
		}
	}
	return out
}

// effectDid is the first thing the step's effect does. An effect written once per card or DON
// is added up over the step.
func (b *builder) effectDid(s *Step) *Effect {
	lines := b.sourceLines(s)
	if len(lines) == 0 {
		return nil
	}
	head, rest, _ := strings.Cut(lines[0].Text, ": ")
	e := &Effect{Head: head}
	c, ok := counted(rest)
	if !ok {
		e.Did = []Did{{Kind: DidText, Text: rest}}
		return e
	}
	for _, l := range lines[1:] {
		if k, ok := counted(l.EffectText()); ok && k.same(c) {
			c.N += k.N
		}
	}
	e.Did = []Did{{Kind: DidCount, Count: c}}
	return e
}

// battleEffectDid is everything an effect during an attack did. Power for one card and effects
// written once per card or DON are added up. A self buff goes to the step's card.
func (b *builder) battleEffectDid(s *Step) *Effect {
	var did []Did
	at := make(map[string]int)
	for _, l := range b.sourceLines(s) {
		rest := l.EffectText()
		var d Did
		var key string
		if c, ok := counted(rest); ok {
			d, key = Did{Kind: DidCount, Count: c}, "count "+c.Verb+" "+c.Object
		} else if e, ok := parseEffect(l); ok && e.kind == StatusPower {
			if e.target == "" {
				e.target = s.Cards[0]
			}
			d, key = Did{Kind: DidPower, Card: e.target, N: e.n}, "power "+e.target
		} else if strings.HasPrefix(rest, "Trash ") && len(l.Cards) == 2 {
			did = append(did, Did{Kind: DidTrash, Card: l.Cards[1]})
			continue
		} else {
			d, key = Did{Kind: DidText, Text: rest}, "text "+rest
		}
		i, seen := at[key]
		if !seen {
			at[key] = len(did)
			did = append(did, d)
			continue
		}
		did[i].N += d.N
		did[i].Count.N += d.Count.N
	}
	return &Effect{Did: did}
}
