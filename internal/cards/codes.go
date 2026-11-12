package cards

import (
	"cmp"
	"regexp"
	"strconv"
	"strings"
)

// Card types as the card data writes them.
const (
	Leader    = "Leader"
	Character = "Character"
	Event     = "Event"
	Stage     = "Stage"
)

func (c Card) IsLeader() bool { return c.Type == Leader }

// HasPower is false for events, stages and cards without data. Some characters print 0 power.
func (c Card) HasPower() bool { return c.Type == Character || c.Type == Leader }

// PaidCost is the DON!! the card takes to play, 0 for a leader, whose Cost is -1.
func (c Card) PaidCost() int { return max(c.Cost, 0) }

// PrintKeyPattern is a printing's key: the card's code, with _p1 and so on for the alternate
// printings. Limitless and Bandai write their keys the same way.
const PrintKeyPattern = `[A-Z0-9]+-\d+(?:_p\d+)?`

var printKey = regexp.MustCompile(`^` + PrintKeyPattern + `$`)

// IsPrintKey reports whether s is a printing's key, such as "OP07-021" or "OP07-021_p1". A
// Bandai reprint's "_r1" isn't one.
func IsPrintKey(s string) bool { return printKey.MatchString(s) }

// SetOf is a card code's set, "OP13" for OP13-001. It works on a printing's key too.
func SetOf(code string) string {
	set, _, _ := strings.Cut(code, "-")
	return set
}

// SetKinds are the kinds of set, newest kind first: main boosters, extra boosters, premium
// boosters, starter decks and promos.
var SetKinds = []string{"OP", "EB", "PRB", "ST", "P"}

// SplitSet splits a set's code into its kind and number, "OP13" into "OP" and 13. ok is false
// for a set without a number, such as the promos' "P", which is all kind.
func SplitSet(set string) (kind string, n int, ok bool) {
	i := strings.IndexAny(set, "0123456789")
	switch {
	case i < 0:
		return set, 0, false
	case i == 0:
		return "", 0, false
	}
	n, err := strconv.Atoi(set[i:])
	return set[:i], n, err == nil
}

// CompareCodes puts the newest sets first: the kinds in SetKinds order, each by number from the
// highest, and the cards in a set by number.
func CompareCodes(a, b string) int {
	sa, na := splitCode(a)
	sb, nb := splitCode(b)
	ka, ia, _ := SplitSet(sa)
	kb, ib, _ := SplitSet(sb)
	return cmp.Or(cmp.Compare(kindRank(ka), kindRank(kb)), -cmp.Compare(ia, ib), cmp.Compare(sa, sb), cmp.Compare(na, nb))
}

func splitCode(code string) (set string, num int) {
	set, n, _ := strings.Cut(code, "-")
	num, _ = strconv.Atoi(n)
	return set, num
}

func kindRank(kind string) int {
	for i, k := range SetKinds {
		if k == kind {
			return i
		}
	}
	return len(SetKinds)
}
