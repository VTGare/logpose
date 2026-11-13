package replay

import (
	"fmt"
	"slices"

	"github.com/VTGare/logpose/internal/simlog"
)

// Card is one physical card in a zone.
type Card struct {
	// ID tells a physical card apart from other copies of it as it moves between zones. It
	// is unique within a game.
	ID     int
	Code   string
	Rested bool
	// Don is how many DON!! cards are attached to it.
	Don int
	// Statuses are the lasting effects on a card on the board, oldest first. A card that
	// leaves the board loses them.
	Statuses []Status
	// played is the turn the card came onto the board, and attacked the turn it last attacked,
	// or 0 once it was set active since.
	played, attacked int
}

type Side struct {
	// Player is the player's ID, or "" when the log doesn't link the owner to a player.
	Player     string
	Owner      int
	Leader     Card
	Characters []Card
	Stage      []Card
	Hand       []Card
	// Life is the life cards with the top one last.
	Life    []Card
	Trash   []Card
	DonDeck int
	// Don is the cost area, one entry per DON!! card, true when it is rested.
	Don []bool
	// deck is the deck with its top card last. Cards the log hasn't named yet have no code.
	deck []Card
}

func (s *Side) Deck() []Card { return s.deck }

func (s *Side) DonActive() int {
	n := 0
	for _, rested := range s.Don {
		if !rested {
			n++
		}
	}
	return n
}

func (s *Side) DonRested() int { return len(s.Don) - s.DonActive() }

func (s *Side) DonAttached() int {
	n := s.Leader.Don
	for _, c := range s.Characters {
		n += c.Don
	}
	return n
}

func codes(cards []Card) []string {
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		out = append(out, c.Code)
	}
	return out
}

// Board is both sides, indexed by RZ1 owner minus one.
type Board struct {
	Sides [2]Side
}

func (b *Board) Side(owner int) *Side {
	if owner < 1 || owner > 2 {
		return nil
	}
	return &b.Sides[owner-1]
}

// clone copies the board. Statuses slices are shared, since they are only ever replaced.
func (b *Board) clone() Board {
	out := *b
	for i := range out.Sides {
		s := &out.Sides[i]
		s.Characters = slices.Clone(s.Characters)
		s.Stage = slices.Clone(s.Stage)
		s.Hand = slices.Clone(s.Hand)
		s.Life = slices.Clone(s.Life)
		s.Trash = slices.Clone(s.Trash)
		s.Don = slices.Clone(s.Don)
		s.deck = slices.Clone(s.deck)
	}
	return out
}

// counts is a side's zone sizes in the order of simlog.Zone.
func (s *Side) counts() [10]int {
	return [10]int{
		len(s.deck), len(s.Hand), len(s.Characters), len(s.Life), s.DonDeck, len(s.Don),
		len(s.Trash), len(s.Stage), 1, s.DonAttached(),
	}
}

func checkCounts(c simlog.Check) [10]int {
	return [10]int{
		c.Deck, c.Hand, c.Character, c.Life, c.DonDeck, c.DonField, c.Trash, c.Stage, c.Leader,
		c.DonAttached,
	}
}

// idBase spaces the owners' card IDs apart. A side has far fewer cards than this.
const idBase = 1000

// setup fills a side from its first check, undoing the move that came before it, so the deck
// and DON deck start at their real sizes. Every deck card gets its ID here.
func (s *Side) setup(c simlog.Check, m simlog.Move) {
	n := checkCounts(c)
	if m.From != m.To {
		n[m.From]++
		n[m.To]--
	}
	s.deck = make([]Card, max(n[simlog.ZoneDeck], 0))
	for i := range s.deck {
		s.deck[i].ID = s.Owner*idBase + i + 1
	}
	s.DonDeck = max(n[simlog.ZoneDonDeck], 0)
}

// donSlot splits the slot of a DON!! card given to a card. The sim writes the card's place
// times 100 plus the DON's place on it, with 99 for the leader.
func donSlot(slot int) (target, index int) { return slot / 100, slot % 100 }

const leaderTarget = 99

// apply moves one card. It trusts the move's card over what the board has at the slot, and
// returns what didn't match.
func (s *Side) apply(m simlog.Move) []string {
	if c := s.inPlace(m); c != nil {
		c.Rested = m.Rested
		return nil
	}
	var issues []string
	note := func(format string, args ...any) { issues = append(issues, fmt.Sprintf(format, args...)) }
	card := s.take(m, note)
	card.Code = m.Card
	s.put(m, card, note)
	return issues
}

// inPlace is the card a move leaves where it is, or nil. The sim logs a card on the board
// resting or standing up as a move to its own slot, and the card keeps its DON and statuses.
func (s *Side) inPlace(m simlog.Move) *Card {
	if m.From != m.To || m.FromSlot != m.ToSlot {
		return nil
	}
	var zone []Card
	switch m.From {
	case simlog.ZoneLeader:
		return &s.Leader
	case simlog.ZoneCharacter:
		zone = s.Characters
	case simlog.ZoneStage:
		zone = s.Stage
	}
	if m.FromSlot < 0 || m.FromSlot >= len(zone) || zone[m.FromSlot].Code != m.Card {
		return nil
	}
	return &zone[m.FromSlot]
}

// take removes the moving card from its zone and returns it, without its rest state, DON and
// statuses, which don't go with it.
func (s *Side) take(m simlog.Move, note func(string, ...any)) Card {
	var card Card
	switch m.From {
	case simlog.ZoneDeck:
		s.deck, card = takeCard(s.deck, m.FromSlot, m.Card, true, note, "deck")
	case simlog.ZoneHand:
		s.Hand, card = takeCard(s.Hand, m.FromSlot, m.Card, false, note, "hand")
	case simlog.ZoneLife:
		s.Life, card = takeCard(s.Life, m.FromSlot, m.Card, true, note, "life")
	case simlog.ZoneTrash:
		s.Trash, card = takeCard(s.Trash, m.FromSlot, m.Card, false, note, "trash")
	case simlog.ZoneCharacter:
		s.Characters, card = takeCard(s.Characters, m.FromSlot, m.Card, false, note, "character")
		if card.Don > 0 {
			note("character %s left with %d DON attached", m.Card, card.Don)
		}
	case simlog.ZoneStage:
		s.Stage, card = takeCard(s.Stage, m.FromSlot, m.Card, false, note, "stage")
	default:
		s.takeDon(m, note)
	}
	return Card{ID: card.ID}
}

func (s *Side) takeDon(m simlog.Move, note func(string, ...any)) {
	switch m.From {
	case simlog.ZoneDonDeck:
		if s.DonDeck == 0 {
			note("DON deck empty")
		}
		s.DonDeck--
	case simlog.ZoneDonField:
		i := m.FromSlot
		if i < 0 || i >= len(s.Don) {
			note("DON slot %d of %d", i, len(s.Don))
			i = len(s.Don) - 1
		}
		if i >= 0 {
			s.Don = slices.Delete(s.Don, i, i+1)
		}
	case simlog.ZoneDonAttached:
		if t := s.donTarget(m.FromSlot); t == nil || t.Don == 0 {
			note("no DON attached at %d", m.FromSlot)
		} else {
			t.Don--
		}
	case simlog.ZoneLeader:
		note("leader moved")
	}
}

func (s *Side) put(m simlog.Move, card Card, note func(string, ...any)) {
	card.Rested = m.Rested
	switch m.To {
	case simlog.ZoneDeck:
		s.deck = insert(s.deck, m.ToSlot, Card{ID: card.ID, Code: card.Code})
	case simlog.ZoneHand:
		s.Hand = insert(s.Hand, m.ToSlot, Card{ID: card.ID, Code: card.Code})
	case simlog.ZoneLife:
		s.Life = insert(s.Life, m.ToSlot, Card{ID: card.ID, Code: card.Code})
	case simlog.ZoneTrash:
		s.Trash = insert(s.Trash, m.ToSlot, Card{ID: card.ID, Code: card.Code})
	case simlog.ZoneCharacter:
		s.Characters = insert(s.Characters, m.ToSlot, card)
	case simlog.ZoneStage:
		s.Stage = insert(s.Stage, m.ToSlot, card)
	case simlog.ZoneDonDeck:
		s.DonDeck++
	case simlog.ZoneDonField:
		s.Don = insert(s.Don, m.ToSlot, m.Rested)
	case simlog.ZoneDonAttached:
		if t := s.donTarget(m.ToSlot); t == nil {
			note("no card to attach DON to at %d", m.ToSlot)
		} else {
			t.Don++
		}
	case simlog.ZoneLeader:
		note("card moved to leader")
	}
}

func (s *Side) donTarget(slot int) *Card {
	t, _ := donSlot(slot)
	if t == leaderTarget {
		return &s.Leader
	}
	if t < 0 || t >= len(s.Characters) {
		return nil
	}
	return &s.Characters[t]
}

// Hidden zones like the deck hold cards without a code until the log names them, which match
// any card.
func takeCard(list []Card, slot int, code string, hidden bool, note func(string, ...any), zone string) ([]Card, Card) {
	i := slot
	if i < 0 || i >= len(list) {
		note("%s slot %d of %d", zone, slot, len(list))
		i = slices.IndexFunc(list, func(c Card) bool { return c.Code == code })
		if i < 0 && hidden {
			i = slices.IndexFunc(list, func(c Card) bool { return c.Code == "" })
		}
		if i < 0 {
			return list, Card{Code: code}
		}
	}
	c := list[i]
	if c.Code != code && (c.Code != "" || !hidden) {
		note("%s slot %d holds %s, not %s", zone, slot, c.Code, code)
	}
	return slices.Delete(list, i, i+1), c
}

func insert[T any](list []T, slot int, v T) []T {
	slot = min(max(slot, 0), len(list))
	return slices.Insert(list, slot, v)
}

// refresh sets the side's cards active at the start of its owner's turn, except cards an
// effect keeps rested, which lose that effect instead.
func (s *Side) refresh() {
	wake := func(c *Card) {
		if c.has(StatusFrozen) {
			c.drop(func(st Status) bool { return st.Kind == StatusFrozen })
			return
		}
		c.Rested = false
	}
	wake(&s.Leader)
	for i := range s.Characters {
		wake(&s.Characters[i])
	}
	for i := range s.Stage {
		wake(&s.Stage[i])
	}
	for i := range s.Don {
		s.Don[i] = false
	}
}

func (s *Side) all(code string) []*Card {
	var out []*Card
	if s.Leader.Code == code {
		out = append(out, &s.Leader)
	}
	for _, list := range [][]Card{s.Characters, s.Stage} {
		for i := range list {
			if list[i].Code == code {
				out = append(out, &list[i])
			}
		}
	}
	return out
}

// find returns the side's card with the code, preferring one in the rest state.
func (s *Side) find(code string, rested bool) *Card {
	all := s.all(code)
	for _, c := range all {
		if c.Rested == rested {
			return c
		}
	}
	if len(all) > 0 {
		return all[0]
	}
	return nil
}

func (s *Side) byID(id int) *Card {
	if s.Leader.ID == id {
		return &s.Leader
	}
	for _, list := range [][]Card{s.Characters, s.Stage} {
		for i := range list {
			if list[i].ID == id {
				return &list[i]
			}
		}
	}
	return nil
}

func (s *Side) cards() []*Card {
	out := []*Card{&s.Leader}
	for _, list := range [][]Card{s.Characters, s.Stage} {
		for i := range list {
			out = append(out, &list[i])
		}
	}
	return out
}
