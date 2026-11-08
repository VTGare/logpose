package arts

import (
	"regexp"
	"strings"

	"github.com/VTGare/logpose/internal/cards"
)

// limitlessName names a printing from Limitless's kind and rarity. Bandai prints SP cards and
// new art for older cards in later sets, so a printing without a kind is a reprint only when
// its artist is the base card's.
func limitlessName(card cards.Card, p cards.Print) string {
	switch p.Kind {
	case "aa":
		if p.Rarity == "Treasure Rare" {
			return "Treasure"
		}
		return "Alt art"
	case "manga":
		return "Manga"
	case "fa":
		return "Full art"
	case "wanted":
		return "Wanted"
	case "jr":
		return "Pirate foil"
	case "tf":
		return "Textured"
	case "panda":
		return "Panda"
	case "":
	default:
		return p.Rarity
	}
	switch {
	case p.Key == card.Code || !strings.Contains(p.Key, "_"):
		return "Base"
	case p.Rarity == "Special Card":
		return "SP"
	case p.Rarity == "Treasure Rare":
		return "Treasure"
	case p.Artist != "" && p.Artist != baseArtist(card):
		return "Alt art"
	}
	return "Reprint"
}

func baseArtist(card cards.Card) string {
	for _, p := range card.Prints {
		if p.Key == card.Code {
			return p.Artist
		}
	}
	return ""
}

// bandaiNames name a Bandai printing by its product. Bandai marks no kind, and the first that
// matches wins.
var bandaiNames = []struct{ part, name string }{
	{"Regional", "Regional"},
	{"Treasure Cup", "Treasure Cup"},
	{"Premium", "Premium"},
	{"Anniversary", "Anniversary"},
	{"Championship", "Championship"},
}

func bandaiName(rarity, product string) string {
	if rarity == "SP CARD" {
		return "SP"
	}
	for _, n := range bandaiNames {
		if strings.Contains(product, n.part) {
			return n.name
		}
	}
	return "Promo"
}

var (
	regionalRole  = regexp.MustCompile(`\b(Participation|Finalist|Champion)\b`)
	regionalRoles = map[string]string{"Participation": "participant", "Finalist": "finalist", "Champion": "champion"}
)

// shortName is a version's line in the version list, above its product. The six Regional
// prizes share one name, so their line says which prize each is: "Offline finalist".
func shortName(name, product string) string {
	if name != "Regional" {
		return name
	}
	m := regionalRole.FindStringSubmatch(product)
	if m == nil {
		return name
	}
	role := regionalRoles[m[1]]
	for _, where := range []string{"Offline", "Online"} {
		if strings.Contains(product, where) {
			return where + " " + role
		}
	}
	return strings.ToUpper(role[:1]) + role[1:]
}
