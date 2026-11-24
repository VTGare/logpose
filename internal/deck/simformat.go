package deck

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// FormatSim writes a decklist in OPTCGSim's clipboard and .deck format.
// The leader comes first as 1xCODE, then each main-deck card as NxCODE on CRLF lines.
// Cards follow order first, and any remaining cards in list are appended sorted by code.
func FormatSim(leader string, order []string, list map[string]int) string {
	var b strings.Builder
	if leader != "" {
		fmt.Fprintf(&b, "1x%s\r\n", leader)
	}
	seen := make(map[string]bool, len(list))
	for _, code := range order {
		if seen[code] {
			continue
		}
		if n := list[code]; n > 0 {
			seen[code] = true
			fmt.Fprintf(&b, "%dx%s\r\n", n, code)
		}
	}
	for _, code := range slices.Sorted(maps.Keys(list)) {
		if seen[code] {
			continue
		}
		if n := list[code]; n > 0 {
			fmt.Fprintf(&b, "%dx%s\r\n", n, code)
		}
	}
	return b.String()
}

func Count(list map[string]int) int {
	n := 0
	for _, c := range list {
		n += c
	}
	return n
}
