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
			move(1, "OP05-097", ZoneHand, ZoneStage),
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
		Expect(ms[0].Played).To(Equal([]string{"OP06-033", "EB01-013", "OP14-031", "ST30-004", "OP05-097"}))
		Expect(ms[0].Characters).To(Equal([]string{"OP06-033", "EB01-013", "OP14-031", "ST30-004"}))
		Expect(ms[0].Removed).To(BeEmpty())
		Expect(ms[0].Board).To(Equal([]string{"OP06-033", "OP14-031", "ST30-004"}))
		Expect(ms[0].New).To(Equal(3))
		Expect(ms[0].Gone).To(Equal([]string{"EB01-013", "OP05-097"}))

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

	It("reads counters, blocks, locks and characters defended or brought back, and flags a hand drain", func() {
		sleeptids, killingham := card("Sleeptids", "OP18-093"), card("Saint Killingham", "OP18-064")
		lines := append(lobby("XNJ8M66"),
			by(alice, "Draw 1 Don"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			by(bob, "Board: [OP18-064,OP18-093,OP18-065]"),
			by(bob, "End Turn"),
			by(alice, "Draw 1 Card"),
			by(alice, card("Monkey D. Luffy", "OP18-118")+": "+sleeptids+" can't be rested until next turn ends"),
			by(alice, card("Monkey D. Luffy", "OP18-118")+": "+card("Sanji", "OP13-027")+" can't attack next turn"),
			by(alice, luffy+" attacking "+kid),
			by(bob, "Discard "+sleeptids+" for Counter 2000"),
			by(bob, card("Mamaragan", "OP15-078")+": Activate Counter"),
			"Attack Fails",
			by(alice, card("Roronoa Zoro", "EB04-007")+" attacking "+killingham),
			by(bob, "Discard "+card("Cerberus", "OP18-085")+" for Counter 2000"),
			by(bob, "Discard "+card("Saint Killingham", "OP18-083")+" for Counter 1000"),
			by(bob, killingham+" Destroyed"),
			move(2, "OP18-064", ZoneCharacter, ZoneTrash),
			move(2, "OP18-064", ZoneTrash, ZoneCharacter),
			move(2, "OP18-065", ZoneCharacter, ZoneTrash),
			by(alice, card("Sanji", "OP13-027")+" attacking "+kid),
			by(bob, sleeptids+" Blocks"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			by(bob, kid+" attacking "+luffy),
			by(alice, "Discard "+card("Nami", "EB02-017")+" for Counter 1000"),
			by(alice, "Discard "+card("Nami", "EB02-017")+" for Counter 1000"),
			by(alice, "Discard "+card("Nami", "EB02-017")+" for Counter 1000"),
			by(alice, "Concedes!"),
		)
		g := parse(logOf(lines...))[0]
		ms := g.Moments(func(code string) int { return map[string]int{"OP18-064": 6000, "OP18-065": 11000}[code] })
		Expect(ms).To(HaveLen(2))
		m := ms[0]
		Expect(m.Turn).To(Equal(3))
		Expect(m.Reasons).To(Equal([]Reason{{Kind: ReasonHandDrain, Player: bobID, N: 4}}))
		Expect(m.Countered).To(Equal([]string{"OP18-093", "OP15-078", "OP18-085", "OP18-083"}))
		Expect(m.Blocked).To(Equal([]string{"OP18-093"}))
		Expect(m.Defended).To(Equal([]Defense{{Card: "OP18-064", Cards: 2}}))
		Expect(m.CameBack).To(Equal([]string{"OP18-064"}))
		Expect(m.Locks).To(Equal([]Lock{{Source: "OP18-118", Cards: []string{"OP18-093"}, Rested: true}}))
		Expect(ms[1].Turn).To(Equal(4))
		Expect(ms[1].Reasons).To(Equal([]Reason{{Kind: ReasonFinish, Player: aliceID}}))
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
