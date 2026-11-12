package cards

import (
	"regexp"
	"slices"
	"strings"
)

var (
	// A searcher's effect says what it takes between "reveal up to 1" and "and add it".
	searchClause = regexp.MustCompile(`(?i)reveal up to (?:a total of )?\d+ (.+?),? (?:and )?add (?:it|them) to your hand`)
	cardType     = regexp.MustCompile(`\b(Event|Stage)s?\b`)
	braceType    = regexp.MustCompile(`\{([^}]+)\}`)
	quotedType   = regexp.MustCompile(`type including "([^"]+)"`)
	attribute    = regexp.MustCompile(`<([A-Za-z]+)> attribute`)
	costAtLeast  = regexp.MustCompile(`cost of (\d+) or more`)
	costAtMost   = regexp.MustCompile(`cost of (\d+) or less`)
	costExactly  = regexp.MustCompile(`cost of (\d+)\b(?: or)?`)
	powerOf      = regexp.MustCompile(`(\d+) power`)
	colorEvent   = regexp.MustCompile(`\b(red|green|blue|purple|black|yellow) Event\b`)
	otherThan    = regexp.MustCompile(`other than \[[^\]]+\]`)
	namedCard    = regexp.MustCompile(`\[([^\]]+)\]`)
)

// Keywords are the bracketed words in an effect, each once, such as "Blocker" or "On Play".
// "DON!! x2" and the like count as "DON!!".
func Keywords(effect string) []string {
	var out []string
	for _, m := range namedCard.FindAllStringSubmatch(effect, -1) {
		k := m[1]
		if strings.HasPrefix(k, "DON!!") {
			k = "DON!!"
		}
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

// SearchTargets reads from a card's effect what it searches the deck for, such as "Land of
// Wano", "Wisdom", "Trigger" or a card's name. It's empty for a card that doesn't search or
// whose wording it doesn't know.
func SearchTargets(effect string) []string {
	m := searchClause.FindStringSubmatch(effect)
	if m == nil {
		return nil
	}
	clause := otherThan.ReplaceAllString(m[1], "")
	var out []string
	add := func(s string) {
		for _, o := range out {
			if o == s {
				return
			}
		}
		out = append(out, s)
	}
	for _, x := range braceType.FindAllStringSubmatch(clause, -1) {
		add(x[1])
	}
	for _, x := range quotedType.FindAllStringSubmatch(clause, -1) {
		add(x[1])
	}
	for _, x := range attribute.FindAllStringSubmatch(clause, -1) {
		add(x[1])
	}
	for _, x := range namedCard.FindAllStringSubmatch(clause, -1) {
		add(x[1])
	}
	if x := colorEvent.FindStringSubmatch(clause); x != nil {
		add(x[1] + " Event")
	} else if x := cardType.FindStringSubmatch(clause); x != nil {
		add(x[1])
	}
	if x := costAtLeast.FindStringSubmatch(clause); x != nil {
		add("Cost " + x[1] + "+")
	}
	if x := costAtMost.FindStringSubmatch(clause); x != nil {
		add("Cost " + x[1] + " or less")
	}
	if x := costExactly.FindStringSubmatch(clause); x != nil && !strings.HasSuffix(x[0], " or") {
		add("Cost " + x[1])
	}
	if x := powerOf.FindStringSubmatch(clause); x != nil {
		add(x[1] + " power")
	}
	return out
}
