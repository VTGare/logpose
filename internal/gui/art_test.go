package gui

import (
	"image"
	"image/color"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

var _ = Describe("artRef", func() {
	It("reads back every kind of picture from its string form", func() {
		for _, s := range []string{"OP07-021", "OP07-021_p3_bd", simOwnRef("OP07-021"), fileRef(`C:\\art\\zoro.jpg`), donCode, donBackCode} {
			Expect(parseArtRef(s).String()).To(Equal(s))
		}
	})

	It("knows the card a picture shows and names its saved file", func() {
		Expect(parseArtRef("OP07-021_p3_bd").card()).To(Equal("OP07-021"))
		Expect(parseArtRef(simOwnRef("OP07-021")).card()).To(Equal("OP07-021"))
		Expect(parseArtRef(fileRef("/art/zoro.jpg")).card()).To(BeEmpty())
		Expect(parseArtRef(simOwnRef("OP07-021")).fileName()).To(Equal("OP07-021_sim.png"))
		Expect(parseArtRef(fileRef("/art/zoro.jpg")).fileName()).To(Equal("zoro.png"))
		Expect(parseArtRef("OP07-021_p1").fileName()).To(Equal("OP07-021_p1.png"))
	})
})
