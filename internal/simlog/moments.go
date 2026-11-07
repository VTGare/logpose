package simlog

import (
	"slices"
	"strings"
)

type ReasonKind int

const (
	// ReasonRemoved is Player removing the other player's characters, as many as Moment.Removed.
	ReasonRemoved ReasonKind = iota + 1
	// ReasonLifeLost is Player losing N life in the turn.
	ReasonLifeLost
	ReasonAtOne
	ReasonAtZero
	// ReasonBoard is Player ending their turn with N more characters than they started with,
	// on the board in Moment.Board.
	ReasonBoard
	// ReasonFinish is the turn the game ended in. Player is the loser.
	ReasonFinish
)

type Reason struct {
	Kind   ReasonKind
	Player string
	N      int
}

// Moment is a turn that changed the game.
type Moment struct {
	Turn int
	// Player is the ID of the player whose turn it was, or "" when the log doesn't say.
	Player string
	// Line is the index in Game.Lines of the turn's first line.
	Line    int
	Reasons []Reason
	// Played is the cards the turn's player put on the board, characters and stages, in order.
	Played []string
	// Removed is the other player's characters that left the board.
	Removed []string
	// Board is the turn's player's characters at its end, in the order they came onto the
	// board.
	Board []string
}

// Removing characters is a moment at bigRemovals of bigPower or more, or at anyRemovals of
// any power.
const (
	bigPower    = 5000
	bigRemovals = 2
	anyRemovals = 3
	lifeLost    = 2
	boardGain   = 3
)

// Moments picks the turns that mattered, in order. power gives a card's printed power, or 0
// when it isn't known. First life taken isn't a moment: it's rarely contested and plays like a
// draw.
func (g *Game) Moments(power func(code string) int) []Moment {
	last := g.lastTurn()
	if last == 0 {
		return nil
	}
	ms := make([]Moment, last+1)
	for i := len(g.Lines) - 1; i >= 0; i-- {
		ms[g.Lines[i].Turn].Line = i
	}
	for t := range ms {
		ms[t].Turn = t
		ms[t].Player = g.TurnPlayer(t)
	}
	g.addCards(ms)
	addRemovals(ms, power)
	for _, p := range g.Players {
		g.addLife(ms, p.ID())
		g.addBoards(ms, p.ID())
	}
	if g.End.Kind != EndUnknown {
		ms[last].Reasons = append(ms[last].Reasons, Reason{Kind: ReasonFinish, Player: g.End.Loser})
	}
	var out []Moment
	for _, m := range ms[1:] {
		if len(m.Reasons) > 0 {
			out = append(out, m)
		}
	}
	return out
}

// addCards fills each turn's played and removed cards and its player's board, from the moves.
func (g *Game) addCards(ms []Moment) {
	owners := g.owners()
	boards := make(map[string][]string)
	for _, mv := range g.Moves {
		m := &ms[g.moveTurn(mv)]
		owner := owners[mv.Owner]
		mine := owner == m.Player
		leaves := mv.From == ZoneCharacter && mv.To != ZoneCharacter
		if leaves {
			if i := slices.Index(boards[owner], mv.Card); i >= 0 {
				boards[owner] = slices.Delete(slices.Clone(boards[owner]), i, i+1)
			}
		}
		if mv.To == ZoneCharacter && mv.From != ZoneCharacter {
			boards[owner] = append(slices.Clone(boards[owner]), mv.Card)
		}
		if !mine {
			if leaves {
				m.Removed = append(m.Removed, mv.Card)
			}
			continue
		}
		m.Board = boards[owner]
		if (mv.To == ZoneCharacter || mv.To == ZoneStage) && mv.From != mv.To {
			m.Played = append(m.Played, mv.Card)
		}
	}
}

func addRemovals(ms []Moment, power func(code string) int) {
	for t := range ms {
		m := &ms[t]
		big := 0
		for _, c := range m.Removed {
			if power(c) >= bigPower {
				big++
			}
		}
		if big >= bigRemovals || len(m.Removed) >= anyRemovals {
			m.Reasons = append(m.Reasons, Reason{Kind: ReasonRemoved, Player: m.Player, N: len(m.Removed)})
		}
	}
}

func (g *Game) addLife(ms []Moment, id string) {
	life := g.LifeByTurn(id)
	// Life can go back up, so 1 and 0 count the first time only.
	atOne, atZero := false, false
	for t := 1; t < len(life); t++ {
		if lost := life[t-1] - life[t]; lost >= lifeLost {
			ms[t].Reasons = append(ms[t].Reasons, Reason{Kind: ReasonLifeLost, Player: id, N: lost})
		}
		switch {
		case life[t] == 1 && life[t-1] > 1 && !atOne:
			atOne = true
			ms[t].Reasons = append(ms[t].Reasons, Reason{Kind: ReasonAtOne, Player: id})
		case life[t] == 0 && life[t-1] > 0 && !atZero:
			atZero = true
			ms[t].Reasons = append(ms[t].Reasons, Reason{Kind: ReasonAtZero, Player: id})
		}
	}
}

func (g *Game) addBoards(ms []Moment, id string) {
	chars := g.CharactersByTurn(id)
	for t := 1; t < len(chars); t++ {
		if ms[t].Player != id {
			continue
		}
		if gain := chars[t] - chars[t-1]; gain >= boardGain {
			ms[t].Reasons = append(ms[t].Reasons, Reason{Kind: ReasonBoard, Player: id, N: gain})
		}
	}
}

// owners maps RZ1 owner numbers to player IDs.
func (g *Game) owners() map[int]string {
	out := make(map[int]string)
	for _, p := range g.Players {
		out[p.Owner] = p.ID()
	}
	return out
}

func (g *Game) moveTurn(mv Move) int {
	if mv.Line < len(g.Lines) {
		return g.Lines[mv.Line].Turn
	}
	return g.lastTurn()
}

// TurnPlayer is the ID of whoever ended turn t, or by turn order when the game ended in it.
func (g *Game) TurnPlayer(t int) string {
	for _, l := range g.Lines {
		if l.Turn == t && l.Text == "End Turn" && l.Player != "" && l.Player != "You" {
			return l.Player
		}
	}
	if g.First == "" || t == 0 {
		return ""
	}
	if t%2 == 1 {
		return g.First
	}
	for _, p := range g.Players {
		if p.ID() != g.First {
			return p.ID()
		}
	}
	return ""
}

type MarkKind int

const (
	// MarkRemoved is a character leaving the board on the other player's turn.
	MarkRemoved MarkKind = iota + 1
	// MarkTrigger is a life card's Trigger activating.
	MarkTrigger
)

// Mark is one removal or Trigger, for marking on a timeline.
type Mark struct {
	Kind MarkKind
	Turn int
	Line int
	// Player is the ID of the card's owner.
	Player string
	Card   string
}

// Marks is every removal and Trigger, in the order the log has them.
func (g *Game) Marks() []Mark {
	owners := g.owners()
	var out []Mark
	for _, mv := range g.Moves {
		if mv.From != ZoneCharacter || mv.To == ZoneCharacter {
			continue
		}
		t := g.moveTurn(mv)
		if owner := owners[mv.Owner]; owner != g.TurnPlayer(t) {
			out = append(out, Mark{Kind: MarkRemoved, Turn: t, Line: mv.Line, Player: owner, Card: mv.Card})
		}
	}
	for i, l := range g.Lines {
		if l.Turn == 0 || l.Player == "" || l.Player == "You" || len(l.Cards) == 0 {
			continue
		}
		if strings.HasSuffix(l.Text, ": Activate Trigger") {
			out = append(out, Mark{Kind: MarkTrigger, Turn: l.Turn, Line: i, Player: l.Player, Card: l.Cards[0]})
		}
	}
	slices.SortStableFunc(out, func(a, b Mark) int { return a.Line - b.Line })
	return out
}
