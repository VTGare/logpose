package cards

import (
	"cmp"
	"encoding/json"
	"errors"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
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

// Card types as the card data writes them.
const (
	Leader    = "Leader"
	Character = "Character"
	Event     = "Event"
	Stage     = "Stage"
)

func (c Card) IsLeader() bool { return c.Type == Leader }

// HasPower is false for events, stages and cards without data. Some characters print 0 power.
func (c Card) HasPower() bool { return c.Type == Character || c.Type == Leader }

// PaidCost is the DON!! the card takes to play, 0 for a leader, whose Cost is -1.
func (c Card) PaidCost() int { return max(c.Cost, 0) }

// PrintKeyPattern is a printing's key: the card's code, with _p1 and so on for the alternate
// printings. Limitless and Bandai write their keys the same way.
const PrintKeyPattern = `[A-Z0-9]+-\d+(?:_p\d+)?`

var printKey = regexp.MustCompile(`^` + PrintKeyPattern + `$`)

// IsPrintKey reports whether s is a printing's key, such as "OP07-021" or "OP07-021_p1". A
// Bandai reprint's "_r1" isn't one.
func IsPrintKey(s string) bool { return printKey.MatchString(s) }

// SetOf is a card code's set, "OP13" for OP13-001. It works on a printing's key too.
func SetOf(code string) string {
	set, _, _ := strings.Cut(code, "-")
	return set
}

// SetKinds are the kinds of set, newest kind first: main boosters, extra boosters, premium
// boosters, starter decks and promos.
var SetKinds = []string{"OP", "EB", "PRB", "ST", "P"}

// SplitSet splits a set's code into its kind and number, "OP13" into "OP" and 13. ok is false
// for a set without a number, such as the promos' "P", which is all kind.
func SplitSet(set string) (kind string, n int, ok bool) {
	i := strings.IndexAny(set, "0123456789")
	switch {
	case i < 0:
		return set, 0, false
	case i == 0:
		return "", 0, false
	}
	n, err := strconv.Atoi(set[i:])
	return set[:i], n, err == nil
}

// CompareCodes puts the newest sets first: the kinds in SetKinds order, each by number from the
// highest, and the cards in a set by number.
func CompareCodes(a, b string) int {
	sa, na := splitCode(a)
	sb, nb := splitCode(b)
	ka, ia, _ := SplitSet(sa)
	kb, ib, _ := SplitSet(sb)
	return cmp.Or(cmp.Compare(kindRank(ka), kindRank(kb)), -cmp.Compare(ia, ib), cmp.Compare(sa, sb), cmp.Compare(na, nb))
}

func splitCode(code string) (set string, num int) {
	set, n, _ := strings.Cut(code, "-")
	num, _ = strconv.Atoi(n)
	return set, num
}

func kindRank(kind string) int {
	if i := slices.Index(SetKinds, kind); i >= 0 {
		return i
	}
	return len(SetKinds)
}

// Loaded is the card database with when its caches were last brought up to date.
type Loaded struct {
	DB *DB
	// Fetched is when the catalog was refreshed, and Checked when the library was.
	Fetched, Checked time.Time
}

// Load layers the card data. pack, OPBounty's catalog from LoadPack, goes first, then the cached
// tcgcsv catalog, which has newer sets, then the Limitless library, which has everything. Names
// from the logs go over all of it with DB.Name. pack can be nil and a path empty. A file that
// doesn't load is left out, and its error returned with what the others give.
func Load(pack *DB, catalogPath, libraryPath string) (Loaded, error) {
	l := Loaded{DB: NewDB()}
	if pack != nil {
		l.DB = pack.Clone()
	}
	var errs []error
	if catalogPath != "" {
		cat, err := LoadCatalog(catalogPath)
		if err == nil {
			cat.Apply(l.DB)
			l.Fetched = cat.Fetched
		}
		errs = append(errs, err)
	}
	if libraryPath != "" {
		lib, err := LoadLibrary(libraryPath)
		if err == nil {
			lib.Apply(l.DB)
			l.Checked = lib.Checked
		}
		errs = append(errs, err)
	}
	return l, errors.Join(errs...)
}

// Refreshed is what a cache refresh did. Saved is false when the cache was fresh, or when the
// refresh failed before fetching anything.
type Refreshed struct {
	Saved bool
	// Sets is how many set pages were fetched, and Cards how many cards the cache holds.
	Sets, Cards int
}

func saveRefreshed(c interface{ Save(path string) error }, path string, sets, cards int, err error) (Refreshed, error) {
	if sets == 0 && err != nil {
		return Refreshed{}, err
	}
	if serr := c.Save(path); serr != nil {
		return Refreshed{Sets: sets, Cards: cards}, errors.Join(err, serr)
	}
	return Refreshed{Saved: true, Sets: sets, Cards: cards}, err
}
