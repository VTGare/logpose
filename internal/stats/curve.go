package stats

// MaxDon is the most DON!! a player has.
const MaxDon = 10

// Don is the DON!! a player has on their own turn n, counting from 1: 1 and then 2 more a turn
// going first, 2 a turn going second, up to MaxDon.
func Don(turn int, first bool) int {
	if turn <= 0 {
		return 0
	}
	d := 2 * turn
	if first {
		d--
	}
	return min(d, MaxDon)
}

func PayTurn(cost int, first bool) int {
	t := 1
	for Don(t, first) < min(cost, MaxDon) {
		t++
	}
	return t
}

// Draws is how many cards the player has drawn by their own turn n. The player going first
// doesn't draw on their first turn.
func Draws(turn int, first bool) int {
	if first {
		return max(turn-1, 0)
	}
	return max(turn, 0)
}

// FromOpening reports whether a card of this cost is paid for with at most one draw after the
// opening hand going first or second, so whether it's in hand by its turn is about the opening
// hand.
func FromOpening(cost int) bool {
	return Draws(PayTurn(cost, true), true) <= 1 && Draws(PayTurn(cost, false), false) <= 1
}
