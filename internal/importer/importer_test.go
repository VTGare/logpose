package importer_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/importer"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/store"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// testdata holds a rematch session as the sim saves it: the first file has game 1, and the
// second, saved after game 2, has both.
var _ = Describe("Dir", func() {
	var (
		ctx context.Context
		st  *store.Store
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		st, err = store.Open(ctx, filepath.Join(GinkgoT().TempDir(), "logpose.db"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(st.Close()).To(Succeed()) })
	})

	It("stores each game once, from the earliest file it's in", func() {
		r, err := importer.Dir(ctx, st, "testdata")
		Expect(err).NotTo(HaveOccurred())
		Expect(r).To(Equal(importer.Result{Files: 2, Games: 3, New: 2}))

		games, err := st.Games(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(games).To(HaveLen(2))
		Expect(games[1].Source.File).To(Equal("2026-11-05T15.37.33.log"))
		Expect(games[0].Source.File).To(Equal("2026-11-05T15.52.10.log"))
		Expect(games[0].Source.Index).To(Equal(1))
	})

	It("adds nothing when run again", func() {
		_, err := importer.Dir(ctx, st, "testdata")
		Expect(err).NotTo(HaveOccurred())
		r, err := importer.Dir(ctx, st, "testdata")
		Expect(err).NotTo(HaveOccurred())
		Expect(r.New).To(BeZero())
	})

	It("takes the play time from the file name", func() {
		_, err := importer.Dir(ctx, st, "testdata")
		Expect(err).NotTo(HaveOccurred())
		games, err := st.Games(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(games[1].PlayedAt).To(BeTemporally("==", time.Date(2026, 11, 5, 15, 37, 33, 0, time.Local)))
	})

	It("keeps the parsed results", func() {
		_, err := importer.Dir(ctx, st, "testdata")
		Expect(err).NotTo(HaveOccurred())
		games, err := st.Games(ctx)
		Expect(err).NotTo(HaveOccurred())

		first, second := games[1], games[0]
		Expect(first.End).To(Equal(simlog.End{Kind: simlog.EndConcede, Loser: "Bob#2222"}))
		Expect(second.End).To(Equal(simlog.End{Kind: simlog.EndLethal, Loser: "Bob#2222"}))
		Expect(second.Lobby).To(Equal(simlog.Lobby{Code: "XNJ8M66", Role: simlog.RoleJoin}))
		Expect(first.First).To(Equal("Alice#1111"))
		alice, ok := first.Player("Alice#1111")
		Expect(ok).To(BeTrue())
		Expect(alice.Leader).To(Equal("OP13-001"))
		Expect(alice.HandBefore).To(HaveLen(5))
	})

	It("keeps a copy of each game's log that doesn't need the sim's files", func() {
		dir := GinkgoT().TempDir()
		src, err := os.ReadFile(filepath.Join("testdata", "2026-11-05T15.52.10.log"))
		Expect(err).NotTo(HaveOccurred())
		path := filepath.Join(dir, "2026-11-05T15.52.10.log")
		Expect(os.WriteFile(path, src, 0o600)).To(Succeed())
		_, err = importer.File(ctx, st, path)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Remove(path)).To(Succeed())

		games, err := st.Games(ctx)
		Expect(err).NotTo(HaveOccurred())
		for _, g := range games {
			text, err := st.GameLog(ctx, g.Fingerprint)
			Expect(err).NotTo(HaveOccurred())
			again, err := simlog.Parse(strings.NewReader(text))
			Expect(err).NotTo(HaveOccurred())
			Expect(again).To(HaveLen(1))
			Expect(again[0].Fingerprint).To(Equal(g.Fingerprint))
		}
	})

	It("finds the local player as the one in the most games", func() {
		_, err := importer.Dir(ctx, st, "testdata")
		Expect(err).NotTo(HaveOccurred())
		Expect(st.LocalPlayer(ctx)).To(Equal("Alice#1111"))
	})
})

var _ = Describe("Watcher", func() {
	It("imports logs already there, then new ones as they're saved", func() {
		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		st, err := store.Open(ctx, filepath.Join(GinkgoT().TempDir(), "logpose.db"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(st.Close()).To(Succeed()) })

		dir := GinkgoT().TempDir()
		copyLog := func(name string) {
			data, err := os.ReadFile(filepath.Join("testdata", name))
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(filepath.Join(dir, name), data, 0o600)).To(Succeed())
		}
		copyLog("2026-11-05T15.37.33.log")

		results := make(chan importer.Result, 4)
		w := &importer.Watcher{Dir: dir, Store: st, Interval: 20 * time.Millisecond, OnImport: func(r importer.Result) { results <- r }}
		done := make(chan error, 1)
		go func() { done <- w.Run(ctx) }()

		Eventually(results).Should(Receive(HaveField("New", 1)))
		copyLog("2026-11-05T15.52.10.log")
		Eventually(results).Should(Receive(HaveField("New", 1)))
		Consistently(results, 100*time.Millisecond).ShouldNot(Receive())

		cancel()
		Eventually(done).Should(Receive(BeNil()))
	})
})
