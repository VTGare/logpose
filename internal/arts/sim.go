package arts

import (
	"context"
	"fmt"
	"image"
	"sync"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"
)

// SimStatus is what the sim shows for a card after inspection.
type SimStatus struct {
	NoSim   bool
	Cur     sim.ArtChoice
	OwnFile string
	Err     error
}

type matchKey struct {
	code string
	hash string
}

// SimManager coordinates sim art overrides, perceptual matching of user files, and picture
// resolution across the sim install and web store.
type SimManager struct {
	store   *Store
	gallery *Gallery
	simDir  string

	mu     sync.Mutex
	simArt *sim.SimArt
	// matched holds (card code, override SHA-256) pairs already compared against printings.
	matched map[matchKey]bool
}

func NewSimManager(install *sim.Install, simDir string, store *Store, gallery *Gallery) *SimManager {
	m := &SimManager{
		store:   store,
		gallery: gallery,
		simDir:  simDir,
	}
	m.SetInstall(install)
	return m
}

func (m *SimManager) SetInstall(in *sim.Install) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if in == nil {
		m.simArt = nil
		m.matched = nil
		return
	}
	if m.simArt != nil && m.simArt.Install.Root == in.Root {
		return
	}
	m.simArt = &sim.SimArt{Install: *in, Dir: m.simDir}
	m.matched = nil
}

// HasSource reports whether either a sim install or a web store is available to resolve art.
func (m *SimManager) HasSource() bool {
	m.mu.Lock()
	hasInstall := m.simArt != nil
	m.mu.Unlock()
	return hasInstall || m.store != nil
}

func (m *SimManager) installCopy() *sim.Install {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.simArt == nil {
		return nil
	}
	cp := m.simArt.Install
	return &cp
}

func (m *SimManager) simArtObj() *sim.SimArt {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.simArt
}

func (m *SimManager) claimMatch(sa *sim.SimArt, code, hash string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.simArt != sa {
		return false
	}
	if m.matched == nil {
		m.matched = make(map[matchKey]bool)
	}
	k := matchKey{code: code, hash: hash}
	if m.matched[k] {
		return false
	}
	m.matched[k] = true
	return true
}

func (m *SimManager) unclaimMatch(sa *sim.SimArt, code, hash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.simArt != sa {
		return
	}
	delete(m.matched, matchKey{code: code, hash: hash})
}

// Inspect reads what the sim shows for card. An unmatched user override is compared against the
// card's printings once per file hash so printings show under their printing key.
func (m *SimManager) Inspect(ctx context.Context, card cards.Card) SimStatus {
	sa := m.simArtObj()
	if sa == nil {
		return SimStatus{NoSim: true}
	}
	st, err := sa.Inspect(card.Code)
	if err != nil {
		return SimStatus{Err: err}
	}
	cur, ownFile := st.Choice, st.OwnFile
	if cur.Kind == sim.ArtFile && st.Override != "" && m.gallery != nil && m.claimMatch(sa, card.Code, st.OverrideHash) {
		if img, err := decode(st.Override); err == nil {
			key, ok := m.gallery.Match(ctx, card, img)
			if ctx.Err() != nil {
				m.unclaimMatch(sa, card.Code, st.OverrideHash)
			} else if ok && sa.RecordMatch(st.OverrideHash, key.String()) == nil {
				cur = sim.ArtChoice{Kind: sim.ArtPrint, Key: key.String()}
				ownFile = ""
			}
		}
	}
	return SimStatus{Cur: cur, OwnFile: ownFile}
}

// Use applies choice to card in the sim and returns the updated status.
func (m *SimManager) Use(ctx context.Context, card cards.Card, choice sim.ArtChoice) (SimStatus, error) {
	err := m.apply(card.Code, choice)
	return m.Inspect(ctx, card), err
}

func (m *SimManager) apply(code string, choice sim.ArtChoice) error {
	sa := m.simArtObj()
	if sa == nil {
		return sim.ErrNotFound
	}
	switch choice.Kind {
	case sim.ArtOwn:
		return sa.UseOwn(code)
	case sim.ArtFile:
		return sa.UseFile(code)
	case sim.ArtPrint:
		if m.store == nil {
			return ErrNoPicture
		}
		img, err := readWeb(m.store, choice.Key, false)
		if err != nil {
			return fmt.Errorf("download %s: %w", choice.Key, err)
		}
		return sa.UsePrint(code, choice.Key, img)
	default:
		return fmt.Errorf("arts: unknown art choice %d", choice.Kind)
	}
}

func (m *SimManager) Scan() []string {
	sa := m.simArtObj()
	if sa == nil {
		return nil
	}
	return sa.Install.Overrides()
}

func (m *SimManager) Resolve(ref Ref, size Size, mirror, simFallback bool) (image.Image, error) {
	return resolve(m.installCopy(), m.store, ref, size, mirror, simFallback)
}
