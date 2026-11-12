package gui

import (
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("sides", func() {
	match := func(opp string) stats.Match {
		return stats.Match{Me: store.GamePlayer{ID: "Me#1"}, Opp: store.GamePlayer{ID: opp + "#2", Name: opp}}
	}

	It("words each side from whose it is, not from its name", func() {
		m := match("You")
		them := sideOf(m, "You#2")
		Expect(them.mine).To(BeFalse())
		Expect(them.possessive()).To(Equal("You's"))
		Expect(them.color()).To(Equal(oppColor))
		Expect(mySide(m).possessive()).To(Equal("Your"))
	})

	It("says who hit whom from the side that lost life", func() {
		m := match("emog")
		Expect(hitText(m, simlog.Reason{Player: "Me#1", N: 2})).To(Equal("emog hit you for 2"))
		Expect(hitText(m, simlog.Reason{Player: "emog#2", N: 1})).To(Equal("You hit emog for 1"))
	})

	It("takes the turns from who ended them, and from turn order for the turn the game ended in", func() {
		m := match("emog")
		g := &simlog.Game{
			Players: []simlog.Player{{Name: "Me", Tag: "1"}, {Name: "emog", Tag: "2"}},
			First:   "emog#2",
			Lines: []simlog.Line{
				{Turn: 1, Player: "emog#2", Kind: simlog.LineEndTurn},
				{Turn: 2, Player: "Me#1", Kind: simlog.LineEndTurn},
			},
		}
		var mine []bool
		for i := range 5 {
			mine = append(mine, mineTurn(m, g, i))
		}
		Expect(mine).To(Equal([]bool{false, false, true, false, true}))
		Expect(mineTurn(m, nil, 2)).To(BeFalse())
	})
})
