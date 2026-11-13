package simsave

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simlog"
)

// keys reads the keys of a JSON object in the order they are written.
func keys(raw json.RawMessage) []string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	_, err := dec.Token()
	Expect(err).NotTo(HaveOccurred())
	var out []string
	for dec.More() {
		k, err := dec.Token()
		Expect(err).NotTo(HaveOccurred())
		out = append(out, k.(string))
		var skip json.RawMessage
		Expect(dec.Decode(&skip)).To(Succeed())
	}
	return out
}

func cardsOf(p player) []cardRef {
	out := append(append(append(append([]cardRef{}, p.Hand...), p.Trash...), p.Deck...), p.Life...)
	for _, c := range p.Board {
		out = append(out, c.cardRef)
	}
	for _, s := range p.Stage {
		out = append(out, s.cardRef)
	}
	return out
}

var _ = Describe("Export", func() {
	var (
		data  []byte
		rep   Result
		turns []turn
	)

	BeforeEach(func() {
		text, err := os.ReadFile("testdata/four-turns.log")
		Expect(err).NotTo(HaveOccurred())
		games, err := simlog.Parse(bytes.NewReader(text))
		Expect(err).NotTo(HaveOccurred())
		r, err := replay.Build(&games[len(games)-1], nil)
		Expect(err).NotTo(HaveOccurred())
		info := func(code string) (int, int) {
			if code == "EB05-010" {
				return 5000, 4
			}
			return 1000, 0
		}
		data, rep, err = Export(r, info, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(data, &turns)).To(Succeed())
	})

	It("writes one entry per turn with the first player as player 1", func() {
		Expect(rep.Turns).To(Equal(4))
		Expect(turns).To(HaveLen(4))
		for i, t := range turns {
			Expect(t.TurnNumber).To(Equal(i + 1))
			Expect(t.IsPlayer1TurnStarting).To(Equal(i%2 == 0))
		}
		Expect(turns[0].Player1.LeaderCard.CardID).To(Equal("EB05-010"))
		Expect(turns[0].Player1.LeaderCard.Card.CardLife).To(Equal(4))
		Expect(turns[0].Player1.LeaderCard.Card.CardPower).To(Equal(5000))
	})

	It("writes the board before the turn's draw", func() {
		Expect(turns[1].Player2.Hand).To(HaveLen(5))
		Expect(turns[0].Player1.ActiveDon + turns[0].Player1.RestedDon).To(Equal(0))
	})

	It("keeps every card with a code and one ID for the whole game", func() {
		seen := make(map[int]string)
		for _, t := range turns {
			for n, p := range []player{t.Player1, t.Player2} {
				cards := cardsOf(p)
				Expect(cards).To(HaveLen(50))
				Expect(p.LeaderCard.UniqueDeckID).To(Equal([]int{1, -1}[n]))
				for _, c := range cards {
					Expect(c.CardID).NotTo(BeEmpty())
					Expect(c.UniqueDeckID % 10).To(Equal(0))
					Expect(c.UniqueDeckID > 0).To(Equal(n == 0))
					if code, ok := seen[c.UniqueDeckID]; ok {
						Expect(c.CardID).To(Equal(code))
					}
					seen[c.UniqueDeckID] = c.CardID
				}
			}
		}
		Expect(seen).To(HaveLen(100))
	})

	It("guesses the deck card the opening shuffle never named", func() {
		Expect(rep.Guessed).To(Equal(1))
	})

	It("takes a leader's life from its life cards when the card data doesn't know it", func() {
		Expect(turns[0].Player2.LeaderCard.Card.CardLife).To(Equal(len(turns[0].Player2.Life)))
		Expect(turns[0].Player2.LeaderCard.Card.CardLife).To(BeNumerically(">", 0))
	})

	It("writes the fields in the sim's order", func() {
		var raw []map[string]json.RawMessage
		Expect(json.Unmarshal(data, &raw)).To(Succeed())
		Expect(keys(data[1:bytes.Index(data, []byte(`,{"turnNumber"`))])).
			To(Equal([]string{"turnNumber", "player1", "player2", "isPlayer1TurnStarting"}))
		Expect(keys(raw[0]["player1"])).To(Equal([]string{
			"hand", "trash", "deck", "board", "life", "stage", "leaderCard", "activeDon", "restedDon", "givenDon",
		}))
		var p map[string]json.RawMessage
		Expect(json.Unmarshal(raw[0]["player1"], &p)).To(Succeed())
		Expect(keys(p["leaderCard"])).To(Equal([]string{"attachedDon", "card", "cardId", "uniqueDeckId", "isFaceUp"}))
		var leader map[string]json.RawMessage
		Expect(json.Unmarshal(p["leaderCard"], &leader)).To(Succeed())
		card := keys(leader["card"])
		Expect(card).To(HaveLen(77))
		Expect(card[:4]).To(Equal([]string{"cardDef", "deckUniqueID", "cardLife", "cardPower"}))
		Expect(string(leader["card"])).To(ContainSubstring(`"lGrantedActions":[]`))
	})

	It("marks characters that came down in the previous turn as summon sick", func() {
		var sick int
		for _, t := range turns {
			for _, c := range append(t.Player1.Board, t.Player2.Board...) {
				if c.Card.BSummonSick {
					sick++
					Expect(c.Card.BFirstTurn).To(BeTrue())
				}
			}
		}
		Expect(sick).To(BeNumerically(">", 0))
	})
})

var _ = Describe("Export of a game conceded before turn 1's draw", func() {
	It("writes the board turn 1 starts with", func() {
		text, err := os.ReadFile("testdata/four-turns.log")
		Expect(err).NotTo(HaveOccurred())
		setup, _, ok := strings.Cut(string(text), "RZ1|170|")
		Expect(ok).To(BeTrue())
		games, err := simlog.Parse(strings.NewReader(setup + "[Bob\u200b#2222] Concedes!\r\n"))
		Expect(err).NotTo(HaveOccurred())
		r, err := replay.Build(&games[len(games)-1], nil)
		Expect(err).NotTo(HaveOccurred())
		data, res, err := Export(r, func(string) (int, int) { return 0, 0 }, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Turns).To(Equal(1))
		var turns []turn
		Expect(json.Unmarshal(data, &turns)).To(Succeed())
		Expect(turns[0].Player1.Hand).To(HaveLen(5))
		Expect(turns[0].Player1.Life).To(HaveLen(4))
	})
})

var _ = Describe("guess", func() {
	// deckOf is cards 0, 1, ... with the codes given, then n unnamed cards.
	deckOf := func(n int, codes ...string) (*exporter, []int) {
		e := &exporter{codes: make(map[int]string)}
		var ids []int
		for i, c := range append(codes, make([]string, n)...) {
			e.codes[i] = c
			ids = append(ids, i)
		}
		return e, ids
	}
	fours := func(codes ...string) []string {
		var out []string
		for _, c := range codes {
			out = append(out, c, c, c, c)
		}
		return out
	}

	It("names a card after the code missing its fourth copy", func() {
		e, ids := deckOf(1, "A", "A", "A", "B", "B")
		Expect(e.guess(ids, nil)).To(Equal(1))
		Expect(e.codes[5]).To(Equal("A"))
	})

	It("takes the player's other lists with the leader first", func() {
		e, ids := deckOf(2, "A", "A", "A", "B", "B")
		guessed, left := e.guess(ids, map[string]int{"A": 3, "B": 4, "C": 1})
		Expect([]int{guessed, left}).To(Equal([]int{2, 0}))
		Expect([]string{e.codes[5], e.codes[6]}).To(Equal([]string{"B", "B"}))
	})

	It("can name a card after one this game never showed", func() {
		e, ids := deckOf(1, fours("A", "B")...)
		Expect(e.guess(ids, map[string]int{"A": 4, "C": 2})).To(Equal(1))
		Expect(e.codes[8]).To(Equal("C"))
	})

	It("leaves a card out rather than give a code a fifth copy", func() {
		e, ids := deckOf(2, fours("A", "B")...)
		guessed, left := e.guess(ids, map[string]int{"A": 4, "B": 6})
		Expect([]int{guessed, left}).To(Equal([]int{0, 2}))
		Expect(e.codes[8]).To(BeEmpty())
	})
})

var _ = Describe("refs", func() {
	It("leaves out the cards no code fit", func() {
		e := &exporter{codes: map[int]string{5: ""}, ids: map[int]int{6: 10}}
		Expect(e.refs([]replay.Card{{ID: 5}, {ID: 6, Code: "A"}})).To(Equal([]cardRef{{CardID: "A", UniqueDeckID: 10}}))
	})
})
