package arts

import (
	"cmp"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/VTGare/logpose/internal/cards"
)

// catalog is Bandai's printings as last read, stored as bandai.json. Each series keeps its own
// page's printings, so reading a page again replaces exactly what that page said.
type catalog struct {
	Version int                `json:"version"`
	Checked time.Time          `json:"checked"`
	Series  map[string]*series `json:"series"`

	byCode map[string][]printing
}

type series struct {
	Name string `json:"name"`
	// Seen is when the series first showed in the list, zero for the ones on the first read
	// other than the newest of each family.
	Seen    time.Time  `json:"seen,omitzero"`
	Fetched time.Time  `json:"fetched,omitzero"`
	Prints  []printing `json:"prints"`
}

// printing is a Bandai printing logpose offers. Reprints (_rN) repeat the base art and are
// dropped when a page is stored.
type printing struct {
	Key     string `json:"key"`
	Code    string `json:"code"`
	Rarity  string `json:"rarity"`
	Product string `json:"product"`
}

const (
	catalogVersion = 1
	promotionID    = "569901"
	otherID        = "569801"
	// pageMaxAge is how old a page read daily may be before it is read again.
	pageMaxAge = 20 * time.Hour
	// recentFor is how long a new series is read daily. Older pages are taken not to gain
	// printings.
	recentFor = 60 * 24 * time.Hour
)

func newCatalog() *catalog {
	c := &catalog{Version: catalogVersion, Series: make(map[string]*series)}
	c.index()
	return c
}

func loadCatalog(path string) (*catalog, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return newCatalog(), nil
	}
	if err != nil {
		return nil, err
	}
	c := newCatalog()
	// The file is a cache of pages that can be read again, so one that doesn't parse starts over.
	if err := json.Unmarshal(data, c); err != nil || c.Version != catalogVersion || c.Series == nil {
		return newCatalog(), nil
	}
	c.index()
	return c, nil
}

func (c *catalog) save(path string) error {
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

// clone copies the catalog, so a refresh fills its own while readers keep the last one. A
// series is replaced whole when stored, so the copies share no series that changes.
func (c *catalog) clone() *catalog {
	out := &catalog{Version: c.Version, Checked: c.Checked, Series: make(map[string]*series, len(c.Series))}
	for id, s := range c.Series {
		cp := *s
		out.Series[id] = &cp
	}
	out.index()
	return out
}

var errBadPage = errors.New("arts: Bandai's card list page lists no series or no printings")

// store replaces a series' printings with a page's and updates the series list it names. A
// series new to a list that was read before gets Seen = now. On the first read, the newest
// id of each family (5690xx starters, 5691xx boosters, 5692xx extras, 5693xx premiums) gets
// Seen = now and the rest stay zero. It reports whether the series' printings changed.
func (c *catalog) store(page cards.BandaiPage, now time.Time) (bool, error) {
	// An empty read means the page changed shape, and storing it would drop every printing
	// the series had.
	if len(page.Series) == 0 || len(page.Prints) == 0 || page.Shown == "" {
		return false, errBadPage
	}
	first := len(c.Series) == 0
	newest := make(map[string]string)
	for _, s := range page.Series {
		family := s.ID[:min(len(s.ID), 4)]
		newest[family] = max(newest[family], s.ID)
	}
	for _, s := range page.Series {
		if cur, ok := c.Series[s.ID]; ok {
			cur.Name = s.Name
			continue
		}
		ns := &series{Name: s.Name}
		if !first || newest[s.ID[:min(len(s.ID), 4)]] == s.ID {
			ns.Seen = now
		}
		c.Series[s.ID] = ns
	}
	var prints []printing
	for _, p := range page.Prints {
		// A site's own key has the same shape on both sites, which leaves out reprints.
		if !limitlessKey.MatchString(p.Key) {
			continue
		}
		prints = append(prints, printing{Key: p.Key, Code: p.Code, Rarity: p.Rarity, Product: p.Product})
	}
	shown, ok := c.Series[page.Shown]
	if !ok {
		shown = &series{Seen: now}
	}
	changed := !slices.Equal(shown.Prints, prints)
	ns := *shown
	ns.Fetched, ns.Prints = now, prints
	c.Series[page.Shown] = &ns
	c.index()
	return changed, nil
}

// due is the series to read now: never read, or promotion, other products and recent series
// older than pageMaxAge. The newest ids come first.
func (c *catalog) due(now time.Time) []string {
	var out []string
	for id, s := range c.Series {
		daily := id == promotionID || id == otherID || (!s.Seen.IsZero() && now.Sub(s.Seen) < recentFor)
		if s.Fetched.IsZero() || (daily && now.Sub(s.Fetched) > pageMaxAge) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	slices.Reverse(out)
	return out
}

func (c *catalog) index() {
	c.byCode = make(map[string][]printing)
	for _, id := range slices.Sorted(maps.Keys(c.Series)) {
		for _, p := range c.Series[id].Prints {
			c.byCode[p.Code] = append(c.byCode[p.Code], p)
		}
	}
	for code, ps := range c.byCode {
		slices.SortStableFunc(ps, func(a, b printing) int {
			return cmp.Or(cmp.Compare(len(a.Key), len(b.Key)), cmp.Compare(a.Key, b.Key))
		})
		c.byCode[code] = slices.CompactFunc(ps, func(a, b printing) bool { return a.Key == b.Key })
	}
}

// forCard is a card's Bandai printings, by key length then key, each key once.
func (c *catalog) forCard(code string) []printing { return c.byCode[code] }
