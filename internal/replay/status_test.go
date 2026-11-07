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
		d := r.Steps[28].Battle
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
		var defense []int
		for _, s := range r.Steps[25:29] {
			defense = append(defense, s.Battle.Defense.Value)
		}
		Expect(defense).To(Equal([]int{5000, 6000, 7000, 7000}))
		Expect(r.Steps[25].Board.Side(2).Leader.Statuses).To(BeEmpty())
		Expect(r.Steps[26].Board.Side(2).Leader.Statuses).To(HaveLen(1))
		Expect(r.Steps[27].Board.Side(2).Leader.Statuses).To(HaveLen(2))
	})

	It("ends combat buffs with the attack", func() {
		Expect(r.Steps[29].Board.Side(2).Leader.Statuses).To(BeEmpty())
	})

	It("puts what the log doesn't explain apart, anchored to the vs line", func() {
		text, err := os.ReadFile("testdata/six-turns.log")
		Expect(err).NotTo(HaveOccurred())
		r, err := Build(parseGame(string(text)), nil)
		Expect(err).NotTo(HaveOccurred())
		a := r.Steps[28].Battle.Attack
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

	It("reads every phrase that sets a status", func() {
		for rest, want := range map[string]effect{
			"Grant X [OP17-039] -1000 until next turn end":           {phrase: PhraseGrant, kind: StatusPower, n: -1000, target: "OP17-039"},
			"Grant X [OP13-004] 2000 until opponent's next turn end": {phrase: PhraseGrantOpp, kind: StatusPower, n: 2000, target: "OP13-004", own: true},
			"Buff X [OP17-020] 4000 for the Combat":                  {phrase: PhraseBuffCombat, kind: StatusPower, n: 4000, target: "OP17-020", own: true},
			"Buff X [EB05-016] -2000":                                {phrase: PhraseBuff, kind: StatusPower, n: -2000, target: "EB05-016"},
			"Buff Self 4000":                                         {phrase: PhraseBuffSelf, kind: StatusPower, n: 4000, own: true},
			"Set X [OP13-079] Base Power to 2000":                    {phrase: PhraseBasePower, kind: StatusPower, n: 2000, target: "OP13-079", own: true},
			"Counter 3000":                                           {phrase: PhraseCounter, kind: StatusPower, n: 3000, own: true},
			"X [EB05-055] can't be rested until next turn ends":      {phrase: PhraseNoRest, kind: StatusNoRest, target: "EB05-055"},
			"X [EB05-055] will not Activate during next Refresh":     {phrase: PhraseFrozen, kind: StatusFrozen, target: "EB05-055"},
			"X [EB05-055] can't attack next turn":                    {phrase: PhraseNoAttack, kind: StatusNoAttack, target: "EB05-055"},
			"Nullify effects of X [OP13-031]":                        {phrase: PhraseNullify, kind: StatusNegated, target: "OP13-031"},
			"Prevent Effect To X [OP17-040]":                         {phrase: PhraseProtect, kind: StatusProtected, target: "OP17-040", own: true},
			"Give X [OP16-098] Rush":                                 {phrase: PhraseRush, kind: StatusRush, target: "OP16-098", own: true},
			"Give X [EB04-007] Character Rush":                       {phrase: PhraseRush, kind: StatusRush, target: "EB04-007", own: true},
			"Grant Unblockable to X [OP17-093]":                      {phrase: PhraseUnblockable, kind: StatusUnblockable, target: "OP17-093", own: true},
			"X [EB02-035] Cost 2 until Opponent's Turn End":          {phrase: PhraseCostOpp, kind: StatusCost, n: 2, target: "EB02-035", own: true},
			"X [OP16-084] Cost 20":                                   {phrase: PhraseCost, kind: StatusCost, n: 20, target: "OP16-084", own: true},
		} {
			got, ok := parseEffect(rest)
			Expect(ok).To(BeTrue(), rest)
			Expect(got).To(Equal(want), rest)
		}
		_, ok := parseEffect("Draw 2 Card")
		Expect(ok).To(BeFalse())
	})
})
