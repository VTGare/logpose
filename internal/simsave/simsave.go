// Package simsave writes a replayed game as an OPTCGSim save state, the JSON the sim's
// "LoadState From File" reads.
package simsave

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/VTGare/logpose/internal/replay"
)

// Info gives a card's printed power and a leader's life, or zeros when they aren't known.
type Info func(code string) (power, life int)

// Lists gives the most copies of each card a player ran with a leader in other games, or nil.
type Lists func(player, leader string) map[string]int

// maxCopies is the most copies of one card a deck may hold.
const maxCopies = 4

type Result struct {
	Turns int
	// Guessed is how many deck cards the log never named that were given a code. The opening
	// shuffle only logs the cards that move, so one that stays put has no code.
	Guessed int
	// Left is how many unnamed cards no code fit without a fifth copy. They are left out of
	// the deck.
	Left int
}

var ErrNoTurns = errors.New("simsave: the game has no turns")

// turn is one entry of a save, the board as a turn starts, before its refresh and draw.
type turn struct {
	TurnNumber int    `json:"turnNumber"`
	Player1    player `json:"player1"`
	Player2    player `json:"player2"`
	// IsPlayer1TurnStarting is true on odd turns. Player 1 is the player who went first.
	IsPlayer1TurnStarting bool `json:"isPlayer1TurnStarting"`
}

type player struct {
	Hand       []cardRef   `json:"hand"`
	Trash      []cardRef   `json:"trash"`
	Deck       []cardRef   `json:"deck"`
	Board      []character `json:"board"`
	Life       []cardRef   `json:"life"`
	Stage      []stage     `json:"stage"`
	LeaderCard character   `json:"leaderCard"`
	ActiveDon  int         `json:"activeDon"`
	RestedDon  int         `json:"restedDon"`
	GivenDon   int         `json:"givenDon"`
}

type cardRef struct {
	CardID       string `json:"cardId"`
	UniqueDeckID int    `json:"uniqueDeckId"`
	IsFaceUp     bool   `json:"isFaceUp"`
}

type character struct {
	AttachedDon int       `json:"attachedDon"`
	Card        cardState `json:"card"`
	cardRef
}

type stage struct {
	cardRef
	IsRested bool `json:"isRested"`
}

type exporter struct {
	info Info
	// startLife is each owner's life cards when turn 1 starts, the leader's life when info
	// doesn't know it.
	startLife [3]int
	codes     map[int]string
	// ids are the sim's IDs by replay card ID. The sim numbers a deck 10, 20, ... grouped by
	// code with the leader at 1, and the second player's are negative.
	ids map[int]int
}

// Export writes the board at the start of every turn the game reached, even one a player
// conceded or decked out at the start of.
func Export(r *replay.Replay, info Info, lists Lists) ([]byte, Result, error) {
	var rep Result
	if len(r.Steps) == 0 {
		return nil, rep, ErrNoTurns
	}
	first := firstOwner(r)
	e := &exporter{info: info, codes: make(map[int]string), ids: make(map[int]int)}
	rep.Guessed, rep.Left = e.name(r, first, lists)
	if i := r.FirstOfTurn(1); i >= 1 {
		for _, sd := range r.Steps[i-1].Board.Sides {
			e.startLife[sd.Owner] = len(sd.Life)
		}
	}
	var turns []turn
	for t := 1; t <= r.Steps[len(r.Steps)-1].Turn; t++ {
		i := r.FirstOfTurn(t)
		if i < 1 {
			continue
		}
		b := r.Steps[i-1].Board
		turns = append(turns, turn{
			TurnNumber:            t,
			Player1:               e.player(b.Side(first), t),
			Player2:               e.player(b.Side(3-first), t),
			IsPlayer1TurnStarting: t%2 == 1,
		})
	}
	if len(turns) == 0 {
		return nil, rep, ErrNoTurns
	}
	rep.Turns = len(turns)
	data, err := json.Marshal(turns)
	return data, rep, err
}

// firstOwner is the side whose turn 1 is, or 1 when the log doesn't link it to a player.
func firstOwner(r *replay.Replay) int {
	i := r.FirstOfTurn(1)
	if i < 0 {
		return 1
	}
	s := r.Steps[i]
	for _, sd := range s.Board.Sides {
		if sd.Player != "" && sd.Player == s.Player {
			return sd.Owner
		}
	}
	return 1
}

// name finds every card's code from the steps where the log names it, guesses the rest and
// gives each card its sim ID. It returns how many it guessed and how many it left out.
func (e *exporter) name(r *replay.Replay, first int, lists Lists) (guessed, left int) {
	leaders := make(map[int]int)
	owners := make(map[int]int)
	var known [3]map[string]int
	for _, s := range r.Steps {
		for _, sd := range s.Board.Sides {
			leaders[sd.Owner] = sd.Leader.ID
			if known[sd.Owner] == nil && lists != nil && sd.Player != "" && sd.Leader.Code != "" {
				known[sd.Owner] = lists(sd.Player, sd.Leader.Code)
			}
			for _, zone := range [][]replay.Card{sd.Deck(), sd.Hand, sd.Life, sd.Trash, sd.Characters, sd.Stage} {
				for _, c := range zone {
					owners[c.ID] = sd.Owner
					if c.Code != "" || e.codes[c.ID] == "" {
						e.codes[c.ID] = c.Code
					}
				}
			}
		}
	}
	for owner := 1; owner <= 2; owner++ {
		var ids []int
		for id, o := range owners {
			if o == owner && id != leaders[owner] {
				ids = append(ids, id)
			}
		}
		slices.Sort(ids)
		g, l := e.guess(ids, known[owner])
		guessed, left = guessed+g, left+l
		ids = slices.DeleteFunc(ids, func(id int) bool { return e.codes[id] == "" })
		slices.SortStableFunc(ids, func(a, b int) int { return strings.Compare(e.codes[a], e.codes[b]) })
		sign := 1
		if owner != first {
			sign = -1
		}
		for i, id := range ids {
			e.ids[id] = sign * (i + 1) * 10
		}
		e.ids[leaders[owner]] = sign
	}
	return guessed, left
}

// guess names the deck's unnamed cards without giving any code a fifth copy. A code the
// player's other lists with this leader ran more copies of comes first, the one missing the
// most. Then a code with three copies, the likeliest to be missing its fourth. A card no code
// fits keeps no code and is left out.
func (e *exporter) guess(ids []int, known map[string]int) (guessed, left int) {
	count := make(map[string]int)
	var blank []int
	for _, id := range ids {
		if c := e.codes[id]; c == "" {
			blank = append(blank, id)
		} else {
			count[c]++
		}
	}
	for _, id := range blank {
		best, missing := "", 0
		for c, n := range known {
			m := min(n, maxCopies) - count[c]
			if m > missing || m == missing && m > 0 && c < best {
				best, missing = c, m
			}
		}
		if best == "" {
			for c, n := range count {
				if n < maxCopies && (best == "" || n > count[best] || n == count[best] && c < best) {
					best = c
				}
			}
		}
		if best == "" {
			left++
			continue
		}
		e.codes[id] = best
		count[best]++
		guessed++
	}
	return guessed, left
}

func (e *exporter) ref(c replay.Card) cardRef {
	code := c.Code
	if code == "" {
		code = e.codes[c.ID]
	}
	return cardRef{CardID: code, UniqueDeckID: e.ids[c.ID]}
}

// refs leaves out the cards no code fit.
func (e *exporter) refs(cs []replay.Card) []cardRef {
	out := make([]cardRef, 0, len(cs))
	for _, c := range cs {
		if r := e.ref(c); r.CardID != "" {
			out = append(out, r)
		}
	}
	return out
}

func (e *exporter) state(c replay.Card, owner, t, order int, leader bool) cardState {
	s := newCardState()
	power, life := e.info(c.Code)
	s.DeckUniqueID, s.CardPower, s.BTapped, s.BFaceUp, s.IHandUIOrder = e.ids[c.ID], power, c.Rested, true, order
	if leader {
		s.CardLife = life
		if life == 0 {
			s.CardLife = e.startLife[owner]
		}
	} else if c.Played() == t-1 {
		s.BSummonSick, s.BFirstTurn = true, true
	}
	for _, st := range c.Statuses {
		switch st.Kind {
		case replay.StatusNoRest:
			s.BCantRest = true
		case replay.StatusNoAttack:
			s.BCantAttack = true
		}
	}
	return s
}

func (e *exporter) player(s *replay.Side, t int) player {
	p := player{
		Hand: e.refs(s.Hand), Trash: e.refs(s.Trash), Deck: e.refs(s.Deck()), Life: e.refs(s.Life),
		Board: make([]character, 0, len(s.Characters)), Stage: make([]stage, 0, len(s.Stage)),
		LeaderCard: character{AttachedDon: s.Leader.Don, Card: e.state(s.Leader, s.Owner, t, 0, true), cardRef: e.ref(s.Leader)},
		ActiveDon:  s.DonActive(), RestedDon: s.DonRested(), GivenDon: s.DonAttached(),
	}
	for i, c := range s.Characters {
		p.Board = append(p.Board, character{AttachedDon: c.Don, Card: e.state(c, s.Owner, t, i+1, false), cardRef: e.ref(c)})
	}
	for _, c := range s.Stage {
		p.Stage = append(p.Stage, stage{cardRef: e.ref(c), IsRested: c.Rested})
	}
	return p
}
