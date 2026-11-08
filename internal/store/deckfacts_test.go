package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/deck"
	"github.com/VTGare/logpose/internal/simlog"
)

func deckGame() simlog.Game {
	text, err := os.ReadFile(filepath.Join("..", "deck", "testdata", "luffy-samurai.log"))
	Expect(err).NotTo(HaveOccurred())
	games, err := simlog.Parse(strings.NewReader(string(text)))
	Expect(err).NotTo(HaveOccurred())
	Expect(games).To(HaveLen(1))
	return games[0]
}

var _ = Describe("Deck facts", func() {
	var (
		ctx  context.Context
		st   *Store
		g    simlog.Game
		want deck.Facts
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		st, err = Open(ctx, filepath.Join(GinkgoT().TempDir(), "logpose.db"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(st.Close()).To(Succeed()) })
		g = deckGame()
		facts, err := deck.Read(&g)
		Expect(err).NotTo(HaveOccurred())
		for _, f := range facts {
			if f.Player == "Alice#1111" {
				want = f
			}
		}
		_, err = st.SaveGame(ctx, &g, Source{File: "game.log", PlayedAt: time.Now()})
		Expect(err).NotTo(HaveOccurred())
	})

	It("reads back what the log says each deck did", func() {
		got, err := st.DeckFacts(ctx, "Alice#1111")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(HaveLen(1))
		Expect(got[g.Fingerprint]).To(Equal(want))
	})

	It("reads only the players asked for", func() {
		got, err := st.DeckFacts(ctx, "Bob#2222")
		Expect(err).NotTo(HaveOccurred())
		Expect(got[g.Fingerprint].Player).To(Equal("Bob#2222"))
		Expect(got[g.Fingerprint].List).NotTo(Equal(want.List))
	})

	It("fills them for a game stored before the deck parser", func() {
		for _, table := range []string{"game_deck_cards", "game_card_events", "game_searches"} {
			_, err := st.write.ExecContext(ctx, `DELETE FROM `+table)
			Expect(err).NotTo(HaveOccurred())
		}
		_, err := st.write.ExecContext(ctx, `UPDATE games SET parser_version = ?`, simlog.Version)
		Expect(err).NotTo(HaveOccurred())

		n, err := st.Reparse(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(1))
		got, err := st.DeckFacts(ctx, "Alice#1111")
		Expect(err).NotTo(HaveOccurred())
		Expect(got[g.Fingerprint]).To(Equal(want))
	})
})
