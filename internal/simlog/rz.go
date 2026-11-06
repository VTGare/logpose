package simlog

import (
	"slices"
	"strconv"
	"strings"
)

type Zone int

const (
	ZoneDeck Zone = iota
	ZoneHand
	ZoneCharacter
	ZoneLife
	ZoneDonDeck
	ZoneDonField
	ZoneTrash
	ZoneStage
	ZoneLeader
	ZoneDonAttached
)

// Move is one RZ1 zone move:
// seq|owner|card|from zone|from slot|to zone|to slot|vp|ve|rest|power delta|cost delta[|instance].
type Move struct {
	Seq      int
	Owner    int
	Card     string
	From     Zone
	FromSlot int
	To       Zone
	ToSlot   int
	VP, VE   int
	Rested   bool
	Power    int
	Cost     int
	Instance int
	// Line is the index in Game.Lines of the next readable line, which places the move in the
	// timeline.
	Line int
}

// Check is an RZ1 CHK line, the true zone sizes for one owner after the move with the same Seq.
type Check struct {
	Seq         int
	Owner       int
	Deck        int
	Hand        int
	Character   int
	Life        int
	DonDeck     int
	DonField    int
	Trash       int
	Stage       int
	Leader      int
	DonAttached int
	// Line places the check in the timeline, as Move.Line does.
	Line int
}

func (b *builder) addRZ(line string) {
	parts := strings.Split(line, "|")
	if len(parts) < 2 {
		return
	}
	switch parts[1] {
	case "HDR":
		b.headers++
	case "PLY":
		b.addRZPlayer(parts)
	case "CHK":
		b.addCheck(parts)
	default:
		b.addMove(parts)
	}
}

func (b *builder) addRZPlayer(parts []string) {
	if len(parts) != 5 {
		return
	}
	owner, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}
	name, tag, _ := strings.Cut(parts[3], "\u200b#")
	if tag == "" {
		name, tag, _ = strings.Cut(parts[3], "#")
	}
	p := b.player(name + "#" + tag)
	p.Owner = owner
	if p.Leader == "" {
		p.Leader = parts[4]
	}
}

func (b *builder) addMove(parts []string) {
	if len(parts) != 13 && len(parts) != 14 {
		return
	}
	n, ok := ints(parts, 1, 2, 4, 5, 6, 7, 8, 9, 10, 11, 12)
	if !ok {
		return
	}
	m := Move{
		Seq: n[0], Owner: n[1], Card: parts[3],
		From: Zone(n[2]), FromSlot: n[3], To: Zone(n[4]), ToSlot: n[5],
		VP: n[6], VE: n[7], Rested: n[8] != 0, Power: n[9], Cost: n[10],
		Line: len(b.game.Lines),
	}
	if len(parts) == 14 {
		m.Instance, _ = strconv.Atoi(parts[13])
	}
	if m.From == ZoneLife && m.To != ZoneLife {
		b.ends.lifeTaken[m.Owner] = true
	}
	b.game.Moves = append(b.game.Moves, m)
	if len(b.print) < fingerprintMoves {
		b.print = append(b.print, strings.Join(parts, "|"))
	}
}

func (b *builder) addCheck(parts []string) {
	if len(parts) != 14 {
		return
	}
	n, ok := ints(parts, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13)
	if !ok {
		return
	}
	c := Check{
		Seq: n[0], Owner: n[1], Deck: n[2], Hand: n[3], Character: n[4], Life: n[5],
		DonDeck: n[6], DonField: n[7], Trash: n[8], Stage: n[9], Leader: n[10], DonAttached: n[11],
		Line: len(b.game.Lines),
	}
	b.ends.lifeByOwner[c.Owner] = c.Life
	b.game.Checks = append(b.game.Checks, c)
}

// inferOwners links players to RZ1 owners in logs without PLY lines. A player's hand before the
// mulligan is the same five cards as their owner's first five draws.
func (b *builder) inferOwners() {
	opening := make(map[int][]string)
	for _, m := range b.game.Moves {
		if m.From == ZoneDeck && m.To == ZoneHand && len(opening[m.Owner]) < 5 {
			opening[m.Owner] = append(opening[m.Owner], m.Card)
		}
	}
	for i := range b.game.Players {
		p := &b.game.Players[i]
		if p.Owner != 0 {
			continue
		}
		before := b.game.Mulligans[p.ID()].Before
		for owner, cards := range opening {
			if len(before) == 5 && sameCards(before, cards) {
				p.Owner = owner
			}
		}
	}
	b.inferLastOwner()
}

// inferLastOwner gives the one player left without an owner the other one. Logs often have only
// the local player's hand before the mulligan.
func (b *builder) inferLastOwner() {
	if len(b.game.Players) != 2 {
		return
	}
	a, c := &b.game.Players[0], &b.game.Players[1]
	if a.Owner == 0 && c.Owner != 0 {
		a, c = c, a
	}
	if a.Owner != 0 && c.Owner == 0 {
		c.Owner = 3 - a.Owner
	}
}

func sameCards(a, b []string) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}

func ints(parts []string, idx ...int) ([]int, bool) {
	out := make([]int, len(idx))
	for i, j := range idx {
		v, err := strconv.Atoi(parts[j])
		if err != nil {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}
