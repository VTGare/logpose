package cards

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/VTGare/logpose/internal/fetch"
)

// TCGCSVBase is tcgcsv.com's mirror of TCGplayer's catalog for category 68, One Piece. It
// lists the sets ("groups"), and each set's products carry the card number, type and color.
const TCGCSVBase = "https://tcgcsv.com/tcgplayer/68"

// Catalog is the cached copy of tcgcsv's cards. Groups holds each set's modifiedOn, so a
// refresh only fetches sets that changed.
type Catalog struct {
	Version int                    `json:"version,omitempty"`
	Fetched time.Time              `json:"fetched"`
	Groups  map[string]string      `json:"groups"`
	Cards   map[string]catalogCard `json:"cards"`
}

type catalogCard struct {
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	Colors []Color `json:"colors"`
	Power  int     `json:"power,omitempty"`
}

// catalogVersion goes up when the catalog keeps more of each card. A catalog saved by an
// older version fetches every set again, and keeps its cards until then.
const catalogVersion = 2

// catalogMaxAge is how often the catalog is refreshed. tcgcsv updates once a day.
const catalogMaxAge = 24 * time.Hour

// LoadCatalog reads the cached catalog, or returns an empty one when there's none yet or the
// file doesn't parse.
func LoadCatalog(path string) (*Catalog, error) {
	var c Catalog
	found, err := fetch.LoadJSON(path, &c)
	if err != nil {
		return nil, err
	}
	if !found {
		return &Catalog{Version: catalogVersion, Groups: make(map[string]string), Cards: make(map[string]catalogCard)}, nil
	}
	if c.Groups == nil || c.Version < catalogVersion {
		c.Groups = make(map[string]string)
		c.Fetched = time.Time{}
		c.Version = catalogVersion
	}
	if c.Cards == nil {
		c.Cards = make(map[string]catalogCard)
	}
	return &c, nil
}

func (c *Catalog) Save(path string) error { return fetch.SaveJSON(path, c) }

func (c *Catalog) Stale(now time.Time) bool { return now.Sub(c.Fetched) > catalogMaxAge }

// Apply adds the catalog's cards to db, over the ones db has. A catalog saved before it kept
// power has none, so a card keeps the power db has.
func (c *Catalog) Apply(db *DB) {
	for code, cc := range c.Cards {
		p := cc.Power
		if p == 0 {
			p = db.cards[code].Power
		}
		db.cards[code] = Card{Code: code, Name: cc.Name, Type: cc.Type, Colors: cc.Colors, Power: p}
	}
}

type tcgGroups struct {
	Results []struct {
		GroupID    int    `json:"groupId"`
		ModifiedOn string `json:"modifiedOn"`
	} `json:"results"`
}

type tcgProducts struct {
	Results []struct {
		Name         string `json:"name"`
		ExtendedData []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"extendedData"`
	} `json:"results"`
}

// RefreshCatalog brings the catalog at path up to date from base when it's stale or force is
// set, and saves it. A refresh that fails after fetching sets still saves them.
func RefreshCatalog(ctx context.Context, client *http.Client, base, path string, now time.Time, force bool) (Refreshed, error) {
	c, err := LoadCatalog(path)
	if err != nil || (!force && !c.Stale(now)) {
		return Refreshed{}, err
	}
	n, err := c.Refresh(ctx, client, base, now)
	return saveRefreshed(c, path, n, len(c.Cards), err)
}

// Refresh fetches the sets that changed since the catalog last saw them. It returns how many
// sets it fetched.
func (c *Catalog) Refresh(ctx context.Context, client *http.Client, base string, now time.Time) (int, error) {
	var groups tcgGroups
	if err := getJSON(ctx, client, base+"/groups", &groups); err != nil {
		return 0, err
	}
	fetched := 0
	for _, g := range groups.Results {
		id := fmt.Sprint(g.GroupID)
		if c.Groups[id] == g.ModifiedOn {
			continue
		}
		var products tcgProducts
		if err := getJSON(ctx, client, fmt.Sprintf("%s/%d/products", base, g.GroupID), &products); err != nil {
			return fetched, err
		}
		c.addProducts(products)
		c.Groups[id] = g.ModifiedOn
		fetched++
	}
	c.Fetched = now
	return fetched, nil
}

func (c *Catalog) addProducts(p tcgProducts) {
	for _, r := range p.Results {
		fields := make(map[string]string, len(r.ExtendedData))
		for _, e := range r.ExtendedData {
			fields[e.Name] = e.Value
		}
		code := fields["Number"]
		if code == "" || fields["CardType"] == "" {
			continue
		}
		// Alternate arts share a number. The first printing's name has the fewest extras.
		if old, ok := c.Cards[code]; ok && len(old.Name) <= len(CleanName(r.Name)) {
			if old.Power == 0 {
				old.Power = power(fields["Power"])
				c.Cards[code] = old
			}
			continue
		}
		c.Cards[code] = catalogCard{
			Name: CleanName(r.Name), Type: fields["CardType"], Colors: colors(fields["Color"]), Power: power(fields["Power"]),
		}
	}
}

func getJSON(ctx context.Context, client *http.Client, url string, v any) error {
	body, err := fetch.Get(ctx, client, url, 64<<20)
	if err != nil {
		return fmt.Errorf("cards: %w", err)
	}
	return json.Unmarshal(body, v)
}
