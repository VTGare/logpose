package replay

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/VTGare/logpose/internal/simlog"
)

type IssueKind int

const (
	// IssueCount is a zone whose size differs from the RZ1 check after a move.
	IssueCount IssueKind = iota + 1
	// IssueMove is a move whose slot is empty or holds another card.
	IssueMove
	// IssueSnapshot is a Hand, Board, Trash or Life line that differs from the board.
	IssueSnapshot
	// IssueRest is an attack by a rested card or on an active character.
	IssueRest
	// IssueNoAttack is an attack by a card that can't attack.
	IssueNoAttack
	// IssueTwice is a second attack by a card in one turn without it being set active between.
	IssueTwice
)

func (k IssueKind) String() string {
	switch k {
	case IssueCount:
		return "count"
	case IssueMove:
		return "move"
	case IssueSnapshot:
		return "snapshot"
	case IssueNoAttack:
		return "no-attack"
	case IssueTwice:
		return "twice"
	default:
		return "rest"
	}
}

// Issue is a place where the rebuilt board and the log disagree.
type Issue struct {
	Kind IssueKind
	Step int
	// Seq is the move's RZ1 number, or 0 for a readable line.
	Seq int
	// Line is the index of the readable line, or -1 for a move.
	Line   int
	Owner  int
	Detail string
}

func (is Issue) String() string {
	at := fmt.Sprintf("seq %d", is.Seq)
	if is.Line >= 0 {
		at = fmt.Sprintf("line %d", is.Line)
	}
	return fmt.Sprintf("step %d %s owner %d %s: %s", is.Step, at, is.Owner, is.Kind, is.Detail)
}

var zoneNames = [10]string{
	"deck", "hand", "character", "life", "DON deck", "DON field", "trash", "stage", "leader",
	"DON attached",
}

func countIssues(s *Side, c simlog.Check) []string {
	got, want := s.counts(), checkCounts(c)
	var out []string
	for z := range got {
		if got[z] != want[z] {
			out = append(out, fmt.Sprintf("%s %d, check says %d", zoneNames[z], got[z], want[z]))
		}
	}
	return out
}

// snapshotIssue compares a "Hand: [...]", "Board: [...]", "Trash: [...]" or "Life: N" line with
// the side. The sim prints them for both players around the end of each turn and after battles.
// Hand and trash are compared as sets of cards, since effects can reorder them.
func snapshotIssue(s *Side, text string) (string, bool) {
	name, list, ok := strings.Cut(text, ": ")
	if !ok {
		return "", false
	}
	switch name {
	case "Hand":
		return compareCodes(name, Codes(s.Hand), list, true), true
	case "Trash":
		return compareCodes(name, Codes(s.Trash), list, true), true
	case "Board":
		codes := make([]string, 0, len(s.Characters))
		for _, c := range s.Characters {
			codes = append(codes, c.Code)
		}
		return compareCodes(name, codes, list, false), true
	case "Life":
		n, err := strconv.Atoi(list)
		if err != nil {
			return "", false
		}
		if n != len(s.Life) {
			return fmt.Sprintf("Life %d, board has %d", n, len(s.Life)), true
		}
		return "", true
	}
	return "", false
}

func compareCodes(name string, have []string, list string, sorted bool) string {
	inner, ok := strings.CutPrefix(list, "[")
	if !ok {
		return ""
	}
	inner, _ = strings.CutSuffix(inner, "]")
	var want []string
	if inner != "" {
		want = strings.Split(inner, ",")
	}
	got := slices.Clone(have)
	if sorted {
		slices.Sort(want)
		slices.Sort(got)
	}
	if slices.Equal(got, want) {
		return ""
	}
	return fmt.Sprintf("%s %v, board has %v", name, want, got)
}
