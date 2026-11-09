package importer_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

	It("reports a game in progress, then the copy the sim saves when it ends, and nothing between", func() {
		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		st, err := store.Open(ctx, filepath.Join(GinkgoT().TempDir(), "logpose.db"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(st.Close()).To(Succeed()) })
		dir := GinkgoT().TempDir()
		data, err := os.ReadFile(filepath.Join("testdata", "2026-11-05T15.37.33.log"))
		Expect(err).NotTo(HaveOccurred())
		// The game needs more moves than go into a fingerprint, so a growing log keeps its own.
		moves := func(from, to int) []byte {
			var b []byte
			for seq := from; seq < to; seq++ {
				b = fmt.Appendf(b, "RZ1|%d|1|Don|5|0|9|0|1|1|0|0|0\n", seq)
			}
			return b
		}
		started := append(slices.Clone(data), moves(100, 170)...)
		partial := filepath.Join(dir, "2026-11-05T15.37.33.partial.log")
		Expect(os.WriteFile(partial, started, 0o600)).To(Succeed())

		results := make(chan importer.Result, 4)
		w := &importer.Watcher{Dir: dir, Store: st, Interval: 20 * time.Millisecond, OnImport: func(r importer.Result) { results <- r }}
		go func() { _ = w.Run(ctx) }()
		Eventually(results).Should(Receive(HaveField("New", 1)))

		grown := append(started, moves(170, 180)...)
		Expect(os.WriteFile(partial, grown, 0o600)).To(Succeed())
		Consistently(results, 150*time.Millisecond).ShouldNot(Receive())
		Expect(st.Games(ctx)).To(HaveLen(1))

		ended := append(grown, "RZ1|END|999|1|opponent_concede|ab12\n"...)
		Expect(os.WriteFile(filepath.Join(dir, "2026-11-05T15.37.33.log"), ended, 0o600)).To(Succeed())
		Expect(os.Remove(partial)).To(Succeed())
		Eventually(results).Should(Receive(And(HaveField("New", 0), HaveField("Updated", 1))))
	})

	It("parses stored games again on its first pass when the parser changed", func() {
		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		path := filepath.Join(GinkgoT().TempDir(), "logpose.db")
		st, err := store.Open(ctx, path)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(st.Close()).To(Succeed()) })
		dir := GinkgoT().TempDir()
		data, err := os.ReadFile(filepath.Join("testdata", "2026-11-05T15.37.33.log"))
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(dir, "2026-11-05T15.37.33.log"), data, 0o600)).To(Succeed())
		_, err = importer.Dir(ctx, st, dir)
		Expect(err).NotTo(HaveOccurred())

		// An older logpose stored this game.
		db, err := sql.Open("sqlite", path)
		Expect(err).NotTo(HaveOccurred())
		_, err = db.ExecContext(ctx, `UPDATE games SET parser_version = 0`)
		Expect(err).NotTo(HaveOccurred())
		Expect(db.Close()).To(Succeed())

		results := make(chan importer.Result, 4)
		w := &importer.Watcher{Dir: dir, Store: st, Interval: 20 * time.Millisecond, OnImport: func(r importer.Result) { results <- r }}
		go func() { _ = w.Run(ctx) }()
		Eventually(results).Should(Receive(And(HaveField("New", 0), HaveField("Reparsed", 1))))
		Consistently(results, 100*time.Millisecond).ShouldNot(Receive())
	})
})
