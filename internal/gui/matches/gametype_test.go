package matches

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/stats"
)

var _ = Describe("gameType", func() {
	It("names the ranked room by its letter, and the sim's format for casual games", func() {
		var m stats.Match
		m.Ranked, m.Lobby.Code = true, "EABCDEF"
		Expect(GameType(m)).To(Equal("Ranked · Extra Regulation"))
		m.Lobby.Code = "WABCDEF"
		Expect(GameType(m)).To(Equal("Ranked · Standard"))
		m.Lobby.Code = "UABCDEF"
		Expect(GameType(m)).To(Equal("Ranked"))
		m.Ranked, m.Lobby.Code, m.SimFormat = false, "QWERTY", "Spoiler"
		Expect(GameType(m)).To(Equal("Casual · Spoiler"))
	})
})
