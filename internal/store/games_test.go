package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/simlog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func fixture() []simlog.Game {
	f, err := os.Open(filepath.Join("..", "importer", "testdata", "2026-11-05T15.52.10.log"))
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = f.Close() }()
	games, err := simlog.Parse(f)
	Expect(err).NotTo(HaveOccurred())
	Expect(games).To(HaveLen(2))
	return games
}

var _ = Describe("Store", func() {
	var (
		ctx  context.Context
		path string
		st   *Store
		src  Source
	)

	BeforeEach(func() {
		ctx = context.Background()
		path = filepath.Join(GinkgoT().TempDir(), "logpose.db")
		var err error
		st, err = Open(ctx, path)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(st.Close()).To(Succeed()) })
		src = Source{File: "2026-11-05T15.52.10.log", PlayedAt: time.Date(2026, 11, 5, 15, 52, 10, 0, time.Local)}
	})

	It("keeps the game's sets and the sim format seen for its log", func() {
		g := fixture()[0]
		_, err := st.SaveGame(ctx, &g, src)
		Expect(err).NotTo(HaveOccurred())
		Expect(st.SetLogFormat(ctx, src.File, "Spoiler")).To(Succeed())
		list, err := st.Games(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(list[0].SimFormat).To(Equal("Spoiler"))
		Expect(list[0].Sets).To(Equal(g.Sets))
		Expect(list[0].Sets).NotTo(BeEmpty())
	})

	It("saves a game once", func() {
		g := fixture()[0]
		created, err := st.SaveGame(ctx, &g, src)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeTrue())
		created, err = st.SaveGame(ctx, &g, src)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeFalse())
	})

	It("returns the log text it was given", func() {
		g := fixture()[1]
		_, err := st.SaveGame(ctx, &g, src)
		Expect(err).NotTo(HaveOccurred())
		Expect(st.GameLog(ctx, g.Fingerprint)).To(Equal(g.Text))
	})

	It("stores the log compressed", func() {
		g := fixture()[0]
		g.Text = strings.Repeat(g.Text, 20)
		_, err := st.SaveGame(ctx, &g, src)
		Expect(err).NotTo(HaveOccurred())
		var size int
		Expect(st.read.QueryRowContext(ctx, `SELECT length(log) FROM games`).Scan(&size)).To(Succeed())
		Expect(size).To(BeNumerically("<", len(g.Text)/5))
	})

	It("keeps the card names the logs give", func() {
		games := fixture()
		_, err := st.SaveGame(ctx, &games[1], src)
		Expect(err).NotTo(HaveOccurred())
		names, err := st.CardNames(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(names).To(HaveKeyWithValue("OP13-001", "Monkey D. Luffy"))
		Expect(names).To(HaveKeyWithValue("OP17-022", "Shanks"))
	})

	It("says when a log isn't stored", func() {
		_, err := st.GameLog(ctx, "nope")
		Expect(err).To(MatchError(ErrNotFound))
	})

	It("replaces a copy saved before the game ended", func() {
		full := fixture()[1]
		early := full
		early.Moves = early.Moves[:2]
		early.Text = "partial\n"
		_, err := st.SaveGame(ctx, &early, src)
		Expect(err).NotTo(HaveOccurred())
		created, err := st.SaveGame(ctx, &full, src)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeFalse())
		Expect(st.GameLog(ctx, full.Fingerprint)).To(Equal(full.Text))
	})

	It("lists games newest first, with their players", func() {
		games := fixture()
		older := src
		older.PlayedAt = src.PlayedAt.Add(-15 * time.Minute)
		_, err := st.SaveGame(ctx, &games[0], older)
		Expect(err).NotTo(HaveOccurred())
		_, err = st.SaveGame(ctx, &games[1], src)
		Expect(err).NotTo(HaveOccurred())

		list, err := st.Games(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(HaveLen(2))
		Expect(list[0].Fingerprint).To(Equal(games[1].Fingerprint))
		Expect(list[0].Players).To(HaveLen(2))
		Expect(list[0].PlayedAt).To(BeTemporally("==", src.PlayedAt))
	})

	It("parses games again after a parser update", func() {
		g := fixture()[0]
		_, err := st.SaveGame(ctx, &g, src)
		Expect(err).NotTo(HaveOccurred())
		_, err = st.write.ExecContext(ctx, `UPDATE games SET parser_version = 0, end_kind = 'unknown', loser = ''`)
		Expect(err).NotTo(HaveOccurred())

		n, err := st.Reparse(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(1))
		list, err := st.Games(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(list[0].End).To(Equal(g.End))

		n, err = st.Reparse(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(BeZero())
	})

	It("keeps settings", func() {
		Expect(st.Setting(ctx, SettingSimDir)).To(BeEmpty())
		Expect(st.SetSetting(ctx, SettingSimDir, `E:\Games\OPTCG`)).To(Succeed())
		Expect(st.SetSetting(ctx, SettingSimDir, `D:\OPTCGSim`)).To(Succeed())
		Expect(st.Setting(ctx, SettingSimDir)).To(Equal(`D:\OPTCGSim`))
	})

	It("reopens an existing database without applying migrations again", func() {
		g := fixture()[0]
		_, err := st.SaveGame(ctx, &g, src)
		Expect(err).NotTo(HaveOccurred())
		Expect(st.Close()).To(Succeed())

		st, err = Open(ctx, path)
		Expect(err).NotTo(HaveOccurred())
		list, err := st.Games(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(HaveLen(1))
	})
})
