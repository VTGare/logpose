package gui

import (
	"github.com/egoist/mygo/ui"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("card art", func() {
	It("draws a card from the smallest image that covers it at the scaling the sizes assume", func() {
		Expect(cardKind(22)).To(Equal(artFull))
		Expect(cardKind(webSmallWidth / artScale)).To(Equal(artFull))
		Expect(cardKind(webSmallWidth/artScale + 1)).To(Equal(artCard))
		Expect(cardKind(cardArtWidth / artScale)).To(Equal(artCard))
		Expect(cardKind(cardArtWidth/artScale + 1)).To(Equal(artLarge))
	})

	It("keeps every mat it fits inside the box", func() {
		for _, box := range [][2]float32{{900, 700}, {1400, 900}, {700, 1200}} {
			m := fitMat(ui.Rect{W: box[0], H: box[1]})
			Expect(m.W).To(BeNumerically("<=", box[0]))
			Expect(m.H).To(BeNumerically("<=", box[1]))
			Expect(newMat(m.w+1).W > box[0] || newMat(m.w+1).H > box[1]).To(BeTrue())
		}
	})
})
