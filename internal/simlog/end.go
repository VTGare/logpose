package simlog

// endTracker works out games that ended without a "Concedes!" or "Out of Cards" line. The sim
// stops logging when a game ends, so a lethal attack shows up as the last attack on a leader
// whose owner has no life left, sometimes with its "hit for" line and sometimes without.
type endTracker struct {
	// lifeByOwner is each RZ1 owner's life count from the latest CHK line.
	lifeByOwner map[int]int
	// lifeByPlayer is each player's life from the latest "Life: N" line, for logs without RZ1
	// owners.
	lifeByPlayer map[string]int
	// lifeTaken records owners who lost a life card since the latest attack was declared.
	lifeTaken map[int]bool
	attack    *attack
}

// attack is the latest attack on a leader that hasn't failed.
type attack struct {
	defender string
	hit      bool
	// tookLife is set when the hit moved a life card. The sim writes that move before the hit
	// line, so a defender at 0 life after a hit either had 0 before it, which loses the game,
	// or just lost the last card.
	tookLife bool
}

func newEndTracker() endTracker {
	return endTracker{
		lifeByOwner:  make(map[int]int),
		lifeByPlayer: make(map[string]int),
		lifeTaken:    make(map[int]bool),
	}
}

func (t *endTracker) attacked(defender string, onLeader bool) {
	clear(t.lifeTaken)
	if onLeader {
		t.attack = &attack{defender: defender}
		return
	}
	t.attack = nil
}

func (t *endTracker) hit(g *Game) {
	if t.attack == nil {
		return
	}
	t.attack.hit = true
	if p, ok := g.Player(t.attack.defender); ok && p.Owner != 0 {
		t.attack.tookLife = t.lifeTaken[p.Owner]
	}
}

func (t *endTracker) failed() { t.attack = nil }

func (t *endTracker) lethal(g *Game) End {
	a := t.attack
	if a == nil || (a.hit && a.tookLife) {
		return End{}
	}
	p, ok := g.Player(a.defender)
	if !ok {
		return End{}
	}
	life, known := t.lifeByPlayer[p.ID()]
	if p.Owner != 0 {
		life, known = t.lifeByOwner[p.Owner]
	}
	if !known || life != 0 {
		return End{}
	}
	return End{Kind: EndLethal, Loser: p.ID()}
}
