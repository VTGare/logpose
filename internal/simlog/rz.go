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

const endPrefix = "RZ1|END|"

// Closed reports whether a game's text has the sim's RZ1|END line, as Game.Closed does.
func Closed(text string) bool {
	return strings.HasPrefix(text, endPrefix) || strings.Contains(text, "\n"+endPrefix)
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
	case "META":
		// META|replay id|OPBounty match number, 0 outside a match the sim's ranked panel set up.
		if len(parts) == 4 {
			b.game.Match, _ = strconv.Atoi(parts[3])
		}
	case "END":
		b.game.Closed = true
		b.addSeal(parts)
	case "CHK":
		b.addCheck(parts)
	default:
		b.addMove(parts)
	}
}

// seal is the sim's RZ1|END line: seq|winner|reason|hash. Winner 1 is the player whose log it
// is and 2 the opponent, and the reason is from the first one's side: win, loss, concede,
// opponent_concede, opponent_dc or self_dc.
type seal struct {
	winner int
	reason string
}

func (b *builder) addSeal(parts []string) {
	if len(parts) < 5 {
		return
	}
	winner, err := strconv.Atoi(parts[3])
	if err != nil || (winner != 1 && winner != 2) {
		return
	}
	b.seal = &seal{winner: winner, reason: parts[4]}
}

// sealedEnd is the end the END line gives, for games whose log has no line saying how they
// ended. The loser ran out of cards when their last check shows an empty deck.
func (b *builder) sealedEnd(g *Game) End {
	if b.seal == nil {
		return End{}
	}
	loser := 3 - b.seal.winner
	var id string
	for _, p := range g.Players {
		if p.Owner == loser {
			id = p.ID()
		}
	}
	if id == "" {
		return End{}
	}
	end := End{Loser: id}
	switch {
	case b.seal.reason == "concede" || b.seal.reason == "opponent_concede":
		end.Kind = EndConcede
	case b.lastDeck(loser) == 0:
		end.Kind = EndDeckOut
	}
	return end
}

// lastDeck is the owner's deck size at their latest check, or -1 without one.
func (b *builder) lastDeck(owner int) int {
	for _, c := range slices.Backward(b.game.Checks) {
		if c.Owner == owner {
			return c.Deck
		}
	}
	return -1
}

func (b *builder) addRZPlayer(parts []string) {
	// The sim writes an empty PLY line per seat before the named one.
	if len(parts) != 5 || parts[3] == "" {
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
