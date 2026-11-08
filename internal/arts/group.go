package arts

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"github.com/VTGare/logpose/internal/cards"
)

// Version is one printing that shows an art's picture.
type Version struct {
	Key Key
	// Name is the short name under a picture: "Alt art", "SP", "Regional".
	Name string
	// Short is the version list's line above the product. It is Name, but for Regional prizes
	// it says which prize, as in "Offline finalist".
	Short string
	// Product is where it was printed: Limitless's set name, or Bandai's "Card Set(s)" line,
	// such as "Treasure Cup November 2025~".
	Product string
}

// Art is one distinct picture of a card.
type Art struct {
	// Versions has the head first. The head's picture is the one shown. A Limitless printing
	// always heads its art, and two Limitless printings never share one.
	Versions []Version
	// Same holds Bandai printings that are the head's own printing on Bandai's site. They are
	// never shown or offered, but a key saved for one still finds this art.
	Same []Key
}

func (a Art) Head() Version { return a.Versions[0] }

func (a Art) Name() string { return a.Versions[0].Name }

// Has reports whether a saved key, as sim-art.json holds it, is one of the art's versions or
// the same printing as its head.
func (a Art) Has(key string) bool {
	_, ok := a.Find(key)
	return ok
}

// Find is the version a saved key names. A key in Same finds the head.
func (a Art) Find(key string) (Version, bool) {
	for _, v := range a.Versions {
		if v.Key.String() == key {
			return v, true
		}
	}
	for _, k := range a.Same {
		if k.String() == key {
			return a.Head(), true
		}
	}
	return Version{}, false
}

// members are the keys whose pictures show the art.
func (a Art) members() []Key {
	out := make([]Key, 0, len(a.Versions)+len(a.Same))
	for _, v := range a.Versions {
		out = append(out, v.Key)
	}
	return append(out, a.Same...)
}

// CardArts is a card's arts: Limitless's in its order, then Bandai's own by key.
type CardArts struct {
	Arts []Art
	// Pending is how many Bandai printings still wait for their picture to be compared.
	Pending int
}

// Find is the art and version a saved key names. A key in an art's Same finds its head.
func (c CardArts) Find(key string) (Art, Version, bool) {
	for _, a := range c.Arts {
		if v, ok := a.Find(key); ok {
			return a, v, true
		}
	}
	return Art{}, Version{}, false
}

// LimitlessArts is one art per Limitless printing. It is what shows before the gallery
// answers, without an art folder, and with art downloads off.
func LimitlessArts(card cards.Card) CardArts { return group(card, nil, noGrids, true) }

func noGrids(Key) (grid, bool) { return grid{}, false }

// printsOf is the card's printings, or its code alone when Limitless has none for it.
func printsOf(card cards.Card) []cards.Print {
	if len(card.Prints) > 0 {
		return card.Prints
	}
	return []cards.Print{{Key: card.Code}}
}

// group applies the grouping rules. gridFor reports the grids known now. When final is false, a
// Bandai printing that can't be placed yet counts as pending. When it is true, a printing
// without a grid is left out, since without its picture it can be neither shown nor compared.
func group(card cards.Card, bandai []printing, gridFor func(Key) (grid, bool), final bool) CardArts {
	var out CardArts
	prints := printsOf(card)
	for _, p := range prints {
		name := limitlessName(card, p)
		out.Arts = append(out.Arts, Art{Versions: []Version{{Key: Key{Limitless, p.Key}, Name: name, Short: name, Product: p.Set}}})
	}
	heads := len(out.Arts)
	twinned := make([]bool, heads)
	var open []printing
	for _, b := range bandai {
		i := settledBy(prints, b)
		if i < 0 {
			open = append(open, b)
			continue
		}
		out.Arts[i].Same = append(out.Arts[i].Same, Key{Bandai, b.Key})
		twinned[i] = true
	}
	taken := twins(out.Arts[:heads], twinned, open, gridFor)
	headsKnown := true
	for _, a := range out.Arts[:heads] {
		if _, ok := gridFor(a.Head().Key); !ok {
			headsKnown = false
		}
	}
	for j, b := range open {
		if taken[j] {
			continue
		}
		k := Key{Bandai, b.Key}
		g, ok := gridFor(k)
		if !ok {
			if !final {
				out.Pending++
			}
			continue
		}
		name := bandaiName(b.Rarity, b.Product)
		v := Version{Key: k, Name: name, Short: shortName(name, b.Product), Product: b.Product}
		if i, ok := closest(out.Arts, g, gridFor); ok {
			out.Arts[i].Versions = append(out.Arts[i].Versions, v)
			continue
		}
		// A Limitless picture not compared yet may be this one.
		if !final && !headsKnown {
			out.Pending++
			continue
		}
		out.Arts = append(out.Arts, Art{Versions: []Version{v}})
	}
	return out
}

// settledBy is the Limitless printing that b is the same printing as, by key and product, or
// -1. Keys are unique on each site, so a Limitless printing settles at most one.
func settledBy(prints []cards.Print, b printing) int {
	return slices.IndexFunc(prints, func(p cards.Print) bool { return p.Key == b.Key && sameProduct(p.Set, b.Product) })
}

// unsettled is the card's Bandai printings that need their picture compared.
func unsettled(card cards.Card, bandai []printing) []printing {
	prints := printsOf(card)
	var out []printing
	for _, b := range bandai {
		if settledBy(prints, b) < 0 {
			out = append(out, b)
		}
	}
	return out
}

// twins finds the Bandai copy of each Limitless printing among the open printings by picture,
// preferring the same product, then the closest. A Bandai printing twins at most one head. It
// reports which open printings it took.
func twins(heads []Art, twinned []bool, open []printing, gridFor func(Key) (grid, bool)) []bool {
	type pair struct {
		head, open int
		same       bool
		d          float64
	}
	var pairs []pair
	for i, a := range heads {
		hg, ok := gridFor(a.Head().Key)
		if twinned[i] || !ok {
			continue
		}
		for j, b := range open {
			bg, ok := gridFor(Key{Bandai, b.Key})
			if !ok {
				continue
			}
			if d := distance(hg, bg); d <= matchMax {
				pairs = append(pairs, pair{i, j, sameProduct(a.Head().Product, b.Product), d})
			}
		}
	}
	slices.SortStableFunc(pairs, func(x, y pair) int {
		if x.same != y.same {
			if x.same {
				return -1
			}
			return 1
		}
		return cmp.Compare(x.d, y.d)
	})
	taken := make([]bool, len(open))
	for _, p := range pairs {
		if twinned[p.head] || taken[p.open] {
			continue
		}
		heads[p.head].Same = append(heads[p.head].Same, Key{Bandai, open[p.open].Key})
		twinned[p.head], taken[p.open] = true, true
	}
	return taken
}

// closest is the art with the member closest to g, when one is within matchMax.
func closest(arts []Art, g grid, gridFor func(Key) (grid, bool)) (int, bool) {
	best, bestD := -1, float64(matchMax)
	for i, a := range arts {
		for _, m := range a.members() {
			mg, ok := gridFor(m)
			if !ok {
				continue
			}
			if d := distance(g, mg); d < bestD || (d == bestD && best < 0) {
				best, bestD = i, d
			}
		}
	}
	return best, best >= 0
}

// match is the version want shows: the closest version or Same printing within matchMax, with
// a Same printing answered as its head.
func match(ca CardArts, want grid, gridFor func(Key) (grid, bool)) (Key, bool) {
	var best Key
	bestD, found := float64(matchMax), false
	consider := func(k, answer Key) {
		g, ok := gridFor(k)
		if !ok {
			return
		}
		if d := distance(want, g); d < bestD || (d == bestD && !found) {
			best, bestD, found = answer, d, true
		}
	}
	for _, a := range ca.Arts {
		for _, v := range a.Versions {
			consider(v.Key, v.Key)
		}
		for _, k := range a.Same {
			consider(k, a.Head().Key)
		}
	}
	return best, found
}

var (
	productCode = regexp.MustCompile(`\[[^\]]*\]`)
	// Bandai's newer pages put the kind of product first, as in "BOOSTER PACK -ROYAL BLOOD- [OP-10]".
	productTitle = regexp.MustCompile(`^(?:[A-Z][A-Z ]*[A-Z] )?-(.+)-$`)
	// Limitless names some decks by their kind, "Ultra Deck: The Three Brothers" and "EX Gear 5",
	// where Bandai has "-The Three Brothers-" and "-GEAR5-".
	setKind  = regexp.MustCompile(`^(?:Ultra Deck: |EX )`)
	notAlnum = regexp.MustCompile(`[^a-z0-9]+`)
)

// sameProduct compares Limitless's set name with Bandai's product line, both cut to lowercase
// letters and digits. "500 Years in the Future" and "-500 YEARS IN THE FUTURE- [OP-07]" match.
// A miss only costs a download.
func sameProduct(set, product string) bool {
	p := strings.TrimSpace(productCode.ReplaceAllString(product, ""))
	if m := productTitle.FindStringSubmatch(p); m != nil {
		p = m[1]
	}
	s := squash(setKind.ReplaceAllString(set, ""))
	return s != "" && s == squash(p)
}

func squash(s string) string { return notAlnum.ReplaceAllString(strings.ToLower(s), "") }
