package replay

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// lethal-turn.log is a whole real game with player names and the room replaced. Alice's
// Luffy deals lethal on turn 15 through Bob's blockers.
var _ = Describe("Attacks", func() {
	It("lists the lethal turn's attacks with the DON open, the answers and what came between", func() {
		text, err := os.ReadFile("testdata/lethal-turn.log")
		Expect(err).NotTo(HaveOccurred())
		g := parseGame(string(text))
		r, err := Build(g, func(code string) int {
			return map[string]int{"EB04-007": 9000, "OP18-060": 5000, "OP13-001": 5000}[code]
		})
		Expect(err).NotTo(HaveOccurred())

		as := r.Attacks(g, 15)
		Expect(as).To(HaveLen(6))
		attackers := make([]string, len(as))
		for i, a := range as {
			attackers[i] = a.Attacker
		}
		Expect(attackers).To(Equal([]string{"OP13-027", "OP13-027", "OP18-118", "OP18-118", "OP13-001", "EB04-007"}))

		Expect(as[0].DonActive).To(Equal(10))
		Expect(as[0].Answers).To(Equal([]Answer{
			{Kind: AnswerEvent, Card: "OP15-078", Power: 1000},
			{Kind: AnswerCounter, Card: "OP18-093", Power: 2000},
		}))
		Expect(as[0].Outcome).To(Equal(OutcomeFailed))
		Expect(as[1].Blocker).To(Equal("OP18-093"))
		Expect(as[2].Outcome).To(Equal(OutcomeHit))
		Expect(as[2].Life).To(Equal(0))
		Expect(as[3].Answers).To(Equal([]Answer{
			{Kind: AnswerBlock, Card: "OP18-065"},
			{Kind: AnswerCounter, Card: "OP18-088", Power: 2000},
		}))

		last := as[5]
		Expect(last.Before).To(Equal([]Action{{Card: "OP15-032", Played: true, Effects: []string{"Rest Saint Gunko [OP18-065]"}}}))
		Expect(last.Lethal).To(BeTrue())
		Expect(last.Power).To(Equal(9000))
		Expect(last.DonActive).To(Equal(0))
		for _, a := range as[:5] {
			Expect(a.Lethal).To(BeFalse())
		}
	})

	It("rests the copy the RZ1 stream rested when one of three copies blocks", func() {
		text, err := os.ReadFile("testdata/lethal-turn.log")
		Expect(err).NotTo(HaveOccurred())
		g := parseGame(string(text))
		r, err := Build(g, nil)
		Expect(err).NotTo(HaveOccurred())
		active := func(s Step) int {
			n := 0
			for _, c := range s.Board.Sides[1].Characters {
				if c.Code == "OP18-065" && !c.Rested {
					n++
				}
			}
			return n
		}
		var blocks []int
		for _, s := range r.Steps {
			if s.Turn == 15 && s.Kind == KindBlock && s.Cards[0] == "OP18-065" {
				blocks = append(blocks, active(s))
			}
			if s.Turn == 15 && s.Kind == KindPlay && s.Cards[0] == "OP15-032" {
				Expect(active(s)).To(Equal(0))
			}
		}
		Expect(blocks).To(Equal([]int{2, 1}))
	})

	// no-rest-copy.log is a whole real game with player names and the room replaced. On turn 10
	// Alice's Luffy says one of Bob's two Saint Shamrocks can't be rested, without saying which.
	// On turn 11 the RZ1 stream rests one of them to attack.
	It("moves can't be rested off the copy that attacks, since attacking rests it", func() {
		text, err := os.ReadFile("testdata/no-rest-copy.log")
		Expect(err).NotTo(HaveOccurred())
		g := parseGame(string(text))
		r, err := Build(g, nil)
		Expect(err).NotTo(HaveOccurred())
		for _, is := range r.Issues {
			Expect(is.Kind).NotTo(Equal(IssueNoAttack), is.String())
		}
		var attack *Step
		for i, s := range r.Steps {
			if s.Turn == 11 && s.Kind == KindAttack && s.Battle.Attacker == "OP18-119" {
				attack = &r.Steps[i]
				break
			}
		}
		Expect(attack).NotTo(BeNil())
		bob := attack.Board.Side(attack.Battle.AttackerOwner)
		locked := 0
		for _, c := range bob.Characters {
			if c.Code != "OP18-119" {
				continue
			}
			if c.ID == attack.Battle.AttackerID {
				Expect(c.Rested).To(BeTrue())
				Expect(c.has(StatusNoRest)).To(BeFalse())
				continue
			}
			Expect(c.has(StatusNoRest)).To(BeTrue())
			locked++
		}
		Expect(locked).To(Equal(1))
	})
})
