package gui

import (
	"image"
	"image/color"
	"image/draw"

	// Registers the JPEG decoder for the sim's card images.
	_ "image/jpeg"
	_ "image/png"
	"os"
	"sync"

	"github.com/egoist/mygo/ui"
	xdraw "golang.org/x/image/draw"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/sim"
)

type artKind int

const (
	// artFull is the whole card, from the web thumb at webSmallWidth or the sim's small image.
	artFull artKind = iota
	// artPortrait is the art area of the full-size card, for banners, with its right edge
	// faded out so it melts into the backdrop.
	artPortrait
	// artBackdrop is the art blurred and darkened, which fills the banner behind everything.
	artBackdrop
	// artCrop is the top of the art of the full-size card, for deck thumbnails.
	artCrop
	// artLarge is the whole full-size card, for the preview of a small one.
	artLarge
	// artCard is the whole card scaled to cardArtWidth, for cards drawn larger than artFull.
	artCard
	// artFaceMine and artFaceTheirs are the art area for the two ends of the match face-off,
	// fading out toward the middle.
	artFaceMine
	artFaceTheirs
)

// fromThumb reports whether the kind is made from a Limitless thumb rather than the full image.
func (k artKind) fromThumb() bool {
	return k == artFull || k == artCard
}

// webSmallWidth is how wide a Limitless image is kept for artFull, the size small cards and
// thumbnails use. Limitless only has 600 px images.
const webSmallWidth = 150

// cardArtWidth keeps cards sharp up to about 190 DIPs wide at 125% scaling, at a third of the
// memory of a full image.
const cardArtWidth = 240

// artCard is scaled from the thumb, and scaling only shrinks. This fails to compile when the
// thumb is narrower than cardArtWidth.
const _ = uint(arts.ThumbWidth - cardArtWidth)

type artKey struct {
	code string
	kind artKind
}

// artCache decodes card images on goroutines and keeps them as bitmaps. Bitmaps are made on the
// main thread, through redraw, since views use them there.
type artCache struct {
	install *sim.Install
	// web is Limitless's and Bandai's art, tried before the sim's. Nil without a folder to keep
	// it in.
	web    *arts.Store
	redraw func(func())

	mu      sync.Mutex
	images  map[artKey]image.Image
	loading map[artKey]bool
	bitmaps map[artKey]*ui.Bitmap
}

func newArtCache(install *sim.Install, web *arts.Store, redraw func(func())) *artCache {
	return &artCache{
		install: install,
		web:     web,
		redraw:  redraw,
		images:  make(map[artKey]image.Image),
		loading: make(map[artKey]bool),
		bitmaps: make(map[artKey]*ui.Bitmap),
	}
}

// get returns the card's bitmap, or nil while it loads or when there's no image for it. code
// can name an alternate printing, as "OP14-020_p1". It runs on the main thread.
func (a *artCache) get(code string, kind artKind) *ui.Bitmap {
	if a == nil || (a.install == nil && a.web == nil) || code == "" {
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
	img, err := a.source(k)
	switch {
	case err != nil:
		img = nil
	case k.kind == artPortrait:
		img = fadeRight(portrait(img), 0.3)
	case k.kind == artBackdrop:
		img = backdrop(portrait(img))
	case k.kind == artFaceMine:
		img = fadeRight(portrait(img), faceFade)
	case k.kind == artFaceTheirs:
		img = fadeLeft(portrait(img), faceFade)
	case k.kind == artCrop:
		img = topArt(img)
	case k.kind == artCard:
		img = arts.ScaleToWidth(img, cardArtWidth)
	}
	a.mu.Lock()
	a.images[k] = img
	a.mu.Unlock()
	a.redraw(func() {})
}

// source is the card's image from Limitless or Bandai, or from the sim when they have none or
// can't be reached.
func (a *artCache) source(k artKey) (image.Image, error) {
	return parseArtRef(k.code).read(a.install, a.web, k.kind, true)
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

// Backdrop size: a small image the banner's shape. Blurred, it stays smooth when the window
// scales it up, and it costs little to make.
const backdropW, backdropH = 288, 45

var backdropDark = color.RGBA{14, 14, 14, 255}

// backdrop blurs the art and darkens it, a little less toward the right, where no text is.
func backdrop(art image.Image) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, backdropW, backdropH))
	cover(out, art)
	for range 3 {
		boxBlur(out, 7)
	}
	for y := range backdropH {
		for x := range backdropW {
			fx := float64(x) / backdropW
			out.SetRGBA(x, y, mix(out.RGBAAt(x, y), backdropDark, 0.62-0.12*fx))
		}
	}
	return out
}

// cover scales src to fill dst, cropping the overflow evenly.
func cover(dst *image.RGBA, src image.Image) {
	sb, db := src.Bounds(), dst.Bounds()
	scale := max(float64(db.Dx())/float64(sb.Dx()), float64(db.Dy())/float64(sb.Dy()))
	cw, ch := int(float64(db.Dx())/scale), int(float64(db.Dy())/scale)
	sx, sy := sb.Min.X+(sb.Dx()-cw)/2, sb.Min.Y+(sb.Dy()-ch)/2
	xdraw.ApproxBiLinear.Scale(dst, db, src, image.Rect(sx, sy, sx+cw, sy+ch), xdraw.Src, nil)
}

// boxBlur averages each pixel with its neighbors r away, across then down. Three passes come
// close to a Gaussian blur.
func boxBlur(img *image.RGBA, r int) {
	b := img.Bounds()
	tmp := image.NewRGBA(b)
	pass := func(src, dst *image.RGBA, dx, dy int) {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				var rr, gg, bb, n int
				for k := -r; k <= r; k++ {
					sx := min(max(x+k*dx, b.Min.X), b.Max.X-1)
					sy := min(max(y+k*dy, b.Min.Y), b.Max.Y-1)
					c := src.RGBAAt(sx, sy)
					rr, gg, bb, n = rr+int(c.R), gg+int(c.G), bb+int(c.B), n+1
				}
				dst.SetRGBA(x, y, color.RGBA{uint8(rr / n), uint8(gg / n), uint8(bb / n), 255})
			}
		}
	}
	pass(img, tmp, 1, 0)
	pass(tmp, img, 0, 1)
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	t = min(max(t, 0), 1)
	l := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}
