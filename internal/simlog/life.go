package simlog

// LifeByTurn is a player's life from the RZ1 checks. Index 0 is the start of the game and index
// t the end of turn t, up to the turn the game ended in. It is nil for a player without an RZ1
// owner or without checks. Life cards are dealt after the mulligans, so checks before then read
// 0 and are skipped.
func (g *Game) LifeByTurn(id string) []int {
	return g.countByTurn(id, func(c Check) int { return c.Life }, true)
}

// HandByTurn is how many cards a player holds, indexed as LifeByTurn is.
func (g *Game) HandByTurn(id string) []int {
	return g.countByTurn(id, func(c Check) int { return c.Hand }, false)
}

// CharactersByTurn is how many characters a player has on the board, indexed as LifeByTurn is.
func (g *Game) CharactersByTurn(id string) []int {
	return g.countByTurn(id, func(c Check) int { return c.Character }, false)
}

// countByTurn takes count from the player's last check in each turn. A turn without a check
// keeps the turn before's count. With dealt set, checks are skipped until count is above 0.
func (g *Game) countByTurn(id string, count func(Check) int, dealt bool) []int {
	p, ok := g.Player(id)
	if !ok || p.Owner == 0 || len(g.Lines) == 0 {
		return nil
	}
	last := g.lastTurn()
	known := make([]bool, last+1)
	out := make([]int, last+1)
	started := !dealt
	for _, c := range g.Checks {
		if c.Owner != p.Owner {
			continue
		}
		n := count(c)
		if started = started || n > 0; !started {
			continue
		}
		t := g.TurnOf(c.Line)
		out[t], known[t] = n, true
	}
	first := -1
	for t := range out {
		if known[t] {
			first = t
			break
		}
	}
	if first < 0 {
		return nil
	}
	for t := range out {
		switch {
		case t < first:
			out[t] = out[first]
		case !known[t]:
			out[t] = out[t-1]
		}
	}
	return out
}

func (g *Game) lastTurn() int {
	if len(g.Lines) == 0 {
		return 0
	}
	return g.Lines[len(g.Lines)-1].Turn
}

// TurnOf is the turn of the line at index i in Lines, as Move.Line and Check.Line give it. An
// index past the last line is in the last line's turn.
func (g *Game) TurnOf(i int) int {
	if i < len(g.Lines) {
		return g.Lines[i].Turn
	}
	return g.lastTurn()
}

// TurnPlayer is the ID of whoever ended turn t, or by turn order when the game ended in it.
func (g *Game) TurnPlayer(t int) string {
	for _, l := range g.Lines {
		if l.Turn == t && l.Kind == LineEndTurn && l.Player != "" {
			return l.Player
		}
	}
	if g.First == "" || t == 0 {
		return ""
	}
	if FirstPlayersTurn(t) {
		return g.First
	}
	for _, p := range g.Players {
		if p.ID() != g.First {
			return p.ID()
		}
	}
	return ""
}

func FirstPlayersTurn(t int) bool { return t%2 == 1 }
