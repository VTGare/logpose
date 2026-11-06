package simlog

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The fixtures follow real AutoSaved logs from sim 1.44a, trimmed, with player names replaced.

const (
	alice = "Alice\u200b#1111"
	bob   = "Bob\u200b#2222"
)

func card(name, code string) string {
	return fmt.Sprintf(`%s [<mark><link="%s">%s</link></mark>]`, name, code, code)
}

func by(player, text string) string { return "[" + player + "] " + text }

func logOf(lines ...string) string { return strings.Join(lines, "\r\n") + "\r\n" }

func parse(log string) []Game {
	games, err := Parse(strings.NewReader(log))
	Expect(err).NotTo(HaveOccurred())
	return games
}

var luffy, kid = card("Monkey D. Luffy", "OP13-001"), card("Eustass\"Captain\"Kid", "OP17-058")

func lobby(code string) []string {
	return []string{
		"Attempting to connect to " + code,
		"Version is 1.44a.24",
		by(bob, "Leader is "+kid),
		"Alice\u200b#1111 Has Connected",
		"Version is 1.44a.24",
		by(alice, "Leader is "+luffy),
		by(bob, "Chose to go Second"),
		"RZ1|HDR|1.44a|2|RZ1",
		"RZ1|PLY|1|Alice\u200b#1111|OP13-001",
		"RZ1|PLY|2|Bob\u200b#2222|OP17-058",
	}
}

func opening() []string {
	return []string{
		"RZ1|1|1|OP12-023|0|49|1|0|1|1|0|0|0",
		"RZ1|CHK|1|1|49|1|0|5|10|0|0|0|1|0",
		by(alice, "Hand before Mulligan: [OP12-023,EB05-001,EB05-001,OP07-022,ST32-002]"),
		by(alice, "Mulligan"),
		by(alice, "Hand after Mulligan: [EB01-013,OP06-033,OP12-023,OP14-031,ST30-004]"),
		by(alice, "Draw 1 Don"),
		by(alice, "End Turn"),
		by(bob, "Draw 1 Card"),
		by(bob, "End Turn"),
	}
}

func lifeCheck(owner, life int) string {
	return fmt.Sprintf("RZ1|CHK|90|%d|30|5|2|%d|5|5|8|0|1|0", owner, life)
}

var _ = Describe("Parse", func() {
	It("reads the lobby, players, turn order and mulligan", func() {
		games := parse(logOf(append(lobby("XNJ8M66"), opening()...)...))
		Expect(games).To(HaveLen(1))
		g := games[0]
		Expect(g.Lobby).To(Equal(Lobby{Code: "XNJ8M66", Role: RoleJoin}))
		Expect(g.SimVersion).To(Equal("1.44a.24"))
		Expect(g.Players).To(ConsistOf(
			Player{Name: "Alice", Tag: "1111", Owner: 1, Leader: "OP13-001"},
			Player{Name: "Bob", Tag: "2222", Owner: 2, Leader: "OP17-058"},
		))
		Expect(g.First).To(Equal("Alice#1111"))
		Expect(g.Turns).To(Equal(2))
		m := g.Mulligans["Alice#1111"]
		Expect(m.Mulliganed).To(BeTrue())
		Expect(m.Before).To(HaveLen(5))
		Expect(m.After).To(ContainElement("OP14-031"))
	})

	It("tells OPBounty lobbies from the sim's own", func() {
		Expect(Lobby{Code: "XNJ8M66"}.OPBounty()).To(BeTrue())
		Expect(Lobby{Code: "7KQ2MP"}.OPBounty()).To(BeFalse())
		Expect(Lobby{}.OPBounty()).To(BeFalse())
	})

	It("reads the host's room code", func() {
		lines := append([]string{"Waiting for a Connection with Room ID:UHJ6MGP"}, lobby("")[1:]...)
		games := parse(logOf(append(lines, opening()...)...))
		Expect(games[0].Lobby).To(Equal(Lobby{Code: "UHJ6MGP", Role: RoleHost}))
	})

	It("cleans card links and rich text in lines", func() {
		games := parse(logOf(append(lobby("XNJ8M66"), append(opening(),
			by(alice, card("Jewelry Bonney", "EB05-001")+": Deploy "+card("Nami", "OP14-031")),
			by(bob, "<b><size=24>gg</size></b>"),
		)...)...))
		lines := games[0].Lines
		deploy := lines[len(lines)-2]
		Expect(deploy.Player).To(Equal("Alice#1111"))
		Expect(deploy.Text).To(Equal("Jewelry Bonney [EB05-001]: Deploy Nami [OP14-031]"))
		Expect(deploy.Cards).To(Equal([]string{"EB05-001", "OP14-031"}))
		Expect(deploy.Turn).To(Equal(3))
		Expect(lines[len(lines)-1].Text).To(Equal("gg"))
	})

	It("puts the lobby and mulligan in turn 0", func() {
		games := parse(logOf(append(lobby("XNJ8M66"), opening()...)...))
		for _, l := range games[0].Lines {
			if strings.HasPrefix(l.Text, "Hand before Mulligan") || strings.HasPrefix(l.Text, "Leader is") {
				Expect(l.Turn).To(Equal(0), l.Text)
			}
			if l.Text == "Draw 1 Don" {
				Expect(l.Turn).To(Equal(1))
			}
		}
	})

	It("reads RZ1 moves and checks", func() {
		g := parse(logOf(append(lobby("XNJ8M66"), opening()...)...))[0]
		Expect(g.Moves).To(HaveLen(1))
		m := g.Moves[0]
		Expect([]int{m.Seq, m.Owner, m.FromSlot}).To(Equal([]int{1, 1, 49}))
		Expect(m.Card).To(Equal("OP12-023"))
		Expect([]Zone{m.From, m.To}).To(Equal([]Zone{ZoneDeck, ZoneHand}))
		Expect(g.Checks[0].Life).To(Equal(5))
		Expect(g.Checks[0].DonDeck).To(Equal(10))
	})

	Describe("how a game ended", func() {
		game := func(end ...string) Game {
			return parse(logOf(append(append(lobby("XNJ8M66"), opening()...), end...)...))[0]
		}

		It("reads a concession", func() {
			Expect(game(by(bob, "Concedes!")).End).To(Equal(End{Kind: EndConcede, Loser: "Bob#2222"}))
		})

		It("reads a deck out", func() {
			g := game(by(alice, "Is Out of Cards, Loses the Game!"))
			Expect(g.End).To(Equal(End{Kind: EndDeckOut, Loser: "Alice#1111"}))
		})

		It("finds a lethal hit on a leader at 0 life", func() {
			g := game(
				lifeCheck(2, 0),
				by(alice, card("Shanks", "OP17-022")+" attacking "+kid),
				"Shanks [OP17-022][12000] vs "+kid+"[5000]",
				kid+" hit for 1 damage",
			)
			Expect(g.End).To(Equal(End{Kind: EndLethal, Loser: "Bob#2222"}))
		})

		It("finds a lethal attack the log stops at", func() {
			g := game(lifeCheck(2, 0), by(alice, card("Shanks", "OP17-022")+" attacking "+kid))
			Expect(g.End).To(Equal(End{Kind: EndLethal, Loser: "Bob#2222"}))
		})

		It("tells mirror leaders apart by who attacked", func() {
			mirror := []string{
				"Attempting to connect to XNJ8M66",
				by(bob, "Leader is "+luffy),
				by(alice, "Leader is "+luffy),
				"RZ1|HDR|1.44a|2|RZ1",
				"RZ1|PLY|1|Alice\u200b#1111|OP13-001",
				"RZ1|PLY|2|Bob\u200b#2222|OP13-001",
			}
			g := parse(logOf(append(append(mirror, opening()...),
				lifeCheck(1, 3), lifeCheck(2, 0),
				by(alice, card("Nami", "OP14-031")+" attacking "+luffy),
				luffy+" hit for 1 damage",
			)...))[0]
			Expect(g.End).To(Equal(End{Kind: EndLethal, Loser: "Bob#2222"}))
		})

		It("doesn't count a hit that took the last life card", func() {
			g := game(
				lifeCheck(2, 1),
				by(alice, card("Shanks", "OP17-022")+" attacking "+kid),
				"RZ1|91|2|OP17-060|3|0|1|6|1|1|0|0|0",
				lifeCheck(2, 0),
				kid+" hit for 1 damage",
			)
			Expect(g.End.Kind).To(Equal(EndUnknown))
		})

		It("doesn't count an attack that failed", func() {
			g := game(lifeCheck(2, 0), by(alice, card("Shanks", "OP17-022")+" attacking "+kid), "Attack Fails")
			Expect(g.End.Kind).To(Equal(EndUnknown))
		})

		It("doesn't count an attack on a character", func() {
			g := game(lifeCheck(2, 0), by(alice, card("Shanks", "OP17-022")+" attacking "+card("Killer", "OP17-060")))
			Expect(g.End.Kind).To(Equal(EndUnknown))
		})
	})

	Describe("logs without RZ1 player lines", func() {
		noPly := func(end ...string) Game {
			lines := []string{
				"Attempting to connect to 7KQ2MP",
				by(bob, "Leader is "+kid),
				by(alice, "Leader is "+luffy),
				"RZ1|HDR|1.44a|2|RZ1",
			}
			for i, c := range []string{"OP12-023", "EB05-001", "EB05-001", "OP07-022", "ST32-002"} {
				lines = append(lines, fmt.Sprintf("RZ1|%d|2|%s|0|%d|1|%d|1|1|0|0|0", i+1, c, 49-i, i))
			}
			lines = append(lines, by(alice, "Hand before Mulligan: [OP12-023,EB05-001,EB05-001,OP07-022,ST32-002]"))
			lines = append(lines, opening()[5:]...)
			return parse(logOf(append(lines, end...)...))[0]
		}

		It("links players to owners by their opening hand", func() {
			g := noPly()
			a, _ := g.Player("Alice#1111")
			b, _ := g.Player("Bob#2222")
			Expect(a.Owner).To(Equal(2))
			Expect(b.Owner).To(Equal(1))
		})

		It("still finds a lethal attack", func() {
			g := noPly(lifeCheck(1, 0), by(alice, card("Shanks", "OP17-022")+" attacking "+kid))
			Expect(g.End).To(Equal(End{Kind: EndLethal, Loser: "Bob#2222"}))
		})
	})

	Describe("rematches", func() {
		first := append(append(lobby("XNJ8M66"), opening()...), by(bob, "Concedes!"))
		// A new shuffle, so the opening draw differs from the first game's.
		reshuffled := append([]string{"RZ1|1|1|OP14-031|0|49|1|0|1|1|0|0|0"}, opening()[1:]...)
		second := append(append(lobby("XNJ8M66")[1:], reshuffled...), by(alice, "Concedes!"))

		It("splits games at the rematch line", func() {
			games := parse(logOf(append(append(first, rematchLine), second...)...))
			Expect(games).To(HaveLen(2))
			Expect(games[0].End.Loser).To(Equal("Bob#2222"))
			Expect(games[1].End.Loser).To(Equal("Alice#1111"))
		})

		It("gives a game the same fingerprint in every file it's saved in", func() {
			alone := parse(logOf(first...))
			later := parse(logOf(append(append(append(first, by(alice, "gg")), rematchLine), second...)...))
			Expect(later[0].Fingerprint).To(Equal(alone[0].Fingerprint))
			Expect(later[1].Fingerprint).NotTo(Equal(alone[0].Fingerprint))
		})

		It("gives a rematch the lobby of the game before it", func() {
			games := parse(logOf(append(append(first, rematchLine), second...)...))
			Expect(games[1].Lobby).To(Equal(Lobby{Code: "XNJ8M66", Role: RoleJoin}))
		})

		It("keeps each game's text, which parses to the same game", func() {
			games := parse(logOf(append(append(first, rematchLine), second...)...))
			for _, g := range games {
				again := parse(g.Text)
				Expect(again).To(HaveLen(1))
				Expect(again[0].Fingerprint).To(Equal(g.Fingerprint))
				Expect(again[0].Lobby).To(Equal(g.Lobby))
				Expect(again[0].End).To(Equal(g.End))
				Expect(again[0].Text).To(Equal(g.Text))
			}
			Expect(games[1].Text).NotTo(ContainSubstring(rematchLine))
		})

		It("leaves out a lobby that was opened but never played", func() {
			games := parse(logOf(append(append(first, rematchLine), lobby("XNJ8M66")[1:]...)...))
			Expect(games).To(HaveLen(1))
		})
	})
})
