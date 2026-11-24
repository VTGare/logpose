package page

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"
)

var _ = Describe("sim art helpers", func() {
	card := cards.Card{
		Code: "OP01-024", Name: "Monkey D. Luffy",
		Prints: []cards.Print{{Key: "OP01-024"}, {Key: "OP01-024_p1", Kind: "aa"}},
	}
	ca := arts.CardArts{Arts: []arts.Art{
		{
			Versions: []arts.Version{{Key: arts.Key{Source: arts.Limitless, Print: "OP01-024"}, Name: "Base"}},
			Same:     []arts.Key{{Source: arts.Bandai, Print: "OP01-024"}},
		},
		{
			Versions: []arts.Version{{Key: arts.Key{Source: arts.Limitless, Print: "OP01-024_p1"}, Name: "Alt art"}},
		},
	}}

	It("resolves mirrored sim art for right-click actions off the Cards page only", func() {
		printInfo := SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024_p1"}}
		fileInfo := SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtFile}, OwnFile: `C:\sim\OP01-024_OVERRIDE.png`}

		e := &Env{Page: Cards}
		Expect(shownArtKey(e, card, printInfo, true)).To(Equal("OP01-024"))

		e.Page = Decks
		Expect(shownArtKey(e, card, printInfo, true)).To(Equal("OP01-024_p1"))
		Expect(shownArtKey(e, card, fileInfo, true)).To(Equal(arts.FileRef(fileInfo.OwnFile).String()))

		e.Prefs = PrefsFrom(map[string]string{KeySimArtOff: "1"})
		Expect(shownArtKey(e, card, printInfo, true)).To(Equal("OP01-024"))
	})

	It("formats sim art status labels and prose consistently", func() {
		label, changed := SimArtStatus(ca, SimArtInfo{}, false)
		Expect(label).To(BeEmpty())
		Expect(changed).To(BeFalse())
		Expect(SimArtInSim(ca, SimArtInfo{}, false)).To(BeEmpty())

		own := SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtOwn}}
		label, changed = SimArtStatus(ca, own, true)
		Expect(label).To(Equal("Sim's own"))
		Expect(changed).To(BeFalse())
		Expect(SimArtInSim(ca, own, true)).To(BeEmpty())

		base := SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024_bd"}}
		label, changed = SimArtStatus(ca, base, true)
		Expect(label).To(Equal("Base"))
		Expect(changed).To(BeTrue())
		Expect(SimArtInSim(ca, base, true)).To(Equal("In the sim: the base card"))

		alt := SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024_p1"}}
		label, changed = SimArtStatus(ca, alt, true)
		Expect(label).To(Equal("Alt art"))
		Expect(changed).To(BeTrue())
		Expect(SimArtInSim(ca, alt, true)).To(Equal("In the sim: Alt art"))

		file := SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtFile}, OwnFile: `C:\sim\OP01-024_OVERRIDE.png`}
		label, changed = SimArtStatus(ca, file, true)
		Expect(label).To(Equal("Your file"))
		Expect(changed).To(BeTrue())
		Expect(SimArtInSim(ca, file, true)).To(Equal("In the sim: your own file"))

		unknown := SimArtInfo{Cur: sim.ArtChoice{Kind: sim.ArtPrint, Key: "OP01-024_p99"}}
		label, changed = SimArtStatus(ca, unknown, true)
		Expect(label).To(Equal("Other art"))
		Expect(changed).To(BeTrue())
		Expect(SimArtInSim(ca, unknown, true)).To(Equal("In the sim: other art"))
	})

	It("resolves viewed art keys and counts choices", func() {
		info := SimArtInfo{OwnFile: `C:\sim\OP01-024_OVERRIDE.png`}
		Expect(ViewedArtKey(card, info, ca, "")).To(Equal("OP01-024"))
		Expect(ViewedArtKey(card, info, ca, "OP01-024_bd")).To(Equal("OP01-024"))
		Expect(ViewedArtKey(card, info, ca, "OP01-024_p1")).To(Equal("OP01-024_p1"))
		Expect(ViewedArtKey(card, info, ca, arts.SimOwnRef("OP01-024").String())).To(Equal("sim:OP01-024"))
		Expect(ViewedArtKey(card, info, ca, arts.FileRef(info.OwnFile).String())).To(Equal("file:" + info.OwnFile))
		Expect(ViewedArtKey(card, info, ca, "missing")).To(Equal("OP01-024"))

		Expect(ArtCount(ca, SimArtInfo{}, false)).To(Equal(2))
		Expect(ArtCount(ca, SimArtInfo{NoSim: true}, true)).To(Equal(2))
		Expect(ArtCount(ca, SimArtInfo{}, true)).To(Equal(3))
		Expect(ArtCount(ca, info, true)).To(Equal(4))
	})
})
