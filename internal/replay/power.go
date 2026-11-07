package replay

import (
	"slices"
)

type PartKind int

const (
	PartPrinted PartKind = iota + 1
	// PartDon is 1000 for each DON!! card given to it, on its owner's turn.
	PartDon
	PartEffect
	PartCounter
	// PartUnlogged is what the sim's "vs" line has that no logged effect explains, such as a
	// continuous effect or a leader's.
	PartUnlogged
)

// PowerPart is one thing a card's power is made of.
type PowerPart struct {
	Kind PartKind
	N    int
	// Source is the code of the card behind an effect or counter, or "".
	Source string
}

// Power is a card's power at a step.
type Power struct {
	Value int
	// Estimated is set when no "vs" line anchors the value, so it comes from card data and
	// the logged effects alone.
	Estimated bool
	// Parts add up to Value, except that the sim shows power below 0 as 0.
	Parts []PowerPart
}

func (p Power) sum() int {
	n := 0
	for _, part := range p.Parts {
		n += part.N
	}
	return n
}

// Floored reports whether the parts add up to less than 0, which the sim shows as 0.
func (p Power) Floored() bool { return p.sum() < 0 }

func (p Power) value() int { return max(p.sum(), 0) }

// PowerCheck compares the power worked out from card data and logged effects with the power
// the sim printed in a "vs" line.
type PowerCheck struct {
	Step     int
	Owner    int
	Code     string
	Attacker bool
	// Want is the sim's value and Got the worked-out one. Known is false when the card data
	// has no printed power for the card.
	Want, Got int
	Known     bool
}

// estimate works out a card's power on the board: printed power, DON on its owner's turn,
// then every power status.
func (b *builder) estimate(c *Card, s *Side, turn int) Power {
	printed := 0
	if b.power != nil {
		printed = b.power(c.Code)
	}
	p := Power{Estimated: true, Parts: []PowerPart{{Kind: PartPrinted, N: printed}}}
	if c.Don > 0 && b.owner[b.g.TurnPlayer(turn)] == s.Owner {
		p.Parts = append(p.Parts, PowerPart{Kind: PartDon, N: 1000 * c.Don})
	}
	for _, st := range c.Statuses {
		if st.Kind != StatusPower {
			continue
		}
		kind := PartEffect
		if st.Counter {
			kind = PartCounter
		}
		p.Parts = append(p.Parts, PowerPart{Kind: kind, N: st.N, Source: st.Source})
	}
	p.Value = p.value()
	return p
}

func (b *builder) known(code string) bool { return b.power != nil && b.power(code) > 0 }

// battleCards finds the attack's attacker and defender, by instance, with their sides.
func (b *builder) battleCards(bt *Battle) (cards [2]*Card, sides [2]*Side) {
	sides = [2]*Side{b.board.Side(bt.AttackerOwner), b.board.Side(3 - bt.AttackerOwner)}
	if sides[0] == nil || sides[1] == nil {
		return cards, sides
	}
	cards = [2]*Card{sides[0].byID(b.attackerID), sides[1].byID(b.defenderID)}
	if cards[1] == nil && b.left.ID == b.defenderID && b.defenderID != 0 {
		cards[1] = &b.left
	}
	return cards, sides
}

// stepPowers notes the attacker's and the defender's worked-out power at the end of a step of
// an attack.
func (b *builder) stepPowers(i int) {
	st := &b.steps[i]
	if st.Battle == nil {
		return
	}
	cards, sides := b.battleCards(st.Battle)
	if cards[0] != nil {
		st.Battle.Attack = b.estimate(cards[0], sides[0], st.Turn)
	}
	if cards[1] != nil {
		st.Battle.Defense = b.estimate(cards[1], sides[1], st.Turn)
	}
}

// checkVs compares the worked-out powers with a "vs" line's, and blames a power guess that
// would explain a difference.
func (b *builder) checkVs(step int, bt *Battle, want [2]int) {
	cards, sides := b.battleCards(bt)
	for i, c := range cards {
		if c == nil {
			continue
		}
		s := sides[i]
		got := b.estimate(c, s, b.steps[step].Turn).Value
		b.powerCompared++
		if got == want[i] || b.blamePower(c.Code, s.Owner, want[i]-got) {
			continue
		}
		b.powerChecks = append(b.powerChecks, PowerCheck{
			Step: step, Owner: s.Owner, Code: c.Code, Attacker: i == 0, Want: want[i], Got: got,
			Known: b.known(c.Code),
		})
	}
}

// pickPower finds the card a power effect names. Among several copies it takes the first on
// the likelier side, unless a "vs" line showed the buff went to another.
func (b *builder) pickPower(code string, n int, sides ...*Side) *Card {
	var cands []*Card
	var owners []int
	for _, s := range sides {
		for _, c := range s.all(code) {
			cands = append(cands, c)
			owners = append(owners, s.Owner)
		}
	}
	switch len(cands) {
	case 0:
		return nil
	case 1:
		return cands[0]
	}
	k := len(b.guesses)
	i := b.flips[k] % len(cands)
	b.guesses = append(b.guesses, guess{kind: guessPower, code: code, owners: owners, chosen: owners[i], n: n})
	return cands[i]
}

// blamePower finds the latest power guess on a copy of the card that a "vs" line's difference
// from the worked-out power would explain: a buff missing from it, or one it got wrongly.
func (b *builder) blamePower(code string, owner, diff int) bool {
	if b.final || diff == 0 {
		return false
	}
	for k := len(b.guesses) - 1; k >= 0; k-- {
		g := b.guesses[k]
		if g.kind != guessPower || g.code != code || b.blamed[k] || !slices.Contains(g.owners, owner) {
			continue
		}
		if g.chosen != owner && g.n == diff || g.chosen == owner && g.n == -diff {
			b.blamed[k] = true
			return true
		}
	}
	return false
}

// anchorPowers sets each attack's powers from its "vs" line where it has one. A step before
// the damage step gets the "vs" value less the changes logged after it, which is its
// worked-out value plus what the log leaves out. The target before a block has no "vs" value,
// and keeps its estimate.
func (b *builder) anchorPowers() {
	for i := range b.steps {
		d := &b.steps[i]
		if d.Kind != KindDamage || d.Battle == nil {
			continue
		}
		vsA, vsD := d.Battle.AttackerPower, d.Battle.TargetPower
		gapA, gapD := gap(vsA, d.Battle.Attack), gap(vsD, d.Battle.Defense)
		defender := d.Battle.Defender()
		for j := i; j >= 0; j-- {
			s := &b.steps[j]
			if s.Battle == nil || s.Battle.Attacker != d.Battle.Attacker {
				break
			}
			anchor(&s.Battle.Attack, gapA)
			if s.Battle.Defender() == defender {
				anchor(&s.Battle.Defense, gapD)
			}
			if s.Kind == KindAttack {
				break
			}
		}
	}
}

// gap is what a "vs" value has that the parts don't explain. A 0 in the line is any sum up to 0.
func gap(vs int, p Power) int {
	if vs == 0 && p.sum() <= 0 {
		return 0
	}
	return vs - p.sum()
}

func anchor(p *Power, gap int) {
	p.Estimated = false
	if gap != 0 {
		p.Parts = append(slices.Clip(p.Parts), PowerPart{Kind: PartUnlogged, N: gap})
	}
	p.Value = p.value()
}
