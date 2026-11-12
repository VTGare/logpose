package simlog

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("opening hands", func() {
	const aliceID, bobID = "Alice#1111", "Bob#2222"
	drew := func(player string, codes ...string) []string {
		var out []string
		for _, code := range codes {
			out = append(out, by(player, "Drew card from deck: "+card("Card", code)))
		}
		return out
	}
	game := func(pregame ...string) Game {
		lines := append(lobby("XNJ8M66"), pregame...)
		lines = append(lines,
			by(alice, "Draw 1 Don"),
			by(alice, "End Turn"),
			by(bob, "Draw 1 Card"),
			by(bob, "Drew card from deck: "+card("Late", "OP01-099")),
			by(bob, "End Turn"),
		)
		return parse(logOf(lines...))[0]
	}

	It("reads the opponent's hand from their opening draws", func() {
		g := game(drew(bob, "OP17-046", "OP17-045", "OP17-054", "OP17-040", "OP17-040")...)
		Expect(g.Mulligans[bobID]).To(Equal(Mulligan{Before: []string{"OP17-046", "OP17-045", "OP17-054", "OP17-040", "OP17-040"}}))
	})

	It("splits a mulligan into the hand before and after it", func() {
		lines := drew(bob, "OP01-055", "ST32-003", "ST32-002", "OP08-036", "OP17-031",
			"OP06-038", "OP12-023", "ST24-004", "OP17-022", "OP06-033")
		g := game(append(lines, by(bob, "Mulligan"))...)
		Expect(g.Mulligans[bobID]).To(Equal(Mulligan{
			Before:     []string{"OP01-055", "ST32-003", "ST32-002", "OP08-036", "OP17-031"},
			Mulliganed: true,
			After:      []string{"OP06-038", "OP12-023", "ST24-004", "OP17-022", "OP06-033"},
		}))
	})

	It("keeps the opening draws out of turn 1 when the host's log starts with the relay lines", func() {
		lines := append([]string{"Setting up a Relay Room...", "Connected to Server: europe-north1"}, lobby("XNJ8M66")...)
		lines = append(lines, drew(bob, "OP01-055", "ST32-003", "ST32-002", "OP08-036", "OP17-031")...)
		lines = append(lines, by(alice, "Draw 1 Don"), by(alice, "End Turn"))
		g := parse(logOf(lines...))[0]
		draws := 0
		for _, l := range g.Lines {
			if strings.HasPrefix(l.Text, "Drew card from deck") {
				Expect(l.Turn).To(BeZero(), l.Text)
				draws++
			}
		}
		Expect(draws).To(Equal(5))
		Expect(g.Mulligans[bobID].Before).To(HaveLen(5))
	})

	It("keeps the hand the log states over the draws", func() {
		lines := drew(alice, "OP12-023", "EB05-001", "EB05-001", "OP07-022", "ST32-002")
		lines = append(lines, by(alice, "Hand before Mulligan: [OP12-023,EB05-001,EB05-001,OP07-022,ST32-002]"))
		g := game(lines...)
		Expect(g.Mulligans[aliceID].Before).To(Equal([]string{"OP12-023", "EB05-001", "EB05-001", "OP07-022", "ST32-002"}))
	})
})
