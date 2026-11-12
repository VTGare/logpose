package replay

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// printed is the printed power of the fixture's cards that the specs look at.
func printed(code string) int {
	return map[string]int{"OP17-039": 5000, "OP13-001": 5000, "OP13-016": 2000, "ST32-001": 3000}[code]
}

var _ = Describe("power", func() {
	var r *Replay

	BeforeEach(func() {
		text, err := os.ReadFile("testdata/six-turns.log")
		Expect(err).NotTo(HaveOccurred())
		r, err = Build(parseGame(string(text)), printed)
		Expect(err).NotTo(HaveOccurred())
	})

	It("works out every power the sim prints in the fixture", func() {
		Expect(r.PowerCompared).To(Equal(8))
		Expect(r.PowerChecks).To(BeEmpty())
	})

	It("adds DON on the owner's turn, combat buffs and counters", func() {
		d := r.Steps[stepAt(r, "T6 Alice: damage")].Battle
		Expect(d.Attack.Parts).To(Equal([]PowerPart{{Kind: PartPrinted, N: 5000}, {Kind: PartDon, N: 1000}}))
		Expect(d.Defense.Parts).To(Equal([]PowerPart{
			{Kind: PartPrinted, N: 5000},
			{Kind: PartEffect, N: 1000, Source: "OP17-049"},
			{Kind: PartCounter, N: 1000, Source: "OP08-051"},
		}))
		Expect(d.Defense.Value).To(Equal(7000))
		Expect(d.Defense.Estimated).To(BeFalse())
	})

	It("gives each step of an attack the power it had then", func() {
		attack, damage := stepAt(r, "T6 Alice: attack"), stepAt(r, "T6 Alice: damage")
		var defense []int
		for _, s := range r.Steps[attack : damage+1] {
			defense = append(defense, s.Battle.Defense.Value)
		}
		Expect(defense).To(Equal([]int{5000, 6000, 7000, 7000}))
		Expect(r.Steps[attack].Board.Side(2).Leader.Statuses).To(BeEmpty())
		Expect(r.Steps[stepAt(r, "T6 Bob: battle effect")].Board.Side(2).Leader.Statuses).To(HaveLen(1))
		Expect(r.Steps[stepAt(r, "T6 Bob: counter")].Board.Side(2).Leader.Statuses).To(HaveLen(2))
	})

	It("ends combat buffs with the attack", func() {
		Expect(r.Steps[stepAt(r, "T6 Alice: damage")+1].Board.Side(2).Leader.Statuses).To(BeEmpty())
	})

	It("puts what the log doesn't explain apart, anchored to the vs line", func() {
		text, err := os.ReadFile("testdata/six-turns.log")
		Expect(err).NotTo(HaveOccurred())
		r, err := Build(parseGame(string(text)), nil)
		Expect(err).NotTo(HaveOccurred())
		a := r.Steps[stepAt(r, "T6 Alice: damage")].Battle.Attack
		Expect(a.Value).To(Equal(6000))
		Expect(a.Parts[len(a.Parts)-1]).To(Equal(PowerPart{Kind: PartUnlogged, N: 5000}))
	})
})

var _ = Describe("statuses", func() {
	header := []string{
		"RZ1|HDR|1.44a|2|RZ1",
		"RZ1|PLY|1|" + alice + "|OP13-001",
		"RZ1|PLY|2|" + bob + "|OP17-039",
		"RZ1|1|1|ST32-001|0|49|1|0|1|1|0|0|0",
		"RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
		by(bob, "Chose to go Second"),
		"RZ1|2|1|ST32-001|1|0|2|0|1|1|0|0|0",
		by(alice, "Deploy "+link("Kin'emon", "ST32-001")),
	}
	xebec, luffy, kinemon := link("Rocks D. Xebec", "OP17-039"), link("Monkey D. Luffy", "OP13-001"), link("Kin'emon", "ST32-001")
	shiki := link("Shiki", "OP17-048")

	statuses := func(r *Replay, step, owner int, code string) []Status {
		for _, c := range r.Steps[step].Board.Side(owner).cards() {
			if c.Code == code {
				return c.Statuses
			}
		}
		return nil
	}
	stepOf := func(r *Replay, turn int) int { return r.FirstOfTurn(turn) }
	lastOf := func(r *Replay, turn int) int { return r.FirstOfTurn(turn+1) - 1 }

	It("ends a grant at the end of the next turn, and a plain buff at the end of this one", func() {
		lines := append(header[:len(header):len(header)],
			by(alice, kinemon+": Grant "+xebec+" -1000 until next turn end"),
			by(alice, kinemon+": Buff "+luffy+" 2000"),
			by(alice, "End Turn"),
			by(bob, "End Turn"),
			by(alice, "End Turn"),
			by(bob, "Concedes!"),
		)
		r := build(logOf(lines...))
		Expect(statuses(r, stepOf(r, 1), 2, "OP17-039")).To(ConsistOf(
			Status{Kind: StatusPower, N: -1000, Source: "ST32-001", EndsTurn: 2}))
		Expect(statuses(r, stepOf(r, 1), 1, "OP13-001")).To(ConsistOf(
			Status{Kind: StatusPower, N: 2000, Source: "ST32-001", EndsTurn: 1}))
		Expect(statuses(r, stepOf(r, 2), 2, "OP17-039")).To(HaveLen(1))
		Expect(statuses(r, stepOf(r, 2), 1, "OP13-001")).To(BeEmpty())
		Expect(statuses(r, stepOf(r, 3), 2, "OP17-039")).To(BeEmpty())
	})

	It("gives a negative buff to the opponent's copy and a positive one to the owner's", func() {
		lines := append(header[:len(header):len(header)],
			by(alice, shiki+": Buff "+xebec+" -3000"),
			by(alice, kinemon+": Buff "+kinemon+" 1000"),
			by(alice, "End Turn"),
			by(bob, "Concedes!"),
		)
		r := build(logOf(lines...))
		Expect(statuses(r, lastOf(r, 1), 2, "OP17-039")).To(HaveLen(1))
		Expect(statuses(r, lastOf(r, 1), 1, "ST32-001")).To(HaveLen(1))
	})

	It("keeps a status a line ends until that line", func() {
		lines := append(header[:len(header):len(header)],
			by(alice, kinemon+": "+xebec+" can't attack next turn"),
			by(alice, "End Turn"),
			by(bob, xebec+" is no longer blocked from attacking"),
			by(bob, "Concedes!"),
		)
		r := build(logOf(lines...))
		Expect(statuses(r, stepOf(r, 1), 2, "OP17-039")).To(ConsistOf(
			Status{Kind: StatusNoAttack, Source: "ST32-001", EndsTurn: 2}))
		Expect(statuses(r, len(r.Steps)-1, 2, "OP17-039")).To(BeEmpty())
	})

	It("keeps a frozen card rested through its owner's refresh, once", func() {
		lines := append(header[:len(header):len(header)],
			by(alice, kinemon+": Rest "+xebec),
			by(alice, kinemon+": "+xebec+" will not Activate during next Refresh"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			by(bob, "End Turn"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			by(bob, "Concedes!"),
		)
		r := build(logOf(lines...))
		frozen := r.Steps[stepOf(r, 1)].Board.Side(2).Leader
		Expect(frozen.Rested).To(BeTrue())
		Expect(statuses(r, stepOf(r, 1), 2, "OP17-039")).To(HaveLen(1))
		Expect(r.Steps[stepOf(r, 2)].Board.Side(2).Leader.Rested).To(BeTrue())
		Expect(statuses(r, stepOf(r, 2), 2, "OP17-039")).To(BeEmpty())
	})

	It("gives each phrase's status to the card it names, for as long as the phrase says", func() {
		from := func(kind StatusKind, n, endsTurn int) Status {
			return Status{Kind: kind, N: n, Source: "ST32-001", EndsTurn: endsTurn}
		}
		combat := from(StatusPower, 4000, 0)
		combat.ForAttack = true
		for _, c := range []struct {
			effect string
			owner  int
			code   string
			want   Status
		}{
			{"Grant " + xebec + " -1000 until next turn end", 2, "OP17-039", from(StatusPower, -1000, 2)},
			{"Grant " + luffy + " 2000 until opponent's next turn end", 1, "OP13-001", from(StatusPower, 2000, 2)},
			{"Buff " + luffy + " 4000 for the Combat", 1, "OP13-001", combat},
			{"Buff " + xebec + " -2000", 2, "OP17-039", from(StatusPower, -2000, 1)},
			{"Buff Self 4000", 1, "ST32-001", from(StatusPower, 4000, 1)},
			{"Set " + luffy + " Base Power to 2000", 1, "OP13-001", from(StatusPower, 2000, 1)},
			{xebec + " can't be rested until next turn ends", 2, "OP17-039", from(StatusNoRest, 0, 2)},
			{xebec + " will not Activate during next Refresh", 2, "OP17-039", from(StatusFrozen, 0, 0)},
			{xebec + " can't attack next turn", 2, "OP17-039", from(StatusNoAttack, 0, 2)},
			{"Nullify effects of " + xebec, 2, "OP17-039", from(StatusNegated, 0, 1)},
			{"Prevent Effect To " + kinemon, 1, "ST32-001", from(StatusProtected, 0, 1)},
			{"Give " + kinemon + " Rush", 1, "ST32-001", from(StatusRush, 0, 1)},
			{"Give " + kinemon + " Character Rush", 1, "ST32-001", from(StatusRush, 0, 1)},
			{"Grant Unblockable to " + luffy, 1, "OP13-001", from(StatusUnblockable, 0, 1)},
			{kinemon + " Cost 2 until Opponent's Turn End", 1, "ST32-001", from(StatusCost, 2, 2)},
			{kinemon + " Cost 20", 1, "ST32-001", from(StatusCost, 20, 1)},
			{"Draw 2 Card", 1, "ST32-001", Status{}},
		} {
			lines := append(header[:len(header):len(header)], by(alice, kinemon+": "+c.effect), by(bob, "Concedes!"))
			r := build(logOf(lines...))
			got := statuses(r, len(r.Steps)-2, c.owner, c.code)
			if c.want == (Status{}) {
				Expect(got).To(BeEmpty(), c.effect)
				continue
			}
			Expect(got).To(ConsistOf(c.want), c.effect)
		}
	})

	It("gives a counter to the card the attack lands on, for the attack", func() {
		lines := append(header[:len(header):len(header)],
			by(alice, luffy+" attacking "+xebec),
			by(bob, link("Guard Point", "OP01-029")+": Counter 3000"),
			by(bob, "Concedes!"),
		)
		r := build(logOf(lines...))
		Expect(statuses(r, len(r.Steps)-2, 2, "OP17-039")).To(ConsistOf(
			Status{Kind: StatusPower, N: 3000, Source: "OP01-029", Counter: true, ForAttack: true}))
	})

	It("gives a buff on a card both players have to the copy the buff's sign points at", func() {
		r := build(logOf(
			"RZ1|HDR|1.44a|2|RZ1",
			"RZ1|PLY|1|"+alice+"|OP13-001",
			"RZ1|PLY|2|"+bob+"|OP17-039",
			"RZ1|1|1|ST32-001|0|49|1|0|1|1|0|0|0",
			"RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
			"RZ1|2|2|ST32-001|0|49|1|0|1|1|0|0|0",
			"RZ1|CHK|2|2|49|1|0|0|10|0|0|0|1|0",
			by(bob, "Chose to go Second"),
			"RZ1|3|1|ST32-001|1|0|2|0|1|1|0|0|0",
			by(alice, "Deploy "+kinemon),
			"RZ1|4|2|ST32-001|1|0|2|0|1|1|0|0|0",
			by(bob, "Deploy "+kinemon),
			by(alice, shiki+": Buff "+kinemon+" -1000"),
			by(alice, shiki+": Buff "+kinemon+" 2000"),
			by(bob, "Concedes!"),
		))
		last := len(r.Steps) - 2
		Expect(statuses(r, last, 1, "ST32-001")).To(ConsistOf(Status{Kind: StatusPower, N: 2000, Source: "OP17-048", EndsTurn: 1}))
		Expect(statuses(r, last, 2, "ST32-001")).To(ConsistOf(Status{Kind: StatusPower, N: -1000, Source: "OP17-048", EndsTurn: 1}))
	})
})
