package page

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"
)

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

var _ = Describe("art", func() {
	It("fades the right part of the portrait out and leaves the rest", func() {
		out := fadeRight(solid(100, 10, color.RGBA{200, 0, 0, 255}), 0.3)
		alpha := func(x int) uint8 { return out.NRGBAAt(x, 5).A }
		Expect(alpha(0)).To(Equal(uint8(255)))
		Expect(alpha(69)).To(Equal(uint8(255)))
		Expect(alpha(85)).To(BeNumerically("<", 255))
		Expect(alpha(85)).To(BeNumerically(">", alpha(95)))
		Expect(alpha(99)).To(BeNumerically("<", 30))
	})

	It("makes a small dark backdrop the banner's shape", func() {
		out := backdrop(solid(400, 300, color.RGBA{220, 220, 220, 255}))
		Expect(out.Bounds()).To(Equal(image.Rect(0, 0, backdropW, backdropH)))
		left, right := out.RGBAAt(0, 20), out.RGBAAt(backdropW-1, 20)
		Expect(left.R).To(BeNumerically("<", 110))
		Expect(right.R).To(BeNumerically(">", left.R))
	})

	It("blurs a sharp edge into a ramp", func() {
		img := solid(40, 4, color.RGBA{0, 0, 0, 255})
		for y := range 4 {
			for x := 20; x < 40; x++ {
				img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
		boxBlur(img, 3)
		Expect(img.RGBAAt(0, 2).R).To(BeZero())
		Expect(img.RGBAAt(39, 2).R).To(Equal(uint8(255)))
		mid := img.RGBAAt(19, 2).R
		Expect(mid).To(BeNumerically(">", 0))
		Expect(mid).To(BeNumerically("<", 255))
	})
})

var _ = Describe("ArtCache", func() {
	It("mirrors sim overrides for plain codes while keeping ExactPrintRef on base art", func() {
		root := GinkgoT().TempDir()
		cardDir := filepath.Join(root, "OPTCGSim_Data", "StreamingAssets", "Cards", "OP13")
		Expect(os.MkdirAll(cardDir, 0o755)).To(Succeed())

		writePNG := func(name string, col color.RGBA) {
			f, err := os.Create(filepath.Join(cardDir, name))
			Expect(err).NotTo(HaveOccurred())
			Expect(png.Encode(f, solid(10, 10, col))).To(Succeed())
			Expect(f.Close()).To(Succeed())
		}
		writePNG("OP13-001.png", color.RGBA{200, 0, 0, 255})
		writePNG("OP13-001_OVERRIDE.png", color.RGBA{0, 200, 0, 255})

		mgr := arts.NewSimManager(&sim.Install{Root: root}, GinkgoT().TempDir(), nil, nil)
		cache := NewArtCache(mgr, func(fn func()) { fn() })
		cache.SetMirror(true)

		img, err := cache.source(artKey{code: "OP13-001", kind: ArtLarge})
		Expect(err).NotTo(HaveOccurred())
		r, g, _, _ := img.At(5, 5).RGBA()
		Expect(g).To(BeNumerically(">", r))

		exact, err := cache.source(artKey{code: arts.ExactPrintRef("OP13-001").String(), kind: ArtLarge})
		Expect(err).NotTo(HaveOccurred())
		er, eg, _, _ := exact.At(5, 5).RGBA()
		Expect(er).To(BeNumerically(">", eg))

		cache.SetMirror(false)
		plain, err := cache.source(artKey{code: "OP13-001", kind: ArtLarge})
		Expect(err).NotTo(HaveOccurred())
		pr, pg, _, _ := plain.At(5, 5).RGBA()
		Expect(pr).To(BeNumerically(">", pg))
	})

	It("scales mirrored ArtFull and caps huge mirrored ArtLarge overrides, falling back on corrupt PNGs", func() {
		root := GinkgoT().TempDir()
		cardDir := filepath.Join(root, "OPTCGSim_Data", "StreamingAssets", "Cards", "OP13")
		Expect(os.MkdirAll(cardDir, 0o755)).To(Succeed())

		writePNG := func(name string, w, h int, col color.RGBA) {
			f, err := os.Create(filepath.Join(cardDir, name))
			Expect(err).NotTo(HaveOccurred())
			Expect(png.Encode(f, solid(w, h, col))).To(Succeed())
			Expect(f.Close()).To(Succeed())
		}
		writePNG("OP13-001.png", 100, 140, color.RGBA{200, 0, 0, 255})
		writePNG("OP13-001_OVERRIDE.png", 1200, 1680, color.RGBA{0, 200, 0, 255})
		writePNG("OP13-002.png", 100, 140, color.RGBA{200, 0, 0, 255})
		Expect(os.WriteFile(filepath.Join(cardDir, "OP13-002_OVERRIDE.png"), []byte("not a png"), 0o600)).To(Succeed())

		mgr := arts.NewSimManager(&sim.Install{Root: root}, GinkgoT().TempDir(), nil, nil)
		cache := NewArtCache(mgr, func(fn func()) { fn() })
		cache.SetMirror(true)

		fullImg, err := cache.source(artKey{code: "OP13-001", kind: ArtFull})
		Expect(err).NotTo(HaveOccurred())
		Expect(fullImg.Bounds().Dx()).To(Equal(WebSmallWidth))

		largeImg, err := cache.source(artKey{code: "OP13-001", kind: ArtLarge})
		Expect(err).NotTo(HaveOccurred())
		Expect(largeImg.Bounds().Dx()).To(Equal(arts.LargeWidth))

		fallback, err := cache.source(artKey{code: "OP13-002", kind: ArtLarge})
		Expect(err).NotTo(HaveOccurred())
		r, g, _, _ := fallback.At(5, 5).RGBA()
		Expect(r).To(BeNumerically(">", g))
	})

	It("invalidates every kind for a card, keeps other cards, and drops stale in-flight loads", func() {
		root := GinkgoT().TempDir()
		cardDir := filepath.Join(root, "OPTCGSim_Data", "StreamingAssets", "Cards", "OP13")
		Expect(os.MkdirAll(cardDir, 0o755)).To(Succeed())
		f, err := os.Create(filepath.Join(cardDir, "OP13-001.png"))
		Expect(err).NotTo(HaveOccurred())
		Expect(png.Encode(f, solid(20, 28, color.RGBA{200, 0, 0, 255}))).To(Succeed())
		Expect(f.Close()).To(Succeed())

		mgr := arts.NewSimManager(&sim.Install{Root: root}, GinkgoT().TempDir(), nil, nil)
		cache := NewArtCache(mgr, func(fn func()) { fn() })
		kFull := artKey{code: "OP13-001", kind: ArtFull}
		kLarge := artKey{code: "OP13-001", kind: ArtLarge}
		kOther := artKey{code: "OP13-002", kind: ArtFull}
		cache.images[kFull] = solid(10, 10, color.RGBA{1, 0, 0, 255})
		cache.images[kLarge] = solid(10, 10, color.RGBA{1, 0, 0, 255})
		cache.images[kOther] = solid(10, 10, color.RGBA{0, 0, 1, 255})
		cache.loading[kFull] = true

		cache.Invalidate("OP13-001")
		Expect(cache.images).NotTo(HaveKey(kFull))
		Expect(cache.images).NotTo(HaveKey(kLarge))
		Expect(cache.loading).NotTo(HaveKey(kFull))
		Expect(cache.images).To(HaveKey(kOther))

		// A load started with the pre-invalidation version must not repopulate the cache.
		cache.load(kFull, 0)
		Expect(cache.images).NotTo(HaveKey(kFull))

		// SimArtState.Done invalidates the card both on success and on write failure.
		e := &Env{Art: cache}
		card := cards.Card{Code: "OP13-001", Name: "Luffy"}
		cache.images[kFull] = solid(10, 10, color.RGBA{1, 0, 0, 255})
		e.SimArt.Done(e, card, sim.ArtChoice{Kind: sim.ArtOwn}, SimArtInfo{}, sim.ArtChoice{Kind: sim.ArtOwn}, errors.New("disk full"))
		Expect(cache.images).NotTo(HaveKey(kFull))

		cache.images[kFull] = solid(10, 10, color.RGBA{1, 0, 0, 255})
		e.SimArt.Done(e, card, sim.ArtChoice{Kind: sim.ArtOwn}, SimArtInfo{}, sim.ArtChoice{Kind: sim.ArtOwn}, nil)
		Expect(cache.images).NotTo(HaveKey(kFull))
	})
})
