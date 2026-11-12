package store

import (
	"context"
	"path/filepath"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/simlog"
)

var _ = Describe("Settings", func() {
	var (
		ctx  context.Context
		path string
		st   *Store
	)

	BeforeEach(func() {
		ctx = context.Background()
		path = filepath.Join(GinkgoT().TempDir(), "logpose.db")
		var err error
		st, err = Open(ctx, path)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(st.Close()).To(Succeed()) })
		// Zoe is in both games and the opponents in one each. Her name sorts last, so only the
		// count can pick her.
		games := fixture()
		rename := func(g *simlog.Game, from, to string) {
			g.Players = slices.Clone(g.Players)
			for i := range g.Players {
				if g.Players[i].Name == from {
					g.Players[i].Name = to
				}
			}
		}
		rename(&games[0], "Alice", "Zoe")
		rename(&games[1], "Alice", "Zoe")
		rename(&games[1], "Bob", "Cat")
		src := Source{File: "2026-11-05T15.52.10.log", PlayedAt: time.Date(2026, 11, 5, 15, 52, 10, 0, time.Local)}
		for i := range games {
			src.Index = i
			_, err := st.SaveGame(ctx, &games[i], src)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("uses the name in the most games until the user picks names", func() {
		Expect(st.PlayerCounts(ctx)).To(Equal([]PlayerCount{{"Zoe#1111", 2}, {"Bob#2222", 1}, {"Cat#2222", 1}}))
		Expect(st.LocalPlayers(ctx)).To(Equal([]string{"Zoe#1111"}))

		Expect(st.SetSetting(ctx, SettingYou, "Bob#2222\n Alt#2 \n\n")).To(Succeed())
		Expect(st.LocalPlayers(ctx)).To(Equal([]string{"Bob#2222", "Alt#2"}))

		all, err := st.Settings(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(all).To(HaveKey(SettingYou))

		Expect(st.SetSetting(ctx, SettingYou, JoinNames([]string{"A#1", "B#2"}))).To(Succeed())
		Expect(st.LocalPlayers(ctx)).To(Equal([]string{"A#1", "B#2"}))
	})

	It("changes a setting from its value, set or not", func() {
		appendX := func(v string) string { return v + "x" }
		Expect(st.ChangeSetting(ctx, "letters", appendX)).To(Succeed())
		Expect(st.ChangeSetting(ctx, "letters", appendX)).To(Succeed())
		Expect(st.Setting(ctx, "letters")).To(Equal("xx"))
	})

	It("backs up to a file that can be opened, replacing one that's there", func() {
		dst := filepath.Join(GinkgoT().TempDir(), "backup.db")
		Expect(st.Backup(ctx, dst)).To(Succeed())
		Expect(st.Backup(ctx, dst)).To(Succeed())
		copied, err := Open(ctx, dst)
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(copied.Close()).To(Succeed()) }()
		games, err := copied.Games(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(games).To(HaveLen(2))
	})
})
