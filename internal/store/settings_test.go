package store

import (
	"context"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		games := fixture()
		src := Source{File: "2026-11-05T15.52.10.log", PlayedAt: time.Date(2026, 11, 5, 15, 52, 10, 0, time.Local)}
		for i := range games {
			src.Index = i
			_, err := st.SaveGame(ctx, &games[i], src)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("uses the name in the most games until the user picks names", func() {
		counts, err := st.PlayerCounts(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(counts)).To(BeNumerically(">=", 2))
		Expect(counts[0].Games).To(BeNumerically(">=", counts[len(counts)-1].Games))

		auto, err := st.LocalPlayers(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(auto).To(Equal([]string{counts[0].ID}))

		Expect(st.SetSetting(ctx, SettingYou, counts[1].ID+"\n Alt#2 \n\n")).To(Succeed())
		Expect(st.LocalPlayers(ctx)).To(Equal([]string{counts[1].ID, "Alt#2"}))

		all, err := st.Settings(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(all).To(HaveKey(SettingYou))
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
