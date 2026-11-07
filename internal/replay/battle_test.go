package replay

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	oden    = link("Kouzuki Oden", "ST32-002")
	shanks  = link("Shanks", "OP17-022")
	stussy  = link("Stussy", "OP17-054")
	linlin  = link("Charlotte Linlin", "OP17-049")
	newgate = link("Edward Newgate", "OP17-040")
	luffy   = link("Monkey D. Luffy", "OP13-001")
	xebec   = link("Rocks D. Xebec", "OP17-039")
	noAuth  = link("There's No Authority in the World", "OP17-055")
	nami    = link("Nami", "OP14-031")
)

// duel is a log's header with Alice on Monkey D. Luffy and Bob on the leader given.
func duel(bobLeader string, lines ...string) []string {
	return append([]string{
		"RZ1|HDR|1.44a|2|RZ1",
		"RZ1|PLY|1|" + alice + "|OP13-001",
		"RZ1|PLY|2|" + bob + "|" + bobLeader,
	}, lines...)
}

// battleKinds are the kinds of the steps that have a battle, in order.
func battleKinds(r *Replay) []Kind {
	var out []Kind
	for _, s := range r.Steps {
		if s.Battle != nil {
			out = append(out, s.Kind)
		}
	}
	return out
}

func firstOf(r *Replay, k Kind) Step {
	for _, s := range r.Steps {
		if s.Kind == k {
			return s
		}
	}
	Fail("no step of the kind")
	return Step{}
}

func printedPower(code string) int {
	return map[string]int{
		"OP13-001": 5000, "OP17-039": 5000, "OP17-054": 5000, "ST32-002": 6000, "OP17-022": 12000,
		"OP13-079": 5000,
	}[code]
}

// opening is a game where Alice has two Kouzuki Odens and two Shanks out, and Bob's Stussy
// stops one of each from attacking in his turn, as in a real game. Alice holds a third Shanks.
func opening() []string {
	return duel("OP17-039",
		"RZ1|1|1|ST32-002|0|49|1|0|1|1|0|0|0",
		"RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
		"RZ1|2|1|ST32-002|0|48|1|1|1|1|0|0|0",
		"RZ1|3|1|OP17-022|0|47|1|2|1|1|0|0|0",
		"RZ1|4|1|OP17-022|0|46|1|3|1|1|0|0|0",
		"RZ1|5|1|OP17-022|0|45|1|4|1|1|0|0|0",
		"RZ1|6|2|OP17-054|0|49|1|0|0|1|0|0|0",
		"RZ1|CHK|6|2|49|1|0|0|10|0|0|0|1|0",
		by(bob, "Chose to go Second"),
		"RZ1|7|1|ST32-002|1|0|2|0|1|1|0|0|0",
		by(alice, "Deploy "+oden),
		"RZ1|8|1|ST32-002|1|0|2|1|1|1|0|0|0",
		by(alice, "Deploy "+oden),
		"RZ1|9|1|OP17-022|1|0|2|2|1|1|0|0|0",
		by(alice, "Deploy "+shanks),
		"RZ1|10|1|OP17-022|1|0|2|3|1|1|0|0|0",
		by(alice, "Deploy "+shanks),
		by(alice, "End Turn"),
		"RZ1|11|2|OP17-054|1|0|2|0|0|1|0|0|0",
		by(bob, "Deploy "+stussy),
		by(bob, stussy+": "+oden+" can't attack next turn"),
		by(bob, stussy+": "+shanks+" can't attack next turn"),
		by(bob, stussy+": Rest "+stussy),
		by(bob, "End Turn"),
	)
}

var _ = Describe("which copy a line means", func() {
	lastBoard := func(r *Replay) []Card { return r.Steps[len(r.Steps)-1].Board.Side(1).Characters }

	It("never credits an attack to a copy that can't attack", func() {
		r := build(logOf(append(opening(),
			by(alice, oden+" attacking "+stussy),
			by(alice, shanks+" attacking "+xebec),
			by(bob, "Concedes!"),
		)...))
		Expect(r.Issues).To(BeEmpty())
		chars := lastBoard(r)
		Expect(looks(chars)).To(Equal([]string{"ST32-002", "ST32-002(R)", "OP17-022", "OP17-022(R)"}))
		Expect(chars[0].has(StatusNoAttack)).To(BeTrue())
		Expect(chars[1].has(StatusNoAttack)).To(BeFalse())
		Expect(chars[2].has(StatusNoAttack)).To(BeTrue())
	})

	It("takes a copy played this turn last, unless it has Rush", func() {
		played := append(opening(),
			"RZ1|12|1|OP17-022|1|0|2|0|1|1|0|0|0",
			by(alice, "Deploy "+shanks),
		)
		r := build(logOf(append(played[:len(played):len(played)],
			by(alice, shanks+" attacking "+xebec),
			by(bob, "Concedes!"),
		)...))
		Expect(r.Issues).To(BeEmpty())
		Expect(looks(lastBoard(r))).To(Equal([]string{"OP17-022", "ST32-002", "ST32-002", "OP17-022", "OP17-022(R)"}))

		r = build(logOf(append(played[:len(played):len(played)],
			by(alice, shanks+": Give "+shanks+" Rush"),
			by(alice, shanks+" attacking "+xebec),
			by(bob, "Concedes!"),
		)...))
		Expect(r.Issues).To(BeEmpty())
		Expect(looks(lastBoard(r))).To(Equal([]string{"OP17-022(R)", "ST32-002", "ST32-002", "OP17-022", "OP17-022"}))
	})

	It("moves a guessed can't attack to the other copy when an attack shows it", func() {
		r := build(logOf(append(opening(),
			"RZ1|12|1|ST32-002|2|1|6|0|1|1|0|0|0",
			by(alice, "Trash "+oden),
			by(alice, oden+" attacking "+stussy),
			by(bob, "Concedes!"),
		)...))
		Expect(r.Issues).To(BeEmpty())
		Expect(r.Corrected).To(Equal(1))
		Expect(looks(lastBoard(r))).To(Equal([]string{"ST32-002(R)", "OP17-022", "OP17-022"}))
	})

	It("reports an attack by a card that can't attack", func() {
		r := build(logOf(duel("OP17-039",
			"RZ1|1|1|ST32-002|0|49|1|0|1|1|0|0|0",
			"RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
			by(bob, "Chose to go Second"),
			"RZ1|2|1|ST32-002|1|0|2|0|1|1|0|0|0",
			by(alice, "Deploy "+oden),
			by(alice, "End Turn"),
			by(bob, stussy+": "+oden+" can't attack next turn"),
			by(bob, "End Turn"),
			by(alice, oden+" attacking "+xebec),
			by(bob, "Concedes!"),
		)...))
		Expect(r.Issues).To(HaveLen(1))
		Expect(r.Issues[0].Kind).To(Equal(IssueNoAttack))
		Expect(r.Issues[0].Detail).To(Equal("attacker ST32-002 can't attack"))
	})
})

// threeShanks is a game where Alice has three Shanks out, whose turn comes round again.
func threeShanks(lines ...string) []string {
	return duel("OP17-039", append([]string{
		"RZ1|1|1|OP17-022|0|49|1|0|1|1|0|0|0",
		"RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
		"RZ1|2|1|OP17-022|0|48|1|1|1|1|0|0|0",
		"RZ1|3|1|OP17-022|0|47|1|2|1|1|0|0|0",
		by(bob, "Chose to go Second"),
		"RZ1|4|1|OP17-022|1|0|2|0|1|1|0|0|0",
		by(alice, "Deploy "+shanks),
		"RZ1|5|1|OP17-022|1|0|2|1|1|1|0|0|0",
		by(alice, "Deploy "+shanks),
		"RZ1|6|1|OP17-022|1|0|2|2|1|1|0|0|0",
		by(alice, "Deploy "+shanks),
		by(alice, "End Turn"),
		by(bob, "End Turn"),
	}, lines...)...)
}

var _ = Describe("each attack's copy", func() {
	It("credits each of three attacks to its own copy and names it on the step", func() {
		r := build(logOf(threeShanks(
			by(alice, shanks+" attacking "+xebec),
			by(alice, shanks+" attacking "+xebec),
			by(alice, shanks+" attacking "+xebec),
			by(bob, "Concedes!"),
		)...))
		Expect(r.Issues).To(BeEmpty())
		var ids []int
		for _, s := range r.Steps {
			if s.Kind != KindAttack {
				continue
			}
			attacker := s.Board.Side(1).byID(s.Battle.AttackerID)
			Expect(attacker).NotTo(BeNil())
			Expect(attacker.Rested).To(BeTrue())
			Expect(s.Battle.DefenderID).To(Equal(s.Board.Side(2).Leader.ID))
			ids = append(ids, s.Battle.AttackerID)
		}
		chars := r.Steps[len(r.Steps)-1].Board.Side(1).Characters
		Expect(ids).To(Equal([]int{chars[0].ID, chars[1].ID, chars[2].ID}))
	})

	It("lets a copy set active attack again, and reports one that attacks twice", func() {
		r := build(logOf(threeShanks(
			by(alice, shanks+" attacking "+xebec),
			by(alice, link("Nami", "OP14-031")+": Set "+shanks+" to Active"),
			by(alice, shanks+" attacking "+xebec),
			by(alice, shanks+" attacking "+xebec),
			by(alice, shanks+" attacking "+xebec),
			by(alice, shanks+" attacking "+xebec),
			by(bob, "Concedes!"),
		)...))
		Expect(r.Issues).To(HaveLen(2))
		Expect(r.Issues[0].Kind).To(Equal(IssueRest))
		Expect(r.Issues[1].Kind).To(Equal(IssueTwice))
		Expect(r.Issues[1].Detail).To(Equal("attacker OP17-022 attacked twice this turn"))
	})
})

var _ = Describe("effects during an attack", func() {
	It("gives the defender's effect its own step, with its cost and its power", func() {
		lines := duel("OP17-039",
			"RZ1|1|2|OP17-054|0|49|1|0|0|1|0|0|0",
			"RZ1|CHK|1|2|49|1|0|0|10|0|0|0|1|0",
			"RZ1|2|2|OP17-049|0|48|1|1|0|1|0|0|0",
			"RZ1|3|2|OP17-040|0|47|1|2|0|1|0|0|0",
			"RZ1|4|2|OP17-055|0|46|1|3|0|1|0|0|0",
			by(alice, "Chose to go Second"),
			by(bob, "Deploy "+stussy),
			"RZ1|5|2|OP17-054|1|0|2|0|1|1|0|0|0",
			by(bob, "Deploy "+linlin),
			"RZ1|6|2|OP17-049|1|0|2|1|1|1|0|0|0",
			by(bob, stussy+": Rest "+stussy),
			by(bob, "End Turn"),
			"RZ1|7|1|Don|4|9|5|0|1|1|0|0|0",
			"RZ1|CHK|7|1|50|0|0|0|9|1|0|0|1|0",
			"RZ1|8|1|Don|4|8|5|1|1|1|0|0|0",
			by(alice, "Draw 2 Don"),
			by(alice, "Attach 2 Don to "+luffy+" (2 Total)"),
			"RZ1|9|1|Don|5|1|9|9900|1|1|0|0|0",
			"RZ1|10|1|Don|5|0|9|9901|1|1|0|0|0",
			by(alice, luffy+" attacking "+stussy),
			"RZ1|11|2|OP17-040|1|0|6|0|1|1|0|0|0",
			by(bob, linlin+": Trash "+newgate),
			by(bob, linlin+": Buff "+stussy+" 1000 for the Combat"),
			by(bob, noAuth+": Activate Counter"),
			"RZ1|12|2|OP17-055|1|0|6|1|1|1|0|0|0",
			by(bob, noAuth+": Buff "+stussy+" 2000 for the Combat"),
			luffy+"[7000] vs "+stussy+"[8000]",
			"Attack Fails",
			by(alice, "Concedes!"),
		)
		g := parseGame(logOf(lines...))
		r, err := Build(g, printedPower)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Issues).To(BeEmpty())
		Expect(battleKinds(r)).To(Equal([]Kind{KindAttack, KindBattleEffect, KindCounter, KindDamage}))
		effect := firstOf(r, KindBattleEffect)
		Expect(effect.Title).To(Equal("Charlotte Linlin: +1000 to Stussy (trashed Edward Newgate)"))
		Expect(effect.Player).To(Equal(bobID))
		Expect(effect.Phase).To(Equal(PhaseBattle))
		Expect(effect.Cards).To(Equal([]string{"OP17-049"}))
		Expect(moveSeqs(g, effect)).To(Equal([]int{11}))
		Expect(codes(effect.Board.Side(2).Trash)).To(Equal([]string{"OP17-040"}))
		Expect(firstOf(r, KindCounter).Title).To(Equal("Countered with There's No Authority in the World +2000"))
		d := firstOf(r, KindDamage).Battle.Defense
		Expect(d.Value).To(Equal(8000))
		Expect(d.Parts).To(Equal([]PowerPart{
			{Kind: PartPrinted, N: 5000},
			{Kind: PartEffect, N: 1000, Source: "OP17-049"},
			{Kind: PartCounter, N: 2000, Source: "OP17-055"},
		}))
		Expect(r.PowerChecks).To(BeEmpty())
	})

	It("adds up the DON an effect rests, and keeps the attacker's own effect in the attack", func() {
		lines := duel("OP17-039",
			"RZ1|1|2|OP17-040|0|49|1|0|0|1|0|0|0",
			"RZ1|CHK|1|2|49|1|0|0|10|0|0|0|1|0",
			by(bob, "Chose to go Second"),
			"RZ1|2|1|Don|4|9|5|0|1|1|0|0|0",
			"RZ1|CHK|2|1|50|0|0|0|9|1|0|0|1|0",
			"RZ1|3|1|Don|4|8|5|1|1|1|0|0|0",
			by(alice, "Draw 2 Don"),
			by(alice, "End Turn"),
			by(bob, xebec+" attacking "+luffy),
			"RZ1|4|2|OP17-040|1|0|6|0|1|1|0|0|0",
			by(bob, xebec+": Trash "+newgate),
			"RZ1|5|1|Don|5|0|5|0|1|1|1|0|0",
			"RZ1|6|1|Don|5|1|5|1|1|1|1|0|0",
			by(alice, luffy+": Rest Don [Don]"),
			by(alice, luffy+": Rest Don [Don]"),
			by(alice, luffy+": Buff "+luffy+" 4000 for the Combat"),
			xebec+"[5000] vs "+luffy+"[9000]",
			"Attack Fails",
			by(bob, "Concedes!"),
		)
		g := parseGame(logOf(lines...))
		r, err := Build(g, printedPower)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Issues).To(BeEmpty())
		Expect(battleKinds(r)).To(Equal([]Kind{KindAttack, KindBattleEffect, KindDamage}))
		Expect(moveSeqs(g, firstOf(r, KindAttack))).To(Equal([]int{4}))
		effect := firstOf(r, KindBattleEffect)
		Expect(effect.Title).To(Equal("Monkey D. Luffy: +4000 to Monkey D. Luffy (rested 2 DON)"))
		Expect(effect.Player).To(Equal("Alice#1111"))
		Expect(moveSeqs(g, effect)).To(Equal([]int{5, 6}))
		Expect(effect.Board.Side(1).DonRested()).To(Equal(2))
		Expect(r.PowerChecks).To(BeEmpty())
	})
})

var _ = Describe("counters", func() {
	rocksPirates := link("Rocks Pirates", "OP17-056")

	It("gives each copy of a counter event its own step and its own buff", func() {
		lines := duel("OP17-039",
			"RZ1|1|2|OP17-056|0|49|1|0|0|1|0|0|0",
			"RZ1|CHK|1|2|49|1|0|0|10|0|0|0|1|0",
			"RZ1|2|2|OP17-056|0|48|1|1|0|1|0|0|0",
			"RZ1|3|2|OP08-051|0|47|1|2|0|1|0|0|0",
			by(bob, "Chose to go Second"),
			by(alice, luffy+" attacking "+xebec),
			by(bob, rocksPirates+": Activate Counter"),
			"RZ1|4|2|OP17-056|1|0|6|0|1|1|0|0|0",
			by(bob, rocksPirates+": Buff "+xebec+" 2000 for the Combat"),
			by(bob, rocksPirates+": Activate Counter"),
			"RZ1|5|2|OP17-056|1|0|6|1|1|1|0|0|0",
			by(bob, rocksPirates+": Buff "+xebec+" 2000 for the Combat"),
			by(bob, "Discard "+link("Buckin", "OP08-051")+" for Counter 1000"),
			"RZ1|6|2|OP08-051|1|0|6|2|1|1|0|0|0",
			luffy+"[5000] vs "+xebec+"[10000]",
			"Attack Fails",
			by(bob, "Concedes!"),
		)
		g := parseGame(logOf(lines...))
		r, err := Build(g, printedPower)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Issues).To(BeEmpty())
		var counters []string
		var seqs [][]int
		for _, s := range r.Steps {
			if s.Kind == KindCounter {
				counters = append(counters, s.Title)
				seqs = append(seqs, moveSeqs(g, s))
			}
		}
		Expect(counters).To(Equal([]string{
			"Countered with Rocks Pirates +2000", "Countered with Rocks Pirates +2000", "Countered with Buckin +1000",
		}))
		Expect(seqs).To(Equal([][]int{{4}, {5}, {6}}))
		Expect(firstOf(r, KindDamage).Battle.Defense.Parts).To(Equal([]PowerPart{
			{Kind: PartPrinted, N: 5000},
			{Kind: PartCounter, N: 2000, Source: "OP17-056"},
			{Kind: PartCounter, N: 2000, Source: "OP17-056"},
			{Kind: PartCounter, N: 1000, Source: "OP08-051"},
		}))
	})

	It("keeps each copy's discard in its own step whether the line or the move comes first", func() {
		buckin := link("Buckin", "OP08-051")
		lines := duel("OP17-039",
			"RZ1|1|2|OP08-051|0|49|1|0|0|1|0|0|0",
			"RZ1|CHK|1|2|49|1|0|0|10|0|0|0|1|0",
			"RZ1|2|2|OP08-051|0|48|1|1|0|1|0|0|0",
			"RZ1|3|2|OP08-051|0|47|1|2|0|1|0|0|0",
			"RZ1|4|2|OP08-051|0|46|1|3|0|1|0|0|0",
			by(bob, "Chose to go Second"),
			by(alice, luffy+" attacking "+xebec),
			by(bob, "Discard "+buckin+" for Counter 1000"),
			"RZ1|5|2|OP08-051|1|0|6|0|1|1|0|0|0",
			by(bob, "Discard "+buckin+" for Counter 1000"),
			"RZ1|6|2|OP08-051|1|0|6|1|1|1|0|0|0",
			luffy+"[5000] vs "+xebec+"[7000]",
			"Attack Fails",
			by(alice, luffy+" attacking "+xebec),
			"RZ1|7|2|OP08-051|1|0|6|2|1|1|0|0|0",
			by(bob, "Discard "+buckin+" for Counter 1000"),
			"RZ1|8|2|OP08-051|1|0|6|3|1|1|0|0|0",
			by(bob, "Discard "+buckin+" for Counter 1000"),
			luffy+"[5000] vs "+xebec+"[7000]",
			"Attack Fails",
			by(bob, "Concedes!"),
		)
		g := parseGame(logOf(lines...))
		r := build(logOf(lines...))
		var seqs [][]int
		for _, s := range r.Steps {
			if s.Kind == KindCounter {
				seqs = append(seqs, moveSeqs(g, s))
			}
		}
		Expect(seqs).To(Equal([][]int{{5}, {6}, {7}, {8}}))
	})
})

var _ = Describe("effect titles", func() {
	It("adds up an effect the sim writes once per DON", func() {
		r := build(logOf(duel("OP17-039",
			"RZ1|1|1|OP14-031|0|49|1|0|1|1|0|0|0",
			"RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
			by(bob, "Chose to go Second"),
			"RZ1|2|1|Don|4|9|5|0|1|1|0|0|0",
			"RZ1|3|1|Don|4|8|5|1|1|1|0|0|0",
			"RZ1|4|1|Don|4|7|5|2|1|1|0|0|0",
			by(alice, "Draw 3 Don"),
			"RZ1|5|1|OP14-031|1|0|2|0|1|1|0|0|0",
			by(alice, "Deploy "+nami),
			"RZ1|6|1|Don|5|0|5|0|1|1|1|0|0",
			"RZ1|7|1|Don|5|1|5|1|1|1|1|0|0",
			"RZ1|8|1|Don|5|2|5|2|1|1|1|0|0",
			by(alice, "Rest 3 Don"),
			by(alice, "End Turn"),
			by(bob, "End Turn"),
			"RZ1|9|1|Don|5|0|5|0|1|1|0|0|0",
			by(alice, nami+": Activate 1 Don"),
			"RZ1|10|1|Don|5|1|5|1|1|1|0|0|0",
			by(alice, nami+": Activate 1 Don"),
			"RZ1|11|1|Don|5|2|5|2|1|1|0|0|0",
			by(alice, nami+": Activate 1 Don"),
			by(bob, "Concedes!"),
		)...))
		effect := firstOf(r, KindEffect)
		Expect(effect.Title).To(Equal("Nami: Set 3 DON active"))
		Expect(effect.Board.Side(1).DonActive()).To(Equal(3))
	})

	It("writes each counted effect's total", func() {
		for phrase, want := range map[string]string{
			"Activate 5 Don":          "Set 5 DON active",
			"Rest Don [Don]":          "Rest 1 DON",
			"Minus 2 Don":             "Return 2 DON",
			"Draw 1 Card":             "Draw 1 card",
			"Draw 2 Card":             "Draw 2 cards",
			"Draw 2 Rested Don":       "Add 2 rested DON",
			"Trash 3 Cards from Deck": "Trash 3 Cards from Deck",
		} {
			p, n, ok := counted(phrase)
			Expect(ok).To(BeTrue(), phrase)
			Expect(countedText(p, n)).To(Equal(want), phrase)
		}
	})
})

var _ = Describe("power the log words oddly", func() {
	yamato := link("Yamato", "EB05-046")
	imu := link("Imu", "OP13-079")

	It("reads a base power the sim sets as a change, which the vs line shows", func() {
		r, err := Build(parseGame(logOf(duel("OP13-079",
			"RZ1|1|2|OP14-096|0|49|1|0|0|1|0|0|0",
			"RZ1|CHK|1|2|49|1|0|0|10|0|0|0|1|0",
			by(bob, "Chose to go Second"),
			by(alice, luffy+" attacking "+imu),
			"RZ1|2|2|OP14-096|1|0|6|0|1|1|0|0|0",
			by(bob, yamato+": Trash "+link("Ground Death", "OP14-096")),
			by(bob, yamato+": Set "+imu+" Base Power to 2000"),
			luffy+"[5000] vs "+imu+"[7000]",
			"Attack Fails",
			by(bob, "Concedes!"),
		)...)), printedPower)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.PowerChecks).To(BeEmpty())
		Expect(firstOf(r, KindBattleEffect).Title).To(Equal("Yamato: +2000 to Imu (trashed Ground Death)"))
		Expect(firstOf(r, KindDamage).Battle.Defense.Parts).To(Equal([]PowerPart{
			{Kind: PartPrinted, N: 5000}, {Kind: PartEffect, N: 2000, Source: "EB05-046"},
		}))
	})

	It("shows power below 0 as 0, as the sim does", func() {
		p := Power{Parts: []PowerPart{{Kind: PartPrinted, N: 5000}, {Kind: PartEffect, N: -7000}}}
		Expect(p.value()).To(Equal(0))
		Expect(p.Floored()).To(BeTrue())
		Expect(gap(0, p)).To(Equal(0))
		Expect(gap(1000, p)).To(Equal(3000))
	})
})
