package store

import (
	"context"
	"path/filepath"
	"time"

	"github.com/VTGare/logpose/internal/simlog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("results the log doesn't give", func() {
	var (
		ctx   context.Context
		st    *Store
		g     simlog.Game
		alice string
		bob   string
	)

	only := func() Game {
		list, err := st.Games(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(HaveLen(1))
		return list[0]
	}

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		st, err = Open(ctx, filepath.Join(GinkgoT().TempDir(), "logpose.db"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(st.Close()).To(Succeed()) })
		g = fixture()[1]
		g.End = simlog.End{}
		alice, bob = g.Players[0].ID(), g.Players[1].ID()
		_, err = st.SaveGame(ctx, &g, Source{File: "a.partial.log", PlayedAt: time.Now()})
		Expect(err).NotTo(HaveOccurred())
	})

	It("counts the player whose connection dropped as the loser", func() {
		changed, err := st.SaveDrop(ctx, g.Fingerprint, bob)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
		changed, err = st.SaveDrop(ctx, g.Fingerprint, bob)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())

		got := only()
		Expect(got.End.Loser).To(Equal(bob))
		Expect(got.ResultBy).To(Equal(ResultDropped))
	})

	It("keeps a drop seen before its game is stored", func() {
		_, err := st.SaveDrop(ctx, "later", bob)
		Expect(err).NotTo(HaveOccurred())
		Expect(only().End.Loser).To(BeEmpty())
	})

	It("puts the user's choice over a drop, and takes it back", func() {
		_, err := st.SaveDrop(ctx, g.Fingerprint, bob)
		Expect(err).NotTo(HaveOccurred())
		Expect(st.SetGameResult(ctx, g.Fingerprint, alice)).To(Succeed())
		got := only()
		Expect(got.End.Loser).To(Equal(alice))
		Expect(got.ResultBy).To(Equal(ResultUser))
		Expect(got.Dropped).To(Equal(bob))

		Expect(st.SetGameResult(ctx, g.Fingerprint, "")).To(Succeed())
		Expect(only().End.Loser).To(Equal(bob))
	})

	It("takes only one of the game's players as the loser", func() {
		Expect(st.SetGameResult(ctx, g.Fingerprint, "Carol#3333")).To(MatchError(ErrNotFound))
	})

	It("keeps the log's result over both", func() {
		won := fixture()[1]
		won.End = simlog.End{Kind: simlog.EndConcede, Loser: alice}
		won.Text += "RZ1|END|999|1|opponent_concede|ab12\n"
		won.Closed = true
		_, err := st.SaveGame(ctx, &won, Source{File: "a.log", PlayedAt: time.Now()})
		Expect(err).NotTo(HaveOccurred())
		_, err = st.SaveDrop(ctx, g.Fingerprint, bob)
		Expect(err).NotTo(HaveOccurred())
		Expect(st.SetGameResult(ctx, g.Fingerprint, bob)).To(Succeed())

		got := only()
		Expect(got.End).To(Equal(simlog.End{Kind: simlog.EndConcede, Loser: alice}))
		Expect(got.ResultBy).To(Equal(ResultLog))
	})
})
