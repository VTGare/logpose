package page

import (
	"time"

	"github.com/egoist/mygo/ui"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"
)

type copyStub struct {
	copied []string
	toasts []ui.Toast
}

func (*copyStub) SetSetting(string, string)                          {}
func (*copyStub) ApplyPrefs(Prefs)                                   {}
func (*copyStub) After(time.Duration, func())                        {}
func (*copyStub) ChooseSimFolder()                                   {}
func (*copyStub) LoadSimArt(cards.Card)                              {}
func (*copyStub) LoadArts(cards.Card, int)                           {}
func (*copyStub) ScanSimArt()                                        {}
func (*copyStub) UseSimArt(cards.Card, sim.ArtChoice, sim.ArtChoice) {}
func (c *copyStub) CopyText(text string, toast ui.Toast) {
	c.copied = append(c.copied, text)
	c.toasts = append(c.toasts, toast)
}

var _ = Describe("decklist helpers", func() {
	It("formats partial deck notes for singular, plural, zero and full lists", func() {
		Expect(DeckPartialNote(0)).To(BeEmpty())
		Expect(DeckPartialNote(50)).To(BeEmpty())
		Expect(DeckPartialNote(49)).To(Equal("49 of 50 cards · 1 card wasn't named in the log."))
		Expect(DeckPartialNote(48)).To(Equal("48 of 50 cards · 2 cards weren't named in the log."))
	})

	It("caps stack fanning at 4 layers for cards with more than 4 copies", func() {
		w4, h4, fan4 := StackSize(4)
		w8, h8, fan8 := StackSize(8)
		Expect(fan4).To(Equal(4))
		Expect(fan8).To(Equal(4))
		Expect(w8).To(Equal(w4))
		Expect(h8).To(Equal(h4))

		w1, _, fan1 := StackSize(0)
		Expect(fan1).To(Equal(1))
		Expect(w1).To(Equal(float32(StackW)))
	})

	It("queues an error toast instead of copying when the leader or list is empty", func() {
		act := &copyStub{}
		e := &Env{Snap: &Snapshot{Cards: cards.NewDB()}, Act: act}

		CopyDeckForSim(e, "", "Luffy", map[string]int{"OP01-016": 4})
		Expect(act.copied).To(BeEmpty())
		Expect(e.Toast).NotTo(BeNil())
		Expect(e.Toast.Type).To(Equal("error"))

		e.Toast = nil
		CopyDeckForSim(e, "OP13-001", "Luffy", nil)
		Expect(act.copied).To(BeEmpty())
		Expect(e.Toast).NotTo(BeNil())
		Expect(e.Toast.Type).To(Equal("error"))
	})
})
