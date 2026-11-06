package simlog

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LifeByTurn", func() {
	It("gives each player's life at the start and at the end of every turn", func() {
		lines := append(lobby("XNJ8M66"),
			lifeCheck(1, 0),
			lifeCheck(1, 5),
			lifeCheck(2, 5),
			by(alice, "Hand before Mulligan: [OP12-023,EB05-001,EB05-001,OP07-022,ST32-002]"),
			by(alice, "Draw 1 Don"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			lifeCheck(1, 4),
			by(bob, "End Turn"),
			by(alice, "Draw 1 Card"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			lifeCheck(1, 3),
			lifeCheck(2, 4),
			by(bob, "Concedes!"),
		)
		g := parse(logOf(lines...))[0]
		Expect(g.LifeByTurn("Alice#1111")).To(Equal([]int{5, 5, 4, 4, 3}))
		Expect(g.LifeByTurn("Bob#2222")).To(Equal([]int{5, 5, 5, 5, 4}))
	})

	It("has nothing for a player who isn't in the game", func() {
		g := parse(logOf(append(lobby("XNJ8M66"), opening()...)...))[0]
		Expect(g.LifeByTurn("Nobody#0")).To(BeNil())
	})
})
