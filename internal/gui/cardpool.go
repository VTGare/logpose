package gui

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
)

// cardsState is the Cards page: its search, filters and the card shown. It resets like the
// other filters, when logpose restarts. The sort and the list switch are kept in prefs.
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
	viewer   bool
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
	field *chipField
	value string
	out   bool
}

// chipField is a kind of chip. suggest offers its values in the search, and more is its place
// under More filters, where 0 leaves it out.
type chipField struct {
	name    string
	values  func(p *cardPool) []string
	match   func(pc poolCard, v string) bool
	suggest bool
	more    int
}

// chipFields are in the order the search suggests them.
var chipFields = []*chipField{
	{
		name: "Trait", suggest: true,
		values: func(p *cardPool) []string { return p.traits },
		match:  func(pc poolCard, v string) bool { return slices.Contains(pc.card.Traits, v) },
	},
	{
		name: "Keyword", suggest: true, more: 3,
		values: func(p *cardPool) []string { return p.keywords },
		match:  func(pc poolCard, v string) bool { return slices.Contains(pc.keywords, v) },
	},
	{
		name: "Attribute", suggest: true, more: 1,
		values: func(*cardPool) []string { return []string{"Slash", "Strike", "Ranged", "Special", "Wisdom"} },
		match:  func(pc poolCard, v string) bool { return slices.Contains(pc.card.Attributes, v) },
	},
	{
		name: "Set", suggest: true,
		values: func(p *cardPool) []string { return p.sets },
		match: func(pc poolCard, v string) bool {
			return slices.ContainsFunc(pc.card.Prints, func(p cards.Print) bool { return p.Set == v })
		},
	},
	{
		name: "Rarity", suggest: true, more: 6,
		values: func(p *cardPool) []string { return p.rarities },
		match:  func(pc poolCard, v string) bool { return pc.card.Rarity == v },
	},
	{
		name: "Counter", more: 2,
		values: func(*cardPool) []string { return []string{counterChip(0), counterChip(1000), counterChip(2000)} },
		match:  func(pc poolCard, v string) bool { return counterChip(pc.card.Counter) == v },
	},
	{
		name: "Block", more: 4,
		values: func(*cardPool) []string { return []string{"1", "2", "3", "4", "5", "X"} },
		match:  func(pc poolCard, v string) bool { return pc.card.Block == v },
	},
	{
		name: "Legality", more: 5,
		values: func(*cardPool) []string { return legalityLabels() },
		match:  func(pc poolCard, v string) bool { return legalityOf(pc.card.Legality).label == v },
	},
}

var cardKinds = []string{"All", cards.Leader, cards.Character, cards.Event, cards.Stage}

const maxCost = 10

// cardPool is the snapshot's cards with what searching and suggesting need, built once per
// card database.
type cardPool struct {
	db    *cards.DB
	cards []poolCard
	// suggestions are every chip a search can turn into, as "Supernovas · Trait".
	suggestions []string
	chips       map[string]cardChip
	traits      []string
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

// minKeywordCards is how many cards a bracketed word needs to be offered as a keyword. It
// leaves out one-off brackets.
const minKeywordCards = 15

func newCardPool(db *cards.DB) *cardPool {
	p := &cardPool{db: db, chips: make(map[string]cardChip)}
	traits, kw, sets, rarities := make(map[string]bool), make(map[string]int), make(map[string]bool), make(map[string]bool)
	for _, c := range db.All() {
		pc := poolCard{card: c, keywords: cards.Keywords(c.Effect)}
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
	slices.SortStableFunc(p.cards, func(a, b poolCard) int { return cards.CompareCodes(a.card.Code, b.card.Code) })
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
	p.traits = slices.Sorted(maps.Keys(traits))
	for _, f := range chipFields {
		if !f.suggest {
			continue
		}
		for _, v := range f.values(p) {
			s := v + " · " + f.name
			p.suggestions = append(p.suggestions, s)
			p.chips[s] = cardChip{field: f, value: v}
		}
	}
	return p
}

// cardSort is the Cards page's order. flip turns the key's first direction around.
type cardSort struct {
	key  string
	flip bool
}

// cardSortFrom reads the card_sort setting, "<key>" or "<key> reversed". An unknown key sorts
// by set.
func cardSortFrom(v string) cardSort {
	key, rest, _ := strings.Cut(v, " ")
	if sortKeyOf(key).id != key {
		return cardSort{key: sortSet}
	}
	return cardSort{key: key, flip: rest == "reversed"}
}

func (s cardSort) String() string {
	if s.flip {
		return s.key + " reversed"
	}
	return s.key
}

// label is the sort as the pill names it, such as "Cost · lowest first".
func (s cardSort) label() string {
	k := sortKeyOf(s.key)
	return k.title + " · " + strings.ToLower(k.dirs[boolInt(s.flip)])
}

// desc is whether the larger values come first, which points the list header's chevron down.
func (s cardSort) desc() bool { return sortKeyOf(s.key).desc != s.flip }

// clicked is the sort after picking key in the menu or the list header. The key in force
// turns around, and another starts at its first direction.
func (s cardSort) clicked(key string) cardSort {
	if sortKeyOf(s.key).id == key {
		return cardSort{key: key, flip: !s.flip}
	}
	return cardSort{key: key}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

const (
	sortSet     = "set"
	sortName    = "name"
	sortCost    = "cost"
	sortPower   = "power"
	sortCounter = "counter"
	sortRarity  = "rarity"
	sortColor   = "color"
)

type sortKey struct {
	id, title string
	// dirs names the first direction and the other.
	dirs [2]string
	desc bool
	// compare orders two cards in the first direction.
	compare func(a, b cards.Card) int
	// last picks the cards that go after the rest in either direction, in the first
	// direction's order among themselves.
	last func(cards.Card) bool
}

var sortKeys = []sortKey{
	{
		id: sortSet, title: "Set", dirs: [2]string{"By number", "Reversed"}, desc: true,
		compare: func(a, b cards.Card) int { return cards.CompareCodes(a.Code, b.Code) },
	},
	{
		id: sortName, title: "Name", dirs: [2]string{"A to Z", "Z to A"},
		compare: func(a, b cards.Card) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) },
	},
	{
		id: sortCost, title: "Cost", dirs: [2]string{"Lowest first", "Highest first"},
		compare: func(a, b cards.Card) int { return cmp.Compare(a.Cost, b.Cost) },
		last:    cards.Card.IsLeader,
	},
	{
		id: sortPower, title: "Power", dirs: [2]string{"Highest first", "Lowest first"}, desc: true,
		compare: func(a, b cards.Card) int { return cmp.Compare(b.Power, a.Power) },
		last:    func(c cards.Card) bool { return !c.HasPower() },
	},
	{
		id: sortCounter, title: "Counter", dirs: [2]string{"Highest first", "Lowest first"}, desc: true,
		compare: func(a, b cards.Card) int { return cmp.Compare(b.Counter, a.Counter) },
	},
	{
		id: sortRarity, title: "Rarity", dirs: [2]string{"Rarest first", "Commonest first"}, desc: true,
		compare: func(a, b cards.Card) int { return cmp.Compare(rarityRank(a.Rarity), rarityRank(b.Rarity)) },
		last:    func(c cards.Card) bool { return rarityRank(c.Rarity) >= rarityRanked },
	},
	{
		id: sortColor, title: "Color", dirs: [2]string{"Game order", "Reversed"},
		compare: compareColors,
		last:    func(c cards.Card) bool { return len(c.Colors) == 0 },
	},
}

func sortKeyOf(id string) sortKey {
	for _, k := range sortKeys {
		if k.id == id {
			return k
		}
	}
	return sortKeys[0]
}

// rarityOrder runs from the rarest. Leader and Special Card aren't rarer or commoner than the
// others, so they go last either way.
var rarityOrder = []string{"Secret Rare", "Super Rare", "Rare", "Uncommon", "Common", "Leader", "Special Card"}

const rarityRanked = 5

func rarityRank(r string) int {
	if i := slices.Index(rarityOrder, r); i >= 0 {
		return i
	}
	return len(rarityOrder)
}

// compareColors puts cards in the game's color order, with multicolor cards after the single
// color they start with. A card's colors are already in that order.
func compareColors(a, b cards.Card) int {
	return slices.CompareFunc(a.Colors, b.Colors, func(x, y cards.Color) int {
		return cmp.Compare(slices.Index(cardColors, x), slices.Index(cardColors, y))
	})
}

// sortCards orders cards by the sort, with ties in set order.
func sortCards(cs []cards.Card, by cardSort) {
	k := sortKeyOf(by.key)
	slices.SortStableFunc(cs, func(a, b cards.Card) int {
		if k.last != nil {
			la, lb := k.last(a), k.last(b)
			if la != lb {
				return cmp.Compare(boolInt(la), boolInt(lb))
			}
			if la {
				return cmp.Or(k.compare(a, b), cards.CompareCodes(a.Code, b.Code))
			}
		}
		r := k.compare(a, b)
		if by.flip {
			r = -r
		}
		return cmp.Or(r, cards.CompareCodes(a.Code, b.Code))
	})
}

// results are the cards that match the page's search and filters, in the sort's order. They're
// worked out again only when those or the cards change.
func (s *cardsState) results(db *cards.DB, by cardSort) []cards.Card {
	if s.pool == nil || s.pool.db != db {
		s.pool = newCardPool(db)
		s.key = ""
	}
	key := s.filterKey() + "|sort " + by.String()
	if key == s.key && s.shown != nil {
		return s.shown
	}
	s.key = key
	var tests []func(poolCard) bool
	for _, f := range cardFilters {
		if f.test == nil || f.key(s) == "" {
			continue
		}
		tests = append(tests, f.test(s))
	}
	s.shown = s.shown[:0]
	s.curve = [maxCost + 1]int{}
	for _, pc := range s.pool.cards {
		if !allPass(tests, pc) {
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
	sortCards(s.shown, by)
	return s.shown
}

// cardFilter is one of the Cards page's filters.
type cardFilter struct {
	// key is the filter's part of the results' cache key, or "" while it's off.
	key func(*cardsState) string
	// test makes the filter's check for one card. Cost has none, since results checks it after
	// counting the curve.
	test  func(*cardsState) func(poolCard) bool
	clear func(*cardsState)
}

var cardFilters = []cardFilter{
	{
		key: func(s *cardsState) string { return s.query },
		test: func(s *cardsState) func(poolCard) bool {
			words := strings.Fields(strings.ToLower(s.query))
			return func(pc poolCard) bool {
				return !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(pc.hay, w) })
			}
		},
		clear: func(s *cardsState) { s.query = "" },
	},
	{
		key: func(s *cardsState) string { return joinAll(s.colors, func(c cards.Color) string { return string(c) }) },
		test: func(s *cardsState) func(poolCard) bool {
			return func(pc poolCard) bool {
				return slices.ContainsFunc(pc.card.Colors, func(col cards.Color) bool { return slices.Contains(s.colors, col) })
			}
		},
		clear: func(s *cardsState) { s.colors = nil },
	},
	{
		key: func(s *cardsState) string {
			if s.kind == 0 {
				return ""
			}
			return cardKinds[s.kind]
		},
		test: func(s *cardsState) func(poolCard) bool {
			return func(pc poolCard) bool { return pc.card.Type == cardKinds[s.kind] }
		},
		clear: func(s *cardsState) { s.kind = 0 },
	},
	{
		key:   func(s *cardsState) string { return joinAll(s.costs, strconv.Itoa) },
		clear: func(s *cardsState) { s.costs = nil },
	},
	{
		key: func(s *cardsState) string {
			return joinAll(s.chips, func(c cardChip) string {
				if c.out {
					return c.field.name + "!=" + c.value
				}
				return c.field.name + "=" + c.value
			})
		},
		test:  chipsTest,
		clear: func(s *cardsState) { s.chips = nil },
	},
}

func joinAll[T any](xs []T, str func(T) string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = str(x)
	}
	return strings.Join(parts, ",")
}

func allPass(tests []func(poolCard) bool, pc poolCard) bool {
	for _, t := range tests {
		if !t(pc) {
			return false
		}
	}
	return true
}

// chipsTest checks the chips. Values of one field are either-or, fields are all required, and a
// hidden value rules a card out whatever else it matches.
func chipsTest(s *cardsState) func(poolCard) bool {
	var out []cardChip
	byField := make(map[*chipField][]string)
	for _, ch := range s.chips {
		if ch.out {
			out = append(out, ch)
		} else {
			byField[ch.field] = append(byField[ch.field], ch.value)
		}
	}
	return func(pc poolCard) bool {
		if slices.ContainsFunc(out, func(ch cardChip) bool { return ch.field.match(pc, ch.value) }) {
			return false
		}
		for field, values := range byField {
			if !slices.ContainsFunc(values, func(v string) bool { return field.match(pc, v) }) {
				return false
			}
		}
		return true
	}
}

func (s *cardsState) filterKey() string {
	parts := make([]string, len(cardFilters))
	for i, f := range cardFilters {
		parts[i] = f.key(s)
	}
	return strings.Join(parts, "|")
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

func (s *cardsState) toggleChip(ch cardChip) {
	if i := s.chipIndex(ch.field, ch.value); i >= 0 {
		s.chips = slices.Delete(s.chips, i, i+1)
		return
	}
	s.chips = append(s.chips, ch)
}

// cycleChip moves a value from off to shown only, to hidden, and back to off.
func (s *cardsState) cycleChip(field *chipField, value string) {
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

func (s *cardsState) chipState(field *chipField, value string) (on, out bool) {
	if i := s.chipIndex(field, value); i >= 0 {
		return true, s.chips[i].out
	}
	return false, false
}

func (s *cardsState) chipIndex(field *chipField, value string) int {
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
	return slices.ContainsFunc(cardFilters, func(f cardFilter) bool { return f.key(s) != "" })
}

func (s *cardsState) clear() {
	for _, f := range cardFilters {
		f.clear(s)
	}
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
