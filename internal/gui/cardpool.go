package gui

import (
	"cmp"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
)

// cardsState is the Cards page: its search, filters and the card shown. It resets like the
// other filters, when logpose restarts.
type cardsState struct {
	// tab is the card panel's tab, which stays as other cards are picked.
	tab    int
	query  string
	colors []cards.Color
	kind   int
	// costs are the costs picked in the strip, where maxCost stands for that cost and more.
	costs []int
	chips []cardChip
	// selected is the card shown on the right, and viewed the key of the picture it shows
	// large: an art's version, or a sim: or file: key. Empty is the first art.
	selected string
	viewed   string
	// versions is the head key of the art whose version list is open.
	versions string
	// viewer is whether the art viewer is open over the page.
	viewer   bool
	list     bool
	more     bool
	wall     ui.ListState
	wallBox  ui.Rect
	splitBox ui.Rect

	pool  *cardPool
	key   string
	shown []cards.Card
	// curve counts the cards at each cost that match every filter but cost.
	curve [maxCost + 1]int
}

// cardChip is a filter picked from the suggestions or More filters, such as a trait. out
// hides the cards that have the value instead of showing only them.
type cardChip struct {
	field string
	value string
	out   bool
}

const (
	chipTrait     = "Trait"
	chipKeyword   = "Keyword"
	chipAttribute = "Attribute"
	chipSet       = "Set"
	chipRarity    = "Rarity"
	chipBlock     = "Block"
	chipLegality  = "Legality"
	chipCounter   = "Counter"
)

var cardKinds = []string{"All", "Leader", "Character", "Event", "Stage"}

const maxCost = 10

// cardPool is the snapshot's cards with what searching and suggesting need, built once per
// card database.
type cardPool struct {
	db    *cards.DB
	cards []poolCard
	// suggestions are every chip a search can turn into, as "Supernovas · Trait".
	suggestions []string
	chips       map[string]cardChip
	keywords    []string
	sets        []string
	rarities    []string
}

type poolCard struct {
	card cards.Card
	// hay is the lowercase name, code, traits and effect that a search looks in.
	hay      string
	keywords []string
}

// keywordRe finds the bracketed keywords in an effect. "DON!! x2" and the like count as one.
var keywordRe = regexp.MustCompile(`\[([^\]]+)\]`)

// minKeywordCards is how many cards a bracketed word needs to be offered as a keyword. It
// leaves out one-off brackets.
const minKeywordCards = 15

func newCardPool(db *cards.DB) *cardPool {
	p := &cardPool{db: db, chips: make(map[string]cardChip)}
	traits, kw, sets, rarities := make(map[string]bool), make(map[string]int), make(map[string]bool), make(map[string]bool)
	for _, c := range db.All() {
		pc := poolCard{card: c, keywords: keywordsOf(c.Effect)}
		pc.hay = strings.ToLower(strings.Join([]string{c.Name, c.Code, strings.Join(c.Traits, " "), c.Effect}, "\n"))
		p.cards = append(p.cards, pc)
		for _, t := range c.Traits {
			traits[t] = true
		}
		for _, k := range pc.keywords {
			kw[k]++
		}
		for _, pr := range c.Prints {
			if pr.Set != "" {
				sets[pr.Set] = true
			}
		}
		if c.Rarity != "" {
			rarities[c.Rarity] = true
		}
	}
	slices.SortStableFunc(p.cards, func(a, b poolCard) int { return compareCodes(a.card.Code, b.card.Code) })
	names := make(map[string]bool)
	for _, pc := range p.cards {
		names[pc.card.Name] = true
	}
	for k, n := range kw {
		// Effects also put card names in brackets, as "[Monkey.D.Luffy]".
		if n >= minKeywordCards && !names[cards.CleanName(k)] {
			p.keywords = append(p.keywords, k)
		}
	}
	slices.Sort(p.keywords)
	p.sets = slices.Sorted(maps.Keys(sets))
	p.rarities = slices.Sorted(maps.Keys(rarities))
	add := func(field string, values []string) {
		for _, v := range values {
			s := v + " · " + field
			p.suggestions = append(p.suggestions, s)
			p.chips[s] = cardChip{field: field, value: v}
		}
	}
	add(chipTrait, slices.Sorted(maps.Keys(traits)))
	add(chipKeyword, p.keywords)
	add(chipAttribute, attributeNames)
	add(chipSet, p.sets)
	add(chipRarity, p.rarities)
	return p
}

var attributeNames = []string{"Slash", "Strike", "Ranged", "Special", "Wisdom"}

func keywordsOf(effect string) []string {
	var out []string
	for _, m := range keywordRe.FindAllStringSubmatch(effect, -1) {
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

// compareCodes puts the newest sets first: main boosters, then extra boosters, premium
// boosters, starter decks and promos, each by number from the highest, and cards in a set by
// number.
func compareCodes(a, b string) int {
	sa, na := splitCode(a)
	sb, nb := splitCode(b)
	return cmp.Or(cmp.Compare(setRank(sa), setRank(sb)), -cmp.Compare(setNumber(sa), setNumber(sb)), cmp.Compare(sa, sb), cmp.Compare(na, nb))
}

func splitCode(code string) (set string, num int) {
	set, n, _ := strings.Cut(code, "-")
	num, _ = strconv.Atoi(n)
	return set, num
}

func setRank(set string) int {
	for i, p := range []string{"OP", "EB", "PRB", "ST", "P"} {
		if strings.TrimRight(set, "0123456789") == p {
			return i
		}
	}
	return 9
}

func setNumber(set string) int {
	n, _ := strconv.Atoi(strings.TrimLeft(set, "ABCDEFGHIJKLMNOPQRSTUVWXYZ"))
	return n
}

// results are the cards that match the page's search and filters. They're worked out again
// only when those or the cards change.
func (s *cardsState) results(db *cards.DB) []cards.Card {
	if s.pool == nil || s.pool.db != db {
		s.pool = newCardPool(db)
		s.key = ""
	}
	key := s.filterKey()
	if key == s.key && s.shown != nil {
		return s.shown
	}
	s.key = key
	words := strings.Fields(strings.ToLower(s.query))
	s.shown = s.shown[:0]
	s.curve = [maxCost + 1]int{}
	for _, pc := range s.pool.cards {
		if !s.matches(pc, words) {
			continue
		}
		cost := pc.card.Cost
		if cost >= 0 {
			s.curve[min(cost, maxCost)]++
		}
		if len(s.costs) == 0 || (cost >= 0 && slices.Contains(s.costs, min(cost, maxCost))) {
			s.shown = append(s.shown, pc.card)
		}
	}
	return s.shown
}

func (s *cardsState) filterKey() string {
	var b strings.Builder
	b.WriteString(s.query)
	for _, c := range s.colors {
		b.WriteString("|" + string(c))
	}
	b.WriteString("|" + strconv.Itoa(s.kind))
	for _, c := range s.costs {
		b.WriteString("|c" + strconv.Itoa(c))
	}
	for _, c := range s.chips {
		op := "="
		if c.out {
			op = "!="
		}
		b.WriteString("|" + c.field + op + c.value)
	}
	return b.String()
}

// matches checks every filter but cost, which results checks after counting the curve.
func (s *cardsState) matches(pc poolCard, words []string) bool {
	c := pc.card
	for _, w := range words {
		if !strings.Contains(pc.hay, w) {
			return false
		}
	}
	if len(s.colors) > 0 && !slices.ContainsFunc(c.Colors, func(col cards.Color) bool { return slices.Contains(s.colors, col) }) {
		return false
	}
	if s.kind > 0 && c.Type != cardKinds[s.kind] {
		return false
	}
	byField := make(map[string][]string)
	for _, ch := range s.chips {
		if ch.out {
			if chipMatches(pc, ch.field, ch.value) {
				return false
			}
			continue
		}
		byField[ch.field] = append(byField[ch.field], ch.value)
	}
	for field, values := range byField {
		if !slices.ContainsFunc(values, func(v string) bool { return chipMatches(pc, field, v) }) {
			return false
		}
	}
	return true
}

// chipMatches reports whether a card has a filter's value. Values of one field are either-or,
// fields are all required, and a hidden value rules a card out whatever else it matches.
func chipMatches(pc poolCard, field, v string) bool {
	c := pc.card
	switch field {
	case chipTrait:
		return slices.Contains(c.Traits, v)
	case chipKeyword:
		return slices.Contains(pc.keywords, v)
	case chipAttribute:
		return slices.Contains(c.Attributes, v)
	case chipSet:
		return slices.ContainsFunc(c.Prints, func(p cards.Print) bool { return p.Set == v })
	case chipRarity:
		return c.Rarity == v
	case chipBlock:
		return c.Block == v
	case chipLegality:
		return legalityLabel(c.Legality) == v
	case chipCounter:
		return counterLabel(c.Counter) == v
	}
	return false
}

func legalityLabel(l cards.Legality) string {
	switch l {
	case cards.Banned:
		return "Banned"
	case cards.Restricted:
		return "Restricted"
	case cards.Rotated:
		return "Rotated"
	case cards.Legal:
		return "Legal"
	case cards.Unreleased:
		return "Upcoming"
	}
	return "Unknown"
}

func counterLabel(n int) string {
	if n == 0 {
		return "No counter"
	}
	return "+" + strconv.Itoa(n)
}

// takeSuggestion turns the search into a chip when it's one of the suggestions, which is what
// picking one does.
func (s *cardsState) takeSuggestion() bool {
	if s.pool == nil {
		return false
	}
	ch, ok := s.pool.chips[s.query]
	if !ok {
		return false
	}
	s.query = ""
	s.toggleChip(ch)
	return true
}

// toggleChip adds a chip, or removes the one with its field and value.
func (s *cardsState) toggleChip(ch cardChip) {
	if i := s.chipIndex(ch.field, ch.value); i >= 0 {
		s.chips = slices.Delete(s.chips, i, i+1)
		return
	}
	s.chips = append(s.chips, ch)
}

// cycleChip moves a value from off to shown only, to hidden, and back to off.
func (s *cardsState) cycleChip(field, value string) {
	i := s.chipIndex(field, value)
	switch {
	case i < 0:
		s.chips = append(s.chips, cardChip{field: field, value: value})
	case !s.chips[i].out:
		s.chips[i].out = true
	default:
		s.chips = slices.Delete(s.chips, i, i+1)
	}
}

// chipState says whether a value is picked, and if so whether it hides cards.
func (s *cardsState) chipState(field, value string) (on, out bool) {
	if i := s.chipIndex(field, value); i >= 0 {
		return true, s.chips[i].out
	}
	return false, false
}

func (s *cardsState) chipIndex(field, value string) int {
	return slices.IndexFunc(s.chips, func(c cardChip) bool { return c.field == field && c.value == value })
}

func (s *cardsState) toggleColor(col cards.Color) {
	if i := slices.Index(s.colors, col); i >= 0 {
		s.colors = slices.Delete(s.colors, i, i+1)
		return
	}
	s.colors = append(s.colors, col)
}

func (s *cardsState) toggleCost(n int) {
	if i := slices.Index(s.costs, n); i >= 0 {
		s.costs = slices.Delete(s.costs, i, i+1)
		return
	}
	s.costs = append(s.costs, n)
}

func (s *cardsState) filtered() bool {
	return s.query != "" || len(s.colors) > 0 || s.kind > 0 || len(s.costs) > 0 || len(s.chips) > 0
}

func (s *cardsState) clear() {
	s.query, s.colors, s.kind, s.costs, s.chips = "", nil, 0, nil, nil
}

// cardSync is a card refresh in progress, which the Cards page shows. It's zero between
// refreshes.
type cardSync struct {
	done, total int
	// sets are the sets read so far, the latest first.
	sets []cards.SetProgress
}

func (s *cardSync) add(p cards.SetProgress) {
	s.done, s.total = p.Done, p.Total
	s.sets = append([]cards.SetProgress{p}, s.sets...)
}

// named is up to n of the sets read, the latest first, leaving out pages with no cards yet.
func (s cardSync) named(n int) []cards.SetProgress {
	var out []cards.SetProgress
	for _, p := range s.sets {
		if p.Cards > 0 && len(out) < n {
			out = append(out, p)
		}
	}
	return out
}

func (s cardSync) running() bool { return s.total > 0 && s.done < s.total }

// artSync is a read of Bandai's card list in progress, by page. It's zero between reads.
type artSync struct{ done, total int }

func (s artSync) running() bool { return s.total > 0 && s.done < s.total }
