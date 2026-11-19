package simlog

import (
	"slices"
	"strings"
)

type ReasonKind int

const (
	// ReasonRemoved is Player removing the other player's characters, as many as Moment.Removed.
	ReasonRemoved ReasonKind = iota + 1
	ReasonLifeLost
	ReasonAtOne
	ReasonAtZero
	// ReasonBoard is Player ending their turn with N more characters than they started with,
	// on the board in Moment.Board.
	ReasonBoard
	// ReasonFinish is the turn the game ended in. Player is the loser.
	ReasonFinish
	// ReasonHandDrain is Player spending N cards on counters in the other player's turn.
	ReasonHandDrain
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
	// Played is the cards the turn's player put on the board, characters and stages, in order,
	// and Characters the characters among them.
	Played     []string
	Characters []string
	// Removed is the other player's characters that left the board.
	Removed []string
	// Board is the turn's player's characters at its end, in the order they came onto the
	// board. Its last New cards came onto it in the turn.
	Board []string
	New   int
	// Gone is the cards in Played that weren't on the board at the turn's end.
	Gone []string
	// Countered is the cards the other player countered with, counter events among them, and
	// Blocked the cards they blocked with, in order.
	Countered []string
	Blocked   []string
	// CameBack is the cards in Removed that came back onto the board in the turn.
	CameBack []string
	// Defended is the other player's characters that drew counter cards and were K.O.'d anyway.
	Defended []Defense
	// Locks is what the turn's player's effects stopped the other player's characters doing.
	Locks []Lock
}

// Defense is a character K.O.'d after its owner spent Cards counter cards on it.
type Defense struct {
	Card  string
	Cards int
}

// Lock is Source's effect on the other player's Cards. Rested means they can't be rested, so
// they can't block in the turn or attack in the next. Otherwise they can't attack next turn.
type Lock struct {
	Source string
	Cards  []string
	Rested bool
}

// Removing characters is a moment at bigRemovals of bigPower or more, or at anyRemovals of
// any power.
const (
	bigPower    = 5000
	bigRemovals = 2
	anyRemovals = 3
	lifeLost    = 2
	boardGain   = 3
	// handDrain counter cards in one turn point at the loser about as often as losing 2 life
	// does, over the games this was checked on. 2 doesn't, and spending them over two turns adds
	// nothing.
	handDrain = 3
	// defendedCards counter cards on a character make losing it anyway worth a line.
	defendedCards = 2
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
	g.addBattles(ms)
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

type onBoard struct {
	card string
	turn int
}

func (g *Game) addCards(ms []Moment) {
	owners := g.owners()
	boards := make(map[string][]onBoard)
	for _, mv := range g.Moves {
		m := &ms[g.TurnOf(mv.Line)]
		owner := owners[mv.Owner]
		mine := owner == m.Player
		leaves := mv.From == ZoneCharacter && mv.To != ZoneCharacter
		enters := mv.To == ZoneCharacter && mv.From != ZoneCharacter
		if leaves {
			if i := slices.IndexFunc(boards[owner], func(b onBoard) bool { return b.card == mv.Card }); i >= 0 {
				boards[owner] = slices.Delete(boards[owner], i, i+1)
			}
		}
		if enters {
			boards[owner] = append(boards[owner], onBoard{mv.Card, m.Turn})
		}
		if !mine {
			if leaves {
				m.Removed = append(m.Removed, mv.Card)
			}
			if enters && slices.Contains(m.Removed, mv.Card) && !slices.Contains(m.CameBack, mv.Card) {
				m.CameBack = append(m.CameBack, mv.Card)
			}
			continue
		}
		m.Board, m.New = m.Board[:0], 0
		for _, b := range boards[owner] {
			m.Board = append(m.Board, b.card)
			if b.turn == m.Turn {
				m.New++
			}
		}
		if enters || (mv.To == ZoneStage && mv.From != ZoneStage) {
			m.Played = append(m.Played, mv.Card)
		}
		if enters {
			m.Characters = append(m.Characters, mv.Card)
		}
	}
	for t := range ms {
		ms[t].Gone = gone(ms[t].Played, ms[t].Board[len(ms[t].Board)-ms[t].New:])
	}
}

func gone(played, stayed []string) []string {
	left := make(map[string]int)
	for _, code := range stayed {
		left[code]++
	}
	var out []string
	for _, code := range played {
		if left[code] > 0 {
			left[code]--
		} else {
			out = append(out, code)
		}
	}
	return out
}

func addRemovals(ms []Moment, power func(code string) int) {
	for t := range ms {
		m := &ms[t]
		big, n := 0, 0
		for _, c := range m.Removed {
			if slices.Contains(m.CameBack, c) {
				continue
			}
			n++
			if power(c) >= bigPower {
				big++
			}
		}
		if big >= bigRemovals || n >= anyRemovals {
			m.Reasons = append(m.Reasons, Reason{Kind: ReasonRemoved, Player: m.Player, N: len(m.Removed)})
		}
	}
}

// addBattles reads each turn's attacks from the lines: the other player's counters and blocks,
// the characters they defended and lost, and the turn player's locks.
func (g *Game) addBattles(ms []Moment) {
	br := battleReader{other: make(map[string]string), leaders: make(map[string]string), boards: make(map[string][]string), turn: -1}
	for _, p := range g.Players {
		br.leaders[p.ID()] = p.Leader
		for _, q := range g.Players {
			if p.ID() != q.ID() {
				br.other[p.ID()] = q.ID()
			}
		}
	}
	for _, l := range g.Lines {
		br.line(&ms[l.Turn], l)
	}
	for t := 1; t < len(ms)-1; t++ {
		if n := len(ms[t].Countered); n >= handDrain && ms[t].Player != "" {
			ms[t].Reasons = append(ms[t].Reasons, Reason{Kind: ReasonHandDrain, Player: br.other[ms[t].Player], N: n})
		}
	}
}

type battleReader struct {
	other, leaders map[string]string
	// boards is each player's last printed board.
	boards map[string][]string
	turn   int
	// target is the character the attack in progress is on, or "" for a leader or a block.
	target string
	// spent counts the counter cards spent on each target this turn.
	spent map[string]int
}

func (br *battleReader) line(m *Moment, l Line) {
	if l.Printout == "Board" && l.Player != "" {
		br.boards[l.Player] = l.Shown
	}
	if l.Turn != br.turn {
		br.turn, br.target, br.spent = l.Turn, "", make(map[string]int)
	}
	if l.Turn == 0 || m.Player == "" {
		return
	}
	def := br.other[m.Player]
	switch l.Player {
	case m.Player:
		br.attacker(m, l, def)
	case def:
		br.defender(m, l)
	}
}

func (br *battleReader) attacker(m *Moment, l Line, def string) {
	switch {
	case l.Kind == LineAttack && len(l.Cards) >= 2:
		br.target = l.Cards[len(l.Cards)-1]
		if br.target == br.leaders[def] {
			br.target = ""
		}
	case l.Kind == LineEffect && len(l.Cards) == 2:
		m.lock(l, br.boards[def])
	}
}

func (br *battleReader) defender(m *Moment, l Line) {
	switch {
	case l.Kind == LineCounterDiscard && len(l.Cards) == 1:
		m.Countered = append(m.Countered, l.Cards[0])
		br.spent[br.target]++
	case l.Kind == LineCounter:
		m.Countered = append(m.Countered, l.Source)
		br.spent[br.target]++
	case l.Kind == LineBlock && len(l.Cards) == 1:
		m.Blocked = append(m.Blocked, l.Cards[0])
		br.target = ""
	case l.Kind == LineKO && len(l.Cards) == 1:
		if c := l.Cards[0]; br.spent[c] >= defendedCards {
			m.Defended = append(m.Defended, Defense{Card: c, Cards: br.spent[c]})
			delete(br.spent, c)
		}
	}
}

// lock adds an effect line like "Card [CODE]: Target [CODE] can't attack next turn" when the
// target is on the other player's last printed board.
func (m *Moment) lock(l Line, board []string) {
	rest := l.EffectText()
	rested := strings.HasSuffix(rest, "can't be rested until next turn ends")
	if !rested && !strings.HasSuffix(rest, "can't attack next turn") {
		return
	}
	card := l.Cards[1]
	if !slices.Contains(board, card) {
		return
	}
	for i := range m.Locks {
		if k := &m.Locks[i]; k.Source == l.Source && k.Rested == rested {
			k.Cards = append(k.Cards, card)
			return
		}
	}
	m.Locks = append(m.Locks, Lock{Source: l.Source, Cards: []string{card}, Rested: rested})
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

func (g *Game) owners() map[int]string {
	out := make(map[int]string)
	for _, p := range g.Players {
		out[p.Owner] = p.ID()
	}
	return out
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
		t := g.TurnOf(mv.Line)
		if owner := owners[mv.Owner]; owner != g.TurnPlayer(t) {
			out = append(out, Mark{Kind: MarkRemoved, Turn: t, Line: mv.Line, Player: owner, Card: mv.Card})
		}
	}
	for i, l := range g.Lines {
		if l.Turn > 0 && l.Player != "" && l.Kind == LineTrigger {
			out = append(out, Mark{Kind: MarkTrigger, Turn: l.Turn, Line: i, Player: l.Player, Card: l.Source})
		}
	}
	slices.SortStableFunc(out, func(a, b Mark) int { return a.Line - b.Line })
	return out
}
