package gui

import (
	"os"
	"strings"

	"github.com/egoist/mygo/ui"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simlog"
)

func flat(ps []textPiece) string {
	var b strings.Builder
	for _, p := range ps {
		b.WriteString(p.text)
	}
	return b.String()
}

var _ = Describe("replay step words", func() {
	It("words every step of a real game", func() {
		text, err := os.ReadFile("../replay/testdata/six-turns.log")
		Expect(err).NotTo(HaveOccurred())
		games, err := simlog.Parse(strings.NewReader(string(text)))
		Expect(err).NotTo(HaveOccurred())
		g := &games[len(games)-1]
		r, err := replay.Build(g, nil)
		Expect(err).NotTo(HaveOccurred())
		var got []string
		for i := range r.Steps[:29] {
			s := &r.Steps[i]
			name, _ := simlog.SplitID(s.Player)
			t := "T" + string(rune('0'+s.Turn)) + " " + name + ": " + flat(stepTitle(g, s, ui.Color{}))
			if res := stepResult(g, s); res != "" {
				t += " → " + res
			}
			got = append(got, t)
		}
		Expect(got).To(Equal([]string{
			"T0 Alice: Drew an opening hand",
			"T0 Bob: Drew an opening hand",
			"T0 Bob: Life cards placed",
			"T0 Alice: Life cards placed",
			"T1 Bob: +1 DON",
			"T2 Alice: Drew Trafalgar Law · +2 DON",
			"T2 Alice: Played Kin'emon",
			"T3 Bob: Drew Rocks D. Xebec · +2 DON",
			"T3 Bob: Played Kyo",
			"T3 Bob: Gave Rocks D. Xebec 1 DON",
			"T3 Bob: Rocks D. Xebec attacked Monkey D. Luffy",
			"T3 Bob: Rocks D. Xebec 6000 vs Monkey D. Luffy 5000 → Hit for 1",
			"T4 Alice: Drew Emporio Ivankov · +2 DON",
			"T4 Alice: Played Monkey D. Garp",
			"T4 Alice: You Can Be My Samurai!!!: Rest Monkey D. Garp",
			"T4 Alice: Gave Monkey D. Luffy 1 DON",
			"T4 Alice: Monkey D. Luffy attacked Rocks D. Xebec",
			"T4 Alice: Monkey D. Luffy 6000 vs Rocks D. Xebec 5000 → Hit for 1",
			"T5 Bob: Drew Shiki · +2 DON",
			"T5 Bob: Rocks D. Xebec attacked Monkey D. Garp",
			"T5 Bob: Rocks D. Xebec 5000 vs Monkey D. Garp 2000 → Monkey D. Garp K.O.'d",
			"T5 Bob: Played Charlotte Linlin",
			"T6 Alice: Drew Kawamatsu · +2 DON",
			"T6 Alice: Played Jewelry Bonney",
			"T6 Alice: Gave Monkey D. Luffy 1 DON",
			"T6 Alice: Monkey D. Luffy attacked Rocks D. Xebec",
			"T6 Bob: Charlotte Linlin: +1000 to Rocks D. Xebec (trashed Wang Zhi)",
			"T6 Bob: Countered with Buckin +1000",
			"T6 Alice: Monkey D. Luffy 6000 vs Rocks D. Xebec 7000 → Attack failed",
		}))
	})

	g := &simlog.Game{CardNames: map[string]string{
		"OP17-049": "Charlotte Linlin", "OP17-054": "Stussy", "OP17-040": "Edward Newgate",
		"OP13-001": "Monkey D. Luffy", "OP14-031": "Nami", "OP13-016": "Monkey D. Garp",
	}}

	It("writes what an effect during an attack did, with what it cost in brackets", func() {
		s := &replay.Step{Kind: replay.KindBattleEffect, Cards: []string{"OP17-049"}, Effect: &replay.Effect{Did: []replay.Did{
			{Kind: replay.DidTrash, Card: "OP17-040"},
			{Kind: replay.DidPower, Card: "OP17-054", N: 1000},
		}}}
		Expect(flat(stepTitle(g, s, ui.Color{}))).To(Equal("Charlotte Linlin: +1000 to Stussy (trashed Edward Newgate)"))
		s = &replay.Step{Kind: replay.KindBattleEffect, Cards: []string{"OP13-001"}, Effect: &replay.Effect{Did: []replay.Did{
			{Kind: replay.DidCount, Count: replay.Count{Kind: replay.CountDonRested, N: 2}},
			{Kind: replay.DidPower, Card: "OP13-001", N: 4000},
		}}}
		Expect(flat(stepTitle(g, s, ui.Color{}))).To(Equal("Monkey D. Luffy: +4000 to Monkey D. Luffy (rested 2 DON)"))
	})

	It("writes each counted effect's total", func() {
		for want, c := range map[string]replay.Count{
			"Set 5 DON active":        {Kind: replay.CountDonActive, N: 5},
			"Rest 1 DON":              {Kind: replay.CountDonRested, N: 1},
			"Return 2 DON":            {Kind: replay.CountDonReturned, N: 2},
			"Draw 1 card":             {Kind: replay.CountCardsDrawn, N: 1},
			"Draw 2 cards":            {Kind: replay.CountCardsDrawn, N: 2},
			"Add 2 rested DON":        {Kind: replay.CountRestedDonAdded, N: 2},
			"Trash 3 Cards from Deck": {Kind: replay.CountOther, N: 3, Verb: "Trash", Object: "Cards from Deck"},
		} {
			Expect(countText(c)).To(Equal(want))
		}
	})

	It("links the cards an effect line names", func() {
		s := &replay.Step{Kind: replay.KindEffect, Cards: []string{"OP14-031"}, Effect: &replay.Effect{
			Head: "Nami [OP14-031]", Did: []replay.Did{{Kind: replay.DidText, Text: "Rest Monkey D. Garp [OP13-016]"}},
		}}
		var texts, codes []string
		for _, p := range stepTitle(g, s, ui.Color{}) {
			texts, codes = append(texts, p.text), append(codes, p.code)
		}
		Expect(texts).To(Equal([]string{"Nami", ": Rest ", "Monkey D. Garp"}))
		Expect(codes).To(Equal([]string{"OP14-031", "", "OP13-016"}))
	})

	It("links a log line's cards the way a step title does", func() {
		l := simlog.Line{Text: "Nami [OP14-031]: Give Unknown [OP99-001] and Monkey D. Garp [OP13-016] 1 [Don]"}
		var texts, codes []string
		for _, p := range logPieces(&stepView{g: g}, l, ui.Color{}) {
			texts, codes = append(texts, p.text), append(codes, p.code)
		}
		Expect(texts).To(Equal([]string{"Nami", ": Give Unknown and ", "Monkey D. Garp", " 1"}))
		Expect(codes).To(Equal([]string{"OP14-031", "", "OP13-016", ""}))
	})

	It("names a part of the turn for every kind of step", func() {
		for k := replay.KindLobby; k <= replay.KindDeckOut; k++ {
			Expect(phaseWord([]replay.Step{{Kind: k}}, 0)).NotTo(BeEmpty(), "kind %d", k)
		}
	})

	It("names an effect during an attack by the step it comes in", func() {
		steps := []replay.Step{{Kind: replay.KindAttack}, {Kind: replay.KindCounter}, {Kind: replay.KindBattleEffect}}
		Expect(phaseWord(steps, 2)).To(Equal("Counter Step"))
	})

	It("adds Lethal to the attack that won the game", func() {
		s := &replay.Step{Kind: replay.KindDamage, Outcome: []replay.Outcome{{Kind: replay.OutcomeHit, N: 1}}, Lethal: true}
		Expect(stepResult(g, s)).To(Equal("Hit for 1 · Lethal"))
		Expect(stepResult(g, &replay.Step{Kind: replay.KindAttack, Lethal: true})).To(Equal("Lethal"))
	})
})
