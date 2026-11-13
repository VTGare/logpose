package matches

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/gui/page"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("card art", func() {
	It("draws a card from the smallest image that covers it at the scaling the sizes assume", func() {
		Expect(page.CardKind(22)).To(Equal(page.ArtFull))
		Expect(page.CardKind(page.WebSmallWidth / page.ArtScale)).To(Equal(page.ArtFull))
		Expect(page.CardKind(page.WebSmallWidth/page.ArtScale + 1)).To(Equal(page.ArtCard))
		Expect(page.CardKind(page.CardArtWidth / page.ArtScale)).To(Equal(page.ArtCard))
		Expect(page.CardKind(page.CardArtWidth/page.ArtScale + 1)).To(Equal(page.ArtLarge))
	})

	It("keeps every mat it fits inside the box", func() {
		for _, box := range [][2]float32{{900, 700}, {1400, 900}, {700, 1200}} {
			m := fitMat(ui.Rect{W: box[0], H: box[1]})
			Expect(m.W).To(BeNumerically("<=", box[0]))
			Expect(m.H).To(BeNumerically("<=", box[1]))
			Expect(newMat(m.W+1).W > box[0] || newMat(m.W+1).H > box[1]).To(BeTrue())
		}
	})
})
