package arts

import (
	"context"
	"image"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/VTGare/logpose/internal/cards"
)

// Pictures reads a printing's full image, downloading it when it isn't kept. A read for a grid
// never counts as using the image, so comparing never pushes out cards that were opened. It
// fails when art downloads are off.
type Pictures interface {
	Picture(k Key) (image.Image, error)
}

type Config struct {
	CatalogPath string
	GridDir     string
	Pictures    Pictures
	Client      *http.Client
	// Site is Bandai's card list, a test server in specs.
	Site  string
	Pause time.Duration
}

// Gallery is the only owner of Bandai's printings and the picture grids.
type Gallery struct {
	cfg   Config
	grids *gridStore

	// refreshMu lets one Refresh run at a time, the only writer of bandai.json.
	refreshMu sync.Mutex

	mu sync.Mutex
	// cat is replaced whole after a refresh, never changed in place.
	cat *catalog
}

// Open reads bandai.json. A missing file is an empty catalog, and a bad one is an error the
// app logs before going on with Limitless only.
func Open(cfg Config) (*Gallery, error) {
	cat, err := loadCatalog(cfg.CatalogPath)
	if err != nil {
		return nil, err
	}
	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}
	return &Gallery{cfg: cfg, grids: newGridStore(cfg.GridDir), cat: cat}, nil
}

func (g *Gallery) catalog() *catalog {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.cat
}

// Stale reports whether the list hasn't been fully read in pageMaxAge. A nil Gallery is
// never stale.
func (g *Gallery) Stale(now time.Time) bool {
	if g == nil {
		return false
	}
	return now.Sub(g.catalog().Checked) > pageMaxAge
}

// Refresh reads the pages that are due, with Pause between two, and stops when ctx ends. Every
// page names every series, so the first page read also refreshes the list: /cardlist/ (the
// newest booster) when nothing was read yet, else the promotion page, which is always due.
// Pages read before an error or a cancel are kept, and Checked moves only when all were read.
// It reports whether Bandai's printings changed.
func (g *Gallery) Refresh(ctx context.Context, now time.Time, progress func(done, total int)) (bool, error) {
	g.refreshMu.Lock()
	defer g.refreshMu.Unlock()
	cat := g.catalog().clone()
	first := promotionID
	if len(cat.Series) == 0 {
		first = ""
	}
	read := 0
	changed := false
	err := func() error {
		page, err := cards.FetchBandaiPage(ctx, g.cfg.Client, g.cfg.Site, first)
		if err != nil {
			return err
		}
		ch, err := cat.store(page, now)
		if err != nil {
			return err
		}
		changed, read = ch, 1
		due := slices.DeleteFunc(cat.due(now), func(id string) bool { return id == page.Shown || id == first })
		total := 1 + len(due)
		if progress != nil {
			progress(read, total)
		}
		for _, id := range due {
			if err := pause(ctx, g.cfg.Pause); err != nil {
				return err
			}
			page, err := cards.FetchBandaiPage(ctx, g.cfg.Client, g.cfg.Site, id)
			if err != nil {
				return err
			}
			ch, err := cat.store(page, now)
			if err != nil {
				return err
			}
			changed = changed || ch
			read++
			if progress != nil {
				progress(read, total)
			}
		}
		cat.Checked = now
		return nil
	}()
	if read == 0 {
		return false, err
	}
	if serr := cat.save(g.cfg.CatalogPath); serr != nil && err == nil {
		err = serr
	}
	g.mu.Lock()
	g.cat = cat
	g.mu.Unlock()
	return changed, err
}

func pause(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
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

// Arts groups the card's printings and shows each step: at once with the grids known, then
// after each new grid, and last with nothing pending. Only Bandai printings that sameProduct
// doesn't settle need pictures, and the Limitless printings they may repeat. A nil Gallery
// shows LimitlessArts once.
func (g *Gallery) Arts(ctx context.Context, card cards.Card, show func(CardArts)) {
	if g == nil {
		show(LimitlessArts(card))
		return
	}
	bandai := g.catalog().forCard(card.Code)
	need := g.need(card, bandai, false)
	if len(need) == 0 {
		show(group(card, bandai, g.grids.known, true))
		return
	}
	show(group(card, bandai, g.grids.known, false))
	made := make(chan error, len(need))
	for _, k := range need {
		go func() {
			_, err := g.grids.get(ctx, k, g.cfg.Pictures)
			made <- err
		}()
	}
	for range need {
		select {
		case <-ctx.Done():
			return
		case err := <-made:
			if err == nil {
				show(group(card, bandai, g.grids.known, false))
			}
		}
	}
	show(group(card, bandai, g.grids.known, true))
}

// Match is the version the image shows: the closest version or Same printing at <= matchMax,
// with a Same printing answered as its head. It makes the grids it needs, so the first match
// for a card with Bandai printings downloads them.
func (g *Gallery) Match(ctx context.Context, card cards.Card, img image.Image) (Key, bool) {
	if g == nil {
		return Key{}, false
	}
	bandai := g.catalog().forCard(card.Code)
	var wg sync.WaitGroup
	for _, k := range g.need(card, bandai, true) {
		wg.Go(func() { _, _ = g.grids.get(ctx, k, g.cfg.Pictures) })
	}
	wg.Wait()
	if ctx.Err() != nil {
		return Key{}, false
	}
	return match(group(card, bandai, g.grids.known, true), gridOf(img), g.grids.known)
}

// need is the keys without a grid that grouping compares: the Limitless printings, when a
// Bandai printing needs comparing or always is true, then those Bandai printings.
func (g *Gallery) need(card cards.Card, bandai []printing, always bool) []Key {
	open := unsettled(card, bandai)
	var out []Key
	if len(open) > 0 || always {
		for _, p := range printsOf(card) {
			if k := (Key{Limitless, p.Key}); !g.has(k) {
				out = append(out, k)
			}
		}
	}
	for _, b := range open {
		if k := (Key{Bandai, b.Key}); !g.has(k) {
			out = append(out, k)
		}
	}
	return out
}

func (g *Gallery) has(k Key) bool {
	_, ok := g.grids.known(k)
	return ok
}
