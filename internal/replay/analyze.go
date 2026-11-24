package replay

import (
	"slices"

	"github.com/VTGare/logpose/internal/simlog"
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

// Analysis holds a match's per-turn curves, key moments, marks, lethal-turn attacks and built
// replay.
type Analysis struct {
	Me, Opp           []int
	MeHand, OppHand   []int
	MeBoard, OppBoard []int
	Moments           []Moment
	Marks             []Mark
	// Finish is the attacks of the turn a game ended in lethal, or nil.
	Finish []TurnAttack
	// Replay is the built replay for RZ1 games, or nil for logs without RZ1 moves.
	Replay *Replay
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

// Analyze builds a match's curves, key moments, timeline marks, lethal-turn attacks and replay
// in one pass. power gives a card's printed power, or 0 when unknown.
func Analyze(g *simlog.Game, me, opp string, power func(code string) int) *Analysis {
	out := &Analysis{}
	if g == nil {
		return out
	}
	var r *Replay
	if g.HasRZ() {
		r, _ = Build(g, power)
		out.Replay = r
	}
	last := g.LastTurn()
	life := make(map[string][]int, len(g.Players))
	hand := make(map[string][]int, len(g.Players))
	board := make(map[string][]int, len(g.Players))
	for _, p := range g.Players {
		id := p.ID()
		life[id], hand[id], board[id] = playerCurves(g, r, p, last)
	}
	out.Me, out.Opp = life[me], life[opp]
	out.MeHand, out.OppHand = hand[me], hand[opp]
	out.MeBoard, out.OppBoard = board[me], board[opp]
	if last == 0 {
		return out
	}

	ms := make([]Moment, last+1)
	for i := len(g.Lines) - 1; i >= 0; i-- {
		ms[g.Lines[i].Turn].Line = i
	}
	for t := range ms {
		ms[t].Turn = t
		if r != nil {
			ms[t].Player = r.TurnPlayer(t)
		} else {
			ms[t].Player = g.TurnPlayer(t)
		}
	}

	other := otherPlayers(g)
	var marks []Mark
	if r != nil {
		addFromReplay(r, ms, &marks, last)
	} else {
		addFromText(g, ms, &marks, other, last)
	}
	addTriggers(g, &marks)
	slices.SortStableFunc(marks, func(a, b Mark) int { return a.Line - b.Line })
	out.Marks = marks

	for t := 1; t < last; t++ {
		if n := len(ms[t].Countered); n >= handDrain && ms[t].Player != "" {
			ms[t].Reasons = append(ms[t].Reasons, Reason{Kind: ReasonHandDrain, Player: other[ms[t].Player], N: n})
		}
	}
	powerOf := power
	if powerOf == nil {
		powerOf = func(string) int { return 0 }
	}
	addRemovals(ms, powerOf)
	for _, p := range g.Players {
		id := p.ID()
		addLife(ms, id, life[id])
		addBoards(ms, id, board[id])
	}
	if g.End.Kind != simlog.EndUnknown {
		ms[last].Reasons = append(ms[last].Reasons, Reason{Kind: ReasonFinish, Player: g.End.Loser})
	}
	for _, m := range ms[1:] {
		if len(m.Reasons) > 0 {
			out.Moments = append(out.Moments, m)
		}
	}
	if r != nil && g.End.Kind == simlog.EndLethal && len(out.Moments) > 0 {
		out.Finish = r.Attacks(out.Moments[len(out.Moments)-1].Turn)
	}
	return out
}

func playerCurves(g *simlog.Game, r *Replay, p simlog.Player, last int) (life, hand, board []int) {
	if len(g.Lines) == 0 {
		return nil, nil, nil
	}
	if p.Owner != 0 && len(g.Checks) > 0 {
		life = countByTurnChecks(g, p.Owner, last, func(c simlog.Check) int { return c.Life }, true)
		hand = countByTurnChecks(g, p.Owner, last, func(c simlog.Check) int { return c.Hand }, false)
		board = countByTurnChecks(g, p.Owner, last, func(c simlog.Check) int { return c.Character }, false)
		if life != nil || hand != nil || board != nil {
			return life, hand, board
		}
	}
	if r != nil && p.Owner != 0 {
		return curvesFromReplay(r, p.Owner, last)
	}
	id := p.ID()
	return countByTurnPrintouts(g, id, last, "Life"),
		countByTurnPrintouts(g, id, last, "Hand"),
		countByTurnPrintouts(g, id, last, "Board")
}

// countByTurnChecks takes the player's last check in each turn. With dealt set, checks before
// life cards are dealt read 0 and are skipped.
func countByTurnChecks(g *simlog.Game, owner, last int, count func(simlog.Check) int, dealt bool) []int {
	known := make([]bool, last+1)
	out := make([]int, last+1)
	started := !dealt
	for _, c := range g.Checks {
		if c.Owner != owner {
			continue
		}
		n := count(c)
		if started = started || n > 0; !started {
			continue
		}
		t := g.TurnOf(c.Line)
		out[t], known[t] = n, true
	}
	return fillTurns(out, known)
}

func curvesFromReplay(r *Replay, owner, last int) (life, hand, board []int) {
	life, hand, board = make([]int, last+1), make([]int, last+1), make([]int, last+1)
	known := make([]bool, last+1)
	for _, s := range r.Steps {
		if s.Turn < 0 || s.Turn > last {
			continue
		}
		side := s.Board.Side(owner)
		if side == nil {
			continue
		}
		life[s.Turn] = len(side.Life)
		hand[s.Turn] = len(side.Hand)
		board[s.Turn] = len(side.Characters)
		known[s.Turn] = true
	}
	return fillTurns(life, known), fillTurns(hand, known), fillTurns(board, known)
}

func countByTurnPrintouts(g *simlog.Game, id string, last int, zone string) []int {
	known := make([]bool, last+1)
	out := make([]int, last+1)
	for _, l := range g.Lines {
		if l.Player != id || l.Printout != zone || l.Turn > last {
			continue
		}
		n := len(l.Shown)
		if zone == "Life" {
			n = l.N
		}
		// The sim prints both players' opening zones at the start of turn 1 before Turn 1's
		// own actions, so the first turn-1 printout seeds turn 0 when turn 0 had none.
		if l.Turn == 1 && !known[0] && !known[1] {
			out[0], known[0] = n, true
		}
		out[l.Turn], known[l.Turn] = n, true
	}
	return fillTurns(out, known)
}

func fillTurns(out []int, known []bool) []int {
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

func addFromReplay(r *Replay, ms []Moment, marks *[]Mark, last int) {
	lastStep := make([]int, last+1)
	for i := range lastStep {
		lastStep[i] = -1
	}
	turn := -1
	spent := make(map[int]int)
	for si, s := range r.Steps {
		if s.Turn <= 0 || s.Turn > last {
			continue
		}
		lastStep[s.Turn] = si
		if s.Turn != turn {
			turn, spent = s.Turn, make(map[int]int)
		}
		applyReplayStep(&ms[s.Turn], s, spent, marks)
	}
	for t := 1; t <= last; t++ {
		if si := lastStep[t]; si >= 0 {
			fillReplayTurnEnd(&ms[t], r.Steps[si], t)
		}
	}
}

func applyReplayStep(m *Moment, s Step, spent map[int]int, marks *[]Mark) {
	turnOwner := ownerOf(s.Board, m.Player)
	recordReplayDefense(m, s, turnOwner, spent)
	for _, ev := range s.boardEvents {
		recordBoardEvent(m, s, ev, turnOwner, spent, marks)
	}
	if s.Kind == KindDamage && s.Battle != nil && s.Battle.DefenderID != 0 {
		for _, o := range s.Outcome {
			if o.Kind == OutcomeKO {
				recordDefended(m, spent, s.Battle.DefenderID, s.Battle.Defender())
				break
			}
		}
	}
}

func recordReplayDefense(m *Moment, s Step, turnOwner int, spent map[int]int) {
	if m.Player == "" || s.Player == m.Player || len(s.Cards) == 0 {
		return
	}
	switch s.Kind {
	case KindCounter:
		m.Countered = append(m.Countered, s.Cards[0])
		if s.Battle != nil && s.Battle.DefenderID != 0 && turnOwner != 0 {
			if def := s.Board.Side(3 - turnOwner); def != nil && s.Battle.DefenderID != def.Leader.ID {
				spent[s.Battle.DefenderID]++
			}
		}
	case KindBlock:
		m.Blocked = append(m.Blocked, s.Cards[0])
	}
}

func recordBoardEvent(m *Moment, s Step, ev boardEvent, turnOwner int, spent map[int]int, marks *[]Mark) {
	if turnOwner != 0 && ev.owner == turnOwner {
		switch ev.kind {
		case boardEnterChar:
			m.Played = append(m.Played, ev.card)
			m.Characters = append(m.Characters, ev.card)
		case boardEnterStage:
			m.Played = append(m.Played, ev.card)
		}
		return
	}
	evPlayer := ""
	if side := s.Board.Side(ev.owner); side != nil {
		evPlayer = side.Player
	}
	switch ev.kind {
	case boardLeaveChar:
		m.Removed = append(m.Removed, ev.card)
		*marks = append(*marks, Mark{Kind: MarkRemoved, Turn: s.Turn, Line: ev.line, Player: evPlayer, Card: ev.card})
		if ev.ko && s.Kind != KindDamage && ev.id != 0 {
			recordDefended(m, spent, ev.id, ev.card)
		}
	case boardEnterChar:
		if slices.Contains(m.Removed, ev.card) && !slices.Contains(m.CameBack, ev.card) {
			m.CameBack = append(m.CameBack, ev.card)
		}
	}
}

func recordDefended(m *Moment, spent map[int]int, id int, card string) {
	if n := spent[id]; n >= defendedCards {
		m.Defended = append(m.Defended, Defense{Card: card, Cards: n})
		delete(spent, id)
	}
}

func fillReplayTurnEnd(m *Moment, s Step, turn int) {
	turnOwner := ownerOf(s.Board, m.Player)
	if turnOwner == 0 {
		return
	}
	if own := s.Board.Side(turnOwner); own != nil {
		for _, c := range own.Characters {
			if c.Played() < turn {
				m.Board = append(m.Board, c.Code)
			}
		}
		for _, c := range own.Characters {
			if c.Played() == turn {
				m.Board = append(m.Board, c.Code)
				m.New++
			}
		}
		m.Gone = gone(m.Played, m.Board[len(m.Board)-m.New:])
	}
	if def := s.Board.Side(3 - turnOwner); def != nil {
		for _, c := range def.Characters {
			for _, st := range c.Statuses {
				if (st.Kind == StatusNoRest || st.Kind == StatusNoAttack) && st.EndsTurn == turn+1 && st.Source != "" {
					addLock(m, st.Source, c.Code, st.Kind == StatusNoRest)
				}
			}
		}
	}
}

func ownerOf(b Board, id string) int {
	if id == "" {
		return 0
	}
	for o := 1; o <= 2; o++ {
		if b.Sides[o-1].Player == id {
			return o
		}
	}
	return 0
}

type textTracker struct {
	leaders   map[string]string
	other     map[string]string
	boards    map[string][]string
	endBoards map[string][][]string
	target    string
	spent     map[string]int
}

// addFromText derives turn moments and removal marks from readable lines alone for logs
// without RZ1 moves.
func addFromText(g *simlog.Game, ms []Moment, marks *[]Mark, other map[string]string, last int) {
	tr := textTracker{
		leaders:   make(map[string]string, len(g.Players)),
		other:     other,
		boards:    make(map[string][]string),
		endBoards: make(map[string][][]string, len(g.Players)),
		spent:     make(map[string]int),
	}
	for _, p := range g.Players {
		tr.leaders[p.ID()] = p.Leader
		tr.endBoards[p.ID()] = make([][]string, last+1)
	}
	turn := -1
	for i, l := range g.Lines {
		tr.recordPrintout(l, last)
		if l.Turn != turn {
			turn, tr.target, tr.spent = l.Turn, "", make(map[string]int)
		}
		if l.Turn <= 0 || l.Turn > last || ms[l.Turn].Player == "" {
			continue
		}
		tr.applyLine(&ms[l.Turn], i, l, marks)
	}
	fillTextTurnEnds(ms, tr.endBoards, last)
}

func (tr *textTracker) recordPrintout(l simlog.Line, last int) {
	if l.Printout != "Board" || l.Player == "" {
		return
	}
	eb := tr.endBoards[l.Player]
	if l.Turn == 1 && eb != nil && eb[0] == nil && eb[1] == nil {
		eb[0] = l.Shown
	}
	tr.boards[l.Player] = l.Shown
	if l.Turn >= 0 && l.Turn <= last && eb != nil {
		eb[l.Turn] = l.Shown
	}
}

func (tr *textTracker) applyLine(m *Moment, lineIdx int, l simlog.Line, marks *[]Mark) {
	def := tr.other[m.Player]
	switch l.Player {
	case m.Player:
		tr.applyTurnLine(m, l, def)
	case def:
		tr.applyDefenderLine(m, lineIdx, l, def, marks)
	}
}

func (tr *textTracker) applyTurnLine(m *Moment, l simlog.Line, def string) {
	switch {
	case l.Kind == simlog.LinePlay && len(l.Cards) > 0:
		m.Played = append(m.Played, l.Cards[0])
		m.Characters = append(m.Characters, l.Cards[0])
	case l.Kind == simlog.LineAttack && len(l.Cards) >= 2:
		tr.target = l.Cards[len(l.Cards)-1]
		if tr.target == tr.leaders[def] {
			tr.target = ""
		}
	case l.Kind == simlog.LineEffect && l.Source != "":
		if e, ok := parseEffect(l); ok && (e.kind == StatusNoRest || e.kind == StatusNoAttack) {
			if slices.Contains(tr.boards[def], e.target) {
				addLock(m, l.Source, e.target, e.kind == StatusNoRest)
			}
		}
	}
}

func (tr *textTracker) applyDefenderLine(m *Moment, lineIdx int, l simlog.Line, def string, marks *[]Mark) {
	switch {
	case l.Kind == simlog.LineCounterDiscard && len(l.Cards) == 1:
		m.Countered = append(m.Countered, l.Cards[0])
		tr.spent[tr.target]++
	case l.Kind == simlog.LineCounter:
		m.Countered = append(m.Countered, l.Source)
		tr.spent[tr.target]++
	case l.Kind == simlog.LineBlock && len(l.Cards) == 1:
		m.Blocked = append(m.Blocked, l.Cards[0])
		tr.target = ""
	case l.Kind == simlog.LineKO && len(l.Cards) == 1:
		c := l.Cards[0]
		m.Removed = append(m.Removed, c)
		*marks = append(*marks, Mark{Kind: MarkRemoved, Turn: l.Turn, Line: lineIdx, Player: def, Card: c})
		if tr.spent[c] >= defendedCards {
			m.Defended = append(m.Defended, Defense{Card: c, Cards: tr.spent[c]})
			delete(tr.spent, c)
		}
	}
}

func fillTextTurnEnds(ms []Moment, endBoards map[string][][]string, last int) {
	for t := 1; t <= last; t++ {
		m := &ms[t]
		if m.Player == "" {
			continue
		}
		eb := endBoards[m.Player]
		if eb == nil || eb[t] == nil {
			continue
		}
		var prev []string
		for k := t - 1; k >= 0; k-- {
			if eb[k] != nil {
				prev = eb[k]
				break
			}
		}
		old, added := splitNew(prev, eb[t])
		m.Board = append(old, added...)
		m.New = len(added)
		m.Gone = gone(m.Played, added)
	}
}

func splitNew(prev, cur []string) (old, added []string) {
	kept := make(map[string]int, len(prev))
	for _, code := range prev {
		kept[code]++
	}
	for _, code := range cur {
		if kept[code] > 0 {
			kept[code]--
			old = append(old, code)
		} else {
			added = append(added, code)
		}
	}
	return old, added
}

func addLock(m *Moment, source, card string, rested bool) {
	for i := range m.Locks {
		if k := &m.Locks[i]; k.Source == source && k.Rested == rested {
			k.Cards = append(k.Cards, card)
			return
		}
	}
	m.Locks = append(m.Locks, Lock{Source: source, Cards: []string{card}, Rested: rested})
}

func addTriggers(g *simlog.Game, marks *[]Mark) {
	for i, l := range g.Lines {
		if l.Turn > 0 && l.Player != "" && l.Kind == simlog.LineTrigger {
			*marks = append(*marks, Mark{Kind: MarkTrigger, Turn: l.Turn, Line: i, Player: l.Player, Card: l.Source})
		}
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

func addLife(ms []Moment, id string, life []int) {
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

func addBoards(ms []Moment, id string, chars []int) {
	for t := 1; t < len(chars); t++ {
		if ms[t].Player != id {
			continue
		}
		if gain := chars[t] - chars[t-1]; gain >= boardGain {
			ms[t].Reasons = append(ms[t].Reasons, Reason{Kind: ReasonBoard, Player: id, N: gain})
		}
	}
}

func otherPlayers(g *simlog.Game) map[string]string {
	out := make(map[string]string, len(g.Players))
	for _, p := range g.Players {
		for _, q := range g.Players {
			if p.ID() != q.ID() {
				out[p.ID()] = q.ID()
			}
		}
	}
	return out
}
