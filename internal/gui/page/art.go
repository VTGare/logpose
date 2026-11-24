package page

import (
	"image"
	"image/draw"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
)

type artKind int

const (
	// ArtFull is the whole card, from the web thumb at webSmallWidth or the sim's small image.
	ArtFull artKind = iota
	// artPortrait is the art area of the full-size card, for banners, with its right edge
	// faded out so it melts into the backdrop.
	artPortrait
	// artCrop is the top of the art of the full-size card, for deck thumbnails.
	artCrop
	// ArtLarge is the whole full-size card, for the preview of a small one.
	ArtLarge
	// ArtCard is the whole card scaled to cardArtWidth, for cards drawn larger than artFull.
	ArtCard
	// ArtFaceMine and ArtFaceTheirs are the art area for the two ends of the match face-off,
	// fading out toward the middle.
	ArtFaceMine
	ArtFaceTheirs
)

func (k artKind) size() arts.Size {
	switch k {
	case ArtFull:
		return arts.SizeSmall
	case ArtCard:
		return arts.SizeThumb
	default:
		return arts.SizeFull
	}
}

// WebSmallWidth is how wide a Limitless image is kept for artFull, the size small cards and
// thumbnails use. Limitless only has 600 px images.
const WebSmallWidth = arts.SmallWidth

// CardArtWidth keeps cards sharp up to about 190 DIPs wide at 125% scaling, at a third of the
// memory of a full image.
const CardArtWidth = 240

// artCard is scaled from the thumb, and scaling only shrinks. This fails to compile when the
// thumb is narrower than cardArtWidth.
const _ = uint(arts.ThumbWidth - CardArtWidth)

type artKey struct {
	code string
	kind artKind
}

// ArtCache decodes card images on goroutines and keeps them as bitmaps. Bitmaps are made on the
// main thread, through redraw, since views use them there.
type ArtCache struct {
	mgr    *arts.SimManager
	redraw func(func())

	mu      sync.Mutex
	mirror  bool
	ver     map[string]int
	images  map[artKey]image.Image
	loading map[artKey]bool
	bitmaps map[artKey]*ui.Bitmap
}

func NewArtCache(mgr *arts.SimManager, redraw func(func())) *ArtCache {
	return &ArtCache{
		mgr:     mgr,
		redraw:  redraw,
		ver:     make(map[string]int),
		images:  make(map[artKey]image.Image),
		loading: make(map[artKey]bool),
		bitmaps: make(map[artKey]*ui.Bitmap),
	}
}

// SetMirror controls whether plain card codes use the sim's _OVERRIDE art when one exists.
func (a *ArtCache) SetMirror(on bool) {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.mirror = on
	a.mu.Unlock()
}

// Invalidate drops every cached size of code so the next frame reloads it. Runs on the main thread.
func (a *ArtCache) Invalidate(code string) {
	if a == nil || code == "" {
		return
	}
	for k := range a.bitmaps {
		if k.code == code {
			delete(a.bitmaps, k)
		}
	}
	a.mu.Lock()
	a.ver[code]++
	for k := range a.images {
		if k.code == code {
			delete(a.images, k)
		}
	}
	for k := range a.loading {
		if k.code == code {
			delete(a.loading, k)
		}
	}
	a.mu.Unlock()
	a.redraw(func() {})
}

// Get returns the card's bitmap, or nil while it loads or when there's no image for it. code
// can name an alternate printing, as "OP14-020_p1". It runs on the main thread.
func (a *ArtCache) Get(code string, kind artKind) *ui.Bitmap {
	if a == nil || !a.mgr.HasSource() || code == "" {
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
		ver := a.ver[k.code]
		go a.load(k, ver)
	}
	return nil
}

func (a *ArtCache) load(k artKey, ver int) {
	img, err := a.source(k)
	switch {
	case err != nil:
		img = nil
	case k.kind == artPortrait:
		img = fadeRight(portrait(img), 0.3)
	case k.kind == ArtFaceMine:
		img = fadeRight(portrait(img), faceFade)
	case k.kind == ArtFaceTheirs:
		img = fadeLeft(portrait(img), faceFade)
	case k.kind == artCrop:
		img = topArt(img)
	case k.kind == ArtCard:
		img = arts.ScaleToWidth(img, CardArtWidth)
	}
	a.mu.Lock()
	if a.ver[k.code] != ver {
		a.mu.Unlock()
		return
	}
	a.images[k] = img
	a.mu.Unlock()
	a.redraw(func() {})
}

func (a *ArtCache) source(k artKey) (image.Image, error) {
	a.mu.Lock()
	mirror := a.mirror
	a.mu.Unlock()
	return a.mgr.Resolve(arts.ParseRef(k.code), k.kind.size(), mirror, true)
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

// topArt crops a card to the top of its art, above the "SAMPLE" watermark the sim's full-size
// images carry across the lower art.
func topArt(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	crop := image.Rect(b.Min.X+w*6/100, b.Min.Y+h*10/100, b.Max.X-w*6/100, b.Min.Y+h*40/100)
	out := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	draw.Draw(out, out.Bounds(), img, crop.Min, draw.Src)
	return out
}

// faceFade is how much of the face-off art fades, so the result in the middle sits on the
// panel.
const faceFade = 0.6

// fadeLeft fades the left part of an image, as a fraction of its width, to transparent.
func fadeLeft(img image.Image, part float64) *image.NRGBA {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	end := float64(b.Dx()) * part
	for x := 0; x < int(end); x++ {
		t := 1 - float64(x)/end
		alpha := uint8(255 * (1 - t*t))
		for y := range b.Dy() {
			out.Pix[out.PixOffset(x, y)+3] = alpha
		}
	}
	return out
}

// fadeRight fades the right part of an image, as a fraction of its width, to transparent.
func fadeRight(img image.Image, part float64) *image.NRGBA {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	start := float64(b.Dx()) * (1 - part)
	for x := int(start); x < b.Dx(); x++ {
		t := (float64(x) - start) / (float64(b.Dx()) - start)
		alpha := uint8(255 * (1 - t*t))
		for y := range b.Dy() {
			out.Pix[out.PixOffset(x, y)+3] = alpha
		}
	}
	return out
}
