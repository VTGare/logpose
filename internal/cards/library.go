package cards

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Library is the cached copy of every card from Limitless and of Bandai's ban list.
type Library struct {
	Version int `json:"version"`
	// Checked is when the set index was last read.
	Checked time.Time `json:"checked"`
	// Sets holds when each set page was fetched, by its Limitless name.
	Sets map[string]time.Time `json:"sets"`
	// Unreleased holds the sets that had cards not out yet when last fetched. Their pages
	// still gain cards and change, so they're fetched again daily until they're out.
	Unreleased map[string]bool `json:"unreleased,omitempty"`
	Cards      map[string]Card `json:"cards"`
	// Rules is nil until Bandai's list has been read once.
	Rules        *Restrictions `json:"rules,omitempty"`
	RulesFetched time.Time     `json:"rulesFetched,omitzero"`
}

// libraryVersion goes up when the same pages would be read into different cards. A library
// saved by an older version fetches every set again, and keeps its cards until then.
const libraryVersion = 6

// latestMaxAge is how old a set the index names as updated may be before it's fetched again.
const latestMaxAge = 20 * time.Hour

func newLibrary() *Library {
	return &Library{
		Version: libraryVersion, Sets: make(map[string]time.Time), Unreleased: make(map[string]bool), Cards: make(map[string]Card),
	}
}

// LoadLibrary reads the cached library, or returns an empty one when there's none yet.
func LoadLibrary(path string) (*Library, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return newLibrary(), nil
	}
	if err != nil {
		return nil, err
	}
	l := newLibrary()
	if err := json.Unmarshal(data, l); err != nil {
		return nil, err
	}
	if l.Unreleased == nil {
		l.Unreleased = make(map[string]bool)
	}
	if l.Version < libraryVersion {
		l.Sets = make(map[string]time.Time)
		l.Checked = time.Time{}
		l.Version = libraryVersion
	}
	return l, nil
}

func (l *Library) Save(path string) error {
	data, err := json.Marshal(l)
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

// Stale reports whether the library was last checked more than maxAge ago.
func (l *Library) Stale(now time.Time, maxAge time.Duration) bool {
	return now.Sub(l.Checked) > maxAge
}

// Source is where a refresh reads from.
type Source struct {
	Client    *http.Client
	Limitless string
	Rules     string
	// Pause is the wait between two pages, to go easy on Limitless.
	Pause time.Duration
	// Progress, when set, hears of each set page read: how many of how many, and the set.
	Progress func(SetProgress)
}

// SetProgress is a refresh's progress after a set page.
type SetProgress struct {
	Done, Total int
	// Set is the set's code and name, as "OP18 The Dominance of God", and Cards how many
	// cards its page has.
	Set   string
	Cards int
}

// Refresh reads Limitless's set index, fetches the sets it hasn't seen, the ones the index
// names as updated and the ones not out yet, then reads Bandai's ban list. It returns how many
// set pages it fetched. What it fetched stays in l when it fails partway, so saving keeps the
// progress.
func (l *Library) Refresh(ctx context.Context, src Source, now time.Time) (int, error) {
	body, err := getPage(ctx, src.Client, src.Limitless+"/cards")
	if err != nil {
		return 0, err
	}
	idx, err := ParseSetIndex(bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	var due []string
	for _, slug := range idx.Sets {
		at, seen := l.Sets[slug]
		changing := slices.Contains(idx.Latest, slug) || l.Unreleased[slug]
		if !seen || (changing && now.Sub(at) > latestMaxAge) {
			due = append(due, slug)
		}
	}
	fetched := 0
	for _, slug := range due {
		if fetched > 0 {
			if err := wait(ctx, src.Pause); err != nil {
				return fetched, err
			}
		}
		page, err := getPage(ctx, src.Client, src.Limitless+"/cards/"+slug+"?display=full")
		if err != nil {
			return fetched, err
		}
		prints, err := parseSetPage(bytes.NewReader(page))
		if err != nil {
			return fetched, fmt.Errorf("cards: %s: %w", slug, err)
		}
		l.merge(prints)
		l.Sets[slug] = now
		// A page with no cards yet is an announced set, which fills in later.
		l.Unreleased[slug] = len(prints) == 0 || slices.ContainsFunc(prints, func(p limitlessPrint) bool {
			return limitlessLegality(p.standard, p.extra) == Unreleased
		})
		fetched++
		if src.Progress != nil {
			src.Progress(SetProgress{Done: fetched, Total: len(due), Set: setTitle(prints), Cards: countCards(prints)})
		}
	}
	l.Checked = now
	if err := l.refreshRules(ctx, src, now); err != nil {
		return fetched, err
	}
	return fetched, nil
}

func (l *Library) refreshRules(ctx context.Context, src Source, now time.Time) error {
	if src.Rules == "" {
		return nil
	}
	body, err := getPage(ctx, src.Client, src.Rules)
	if err != nil {
		return err
	}
	r, err := ParseRestrictions(bytes.NewReader(body))
	if err != nil {
		return err
	}
	// The list has never been empty. An empty read means the page changed shape, and
	// trusting it would lift every ban.
	if len(r.Banned) == 0 {
		return errors.New("cards: no banned cards on Bandai's page")
	}
	l.Rules, l.RulesFetched = &r, now
	return nil
}

// merge adds a set page's printings. A card's data comes from its first printing in its own
// set, OP01-024's from OP01, since reprints elsewhere name their own set and rarity. Until that
// one is seen, the first printing read stands in.
func (l *Library) merge(prints []limitlessPrint) {
	for _, p := range prints {
		code := p.card.Code
		set, _, _ := strings.Cut(code, "-")
		ownSet := p.setCode == set
		original := p.print.Key == code && ownSet
		c, ok := l.Cards[code]
		if !ok || original {
			keep := c.Prints
			c = p.card
			c.Prints = keep
			c.Legality = limitlessLegality(p.standard, p.extra)
		}
		i := slices.IndexFunc(c.Prints, func(x Print) bool { return x.Key == p.print.Key })
		switch {
		case i < 0:
			c.Prints = append(c.Prints, p.print)
			slices.SortFunc(c.Prints, func(a, b Print) int {
				return cmp.Or(cmp.Compare(len(a.Key), len(b.Key)), cmp.Compare(a.Key, b.Key))
			})
		case ownSet, c.Prints[i].Block == "" && p.print.Block != "":
			// A printing is read from its own set's page when it can be. Reprint pages leave
			// out the block, and a cache from before blocks were kept has none.
			c.Prints[i] = p.print
		}
		c.Block = newestBlock(c.Prints)
		l.Cards[code] = c
	}
}

// limitlessLegality reads a card's Standard and Extra badges. A rotated card is still legal in
// Extra, and one from a set that isn't out yet is legal in neither.
// newestBlock is the newest block icon among the printings, X above all numbers.
func newestBlock(prints []Print) string {
	best := ""
	for _, p := range prints {
		if blockRank(p.Block) > blockRank(best) {
			best = p.Block
		}
	}
	return best
}

func blockRank(b string) int {
	if b == "X" {
		return 100
	}
	n, _ := strconv.Atoi(b)
	return n
}

// setTitle names the set a page shows, from the set most of its printings name.
func setTitle(prints []limitlessPrint) string {
	n := make(map[string]int)
	best := ""
	for _, p := range prints {
		t := strings.TrimSpace(p.setCode + " " + p.print.Set)
		n[t]++
		if n[t] > n[best] {
			best = t
		}
	}
	return best
}

func countCards(prints []limitlessPrint) int {
	seen := make(map[string]bool)
	for _, p := range prints {
		seen[p.card.Code] = true
	}
	return len(seen)
}

func limitlessLegality(standard, extra string) Legality {
	switch standard {
	case "legal":
		return Legal
	case "not-legal":
		if extra == "not-legal" {
			return Unreleased
		}
		return Rotated
	case "banned":
		return Banned
	}
	return ""
}

// Apply adds the library's cards to db, over the ones db has. Bandai's list, once read, decides
// bans and restrictions, and Limitless's legality decides rotation.
func (l *Library) Apply(db *DB) {
	for code, c := range l.Cards {
		if l.Rules != nil {
			switch {
			case slices.Contains(l.Rules.Banned, code):
				c.Legality = Banned
			case slices.Contains(l.Rules.Restricted, code):
				c.Legality = Restricted
			case c.Legality == Banned:
				c.Legality = Legal
			}
			c.BannedWith = nil
			for _, pair := range l.Rules.Pairs {
				if slices.Contains(pair, code) {
					for _, other := range pair {
						if other != code {
							c.BannedWith = append(c.BannedWith, other)
						}
					}
				}
			}
		}
		db.cards[code] = c
	}
}

func getPage(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cards: %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
}

func wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
