// Package deck reads from a parsed game what each player's deck was and what its cards did.
package deck

import (
	"strings"

	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simlog"
)

// Version goes up whenever Read gives different facts for the same game, so stored games are
// read again. Lists come from the replay, so a new replay.Version is a new Version too. Bump
// readVersion for changes to Read itself.
const Version = readVersion + replay.Version

const readVersion = 1

// Facts is what one player's deck did in one game.
type Facts struct {
	Player string
	// List is the deck's cards by code, as many as the log names. The log names a card only once
	// it moves, so a card that never left its slot can be missing.
	List map[string]int
	// Don is the player's DON!! in play during each game turn, by turn number. Turns before the
	// player had any are 0.
	Don      []int
	Events   []Event
	Searches []Search
}

// Named is how many of the deck's cards the log names.
func (f Facts) Named() int {
	n := 0
	for _, c := range f.List {
		n += c
	}
	return n
}

type EventKind int

const (
	// Drew is a card from the deck to the hand during the game, searches included.
	Drew EventKind = iota + 1
	// FromLife is a life card taken into the hand.
	FromLife
	// PaidPlay is a card the player put on the board from their hand: "Deploy X".
	PaidPlay
	// EffectPlay is a card an effect put on the board from the hand: "Y: Deploy X".
	EffectPlay
	// Counter is a card trashed from the hand for its counter, or a counter event.
	Counter
	// Trigger is a card whose trigger was activated, which then left the hand.
	Trigger
	// OwnLines is a card trashed from the hand next to lines of its own effect. For an Event
	// that is the event being played. For anything else it's a discard. The log doesn't say
	// which, so the card's type decides.
	OwnLines
	// Discard is any other card trashed from the hand, usually by an effect.
	Discard
	eventKindCount
)

var EventKinds = func() []EventKind {
	out := make([]EventKind, 0, eventKindCount-Drew)
	for k := Drew; k < eventKindCount; k++ {
		out = append(out, k)
	}
	return out
}()

var eventNames = map[EventKind]string{
	Drew: "drew", FromLife: "from_life", PaidPlay: "paid_play", EffectPlay: "effect_play",
	Counter: "counter", Trigger: "trigger", OwnLines: "own_lines", Discard: "discard",
}

// String is the kind's stored name, which the card events table's CHECK lists.
func (k EventKind) String() string { return eventNames[k] }

func ParseEventKind(name string) (EventKind, bool) {
	for k, n := range eventNames {
		if n == name {
			return k, true
		}
	}
	return 0, false
}

// Event is a card entering or leaving the hand. Turn is the game turn, 0 for the setup.
type Event struct {
	Card string
	Turn int
	Kind EventKind
}

// Search is a searcher looking at the top of the deck. Taken is "" when the player took
// nothing. Passed are the cards it showed that went to the bottom.
type Search struct {
	Source string
	Taken  string
	Passed []string
	Turn   int
}

// Read returns the facts for every player the log links to an RZ1 owner. Logs without RZ1 have
// none.
func Read(g *simlog.Game) ([]Facts, error) {
	if !g.HasRZ() {
		return nil, nil
	}
	r, err := replay.Build(g, nil)
	if err != nil {
		return nil, err
	}
	var out []Facts
	for _, p := range g.Players {
		if p.Owner < 1 || p.Owner > 2 {
			continue
		}
		out = append(out, Facts{
			Player:   p.ID(),
			List:     list(r, p.Owner),
			Don:      don(g, p.Owner),
			Events:   events(g, r, p.Owner),
			Searches: searches(g, p),
		})
	}
	return out, nil
}

// list names each physical card by the code it shows anywhere in the game.
func list(r *replay.Replay, owner int) map[string]int {
	named := make(map[int]string)
	for _, st := range r.Steps {
		s := &st.Board.Sides[owner-1]
		for _, zone := range [][]replay.Card{s.Deck(), s.Hand, s.Characters, s.Life, s.Trash, s.Stage} {
			for _, c := range zone {
				if c.Code != "" {
					named[c.ID] = c.Code
				}
			}
		}
	}
	cards := make(map[string]int)
	for _, code := range named {
		cards[code]++
	}
	return cards
}

// don reads the DON!! in play from the DON deck count on CHK lines. DON returned by an effect
// counts as gone.
func don(g *simlog.Game, owner int) []int {
	var out []int
	for _, c := range g.Checks {
		if c.Owner != owner {
			continue
		}
		t := g.TurnOf(c.Line)
		for len(out) <= t {
			out = append(out, 0)
		}
		out[t] = max(out[t], 10-c.DonDeck)
	}
	return out
}

func events(g *simlog.Game, r *replay.Replay, owner int) []Event {
	stepOf := make(map[int]*replay.Step)
	for i := range r.Steps {
		for _, mi := range r.Steps[i].Moves {
			stepOf[mi] = &r.Steps[i]
		}
	}
	var out []Event
	for mi, m := range g.Moves {
		if m.Owner != owner || m.Card == "" {
			continue
		}
		t := g.TurnOf(m.Line)
		if t == 0 {
			continue
		}
		var k EventKind
		switch {
		case m.To == simlog.ZoneHand && m.From == simlog.ZoneDeck:
			k = Drew
		case m.To == simlog.ZoneHand && m.From == simlog.ZoneLife:
			k = FromLife
		case m.From == simlog.ZoneHand && (m.To == simlog.ZoneCharacter || m.To == simlog.ZoneStage):
			k = playKind(stepOf[mi], m.Card)
		case m.From == simlog.ZoneHand && m.To == simlog.ZoneTrash:
			k = trashKind(g, m)
		default:
			continue
		}
		out = append(out, Event{Card: m.Card, Turn: t, Kind: k})
	}
	return out
}

// playKind tells "Deploy X" from "Y: Deploy X" by the step the move is in.
func playKind(s *replay.Step, code string) EventKind {
	if s == nil || s.Kind == replay.KindPlay && s.Cards[0] == code {
		return PaidPlay
	}
	return EffectPlay
}

// linesBack is how far before a card's move its own lines can be. An event's move to the trash
// comes after its effect, such as a "Draw 2 Card". Trashed cards are read from the lines and not
// from their replay step, since the replay folds an event played after a search into the
// searcher's step, where it looks like a discard.
const linesBack = 5

// trashKind looks at every line near the move. A counter event's own effect lines follow its
// "Activate Counter", so a counter line wins over any other line of the card's own.
func trashKind(g *simlog.Game, m simlog.Move) EventKind {
	tag := "[" + m.Card + "]"
	kind := Discard
	for i := m.Line - linesBack; i <= m.Line+1; i++ {
		if i < 0 || i >= len(g.Lines) {
			continue
		}
		l := g.Lines[i]
		if !strings.Contains(l.Text, tag) {
			continue
		}
		if l.Kind == simlog.LineCounterDiscard {
			return Counter
		}
		if l.Source != m.Card {
			continue
		}
		switch {
		case l.Kind == simlog.LineCounter:
			return Counter
		case l.Kind == simlog.LineTrigger:
			kind = Trigger
		case kind == Discard:
			kind = OwnLines
		}
	}
	return kind
}

// searchBack is how far after a search line the cards it passed over move to the bottom. A
// "Placing Cards on Bottom of Deck" line sits between them.
const searchBack = 3

// searches reads "X: Reveal and Draw Y" and "Picked no cards to Draw", which names no searcher.
// That one belongs to the card deployed on the line before.
func searches(g *simlog.Game, p simlog.Player) []Search {
	id := p.ID()
	byLine := make(map[int]*Search)
	var order []int
	for i, l := range g.Lines {
		if l.Player != id {
			continue
		}
		switch {
		case l.Source != "" && strings.HasPrefix(l.EffectText(), "Reveal and Draw ") && len(l.Cards) >= 2:
			byLine[i] = &Search{Source: l.Cards[0], Taken: l.Cards[len(l.Cards)-1], Turn: l.Turn}
		case l.Text == "Picked no cards to Draw" && i > 0 && len(g.Lines[i-1].Cards) > 0:
			prev := g.Lines[i-1].Cards
			byLine[i] = &Search{Source: prev[len(prev)-1], Turn: l.Turn}
		default:
			continue
		}
		order = append(order, i)
	}
	for _, m := range g.Moves {
		if m.Owner != p.Owner || m.From != simlog.ZoneDeck || m.To != simlog.ZoneDeck || m.Card == "" {
			continue
		}
		for back := 1; back <= searchBack; back++ {
			if s, ok := byLine[m.Line-back]; ok {
				s.Passed = append(s.Passed, m.Card)
				break
			}
		}
	}
	out := make([]Search, 0, len(order))
	for _, i := range order {
		out = append(out, *byLine[i])
	}
	return out
}
