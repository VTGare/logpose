package arts

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"sync"
)

// A grid samples a card on 16×22 cells, about a card's shape, one pixel per cell centre.
// matchMax was measured with exactly this sampling.
const gridW, gridH = 16, 22

// gridDir is the folder grids go in, inside the art folder. Grid files carry no version, so
// a change to gridW, gridH or gridOf needs a new name.
const gridDir = "grids-1"

type grid [gridW * gridH * 3]uint8

// matchMax is the largest distance at which two pictures are the same art. Stamp versions of
// one art measured 102 to 530 apart, one printing on both sites 145 to 500, and different
// pictures 2,211 and up.
const matchMax = 800

func gridOf(img image.Image) grid {
	var g grid
	b := img.Bounds()
	i := 0
	for y := range gridH {
		for x := range gridW {
			r, gr, bl, _ := img.At(b.Min.X+(x*2+1)*b.Dx()/(gridW*2), b.Min.Y+(y*2+1)*b.Dy()/(gridH*2)).RGBA()
			g[i], g[i+1], g[i+2] = uint8(r>>8), uint8(gr>>8), uint8(bl>>8)
			i += 3
		}
	}
	return g
}

func distance(a, b grid) float64 {
	sum := 0
	for i := range a {
		d := int(a[i]) - int(b[i])
		sum += d * d
	}
	return float64(sum) / float64(len(a))
}

var errNoGrid = errors.New("arts: no picture to compare")

// gridStore keeps one file per key, <dir>/<key>.grid. A grid is written once, through a temp
// file and a rename, so two writers write the same bytes and a crash leaves no half file.
type gridStore struct {
	dir string

	mu       sync.Mutex
	mem      map[Key]grid
	inflight map[Key]chan struct{}
}

func newGridStore(dir string) *gridStore {
	return &gridStore{dir: dir, mem: make(map[Key]grid), inflight: make(map[Key]chan struct{})}
}

func (s *gridStore) path(k Key) string { return filepath.Join(s.dir, k.String()+".grid") }

// known is the key's grid from memory or disk, never the network.
func (s *gridStore) known(k Key) (grid, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.knownLocked(k)
}

func (s *gridStore) knownLocked(k Key) (grid, bool) {
	if g, ok := s.mem[k]; ok {
		return g, true
	}
	if s.dir == "" {
		return grid{}, false
	}
	data, err := os.ReadFile(s.path(k))
	var g grid
	if err != nil || len(data) != len(g) {
		return grid{}, false
	}
	copy(g[:], data)
	s.mem[k] = g
	return g, true
}

// get is the key's grid, made from its picture when no file has it. One caller makes it and
// the others for the same key wait for that.
func (s *gridStore) get(ctx context.Context, k Key, pics Pictures) (grid, error) {
	s.mu.Lock()
	if g, ok := s.knownLocked(k); ok {
		s.mu.Unlock()
		return g, nil
	}
	if wait, ok := s.inflight[k]; ok {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return grid{}, ctx.Err()
		case <-wait:
		}
		if g, ok := s.known(k); ok {
			return g, nil
		}
		return grid{}, errNoGrid
	}
	done := make(chan struct{})
	s.inflight[k] = done
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.inflight, k)
		s.mu.Unlock()
		close(done)
	}()

	if err := ctx.Err(); err != nil {
		return grid{}, err
	}
	if pics == nil {
		return grid{}, errNoGrid
	}
	img, err := pics.Picture(k)
	if err != nil {
		return grid{}, err
	}
	g := gridOf(img)
	// A grid that can't be written is made again next run, which only costs a download.
	_ = s.write(k, g)
	s.mu.Lock()
	s.mem[k] = g
	s.mu.Unlock()
	return g, nil
}

func (s *gridStore) write(k Key, g grid) error {
	if s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, "grid-*")
	if err != nil {
		return err
	}
	_, err = f.Write(g[:])
	if err = errors.Join(err, f.Close()); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	if err := os.Rename(f.Name(), s.path(k)); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	return nil
}
