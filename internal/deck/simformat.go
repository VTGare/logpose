package deck

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// FormatSim writes a decklist as OPTCGSim imports it from the clipboard or a .deck file: the
// leader first as 1xCODE, then each main-deck card as NxCODE on CRLF lines. Cards follow order
// when given, or sort by code.
func FormatSim(leader string, order []string, list map[string]int) string {
	var b strings.Builder
	if leader != "" {
		fmt.Fprintf(&b, "1x%s\r\n", leader)
	}
	if len(order) == 0 {
		order = slices.Sorted(maps.Keys(list))
	}
	for _, code := range order {
		if n := list[code]; n > 0 {
			fmt.Fprintf(&b, "%dx%s\r\n", n, code)
		}
	}
	return b.String()
}

// Count is the total copies in a decklist.
func Count(list map[string]int) int {
	n := 0
	for _, c := range list {
		n += c
	}
	return n
}
