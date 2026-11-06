package cards

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// TCGCSVBase is tcgcsv.com's mirror of TCGplayer's catalog for category 68, One Piece. It
// lists the sets ("groups"), and each set's products carry the card number, type and color.
const TCGCSVBase = "https://tcgcsv.com/tcgplayer/68"

const userAgent = "logpose (+https://github.com/VTGare/logpose)"

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

func LoadCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Catalog{Version: catalogVersion, Groups: make(map[string]string), Cards: make(map[string]catalogCard)}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
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

func (c *Catalog) Save(path string) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Stale reports whether the catalog is older than maxAge.
func (c *Catalog) Stale(now time.Time, maxAge time.Duration) bool {
	return now.Sub(c.Fetched) > maxAge
}

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

// Refresh fetches the sets that changed since the catalog last saw them. It returns how many
// sets it fetched.
func (c *Catalog) Refresh(ctx context.Context, client *http.Client, base string) (int, error) {
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
	c.Fetched = time.Now()
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
		if old, ok := c.Cards[code]; ok && len(old.Name) <= len(cleanName(r.Name)) {
			if old.Power == 0 {
				old.Power = power(fields["Power"])
				c.Cards[code] = old
			}
			continue
		}
		c.Cards[code] = catalogCard{
			Name: cleanName(r.Name), Type: fields["CardType"], Colors: colors(fields["Color"]), Power: power(fields["Power"]),
		}
	}
}

func getJSON(ctx context.Context, client *http.Client, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cards: %s: %s", url, resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(v)
}
