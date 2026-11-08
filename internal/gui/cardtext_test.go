package gui

import (
	"github.com/egoist/mygo/ui"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("card names in text", func() {
	It("splits a title at card names, the longer name first", func() {
		names := map[string]string{"Monkey D. Luffy": "OP13-001", "Monkey D. Garp": "OP14-002", "Monkey": "X"}
		var got []string
		var codes []string
		for _, p := range namePieces("Monkey D. Luffy attacked Monkey D. Garp", names, ui.Color{}) {
			got = append(got, p.text)
			codes = append(codes, p.code)
		}
		Expect(got).To(Equal([]string{"Monkey D. Luffy", " attacked ", "Monkey D. Garp"}))
		Expect(codes).To(Equal([]string{"OP13-001", "", "OP14-002"}))
	})

	It("leaves text without names whole", func() {
		ps := namePieces("+1 DON", map[string]string{"Kyo": "OP17-001"}, ui.Color{})
		Expect(ps).To(HaveLen(1))
		Expect(ps[0].text).To(Equal("+1 DON"))
	})
})
