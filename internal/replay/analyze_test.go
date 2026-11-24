package replay

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/simlog"
)

func rzLobby(code string) []string {
	luffy := link("Monkey D. Luffy", "OP13-001")
	kid := link(`Eustass"Captain"Kid`, "OP17-058")
	return []string{
		"Attempting to connect to " + code,
		"Version is 1.44a.24",
		by(bob, "Leader is "+kid),
		"Alice\u200b#1111 Has Connected",
		"Version is 1.44a.24",
		by(alice, "Leader is "+luffy),
		by(bob, "Chose to go Second"),
		"RZ1|HDR|1.44a|2|RZ1",
		"RZ1|PLY|1|" + alice + "|OP13-001",
		"RZ1|PLY|2|" + bob + "|OP17-058",
	}
}

func boardCheck(owner, chars, life int) string {
	return fmt.Sprintf("RZ1|CHK|90|%d|30|5|%d|%d|5|5|8|0|1|0", owner, chars, life)
}

func rzMove(seq, owner int, code string, from, fromSlot, to, toSlot simlog.Zone) string {
	return fmt.Sprintf("RZ1|%d|%d|%s|%d|%d|%d|%d|0|0|0|0|0", seq, owner, code, from, fromSlot, to, toSlot)
}

var _ = Describe("Analyze", func() {
	const aliceID = "Alice#1111"
	luffy := link("Monkey D. Luffy", "OP13-001")
	kid := link(`Eustass"Captain"Kid`, "OP17-058")
	power := map[string]int{"OP06-033": 6000, "OP14-031": 5000, "EB01-013": 1000, "ST30-004": 2000}
	powerOf := func(code string) int { return power[code] }

	It("gives each player's life at the start and at the end of every turn", func() {
		lines := append(rzLobby("XNJ8M66"),
			boardCheck(1, 0, 0),
			boardCheck(1, 0, 5),
			boardCheck(2, 0, 5),
			by(alice, "Hand before Mulligan: [OP12-023,EB05-001,EB05-001,OP07-022,ST32-002]"),
			by(alice, "Draw 1 Don"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			boardCheck(1, 0, 4),
			by(bob, "End Turn"),
			by(alice, "Draw 1 Card"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			boardCheck(1, 0, 3),
			boardCheck(2, 0, 4),
			by(bob, "Concedes!"),
		)
		g := parseGame(logOf(lines...))
		a := Analyze(g, aliceID, bobID, nil)
		Expect(a.Me).To(Equal([]int{5, 5, 4, 4, 3}))
		Expect(a.Opp).To(Equal([]int{5, 5, 5, 5, 4}))
		Expect(Analyze(g, "Nobody#0", bobID, nil).Me).To(BeNil())
	})

	// game builds a valid RZ1 game whose turn 3 puts 3 characters on Alice's board and whose
	// turn 4 executes removals on Alice's board.
	game := func(removals ...string) *simlog.Game {
		lines := append(rzLobby("XNJ8M66"),
			rzMove(1, 1, "OP06-033", simlog.ZoneDeck, 49, simlog.ZoneHand, 0),
			"RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
			rzMove(2, 1, "EB01-013", simlog.ZoneDeck, 48, simlog.ZoneHand, 1),
			rzMove(3, 1, "ST30-004", simlog.ZoneDeck, 47, simlog.ZoneHand, 2),
			rzMove(4, 1, "OP05-097", simlog.ZoneDeck, 46, simlog.ZoneHand, 3),
			boardCheck(1, 0, 5), boardCheck(2, 0, 5),
			by(alice, "Hand before Mulligan: [OP06-033,EB01-013,ST30-004,OP05-097,OP14-031]"),
			by(alice, "Draw 1 Don"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			boardCheck(1, 0, 4),
			by(bob, "End Turn"),
			by(alice, "Draw 1 Card"),
			rzMove(10, 1, "OP06-033", simlog.ZoneHand, 0, simlog.ZoneCharacter, 0),
			rzMove(11, 1, "EB01-013", simlog.ZoneHand, 0, simlog.ZoneCharacter, 1),
			rzMove(12, 1, "EB01-013", simlog.ZoneCharacter, 1, simlog.ZoneTrash, 0),
			rzMove(13, 1, "OP14-031", simlog.ZoneDeck, 45, simlog.ZoneCharacter, 1),
			rzMove(14, 1, "ST30-004", simlog.ZoneHand, 0, simlog.ZoneCharacter, 2),
			rzMove(15, 1, "OP05-097", simlog.ZoneHand, 0, simlog.ZoneStage, 0),
			boardCheck(1, 3, 4),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
		)
		lines = append(lines, removals...)
		lines = append(lines,
			by(alice, link("Lilith", "OP13-113")+": Activate Trigger"),
			boardCheck(1, 3-len(removals), 1),
			by(bob, "End Turn"),
			by(alice, "Draw 1 Card"),
			by(alice, "Concedes!"),
		)
		return parseGame(logOf(lines...))
	}

	It("picks big turns, low life, a new board and the finish, and skips the first life taken", func() {
		g := game(
			rzMove(20, 1, "OP06-033", simlog.ZoneCharacter, 0, simlog.ZoneTrash, 1),
			rzMove(21, 1, "OP14-031", simlog.ZoneCharacter, 0, simlog.ZoneHand, 0),
		)
		a := Analyze(g, aliceID, bobID, powerOf)
		Expect(a.Replay).NotTo(BeNil())
		Expect(a.Finish).To(BeNil())
		ms := a.Moments
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
		lines := append(rzLobby("XNJ8M66"),
			rzMove(1, 1, "OP17-022", simlog.ZoneDeck, 49, simlog.ZoneHand, 0),
			"RZ1|CHK|1|1|49|1|0|0|10|0|0|0|1|0",
			boardCheck(1, 0, 5), boardCheck(2, 0, 2),
			by(alice, "Hand before Mulligan: [OP17-022,EB05-001,EB05-001,OP07-022,ST32-002]"),
			by(alice, "Draw 1 Don"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			by(bob, "End Turn"),
			by(alice, "Draw 1 Card"),
			rzMove(2, 1, "OP17-022", simlog.ZoneHand, 0, simlog.ZoneCharacter, 0),
			by(alice, "Deploy "+link("Shanks", "OP17-022")),
			boardCheck(2, 0, 0),
			by(alice, link("Shanks", "OP17-022")+" attacking "+kid),
			"Shanks [OP17-022][12000] vs "+kid+"[5000]",
			kid+" hit for 1 damage",
		)
		g := parseGame(logOf(lines...))
		Expect(g.End).To(Equal(simlog.End{Kind: simlog.EndLethal, Loser: bobID}))
		a := Analyze(g, aliceID, bobID, powerOf)
		Expect(a.Moments).To(HaveLen(1))
		Expect(a.Moments[0].Turn).To(Equal(3))
		Expect(a.Moments[0].Reasons).To(Equal([]Reason{
			{Kind: ReasonLifeLost, Player: bobID, N: 2},
			{Kind: ReasonAtZero, Player: bobID},
			{Kind: ReasonFinish, Player: bobID},
		}))
		Expect(a.Finish).To(HaveLen(1))
		Expect(a.Finish[0].Attacker).To(Equal("OP17-022"))
		Expect(a.Finish[0].Lethal).To(BeTrue())
	})

	It("needs 2 removals of 5000 power or more, or 3 of any power", func() {
		removed := func(g *simlog.Game) bool {
			for _, m := range Analyze(g, aliceID, bobID, powerOf).Moments {
				for _, r := range m.Reasons {
					if r.Kind == ReasonRemoved {
						return true
					}
				}
			}
			return false
		}
		Expect(removed(game(
			rzMove(20, 1, "OP06-033", simlog.ZoneCharacter, 0, simlog.ZoneTrash, 1),
			rzMove(21, 1, "ST30-004", simlog.ZoneCharacter, 1, simlog.ZoneDeck, 0),
		))).To(BeFalse())
		Expect(removed(game(
			rzMove(20, 1, "OP06-033", simlog.ZoneCharacter, 0, simlog.ZoneTrash, 1),
			rzMove(21, 1, "ST30-004", simlog.ZoneCharacter, 1, simlog.ZoneDeck, 0),
			rzMove(22, 1, "OP14-031", simlog.ZoneCharacter, 0, simlog.ZoneLife, 0),
		))).To(BeTrue())
	})

	It("reads counters, blocks, locks and characters defended or brought back, and flags a hand drain", func() {
		sleeptids := link("Sleeptids", "OP18-093")
		killingham := link("Saint Killingham", "OP18-064")
		gunko := link("Saint Gunko", "OP18-065")
		lines := append(rzLobby("XNJ8M66"),
			rzMove(1, 2, "OP18-064", simlog.ZoneDeck, 49, simlog.ZoneHand, 0),
			"RZ1|CHK|1|2|49|1|0|0|10|0|0|0|1|0",
			rzMove(2, 2, "OP18-093", simlog.ZoneDeck, 48, simlog.ZoneHand, 1),
			rzMove(3, 2, "OP18-065", simlog.ZoneDeck, 47, simlog.ZoneHand, 2),
			rzMove(4, 2, "OP18-093", simlog.ZoneDeck, 46, simlog.ZoneHand, 3),
			rzMove(5, 2, "OP15-078", simlog.ZoneDeck, 45, simlog.ZoneHand, 4),
			rzMove(6, 2, "OP18-085", simlog.ZoneDeck, 44, simlog.ZoneHand, 5),
			rzMove(7, 2, "OP18-083", simlog.ZoneDeck, 43, simlog.ZoneHand, 6),
			by(alice, "Draw 1 Don"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			rzMove(10, 2, "OP18-064", simlog.ZoneHand, 0, simlog.ZoneCharacter, 0),
			by(bob, "Deploy "+killingham),
			rzMove(11, 2, "OP18-093", simlog.ZoneHand, 0, simlog.ZoneCharacter, 1),
			by(bob, "Deploy "+sleeptids),
			rzMove(12, 2, "OP18-065", simlog.ZoneHand, 0, simlog.ZoneCharacter, 2),
			by(bob, "Deploy "+gunko),
			by(bob, "End Turn"),
			by(alice, "Draw 1 Card"),
			by(alice, link("Monkey D. Luffy", "OP18-118")+": "+sleeptids+" can't be rested until next turn ends"),
			by(alice, link("Monkey D. Luffy", "OP18-118")+": "+link("Sanji", "OP13-027")+" can't attack next turn"),
			by(alice, luffy+" attacking "+kid),
			by(bob, "Discard "+sleeptids+" for Counter 2000"),
			rzMove(20, 2, "OP18-093", simlog.ZoneHand, 0, simlog.ZoneTrash, 0),
			by(bob, link("Mamaragan", "OP15-078")+": Activate Counter"),
			rzMove(21, 2, "OP15-078", simlog.ZoneHand, 0, simlog.ZoneTrash, 1),
			luffy+"[5000] vs "+kid+"[8000]",
			"Attack Fails",
			by(alice, link("Roronoa Zoro", "EB04-007")+" attacking "+killingham),
			by(bob, "Discard "+link("Cerberus", "OP18-085")+" for Counter 2000"),
			rzMove(22, 2, "OP18-085", simlog.ZoneHand, 0, simlog.ZoneTrash, 2),
			by(bob, "Discard "+link("Saint Killingham", "OP18-083")+" for Counter 1000"),
			rzMove(23, 2, "OP18-083", simlog.ZoneHand, 0, simlog.ZoneTrash, 3),
			rzMove(24, 2, "OP18-064", simlog.ZoneCharacter, 0, simlog.ZoneTrash, 4),
			link("Roronoa Zoro", "EB04-007")+"[9000] vs "+killingham+"[9000]",
			by(bob, killingham+" Destroyed"),
			rzMove(25, 2, "OP18-064", simlog.ZoneTrash, 4, simlog.ZoneCharacter, 2),
			rzMove(26, 2, "OP18-065", simlog.ZoneCharacter, 1, simlog.ZoneTrash, 4),
			by(alice, link("Sanji", "OP13-027")+" attacking "+kid),
			by(bob, sleeptids+" Blocks"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			by(bob, kid+" attacking "+luffy),
			by(alice, "Discard "+link("Nami", "EB02-017")+" for Counter 1000"),
			by(alice, "Discard "+link("Nami", "EB02-017")+" for Counter 1000"),
			by(alice, "Discard "+link("Nami", "EB02-017")+" for Counter 1000"),
			by(alice, "Concedes!"),
		)
		g := parseGame(logOf(lines...))
		ms := Analyze(g, aliceID, bobID, func(code string) int {
			return map[string]int{"OP18-064": 6000, "OP18-065": 11000}[code]
		}).Moments
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

	It("does not mix counter cards across two copies of the same character in one turn", func() {
		killingham := link("Saint Killingham", "OP18-064")
		zoro := link("Roronoa Zoro", "EB04-007")
		sanji := link("Sanji", "OP13-027")
		cerberus := link("Cerberus", "OP18-085")
		lines := append(rzLobby("XNJ8M66"),
			rzMove(1, 2, "OP18-064", simlog.ZoneDeck, 49, simlog.ZoneHand, 0),
			"RZ1|CHK|1|2|49|1|0|0|10|0|0|0|1|0",
			rzMove(2, 2, "OP18-064", simlog.ZoneDeck, 48, simlog.ZoneHand, 1),
			rzMove(3, 2, "OP18-085", simlog.ZoneDeck, 47, simlog.ZoneHand, 2),
			rzMove(4, 2, "OP18-085", simlog.ZoneDeck, 46, simlog.ZoneHand, 3),
			rzMove(5, 2, "OP18-085", simlog.ZoneDeck, 45, simlog.ZoneHand, 4),
			rzMove(6, 1, "EB04-007", simlog.ZoneDeck, 49, simlog.ZoneHand, 0),
			"RZ1|CHK|6|1|49|1|0|0|10|0|0|0|1|0",
			rzMove(7, 1, "OP13-027", simlog.ZoneDeck, 48, simlog.ZoneHand, 1),
			by(alice, "Draw 1 Don"),
			rzMove(8, 1, "EB04-007", simlog.ZoneHand, 0, simlog.ZoneCharacter, 0),
			by(alice, "Deploy "+zoro),
			rzMove(9, 1, "OP13-027", simlog.ZoneHand, 0, simlog.ZoneCharacter, 1),
			by(alice, "Deploy "+sanji),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			rzMove(10, 2, "OP18-064", simlog.ZoneHand, 0, simlog.ZoneCharacter, 0),
			by(bob, "Deploy "+killingham),
			rzMove(11, 2, "OP18-064", simlog.ZoneHand, 0, simlog.ZoneCharacter, 1),
			by(bob, "Deploy "+killingham),
			by(bob, "End Turn"),
			by(alice, "Draw 1 Card"),
			// First copy is active, second copy is rested: attacks hit distinct physical copies.
			fmt.Sprintf("RZ1|12|2|OP18-064|%d|0|%d|0|0|0|1|0|0", simlog.ZoneCharacter, simlog.ZoneCharacter),
			by(alice, zoro+" attacking "+killingham),
			by(bob, "Discard "+cerberus+" for Counter 2000"),
			rzMove(13, 2, "OP18-085", simlog.ZoneHand, 0, simlog.ZoneTrash, 0),
			zoro+"[7000] vs "+killingham+"[8000]",
			"Attack Fails",
			// Set copy 0 active and rest copy 1 so the second attack targets the other physical copy.
			fmt.Sprintf("RZ1|14|2|OP18-064|%d|0|%d|0|0|0|0|0|0", simlog.ZoneCharacter, simlog.ZoneCharacter),
			fmt.Sprintf("RZ1|15|2|OP18-064|%d|1|%d|1|0|0|1|0|0", simlog.ZoneCharacter, simlog.ZoneCharacter),
			by(alice, sanji+" attacking "+killingham),
			by(bob, "Discard "+cerberus+" for Counter 2000"),
			rzMove(16, 2, "OP18-085", simlog.ZoneHand, 0, simlog.ZoneTrash, 1),
			rzMove(17, 2, "OP18-064", simlog.ZoneCharacter, 1, simlog.ZoneTrash, 2),
			sanji+"[9000] vs "+killingham+"[8000]",
			by(bob, killingham+" Destroyed"),
			by(alice, luffy+" attacking "+kid),
			by(bob, "Discard "+cerberus+" for Counter 2000"),
			rzMove(18, 2, "OP18-085", simlog.ZoneHand, 0, simlog.ZoneTrash, 3),
			luffy+"[5000] vs "+kid+"[7000]",
			"Attack Fails",
			by(alice, "End Turn"),
			by(bob, "Concedes!"),
		)
		g := parseGame(logOf(lines...))
		ms := Analyze(g, aliceID, bobID, nil).Moments
		Expect(ms).To(HaveLen(2))
		Expect(ms[0].Turn).To(Equal(3))
		Expect(ms[0].Countered).To(HaveLen(3))
		Expect(ms[0].Defended).To(BeEmpty())
	})

	It("marks each removal and Trigger with its owner and turn, but not a player trashing their own", func() {
		g := game(rzMove(20, 1, "OP06-033", simlog.ZoneCharacter, 0, simlog.ZoneTrash, 1))
		marks := Analyze(g, aliceID, bobID, powerOf).Marks
		for i := range marks {
			marks[i].Line = 0
		}
		Expect(marks).To(Equal([]Mark{
			{Kind: MarkRemoved, Turn: 4, Player: aliceID, Card: "OP06-033"},
			{Kind: MarkTrigger, Turn: 4, Player: aliceID, Card: "OP13-113"},
		}))
	})

	It("derives locks from replay statuses on real RZ1 games and returns Finish and Replay in one call", func() {
		text, err := os.ReadFile("testdata/no-rest-copy.log")
		Expect(err).NotTo(HaveOccurred())
		g := parseGame(string(text))
		a := Analyze(g, aliceID, bobID, nil)
		Expect(a.Replay).NotTo(BeNil())
		Expect(a.Me).NotTo(BeEmpty())
		Expect(a.Opp).NotTo(BeEmpty())
		locksByTurn := make(map[int][]Lock)
		for _, m := range a.Moments {
			if len(m.Locks) > 0 {
				locksByTurn[m.Turn] = m.Locks
			}
		}
		Expect(locksByTurn[10]).To(Equal([]Lock{{Source: "OP18-118", Cards: []string{"OP18-119"}, Rested: true}}))
		Expect(locksByTurn[11]).To(Equal([]Lock{{Source: "OP18-064", Cards: []string{"EB02-017", "OP18-034"}, Rested: false}}))

		lethalText, err := os.ReadFile("testdata/lethal-turn.log")
		Expect(err).NotTo(HaveOccurred())
		lg := parseGame(string(lethalText))
		la := Analyze(lg, aliceID, bobID, func(code string) int {
			return map[string]int{"EB04-007": 9000, "OP18-060": 5000, "OP13-001": 5000}[code]
		})
		Expect(la.Replay).NotTo(BeNil())
		Expect(la.Finish).To(HaveLen(6))
		Expect(la.Finish[5].Lethal).To(BeTrue())
		Expect(la.Finish[5].Before).To(Equal([]Action{{
			Card:    "OP15-032",
			Played:  true,
			Effects: []ActionEffect{{Text: "Rest Saint Gunko", Targets: []string{"OP18-065"}}},
		}}))
	})

	It("analyzes pre-RZ1 text-only logs using printouts and readable lines", func() {
		sleeptids := link("Sleeptids", "OP18-093")
		killingham := link("Saint Killingham", "OP18-064")
		lines := []string{
			"Attempting to connect to XNJ8M66",
			"Version is 1.25a",
			by(bob, "Leader is "+kid),
			"Alice\u200b#1111 Has Connected",
			"Version is 1.25a",
			by(alice, "Leader is "+luffy),
			by(bob, "Chose to go Second"),
			by(alice, "Hand: [OP06-033,EB01-013,OP14-031,ST30-004,OP05-097]"),
			by(alice, "Board: []"),
			by(alice, "Life: 5"),
			by(bob, "Hand: [OP18-064,OP18-093,OP18-085,OP18-083,OP15-078]"),
			by(bob, "Board: []"),
			by(bob, "Life: 5"),
			by(alice, "Draw 1 Don"),
			by(alice, "Hand: [OP06-033,EB01-013,OP14-031,ST30-004,OP05-097]"),
			by(alice, "Board: []"),
			by(alice, "Life: 5"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			by(bob, "Deploy "+killingham),
			by(bob, "Deploy "+sleeptids),
			by(bob, "Hand: [OP18-085,OP18-083,OP15-078]"),
			by(bob, "Board: [OP18-064,OP18-093]"),
			by(bob, "Life: 5"),
			by(bob, "End Turn"),
			by(alice, "Draw 1 Card"),
			by(alice, link("Monkey D. Luffy", "OP18-118")+": "+sleeptids+" can't be rested until next turn ends"),
			by(alice, luffy+" attacking "+kid),
			by(bob, "Discard "+sleeptids+" for Counter 2000"),
			by(bob, link("Mamaragan", "OP15-078")+": Activate Counter"),
			"Attack Fails",
			by(alice, link("Roronoa Zoro", "EB04-007")+" attacking "+killingham),
			by(bob, "Discard "+link("Cerberus", "OP18-085")+" for Counter 2000"),
			by(bob, "Discard "+link("Saint Killingham", "OP18-083")+" for Counter 1000"),
			by(bob, killingham+" Destroyed"),
			by(bob, link("Lilith", "OP13-113")+": Activate Trigger"),
			by(bob, "Hand: [OP18-088]"),
			by(bob, "Board: [OP18-093]"),
			by(bob, "Life: 3"),
			by(alice, "End Turn"),
			by(bob, "Concedes!"),
		}
		g := parseGame(logOf(lines...))
		Expect(g.HasRZ()).To(BeFalse())
		a := Analyze(g, aliceID, bobID, powerOf)
		Expect(a.Replay).To(BeNil())
		Expect(a.Finish).To(BeNil())
		Expect(a.Me).To(Equal([]int{5, 5, 5, 5, 5}))
		Expect(a.Opp).To(Equal([]int{5, 5, 5, 3, 3}))
		Expect(a.OppBoard).To(Equal([]int{0, 0, 2, 1, 1}))
		Expect(a.Moments).To(HaveLen(2))
		Expect(a.Moments[0].Turn).To(Equal(3))
		Expect(a.Moments[0].Countered).To(Equal([]string{"OP18-093", "OP15-078", "OP18-085", "OP18-083"}))
		Expect(a.Moments[0].Defended).To(Equal([]Defense{{Card: "OP18-064", Cards: 2}}))
		Expect(a.Moments[0].Locks).To(Equal([]Lock{{Source: "OP18-118", Cards: []string{"OP18-093"}, Rested: true}}))
		Expect(a.Moments[0].Reasons).To(ConsistOf(
			Reason{Kind: ReasonHandDrain, Player: bobID, N: 4},
			Reason{Kind: ReasonLifeLost, Player: bobID, N: 2},
		))
		Expect(a.Moments[1].Turn).To(Equal(4))
		Expect(a.Moments[1].Reasons).To(Equal([]Reason{{Kind: ReasonFinish, Player: bobID}}))
		Expect(a.Marks).To(HaveLen(2))
		Expect(a.Marks[0].Kind).To(Equal(MarkRemoved))
		Expect(a.Marks[0].Card).To(Equal("OP18-064"))
		Expect(a.Marks[1].Kind).To(Equal(MarkTrigger))
		Expect(a.Marks[1].Card).To(Equal("OP13-113"))
	})
})
