package replay

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/VTGare/logpose/internal/simlog"
)

// event is one move or one readable line, in the log's order.
type event struct {
	move, line int
}

// draft is a step before the board is played through it. start is its first event.
type draft struct {
	start   int
	kind    Kind
	turn    int
	player  string
	starter int
	cards   []string
	battle  *Battle
}

type builder struct {
	g       *simlog.Game
	events  []event
	owner   map[string]int
	player  map[int]string
	checks  map[[2]int]simlog.Check
	drafts  []draft
	steps   []Step
	issues  []Issue
	initial Board
	board   Board
	started [3]bool
	// guesses are the "Rest" and "Active" effects on a card both sides had, and flips the ones
	// a later attack showed were the other side's.
	guesses []guess
	flips   map[int]int
	// refreshed is how many guesses there were at each owner's last refresh. Earlier ones
	// can't explain the owner's cards being rested or active.
	refreshed [3]int
	blamed    map[int]bool
	// final is set on the last pass, which reports contradictions instead of blaming guesses.
	final bool
	// power is a card's printed power, or 0 when unknown.
	power func(code string) int
	// attackerID and defenderID are the cards of the attack in progress: the target, or the
	// blocker after a block.
	attackerID, defenderID int
	powerChecks            []PowerCheck
	powerCompared          int
	// left is the attack's defender as it was when it left the board. The sim moves a K.O.'d
	// card to the trash before it writes the "vs" line.
	left Card
}

const replays = 5

// guess is a line naming a card that was on the board more than once in the state the line
// acts on, such as two copies on one side or one on each in a mirror match. The log doesn't
// say which copy it was.
type guess struct {
	kind   guessKind
	code   string
	owners []int
	chosen int
	// n is the power a power effect gives, and status the status a status effect gives.
	n      int
	status StatusKind
}

type guessKind int

const (
	// guessRest is a card rested or set active, which attacks on it correct.
	guessRest guessKind = iota
	// guessPower is a power effect, which "vs" lines correct.
	guessPower
	// guessStatus is another status, which attacks by the card correct.
	guessStatus
)

func newBuilder(g *simlog.Game, power func(string) int) *builder {
	b := &builder{
		g: g, owner: make(map[string]int), player: make(map[int]string),
		checks: make(map[[2]int]simlog.Check), flips: make(map[int]int), power: power,
	}
	for _, p := range g.Players {
		if p.Owner == 1 || p.Owner == 2 {
			b.owner[p.ID()] = p.Owner
			b.player[p.Owner] = p.ID()
			s := b.initial.Side(p.Owner)
			s.Player, s.Leader.Code, s.Leader.ID = p.ID(), p.Leader, p.Owner*idBase+idBase-1
		}
	}
	for o := 1; o <= 2; o++ {
		b.initial.Side(o).Owner = o
	}
	for _, c := range g.Checks {
		b.checks[[2]int{c.Seq, c.Owner}] = c
	}
	// A rematch's stream starts with the end of the game before it, such as DON coming back,
	// and the sim then resets the board without logging moves for it. This game's own moves
	// start at its first shuffle or draw.
	mi := 0
	for mi < len(g.Moves) && g.Moves[mi].From != simlog.ZoneDeck {
		mi++
	}
	for li := range g.Lines {
		for ; mi < len(g.Moves) && g.Moves[mi].Line <= li; mi++ {
			b.events = append(b.events, event{move: mi, line: -1})
		}
		b.events = append(b.events, event{move: -1, line: li})
	}
	for ; mi < len(g.Moves); mi++ {
		b.events = append(b.events, event{move: mi, line: -1})
	}
	return b
}

// run plays the game through again while attacks show that a guessed card was the other
// side's.
func (b *builder) run() {
	b.cut()
	for pass := range replays {
		b.blamed = make(map[int]bool)
		b.final = pass == replays-1
		b.play()
		if len(b.blamed) == 0 {
			break
		}
		for k := range b.blamed {
			b.flips[k]++
		}
	}
	b.anchorPowers()
	for i := range b.steps {
		b.steps[i].Title, b.steps[i].Result = b.describe(&b.steps[i])
	}
	if b.g.End.Kind == simlog.EndLethal {
		for i := len(b.steps) - 1; i >= 0; i-- {
			if s := &b.steps[i]; s.Battle != nil {
				s.Result = strings.TrimPrefix(s.Result+" · Lethal", " · ")
				break
			}
		}
	}
}

var (
	hitText     = regexp.MustCompile(`^(.*) \[[A-Z0-9-]+\] hit for (\d+) damage$`)
	vsText      = regexp.MustCompile(`\[[A-Z0-9-]+\]\[(\d+)\] vs .*\[[A-Z0-9-]+\]\[(\d+)\]$`)
	counterText = regexp.MustCompile(`^Discard .* for Counter (\d+)$`)
	// counterEffect is an event's counter that the sim writes without "Activate Counter".
	counterEffect = regexp.MustCompile(`\]: Counter (\d+)$`)
	attachText    = regexp.MustCompile(`^Attach (\d+) (?:Rested )?Don to `)
	codeTag       = regexp.MustCompile(` ?\[[A-Za-z][A-Za-z0-9-]*\]`)
)

// cut splits the events into steps. The sim writes a readable line before its moves for the
// opponent's actions, and often after them for the local player's, so a step that starts on a
// line reaches back for its card's move and the DON rested to pay for it.
func (b *builder) cut() {
	g := b.g
	setup := b.setupEnd()
	st := cutState{involved: make(map[string]bool)}
	add := func(d draft) {
		n := len(b.drafts)
		if n == 1 && b.drafts[0].kind == KindLobby {
			d.start = 0
		}
		if n > 0 && b.drafts[n-1].start >= d.start {
			b.drafts[n-1] = d
		} else {
			b.drafts = append(b.drafts, d)
		}
		clear(st.involved)
		for _, c := range d.cards {
			st.involved[c] = true
		}
	}
	add(draft{kind: KindLobby, starter: -1})
	for p, e := range b.events {
		if p == setup {
			add(draft{start: p, kind: KindTurnStart, turn: 1, player: g.TurnPlayer(1), starter: -1})
		}
		var d draft
		var ok bool
		if e.move >= 0 {
			m := g.Moves[e.move]
			if m.To == simlog.ZoneCharacter || m.To == simlog.ZoneStage {
				st.involved[m.Card] = true
			}
			if p < setup {
				d, ok = b.setupStep(p, m, &st)
			}
		} else if l := g.Lines[e.line]; p >= setup && l.Turn > 0 {
			d, ok = b.starts(p, e.line, l, &st)
		}
		if ok {
			add(d)
		}
		if e.line >= 0 {
			for _, c := range g.Lines[e.line].Cards {
				st.involved[c] = true
			}
		}
	}
}

// cutState is what cutting the game into steps remembers between events.
type cutState struct {
	opened, mulled, dealt [3]bool
	// battle is set from an attack's declaration to its damage, and attack is that attack.
	battle bool
	attack Battle
	// source is the card whose effects belong to the current step of the attack.
	source string
	// involved is the cards the current step has named or played. Their effects belong to it.
	involved map[string]bool
}

// setupStep returns the step a move before the first turn begins: each player's shuffle and
// opening hand, mulligan and life.
func (b *builder) setupStep(p int, m simlog.Move, st *cutState) (draft, bool) {
	o := m.Owner
	if o < 1 || o > 2 {
		return draft{}, false
	}
	d := draft{start: p, player: b.player[o], starter: -1}
	switch {
	case !st.opened[o]:
		st.opened[o] = true
		d.kind = KindOpeningHand
	case m.From == simlog.ZoneHand && m.To == simlog.ZoneDeck && !st.mulled[o]:
		st.mulled[o] = true
		d.kind = KindMulligan
	case m.To == simlog.ZoneLife && !st.dealt[o]:
		st.dealt[o] = true
		d.kind = KindLife
	default:
		return d, false
	}
	return d, true
}

// starts returns the step the line at p begins: a new turn, the game's end, or an action of
// the turn's player.
func (b *builder) starts(p, li int, l simlog.Line, st *cutState) (draft, bool) {
	g := b.g
	cur := b.drafts[len(b.drafts)-1]
	d := draft{start: p, turn: l.Turn, player: l.Player, starter: li}
	switch {
	case l.Text == "End Turn":
		st.battle = false
		next := l.Turn + 1
		d.player, d.turn, d.kind = g.TurnPlayer(next), next, KindTurnStart
		d.start = b.backRefresh(p, cur.start, b.owner[d.player])
	case l.Text == "Concedes!" && l.Player != "":
		d.kind = KindConcede
	case l.Text == "Is Out of Cards, Loses the Game!":
		d.kind = KindDeckOut
	case st.battle && b.battleStep(&d, l, cur.start, st):
	case l.Player == "" || l.Player == "You" || l.Player != g.TurnPlayer(l.Turn):
		if l.Text == "Attack Fails" || hitText.MatchString(l.Text) {
			st.battle = false
		}
		return d, false
	default:
		return b.action(d, l, cur.start, st)
	}
	return d, true
}

// action returns the step a line of the turn's player begins, when it begins one.
func (b *builder) action(d draft, l simlog.Line, floor int, st *cutState) (draft, bool) {
	owner := b.owner[l.Player]
	source, effect := effectSource(l)
	switch {
	case strings.HasPrefix(l.Text, "Deploy ") && len(l.Cards) > 0:
		code := l.Cards[0]
		d.kind, d.cards = KindPlay, []string{code}
		d.start = b.back(d.start, floor, owner, func(m simlog.Move) bool {
			return m.Card == code && m.From == simlog.ZoneHand &&
				(m.To == simlog.ZoneCharacter || m.To == simlog.ZoneStage)
		})
	case attachText.MatchString(l.Text) && len(l.Cards) > 0:
		d.kind, d.cards = KindAttach, l.Cards[len(l.Cards)-1:]
		d.start = b.back(d.start, floor, owner, func(m simlog.Move) bool {
			return m.From == simlog.ZoneDonField && m.To == simlog.ZoneDonAttached
		})
	case strings.Contains(l.Text, " attacking ") && len(l.Cards) >= 2:
		st.battle = true
		st.attack = Battle{Attacker: l.Cards[0], Target: l.Cards[1], AttackerOwner: owner}
		st.source = l.Cards[0]
		a := st.attack
		d.kind, d.cards, d.battle = KindAttack, l.Cards[:2], &a
		d.start = b.back(d.start, floor, owner, nil)
	case strings.HasSuffix(l.Text, " Destroyed"):
		st.battle = false
		return d, false
	case effect && !st.battle && !st.involved[source]:
		d.kind, d.cards = KindEffect, []string{source}
		d.start = b.back(d.start, floor, owner, func(m simlog.Move) bool {
			return m.Card == source && m.From == simlog.ZoneHand && m.To == simlog.ZoneTrash
		})
	default:
		return d, false
	}
	return d, true
}

// battleStep starts a step of the attack in progress: a block, each counter, and the damage,
// which the "X [power] vs Y [power]" line opens. It reports whether the line starts one.
func (b *builder) battleStep(d *draft, l simlog.Line, floor int, st *cutState) bool {
	owner := b.owner[l.Player]
	source, effect := effectSource(l)
	switch {
	case l.Player == "" && vsText.MatchString(l.Text):
		m := vsText.FindStringSubmatch(l.Text)
		st.battle = false
		st.attack.AttackerPower, _ = strconv.Atoi(m[1])
		st.attack.TargetPower, _ = strconv.Atoi(m[2])
		d.kind, d.player = KindDamage, b.player[st.attack.AttackerOwner]
		d.cards = []string{st.attack.Attacker, st.attack.Defender()}
		d.start = b.backDamage(d.start, floor, 3-st.attack.AttackerOwner)
	case strings.HasSuffix(l.Text, " Blocks") && len(l.Cards) == 1:
		st.attack.Blocker = l.Cards[0]
		d.kind, d.cards = KindBlock, l.Cards
	case counterText.MatchString(l.Text) && len(l.Cards) == 1:
		d.kind, d.cards = KindCounter, l.Cards
		d.start = b.backCounter(d.start, floor, owner, l.Cards[0])
	case effect && (strings.HasSuffix(l.Text, ": Activate Counter") || counterEffect.MatchString(l.Text)):
		d.kind, d.cards = KindCounter, []string{source}
		d.start = b.backCounter(d.start, floor, owner, source)
	case effect && source != st.source:
		named := l.Cards[1:]
		d.kind, d.cards = KindBattleEffect, []string{source}
		d.start = b.back(d.start, b.lastLine(d.start, floor), owner, func(m simlog.Move) bool {
			from := m.From == simlog.ZoneHand || m.From == simlog.ZoneCharacter
			return from && m.To == simlog.ZoneTrash && slices.Contains(named, m.Card)
		})
	default:
		return false
	}
	st.source = d.cards[0]
	a := st.attack
	d.battle = &a
	return true
}

// effectSource is the card a "Card [CODE]: effect" line comes from.
func effectSource(l simlog.Line) (string, bool) {
	if len(l.Cards) == 0 {
		return "", false
	}
	head, _, ok := strings.Cut(l.Text, ": ")
	if !ok || !strings.HasSuffix(head, "["+l.Cards[0]+"]") {
		return "", false
	}
	return l.Cards[0], true
}

// setupEnd is the first event of turn 1: its first DON, or its first line.
func (b *builder) setupEnd() int {
	for p, e := range b.events {
		if e.move >= 0 {
			m := b.g.Moves[e.move]
			if m.From == simlog.ZoneDonDeck && m.To == simlog.ZoneDonField {
				return p
			}
			continue
		}
		if b.g.Lines[e.line].Turn >= 1 {
			return p
		}
	}
	return len(b.events)
}

// back finds where a step that starts on the line at p begins. When key matches a move of the
// owner since the step before began, the step begins at that move. It then takes in the DON the
// owner rested just before, which paid for it.
func (b *builder) back(p, floor, owner int, key func(simlog.Move) bool) int {
	q := p
	if key != nil {
		for i := floor + 1; i < p; i++ {
			e := b.events[i]
			if e.move >= 0 && b.g.Moves[e.move].Owner == owner && key(b.g.Moves[e.move]) {
				q = i
				break
			}
		}
	}
	for q-1 > floor {
		e := b.events[q-1]
		if e.move >= 0 {
			m := b.g.Moves[e.move]
			if m.Owner != owner || m.From != simlog.ZoneDonField || m.To != simlog.ZoneDonField || !m.Rested {
				break
			}
		} else if !quiet(b.g.Lines[e.line]) {
			break
		}
		q--
	}
	return q
}

// backCounter finds where a counter step that starts on the line at p begins: at the card's move
// to the trash since the step before began. Two copies of a card can counter one after the
// other, and the opponent's line comes before its move. So when the step before is the same
// card's and doesn't begin on its move, the first such move is that step's.
func (b *builder) backCounter(p, floor, owner int, code string) int {
	discard := func(m simlog.Move) bool {
		return m.Owner == owner && m.Card == code && m.From == simlog.ZoneHand && m.To == simlog.ZoneTrash
	}
	isDiscard := func(i int) bool { e := b.events[i]; return e.move >= 0 && discard(b.g.Moves[e.move]) }
	cur := b.drafts[len(b.drafts)-1]
	if cur.kind == KindCounter && cur.cards[0] == code && !isDiscard(cur.start) {
		for i := floor + 1; i < p; i++ {
			if isDiscard(i) {
				floor = i
				break
			}
		}
	}
	// Steps are runs of events, so this one can't begin before the line of the one before.
	for i := floor + 1; i < p && cur.starter >= 0; i++ {
		if b.events[i].line == cur.starter {
			floor = i
			break
		}
	}
	return b.back(p, floor, owner, discard)
}

// lastLine is the last event before p that is a readable line other than a quiet one, or floor.
func (b *builder) lastLine(p, floor int) int {
	for q := p - 1; q > floor; q-- {
		if e := b.events[q]; e.line >= 0 && !quiet(b.g.Lines[e.line]) {
			return q
		}
	}
	return floor
}

// backDamage takes the defender's life cards and characters that left just before the "vs"
// line into the damage step. The sim moves them before it writes the line.
func (b *builder) backDamage(p, floor, owner int) int {
	q := p
	for q-1 > floor {
		e := b.events[q-1]
		if e.move >= 0 {
			m := b.g.Moves[e.move]
			from := m.From
			if m.Owner != owner || (from != simlog.ZoneLife && from != simlog.ZoneCharacter && from != simlog.ZoneDonAttached) {
				break
			}
		} else if !quiet(b.g.Lines[e.line]) {
			break
		}
		q--
	}
	return q
}

// backRefresh takes the DON returned to the next player before the "End Turn" line into the
// next turn, since they come back in its refresh.
func (b *builder) backRefresh(p, floor, owner int) int {
	q := p
	for q-1 > floor {
		e := b.events[q-1]
		if e.move >= 0 {
			m := b.g.Moves[e.move]
			if m.Owner != owner || m.From != simlog.ZoneDonAttached {
				break
			}
		} else if !snapshot(b.g.Lines[e.line]) {
			break
		}
		q--
	}
	return q
}

// quiet lines don't start or end anything: draws, which come just before their move, and the
// sim's board printouts.
func quiet(l simlog.Line) bool {
	return strings.HasPrefix(l.Text, "Drew card from deck: ") || snapshot(l)
}

func snapshot(l simlog.Line) bool {
	for _, p := range []string{"Hand: ", "Board: ", "Trash: ", "Life: "} {
		if strings.HasPrefix(l.Text, p) {
			return true
		}
	}
	return false
}

// play walks the events through the board, checking it as it goes, and fills each step.
func (b *builder) play() {
	b.board = b.initial.clone()
	b.issues, b.guesses, b.started, b.refreshed = nil, nil, [3]bool{}, [3]int{}
	b.powerChecks, b.powerCompared, b.attackerID, b.defenderID = nil, 0, 0, 0
	b.steps = make([]Step, len(b.drafts))
	for i, d := range b.drafts {
		b.steps[i] = Step{
			Turn: d.turn, Kind: d.kind, Phase: phaseOf(d.kind), Player: d.player, Cards: d.cards,
			starter: d.starter,
		}
		if d.battle != nil {
			bt := *d.battle
			b.steps[i].Battle = &bt
		}
	}
	si := 0
	refresh := 0
	for p, e := range b.events {
		for si+1 < len(b.drafts) && b.drafts[si+1].start <= p {
			b.finishStep(si)
			si++
			switch b.drafts[si].kind {
			case KindTurnStart:
				refresh = b.owner[b.drafts[si].player]
			case KindBlock, KindCounter, KindDamage, KindBattleEffect:
			default:
				b.endCombat()
			}
		}
		st := &b.steps[si]
		if e.move >= 0 {
			m := b.g.Moves[e.move]
			if refresh != 0 && (m.Owner != refresh || m.From != simlog.ZoneDonAttached) {
				b.refresh(refresh)
				refresh = 0
			}
			st.Moves = append(st.Moves, e.move)
			b.move(si, m)
			continue
		}
		l := b.g.Lines[e.line]
		if refresh != 0 && l.Text != "End Turn" && !snapshot(l) {
			b.refresh(refresh)
			refresh = 0
		}
		st.Lines = append(st.Lines, e.line)
		b.line(si, e.line, l)
		if l.Player == "" {
			b.vsLine(si, l)
		}
	}
	if refresh != 0 {
		b.refresh(refresh)
	}
	b.finishStep(si)
}

func (b *builder) refresh(owner int) {
	b.board.Side(owner).refresh()
	b.refreshed[owner] = len(b.guesses)
}

func (b *builder) finishStep(i int) {
	if b.steps[i].Kind != KindDamage {
		b.stepPowers(i)
	}
	if bt := b.steps[i].Battle; bt != nil {
		bt.AttackerID, bt.DefenderID = b.attackerID, b.defenderID
	}
	b.steps[i].Board = b.board.clone()
}

func phaseOf(k Kind) Phase {
	switch k {
	case KindLobby, KindOpeningHand, KindMulligan, KindLife:
		return PhaseSetup
	case KindTurnStart:
		return PhaseStart
	case KindAttack, KindBlock, KindCounter, KindDamage, KindBattleEffect:
		return PhaseBattle
	case KindConcede, KindDeckOut:
		return PhaseEnd
	default:
		return PhaseMain
	}
}

func (b *builder) move(step int, m simlog.Move) {
	s := b.board.Side(m.Owner)
	if s == nil {
		b.issues = append(b.issues, Issue{Kind: IssueMove, Step: step, Seq: m.Seq, Line: -1, Owner: m.Owner, Detail: "unknown owner"})
		return
	}
	c, hasCheck := b.checks[[2]int{m.Seq, m.Owner}]
	if !b.started[m.Owner] && hasCheck {
		s.setup(c, m)
	}
	b.started[m.Owner] = true
	if m.From == simlog.ZoneCharacter && m.FromSlot >= 0 && m.FromSlot < len(s.Characters) &&
		s.Characters[m.FromSlot].ID == b.defenderID {
		b.left = s.Characters[m.FromSlot]
	}
	for _, d := range s.apply(m) {
		b.issues = append(b.issues, Issue{Kind: IssueMove, Step: step, Seq: m.Seq, Line: -1, Owner: m.Owner, Detail: d})
	}
	if m.To == simlog.ZoneCharacter && len(s.Characters) > 0 {
		s.Characters[min(max(m.ToSlot, 0), len(s.Characters)-1)].played = b.steps[step].Turn
	}
	if !hasCheck {
		return
	}
	for _, d := range countIssues(s, c) {
		b.issues = append(b.issues, Issue{Kind: IssueCount, Step: step, Seq: m.Seq, Line: -1, Owner: m.Owner, Detail: d})
	}
}

// line applies what a readable line says about cards resting and standing up, which the RZ1
// stream doesn't record, and checks the sim's board printouts.
func (b *builder) line(step, li int, l simlog.Line) {
	owner := b.owner[l.Player]
	mine := b.board.Side(owner)
	if mine == nil {
		return
	}
	theirs := b.board.Side(3 - owner)
	issue := func(kind IssueKind, d string) {
		b.issues = append(b.issues, Issue{Kind: kind, Step: step, Line: li, Owner: owner, Detail: d})
	}
	if snapshot(l) {
		if d, ok := snapshotIssue(mine, l.Text); ok && d != "" {
			issue(IssueSnapshot, d)
		}
		return
	}
	_, effect := effectSource(l)
	switch {
	case l.Text == "End Turn":
		b.endTurn(l.Turn)
	case strings.Contains(l.Text, " attacking ") && len(l.Cards) >= 2 && !effect:
		b.attack(l, l.Turn, mine, theirs, issue)
	case strings.HasSuffix(l.Text, " Blocks") && len(l.Cards) == 1:
		if c := b.pick(l.Cards[0], false, mine); c != nil {
			c.Rested = true
			b.defenderID = c.ID
		}
	case effect && len(l.Cards) >= 2:
		b.effect(l, mine, theirs)
	}
	b.statusLine(step, l)
}

// vsLine checks the powers of a "vs" line against the worked-out ones, and notes both on the
// damage step it opens.
func (b *builder) vsLine(step int, l simlog.Line) {
	st := &b.steps[step]
	m := vsText.FindStringSubmatch(l.Text)
	if st.Kind != KindDamage || st.Battle == nil || m == nil {
		return
	}
	var want [2]int
	want[0], _ = strconv.Atoi(m[1])
	want[1], _ = strconv.Atoi(m[2])
	b.checkVs(step, st.Battle, want)
	b.stepPowers(step)
}

// attack rests the attacker. An attacker that was already rested or can't attack, or a
// character attacked while active, means the board went wrong earlier, unless a guess explains
// it.
func (b *builder) attack(l simlog.Line, turn int, mine, theirs *Side, issue func(IssueKind, string)) {
	if c := b.chooseAttacker(l.Cards[0], turn, mine); c != nil {
		switch {
		case c.Rested && !b.blame(c.Code, mine.Owner, true):
			issue(IssueRest, "attacker "+c.Code+" is already rested")
		case c.has(StatusNoAttack) && !b.blameStatus(c.Code, mine.Owner, StatusNoAttack):
			issue(IssueNoAttack, "attacker "+c.Code+" can't attack")
		}
		if c.attacked == turn {
			issue(IssueTwice, "attacker "+c.Code+" attacked twice this turn")
		}
		c.Rested, c.attacked = true, turn
		b.attackerID = c.ID
	}
	c := theirs.find(l.Cards[1], true)
	if c != nil {
		b.defenderID = c.ID
	}
	if c != nil && c != &theirs.Leader && !c.Rested && !b.blame(c.Code, theirs.Owner, false) {
		issue(IssueRest, "target "+c.Code+" is active")
	}
}

// chooseAttacker finds the copy that attacked. It has to be active and free to attack. A copy
// played this turn comes last unless it has Rush, since the log doesn't name keyword Rush.
// With no such copy it takes an active one, which the caller reports.
func (b *builder) chooseAttacker(code string, turn int, s *Side) *Card {
	c := b.choose(code, guess{}, func(c *Card) bool { return !c.Rested && !c.has(StatusNoAttack) },
		func(c *Card) bool { return c.played == turn && !c.has(StatusRush) }, s)
	if c != nil && (c.Rested || c.has(StatusNoAttack)) {
		if a := s.find(code, false); a != nil {
			return a
		}
	}
	return c
}

// effect applies a "Card: Rest X" or "Card: Set X to Active" line. Rests mostly hit the
// other side, and the other the own side. A copy that can't be rested is never the one rested.
func (b *builder) effect(l simlog.Line, mine, theirs *Side) {
	target := l.Cards[len(l.Cards)-1]
	_, rest, _ := strings.Cut(l.Text, ": ")
	switch {
	case strings.HasPrefix(rest, "Rest "):
		if c := b.choose(target, guess{}, func(c *Card) bool { return !c.Rested && !c.has(StatusNoRest) }, nil, theirs, mine); c != nil {
			c.Rested = true
		}
	case strings.HasPrefix(rest, "Set ") && strings.HasSuffix(rest, " to Active"):
		if c := b.pick(target, true, mine, theirs); c != nil {
			c.Rested, c.attacked = false, 0
		}
	}
}

// pick finds the card a line names, in the rest state the line acts on, looking at the sides
// in order.
func (b *builder) pick(code string, rested bool, sides ...*Side) *Card {
	return b.choose(code, guess{}, func(c *Card) bool { return c.Rested == rested }, nil, sides...)
}

// choose finds the card a line names among the copies the line can act on, which fits tells,
// and notes g as a guess when there are several. A copy that doesn't fit is never chosen while
// one does. Copies that fit are taken in the order of the sides, those that later reports
// last, and the first unless a replay found it was another. With no copy that fits it returns
// the first copy, or nil.
func (b *builder) choose(code string, g guess, fits, later func(*Card) bool, sides ...*Side) *Card {
	var cands, last []*Card
	var owners, lastOwners []int
	var any *Card
	for _, s := range sides {
		for _, c := range s.all(code) {
			if any == nil {
				any = c
			}
			switch {
			case fits != nil && !fits(c):
			case later != nil && later(c):
				last = append(last, c)
				lastOwners = append(lastOwners, s.Owner)
			default:
				cands = append(cands, c)
				owners = append(owners, s.Owner)
			}
		}
	}
	cands, owners = append(cands, last...), append(owners, lastOwners...)
	switch len(cands) {
	case 0:
		return any
	case 1:
		return cands[0]
	}
	k := len(b.guesses)
	i := b.flips[k] % len(cands)
	g.code, g.owners, g.chosen = code, owners, owners[i]
	b.guesses = append(b.guesses, g)
	return cands[i]
}

// blame finds the latest guess that explains an owner's card being rested when it was chosen,
// or active when it may not have been, and marks it to change. It reports whether it found one.
func (b *builder) blame(code string, owner int, chosen bool) bool {
	if b.final {
		return false
	}
	for k := len(b.guesses) - 1; k >= b.refreshed[owner]; k-- {
		g := b.guesses[k]
		if g.kind != guessRest || g.code != code || b.blamed[k] {
			continue
		}
		if chosen && g.chosen == owner || !chosen && slices.Contains(g.owners, owner) {
			b.blamed[k] = true
			return true
		}
	}
	return false
}

// blameStatus finds the latest guess that gave a status to the owner's copy of the card, and
// marks it to change. It reports whether it found one.
func (b *builder) blameStatus(code string, owner int, kind StatusKind) bool {
	if b.final {
		return false
	}
	for k := len(b.guesses) - 1; k >= 0; k-- {
		g := b.guesses[k]
		if g.kind != guessStatus || g.status != kind || g.code != code || b.blamed[k] || g.chosen != owner {
			continue
		}
		b.blamed[k] = true
		return true
	}
	return false
}

// describe writes a step's title and, for attacks, how the attack ended.
func (b *builder) describe(s *Step) (title, result string) {
	g := b.g
	name := func(code string) string {
		if n, ok := g.CardNames[code]; ok {
			return n
		}
		return code
	}
	if t, ok := fixedTitles[s.Kind]; ok {
		return t, ""
	}
	switch s.Kind {
	case KindTurnStart:
		return b.turnStartTitle(s, name), ""
	case KindPlay:
		return "Played " + name(s.Cards[0]), ""
	case KindAttach:
		n := "1"
		if m := attachText.FindStringSubmatch(g.Lines[s.starter].Text); m != nil {
			n = m[1]
		}
		return "Gave " + name(s.Cards[0]) + " " + n + " DON", ""
	case KindAttack:
		return name(s.Cards[0]) + " attacked " + name(s.Cards[1]), ""
	case KindBlock:
		return "Blocked with " + name(s.Cards[0]), ""
	case KindCounter:
		return b.counterTitle(s, name), ""
	case KindDamage:
		bt := s.Battle
		return fmt.Sprintf("%s %d vs %s %d", name(bt.Attacker), bt.AttackerPower, name(bt.Defender()), bt.TargetPower),
			b.attackResult(s, name)
	case KindEffect:
		return b.effectTitle(s, name), ""
	case KindBattleEffect:
		return b.battleEffectTitle(s, name), ""
	}
	return "", ""
}

var fixedTitles = map[Kind]string{
	KindLobby:       "Lobby",
	KindOpeningHand: "Drew an opening hand",
	KindMulligan:    "Mulliganed",
	KindLife:        "Life cards placed",
	KindConcede:     "Conceded",
	KindDeckOut:     "Ran out of cards",
}

func (b *builder) turnStartTitle(s *Step, name func(string) string) string {
	var drew []string
	don := 0
	owner := b.owner[s.Player]
	for _, mi := range s.Moves {
		m := b.g.Moves[mi]
		if m.Owner != owner {
			continue
		}
		switch {
		case m.From == simlog.ZoneDeck && m.To == simlog.ZoneHand:
			drew = append(drew, name(m.Card))
		case m.From == simlog.ZoneDonDeck && m.To == simlog.ZoneDonField:
			don++
		}
		if don > 0 && m.From != simlog.ZoneDonDeck {
			break
		}
	}
	var parts []string
	if len(drew) > 0 {
		parts = append(parts, "Drew "+strings.Join(drew, ", "))
	}
	if don > 0 {
		parts = append(parts, "+"+strconv.Itoa(don)+" DON")
	}
	if len(parts) == 0 {
		return "Turn " + strconv.Itoa(s.Turn)
	}
	return strings.Join(parts, " · ")
}

func (b *builder) attackResult(s *Step, name func(string) string) string {
	var parts []string
	for _, li := range s.Lines {
		l := b.g.Lines[li]
		switch {
		case l.Text == "Attack Fails":
			parts = append(parts, "Attack failed")
		case strings.HasSuffix(l.Text, " Destroyed") && len(l.Cards) == 1:
			parts = append(parts, name(l.Cards[0])+" K.O.'d")
		default:
			if m := hitText.FindStringSubmatch(l.Text); m != nil {
				parts = append(parts, "Hit for "+m[2])
			}
		}
	}
	return strings.Join(parts, " · ")
}

func (b *builder) counterTitle(s *Step, name func(string) string) string {
	for _, li := range s.Lines {
		if m := counterText.FindStringSubmatch(b.g.Lines[li].Text); m != nil {
			return "Countered with " + name(s.Cards[0]) + " +" + m[1]
		}
	}
	n := 0
	for _, l := range b.sourceLines(s) {
		_, rest, _ := strings.Cut(l.Text, ": ")
		if e, ok := parseEffect(rest); ok && e.kind == StatusPower {
			n += e.n
		}
	}
	if n > 0 {
		return "Countered with " + name(s.Cards[0]) + " +" + strconv.Itoa(n)
	}
	return "Countered with " + name(s.Cards[0])
}

// LineText is a readable line without card codes, as "Kyo: Draw 1 Card".
func LineText(l simlog.Line) string {
	return codeTag.ReplaceAllString(l.Text, "")
}
