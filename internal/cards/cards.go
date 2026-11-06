package cards

import (
	"encoding/json"
	"maps"
	"regexp"
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
			db.cards[c.Number] = Card{Code: c.Number, Name: cleanName(c.Name), Type: c.CardType, Colors: colors(c.Color)}
		}
	}
	return db, nil
}

func cleanName(n string) string {
	n = nameSuffix.ReplaceAllString(n, "")
	n = strings.ReplaceAll(n, ".D.", " D. ")
	return dottedName.ReplaceAllString(n, "$1. $2")
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
