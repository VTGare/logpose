package gui

import (
	"image"
	"image/draw"

	// Registers the JPEG decoder for the sim's card images.
	_ "image/jpeg"
	"os"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/sim"
)

type artKind int

const (
	// artFull is the whole card, from the sim's small image.
	artFull artKind = iota
	// artPortrait is the art area of the full-size card, for banners.
	artPortrait
)

type artKey struct {
	code string
	kind artKind
}

// artCache decodes card images on goroutines and keeps them as bitmaps. Bitmaps are made on the
// main thread, through redraw, since views use them there.
type artCache struct {
	install *sim.Install
	redraw  func(func())

	mu      sync.Mutex
	images  map[artKey]image.Image
	loading map[artKey]bool
	bitmaps map[artKey]*ui.Bitmap
}

func newArtCache(install *sim.Install, redraw func(func())) *artCache {
	return &artCache{
		install: install,
		redraw:  redraw,
		images:  make(map[artKey]image.Image),
		loading: make(map[artKey]bool),
		bitmaps: make(map[artKey]*ui.Bitmap),
	}
}

// get returns the card's bitmap, or nil while it loads or when the sim has no image for it.
// It runs on the main thread.
func (a *artCache) get(code string, kind artKind) *ui.Bitmap {
	if a == nil || a.install == nil || code == "" {
		return nil
	}
	k := artKey{code, kind}
	if b, ok := a.bitmaps[k]; ok {
		return b
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if img, ok := a.images[k]; ok {
		var b *ui.Bitmap
		if img != nil {
			b = ui.NewBitmap(img)
		}
		a.bitmaps[k] = b
		delete(a.images, k)
		return b
	}
	if !a.loading[k] {
		a.loading[k] = true
		go a.load(k)
	}
	return nil
}

func (a *artCache) load(k artKey) {
	img, err := decode(a.install.CardImage(k.code, k.kind == artFull))
	if err == nil && k.kind == artPortrait {
		img = portrait(img)
	}
	if err != nil {
		img = nil
	}
	a.mu.Lock()
	a.images[k] = img
	a.mu.Unlock()
	a.redraw(func() {})
}

func decode(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	img, _, err := image.Decode(f)
	return img, err
}

// portrait crops a card to its art: the top of the card, under the cost and above the text box.
func portrait(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	crop := image.Rect(b.Min.X+w*6/100, b.Min.Y+h*9/100, b.Max.X-w*6/100, b.Min.Y+h*58/100)
	out := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	draw.Draw(out, out.Bounds(), img, crop.Min, draw.Src)
	return out
}
