package simlog

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func boardCheck(owner, chars, life int) string {
	return fmt.Sprintf("RZ1|CHK|90|%d|30|5|%d|%d|5|5|8|0|1|0", owner, chars, life)
}

func move(owner int, code string, from, to Zone) string {
	return fmt.Sprintf("RZ1|50|%d|%s|%d|0|%d|0|0|0|0|0|0", owner, code, from, to)
}

var _ = Describe("Moments", func() {
	const aliceID, bobID = "Alice#1111", "Bob#2222"
	power := map[string]int{"OP06-033": 6000, "OP14-031": 5000, "EB01-013": 1000, "ST30-004": 2000}
	powerOf := func(code string) int { return power[code] }

	// removals is turn 4, where Bob removes Alice's characters.
	game := func(removals ...string) Game {
		lines := append(lobby("XNJ8M66"),
			boardCheck(1, 0, 5), boardCheck(2, 0, 5),
			by(alice, "Hand before Mulligan: [OP12-023,EB05-001,EB05-001,OP07-022,ST32-002]"),
			by(alice, "Draw 1 Don"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			boardCheck(1, 0, 4),
			by(bob, "End Turn"),
			by(alice, "Draw 1 Card"),
			move(1, "OP06-033", ZoneHand, ZoneCharacter),
			move(1, "EB01-013", ZoneHand, ZoneCharacter),
			move(1, "EB01-013", ZoneCharacter, ZoneTrash),
			move(1, "OP14-031", ZoneDeck, ZoneCharacter),
			move(1, "ST30-004", ZoneHand, ZoneCharacter),
			boardCheck(1, 3, 4),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
		)
		lines = append(lines, removals...)
		lines = append(lines,
			by(alice, card("Lilith", "OP13-113")+": Activate Trigger"),
			boardCheck(1, 3-len(removals), 1),
			by(bob, "End Turn"),
			by(alice, "Draw 1 Card"),
			by(alice, "Concedes!"),
		)
		return parse(logOf(lines...))[0]
	}

	It("picks big turns, low life, a new board and the finish, and skips the first life taken", func() {
		g := game(
			move(1, "OP06-033", ZoneCharacter, ZoneTrash),
			move(1, "OP14-031", ZoneCharacter, ZoneHand),
		)
		ms := g.Moments(powerOf)
		Expect(ms).To(HaveLen(3))
		Expect(ms[0].Turn).To(Equal(3))
		Expect(ms[0].Player).To(Equal(aliceID))
		Expect(ms[0].Reasons).To(Equal([]Reason{{Kind: ReasonBoard, Player: aliceID, N: 3}}))
		Expect(ms[0].Played).To(Equal([]string{"OP06-033", "EB01-013", "OP14-031", "ST30-004"}))
		Expect(ms[0].Removed).To(BeEmpty())
		Expect(ms[0].Board).To(Equal([]string{"OP06-033", "OP14-031", "ST30-004"}))

		Expect(ms[1].Turn).To(Equal(4))
		Expect(ms[1].Player).To(Equal(bobID))
		Expect(ms[1].Removed).To(Equal([]string{"OP06-033", "OP14-031"}))
		Expect(ms[1].Reasons).To(ConsistOf(
			Reason{Kind: ReasonRemoved, Player: bobID, N: 2},
			Reason{Kind: ReasonLifeLost, Player: aliceID, N: 3},
			Reason{Kind: ReasonAtOne, Player: aliceID},
		))
		Expect(ms[2].Turn).To(Equal(5))
		Expect(ms[2].Reasons).To(Equal([]Reason{{Kind: ReasonFinish, Player: aliceID}}))
		Expect(g.Lines[ms[2].Line].Text).To(Equal("Draw 1 Card"))
	})

	It("keeps a player dropping to 0 apart from the lethal attack after it", func() {
		lines := append(lobby("XNJ8M66"),
			boardCheck(1, 0, 5), boardCheck(2, 0, 2),
			by(alice, "Hand before Mulligan: [OP12-023,EB05-001,EB05-001,OP07-022,ST32-002]"),
			by(alice, "Draw 1 Don"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			by(bob, "End Turn"),
			by(alice, "Draw 1 Card"),
			boardCheck(2, 0, 0),
			by(alice, card("Shanks", "OP17-022")+" attacking "+kid),
			"Shanks [OP17-022][12000] vs "+kid+"[5000]",
			kid+" hit for 1 damage",
		)
		g := parse(logOf(lines...))[0]
		Expect(g.End).To(Equal(End{Kind: EndLethal, Loser: bobID}))
		ms := g.Moments(powerOf)
		Expect(ms).To(HaveLen(1))
		Expect(ms[0].Turn).To(Equal(3))
		Expect(ms[0].Reasons).To(Equal([]Reason{
			{Kind: ReasonLifeLost, Player: bobID, N: 2},
			{Kind: ReasonAtZero, Player: bobID},
			{Kind: ReasonFinish, Player: bobID},
		}))
	})

	It("needs 2 removals of 5000 power or more, or 3 of any power", func() {
		removed := func(g Game) bool {
			for _, m := range g.Moments(powerOf) {
				for _, r := range m.Reasons {
					if r.Kind == ReasonRemoved {
						return true
					}
				}
			}
			return false
		}
		Expect(removed(game(
			move(1, "OP06-033", ZoneCharacter, ZoneTrash),
			move(1, "ST30-004", ZoneCharacter, ZoneDeck),
		))).To(BeFalse())
		Expect(removed(game(
			move(1, "OP06-033", ZoneCharacter, ZoneTrash),
			move(1, "ST30-004", ZoneCharacter, ZoneDeck),
			move(1, "OP14-031", ZoneCharacter, ZoneLife),
		))).To(BeTrue())
	})

	It("marks each removal and Trigger with its owner and turn, but not a player trashing their own", func() {
		g := game(move(1, "OP06-033", ZoneCharacter, ZoneTrash))
		marks := g.Marks()
		for i := range marks {
			marks[i].Line = 0
		}
		Expect(marks).To(Equal([]Mark{
			{Kind: MarkRemoved, Turn: 4, Player: aliceID, Card: "OP06-033"},
			{Kind: MarkTrigger, Turn: 4, Player: aliceID, Card: "OP13-113"},
		}))
	})
})
