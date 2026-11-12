package cards

import (
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Color string

const (
	Red    Color = "Red"
	Green  Color = "Green"
	Blue   Color = "Blue"
	Purple Color = "Purple"
	Black  Color = "Black"
	Yellow Color = "Yellow"
)

type Card struct {
	Code   string
	Name   string
	Type   string
	Colors []Color
	// Power is the printed power, or 0 for cards without one or not in the data.
	Power int

	// The rest comes only from Limitless. Detailed is set on cards that have it.
	Detailed bool
	// Cost is -1 on leaders, which have Life instead. IsLeader tells them apart.
	Cost       int
	Life       int
	Counter    int
	Attributes []string
	Traits     []string
	// Effect is the rules text, one ability per line, the trigger included.
	Effect string
	// Block is the newest block icon among the card's printings, "1" to "5" or "X", and empty
	// when the data has none. A reprint with a newer icon, or a manga printing with X, keeps
	// the card out of rotation.
	Block    string
	Legality Legality
	// BannedWith lists the cards this one can't share a deck with.
	BannedWith []string
	Rarity     string
	Set        string
	Prints     []Print
}

// Print is one printing of a card. Key names its image, the code with "_p1" and so on for
// the alternate ones.
type Print struct {
	Key    string
	Block  string
	Kind   string
	Rarity string
	Artist string
	Set    string
}

type Legality string

const (
	Legal      Legality = "legal"
	Rotated    Legality = "rotated"
	Banned     Legality = "banned"
	Restricted Legality = "restricted"
	// Unreleased cards are from a set that isn't out yet. Sets come out worldwide at about the
	// same time.
	Unreleased Legality = "unreleased"
)

// All returns the cards that have the full data, ordered by code.
func (db *DB) All() []Card {
	var out []Card
	for _, c := range db.cards {
		if c.Detailed {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b Card) int { return strings.Compare(a.Code, b.Code) })
	return out
}

// DB looks cards up by code. A card it doesn't know has only the name the logs gave it, or its
// code.
type DB struct {
	cards map[string]Card
}

func NewDB() *DB { return &DB{cards: make(map[string]Card)} }

func (db *DB) Card(code string) Card {
	if c, ok := db.cards[code]; ok {
		return c
	}
	return Card{Code: code, Name: code}
}

// Name records the name the sim's logs use for a card. It replaces the catalog's, whose
// spelling differs ("Monkey.D.Luffy"), and covers cards from sets newer than the catalog.
func (db *DB) Name(code, name string) {
	c := db.cards[code]
	c.Code, c.Name = code, name
	db.cards[code] = c
}

func (db *DB) Len() int { return len(db.cards) }

func (db *DB) Clone() *DB { return &DB{cards: maps.Clone(db.cards)} }

// packPath is OPBounty's bundled card database, TCGplayer's catalog as tcgcsv.com serves it.
const packPath = "Resourcers/Cards.json"

// LoadPack reads the card database from the OPBounty client the sim ships.
func LoadPack(pck string) (*DB, error) {
	data, err := readPackFile(pck, packPath)
	if err != nil {
		return nil, err
	}
	return parseCatalog(data)
}

type catalog struct {
	Groups []struct {
		Cards []struct {
			Number   string `json:"Number"`
			Name     string `json:"Name"`
			CardType string `json:"CardType"`
			Color    string `json:"Color"`
			Power    string `json:"Power"`
		} `json:"cards"`
	} `json:"groups"`
}

// TCGplayer names tell printings apart with a suffix in parentheses, as in "Buggy (042)" or
// "Monkey.D.Luffy - OP13-001 (Ichiban Kuji Exclusive)".
var (
	nameSuffix = regexp.MustCompile(`\s*(?:-\s*[A-Z0-9]+-\d+\s*)?\([^)]*\)$`)
	dottedName = regexp.MustCompile(`([A-Z])\.([A-Z])`)
)

func parseCatalog(data []byte) (*DB, error) {
	var cat catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, err
	}
	db := NewDB()
	for _, g := range cat.Groups {
		for _, c := range g.Cards {
			if c.Number == "" {
				continue
			}
			if _, seen := db.cards[c.Number]; seen {
				continue
			}
			db.cards[c.Number] = Card{
				Code: c.Number, Name: CleanName(c.Name), Type: c.CardType, Colors: colors(c.Color), Power: power(c.Power),
			}
		}
	}
	return db, nil
}

// CleanName writes a name the way the sim's logs do: "Monkey.D.Luffy" becomes "Monkey D. Luffy".
func CleanName(n string) string {
	n = nameSuffix.ReplaceAllString(n, "")
	n = strings.ReplaceAll(n, ".D.", " D. ")
	return dottedName.ReplaceAllString(n, "$1. $2")
}

// power reads a printed power, which the data writes as text and leaves empty or "-" on cards
// without one.
func power(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func colors(s string) []Color {
	var out []Color
	for _, c := range strings.Split(s, ";") {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, Color(c))
		}
	}
	return out
}
