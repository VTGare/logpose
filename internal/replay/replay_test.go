package replay

import (
	"os"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/simlog"
)

// The sim writes a zero-width space before the '#' in names, and player IDs leave it out.
const (
	alice = "Alice\u200b#1111"
	bob   = "Bob\u200b#2222"
	bobID = "Bob#2222"
)

func parseGame(text string) *simlog.Game {
	games, err := simlog.Parse(strings.NewReader(text))
	Expect(err).NotTo(HaveOccurred())
	Expect(games).NotTo(BeEmpty())
	return &games[len(games)-1]
}

func build(text string) *Replay {
	r, err := Build(parseGame(text), nil)
	Expect(err).NotTo(HaveOccurred())
	return r
}

func link(name, code string) string {
	return name + ` [<mark><link="` + code + `">` + code + `</link></mark>]`
}

func by(player, text string) string { return "[" + player + "] " + text }

func logOf(lines ...string) string { return strings.Join(lines, "\r\n") + "\r\n" }

var outcomeWords = map[OutcomeKind]string{OutcomeFailed: "failed", OutcomeKO: "ko", OutcomeHit: "hit"}

var kindWords = map[Kind]string{
	KindLobby: "lobby", KindOpeningHand: "opening", KindMulligan: "mulligan", KindLife: "life",
	KindTurnStart: "turn", KindPlay: "play", KindAttach: "attach", KindAttack: "attack", KindBlock: "block",
	KindCounter: "counter", KindDamage: "damage", KindBattleEffect: "battle effect", KindEffect: "effect",
	KindConcede: "concede", KindDeckOut: "deck out",
}

// summary is a step as "T3 Bob: attack OP17-039 OP13-001", with what else the step holds.
func summary(s Step) string {
	name, _, _ := strings.Cut(s.Player, "#")
	parts := append([]string{"T" + strconv.Itoa(s.Turn) + " " + name + ": " + kindWords[s.Kind]}, s.Cards...)
	if len(s.Drew) > 0 {
		parts = append(parts, "drew "+strings.Join(s.Drew, ","))
	}
	if s.Don > 0 {
		parts = append(parts, "don "+strconv.Itoa(s.Don))
	}
	if s.Counter > 0 {
		parts = append(parts, "+"+strconv.Itoa(s.Counter))
	}
	if s.Battle != nil && s.Kind == KindDamage {
		parts = append(parts, strconv.Itoa(s.Battle.AttackerPower)+" vs "+strconv.Itoa(s.Battle.TargetPower))
	}
	for _, o := range s.Outcome {
		parts = append(parts, outcomeWords[o.Kind]+strings.TrimSuffix(" "+o.Card, " "))
		if o.Kind == OutcomeHit {
			parts = append(parts, strconv.Itoa(o.N))
		}
	}
	if s.Lethal {
		parts = append(parts, "lethal")
	}
	if e := s.Effect; e != nil {
		if e.Head != "" {
			parts = append(parts, "head "+e.Head)
		}
		for _, d := range e.Did {
			switch d.Kind {
			case DidText:
				parts = append(parts, strconv.Quote(d.Text))
			case DidCount:
				parts = append(parts, d.Count.Verb+" "+strconv.Itoa(d.Count.N)+" "+d.Count.Object)
			case DidPower:
				parts = append(parts, "power "+d.Card+" "+strconv.Itoa(d.N))
			case DidTrash:
				parts = append(parts, "trash "+d.Card)
			}
		}
	}
	return strings.Join(parts, " ")
}

// stepAt is the index of the one step whose summary starts with prefix.
func stepAt(r *Replay, prefix string) int {
	at := -1
	for i, s := range r.Steps {
		if strings.HasPrefix(summary(s), prefix) {
			Expect(at).To(Equal(-1), "two steps start with "+prefix)
			at = i
		}
	}
	Expect(at).NotTo(Equal(-1), "no step starts with "+prefix)
	return at
}

func summaries(r *Replay, from, to int) []string {
	var out []string
	for _, s := range r.Steps[from:to] {
		out = append(out, summary(s))
	}
	return out
}

// look is a card as "code", with "(R)" when rested and "+N" for its DON.
func look(c Card) string {
	t := c.Code
	if c.Rested {
		t += "(R)"
	}
	if c.Don > 0 {
		t += "+" + strconv.Itoa(c.Don)
	}
	return t
}

func looks(cards []Card) []string {
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		out = append(out, look(c))
	}
	return out
}

func moveSeqs(g *simlog.Game, s Step) []int {
	out := make([]int, 0, len(s.Moves))
	for _, m := range s.Moves {
		out = append(out, g.Moves[m].Seq)
	}
	return out
}

// six-turns.log is a real game's first six turns, with the deck shuffles left out and player
// names replaced. Alice is owner 1 and goes second.
var _ = Describe("Build on a real game", func() {
	var (
		g *simlog.Game
		r *Replay
	)

	BeforeEach(func() {
		text, err := os.ReadFile("testdata/six-turns.log")
		Expect(err).NotTo(HaveOccurred())
		g = parseGame(string(text))
		r, err = Build(g, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	It("matches every zone count, slot and board printout in the log", func() {
		Expect(r.Issues).To(BeEmpty())
		Expect(r.Unverified).To(Equal(-1))
	})

	It("cuts the game into actions, and attacks into their sim steps", func() {
		Expect(summaries(r, 0, 29)).To(Equal([]string{
			"T0 Alice: opening",
			"T0 Bob: opening",
			"T0 Bob: life",
			"T0 Alice: life",
			"T1 Bob: turn don 1",
			"T2 Alice: turn drew OP13-031 don 2",
			"T2 Alice: play ST32-001",
			"T3 Bob: turn drew OP17-118 don 2",
			"T3 Bob: play OP17-045",
			"T3 Bob: attach OP17-039 don 1",
			"T3 Bob: attack OP17-039 OP13-001",
			"T3 Bob: damage OP17-039 OP13-001 6000 vs 5000 hit 1",
			"T4 Alice: turn drew ST30-004 don 2",
			"T4 Alice: play OP13-016",
			`T4 Alice: effect OP01-055 head You Can Be My Samurai!!! [OP01-055] "Rest Monkey D. Garp [OP13-016]"`,
			"T4 Alice: attach OP13-001 don 1",
			"T4 Alice: attack OP13-001 OP17-039",
			"T4 Alice: damage OP13-001 OP17-039 6000 vs 5000 hit 1",
			"T5 Bob: turn drew OP17-048 don 2",
			"T5 Bob: attack OP17-039 OP13-016",
			"T5 Bob: damage OP17-039 OP13-016 5000 vs 2000 ko OP13-016",
			"T5 Bob: play OP17-049",
			"T6 Alice: turn drew OP12-023 don 2",
			"T6 Alice: play EB05-001",
			"T6 Alice: attach OP13-001 don 1",
			"T6 Alice: attack OP13-001 OP17-039",
			"T6 Bob: battle effect OP17-049 trash OP17-041 power OP17-039 1000",
			"T6 Bob: counter OP08-051 +1000",
			"T6 Alice: damage OP13-001 OP17-039 6000 vs 7000 failed",
		}))
	})

	It("takes the life card and the K.O. the sim moves before the vs line into the damage step", func() {
		hit, ko := stepAt(r, "T3 Bob: damage"), stepAt(r, "T5 Bob: damage")
		Expect(moveSeqs(g, r.Steps[hit])).To(Equal([]int{135}))
		Expect(r.Steps[hit-1].Board.Side(1).Life).To(HaveLen(4))
		Expect(r.Steps[hit].Board.Side(1).Life).To(HaveLen(3))
		Expect(moveSeqs(g, r.Steps[ko])).To(Equal([]int{159}))
	})

	It("gives every step of an attack its cards, and the damage step their powers", func() {
		bt := r.Steps[stepAt(r, "T5 Bob: attack")].Battle
		Expect([]string{bt.Attacker, bt.Target, bt.Blocker}).To(Equal([]string{"OP17-039", "OP13-016", ""}))
		Expect(bt.AttackerOwner).To(Equal(2))
		Expect(bt.AttackerPower).To(Equal(0))
		bt = r.Steps[stepAt(r, "T5 Bob: damage")].Battle
		Expect([]int{bt.AttackerPower, bt.TargetPower}).To(Equal([]int{5000, 2000}))
		Expect([]int{bt.Attack.Value, bt.Defense.Value}).To(Equal([]int{5000, 2000}))
		counter := r.Steps[stepAt(r, "T6 Bob: counter")]
		Expect(counter.Player).To(Equal(bobID))
		Expect(counter.Battle.Attacker).To(Equal("OP13-001"))
		Expect(r.Steps[stepAt(r, "T5 Bob: play OP17-049")].Battle).To(BeNil())
	})

	It("takes the moves the local player's line comes after into its step, with the DON that paid", func() {
		Expect(moveSeqs(g, r.Steps[stepAt(r, "T2 Alice: play ST32-001")])).To(Equal([]int{117, 118, 119, 120, 121, 122}))
	})

	It("takes the moves after the opponent's line into its step", func() {
		Expect(moveSeqs(g, r.Steps[stepAt(r, "T3 Bob: play OP17-045")])).To(Equal([]int{126, 127, 128, 129}))
	})

	It("keeps a deploy's effect chain in one step", func() {
		s := r.Steps[stepAt(r, "T6 Alice: play EB05-001")].Board.Side(1)
		Expect(codes(s.Characters)).To(Equal([]string{"ST32-001", "EB05-001", "ST30-004", "ST32-002"}))
	})

	It("shows the board after an attack", func() {
		b := r.Steps[stepAt(r, "T3 Bob: damage")].Board
		me, them := b.Side(1), b.Side(2)
		Expect(me.Life).To(HaveLen(3))
		Expect(me.Hand).To(HaveLen(7))
		Expect(me.Hand[len(me.Hand)-1].Code).To(Equal("EB05-001"))
		Expect(look(them.Leader)).To(Equal("OP17-039(R)+1"))
		Expect(them.DonActive()).To(Equal(0))
		Expect(them.DonRested()).To(Equal(2))
		Expect(them.DonAttached()).To(Equal(1))
		Expect(len(them.Deck())).To(Equal(36))
		Expect(codes(them.Trash)).To(Equal([]string{"OP17-040"}))
	})

	It("rests the characters an effect names", func() {
		Expect(looks(r.Steps[stepAt(r, "T4 Alice: effect OP01-055")].Board.Side(1).Characters)).To(Equal([]string{"ST32-001(R)", "OP13-016(R)"}))
	})

	It("returns attached DON and stands cards up in the owner's refresh", func() {
		refresh := r.FirstOfTurn(5)
		before, after := r.Steps[refresh-1].Board.Side(2), r.Steps[refresh].Board.Side(2)
		Expect(before.Leader.Rested).To(BeTrue())
		Expect(look(after.Leader)).To(Equal("OP17-039"))
		Expect(after.DonActive()).To(Equal(5))
		Expect(after.DonAttached()).To(Equal(0))
		Expect(r.Steps[refresh].Board.Side(1).Leader.Rested).To(BeTrue())
	})

	It("gives each step its player's ID", func() {
		Expect(r.Steps[stepAt(r, "T5 Bob: attack")].Player).To(Equal(bobID))
	})

	It("finds the step of a line and the first step of a turn", func() {
		attack := stepAt(r, "T5 Bob: attack")
		Expect(r.StepAtLine(r.Steps[attack].Lines[0])).To(Equal(attack))
		Expect(r.FirstOfTurn(5)).To(Equal(stepAt(r, "T5 Bob: turn")))
		Expect(r.FirstOfTurn(99)).To(Equal(-1))
	})
})

var _ = Describe("Build", func() {
	header := []string{
		"RZ1|HDR|1.44a|2|RZ1",
		"RZ1|PLY|1|" + alice + "|OP13-001",
		"RZ1|PLY|2|" + bob + "|OP13-001",
	}
	kinemon := link("Kin'emon", "ST32-001")

	It("refuses a game without RZ1 moves", func() {
		g := parseGame(logOf(by(alice, "Draw 1 Don"), by(alice, "End Turn"), by(bob, "Concedes!")))
		_, err := Build(g, nil)
		Expect(err).To(MatchError(ErrNoMoves))
	})

	It("skips the end of the game before a rematch", func() {
		lines := append(header[:len(header):len(header)],
			"RZ1|1|2|Don|9|9900|5|0|1|1|1|0|0",
			"RZ1|CHK|1|2|20|1|5|0|0|9|24|0|1|1",
			"RZ1|2|1|ST32-001|0|49|1|0|1|1|0|0|0",
			"RZ1|CHK|2|1|49|1|0|0|10|0|0|0|1|0",
			by(bob, "Chose to go Second"),
			"RZ1|3|1|Don|4|9|5|0|1|1|0|0|0",
			"RZ1|CHK|3|1|49|1|0|0|9|1|0|0|1|0",
			by(alice, "Draw 1 Don"),
			by(alice, "End Turn"),
			by(bob, "Concedes!"),
		)
		r := build(logOf(lines...))
		Expect(r.Issues).To(BeEmpty())
		Expect(r.Steps[0].Moves).NotTo(ContainElement(0))
		Expect(r.Steps[len(r.Steps)-1].Kind).To(Equal(KindConcede))
	})

	It("works out whose card an effect rested from a later attack on it", func() {
		lines := append(header[:len(header):len(header)],
			"RZ1|1|1|ST32-001|0|49|1|0|1|1|0|0|0",
			"RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
			"RZ1|2|2|ST32-001|0|49|1|0|0|1|0|0|0",
			"RZ1|CHK|2|2|49|1|0|0|10|0|0|0|1|0",
			by(bob, "Chose to go Second"),
			"RZ1|3|1|Don|4|9|5|0|1|1|0|0|0",
			by(alice, "Draw 1 Don"),
			by(alice, "Deploy "+kinemon),
			"RZ1|4|1|ST32-001|1|0|2|0|1|1|0|0|0",
			by(alice, "End Turn"),
			by(bob, "Deploy "+kinemon),
			"RZ1|5|2|ST32-001|1|0|2|0|1|1|0|0|0",
			by(bob, "End Turn"),
			by(alice, link("You Can Be My Samurai!!!", "OP01-055")+": Rest "+kinemon),
			by(alice, "End Turn"),
			by(bob, link("Monkey D. Luffy", "OP13-001")+" attacking "+kinemon),
			by(bob, "Concedes!"),
		)
		r := build(logOf(lines...))
		Expect(r.Issues).To(BeEmpty())
		var effect Step
		for _, s := range r.Steps {
			if s.Kind == KindEffect {
				effect = s
			}
		}
		Expect(looks(effect.Board.Side(1).Characters)).To(Equal([]string{"ST32-001(R)"}))
		Expect(looks(effect.Board.Side(2).Characters)).To(Equal([]string{"ST32-001"}))
	})

	It("reports an attack by a card it has as rested", func() {
		lines := append(header[:len(header):len(header)],
			"RZ1|1|1|ST32-001|0|49|1|0|1|1|0|0|0",
			"RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
			by(bob, "Chose to go Second"),
			"RZ1|2|1|Don|4|9|5|0|1|1|0|0|0",
			by(alice, "Draw 1 Don"),
			"RZ1|3|1|ST32-001|1|0|2|0|1|1|1|0|0",
			by(alice, "Deploy "+kinemon),
			by(alice, kinemon+" attacking "+link("Monkey D. Luffy", "OP13-001")),
			by(bob, "Concedes!"),
		)
		r := build(logOf(lines...))
		Expect(r.Issues).To(HaveLen(1))
		Expect(r.Issues[0].Kind).To(Equal(IssueRest))
		Expect(r.Issues[0].Detail).To(Equal("attacker ST32-001 is already rested"))
	})

	It("marks the replay unverified from the first count the log disagrees with", func() {
		lines := append(header[:len(header):len(header)],
			"RZ1|1|1|ST32-001|0|49|1|0|1|1|0|0|0",
			"RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
			by(bob, "Chose to go Second"),
			"RZ1|2|1|Don|4|9|5|0|1|1|0|0|0",
			"RZ1|CHK|2|1|49|1|0|0|9|1|0|0|1|0",
			by(alice, "Draw 1 Don"),
			by(alice, "End Turn"),
			"RZ1|3|1|ST32-001|1|0|2|0|1|1|0|0|0",
			"RZ1|CHK|3|1|49|1|1|0|9|1|0|0|1|0",
			by(alice, "Deploy "+kinemon),
		)
		r := build(logOf(lines...))
		Expect(r.Unverified).To(Equal(len(r.Steps) - 1))
		Expect(r.Issues[0].Kind).To(Equal(IssueCount))
		Expect(r.Issues[0].Detail).To(Equal("hand 0, check says 1"))
	})

	It("moves the attack to the blocker", func() {
		luffy := link("Monkey D. Luffy", "OP13-001")
		lines := append(header[:len(header):len(header)],
			"RZ1|1|1|ST32-001|0|49|1|0|1|1|0|0|0",
			"RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
			by(bob, "Chose to go Second"),
			by(alice, "Draw 1 Don"),
			by(alice, "End Turn"),
			by(bob, luffy+" attacking "+luffy),
			by(alice, kinemon+" Blocks"),
			luffy+"[5000] vs "+kinemon+"[6000]",
			"Attack Fails",
			by(bob, "End Turn"),
			by(alice, "Concedes!"),
		)
		r := build(logOf(lines...))
		var kinds []Kind
		var damage Step
		for _, s := range r.Steps {
			if s.Battle != nil {
				kinds = append(kinds, s.Kind)
			}
			if s.Kind == KindDamage {
				damage = s
			}
		}
		Expect(kinds).To(Equal([]Kind{KindAttack, KindBlock, KindDamage}))
		Expect(damage.Battle.Defender()).To(Equal("ST32-001"))
		Expect([]int{damage.Battle.AttackerPower, damage.Battle.TargetPower}).To(Equal([]int{5000, 6000}))
		Expect(damage.Outcome).To(Equal([]Outcome{{Kind: OutcomeFailed}}))
	})

	It("marks the attack that ended a lethal game", func() {
		lines := append(header[:len(header):len(header)],
			"RZ1|1|2|OP15-088|0|49|3|0|0|1|0|0|0",
			"RZ1|CHK|1|2|49|0|0|1|10|0|0|0|1|0",
			"RZ1|2|2|OP15-088|3|0|1|0|0|1|0|0|0",
			"RZ1|CHK|2|2|49|1|0|0|10|0|0|0|1|0",
			by(bob, "Chose to go Second"),
			by(alice, "Draw 1 Don"),
			by(alice, link("Monkey D. Luffy", "OP13-001")+" attacking "+link("Monkey D. Luffy", "OP13-001")),
		)
		r := build(logOf(lines...))
		last := r.Steps[len(r.Steps)-1]
		Expect(last.Kind).To(Equal(KindAttack))
		Expect(last.Lethal).To(BeTrue())
	})
})
