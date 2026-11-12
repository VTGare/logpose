package gui

import (
	"context"
	"os"
	"path/filepath"

	"github.com/VTGare/logpose/internal/sim"
	"github.com/VTGare/logpose/internal/store"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LocateSim", func() {
	var (
		ctx context.Context
		st  *store.Store
	)
	simFolder := func() string {
		dir := GinkgoT().TempDir()
		Expect(os.Mkdir(filepath.Join(dir, "OPTCGSim_Data"), 0o700)).To(Succeed())
		return dir
	}
	setting := func(key string) string {
		v, err := st.Setting(ctx, key)
		Expect(err).NotTo(HaveOccurred())
		return v
	}

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		st, err = store.Open(ctx, filepath.Join(GinkgoT().TempDir(), "logpose.db"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(st.Close()).To(Succeed()) })
	})

	It("takes the folder the flag names without remembering it", func() {
		dir := simFolder()
		Expect(LocateSim(ctx, st, dir)).To(Equal(sim.Install{Root: dir}))
		Expect(setting(keySimDir)).To(BeEmpty())
	})

	It("takes the folder the user chose over the one it last found", func() {
		chosen, last := simFolder(), simFolder()
		Expect(rememberSim(ctx, st, sim.Install{Root: last}, false)).To(Succeed())
		Expect(rememberSim(ctx, st, sim.Install{Root: chosen}, true)).To(Succeed())
		Expect(rememberSim(ctx, st, sim.Install{Root: last}, false)).To(Succeed())
		Expect(LocateSim(ctx, st, "")).To(Equal(sim.Install{Root: chosen}))

		Expect(forgetChosenSim(ctx, st)).To(Succeed())
		Expect(setting(keySimDirChosen)).To(BeEmpty())
	})

	It("falls back to the folder it last found when the sim isn't running", func() {
		if sim.Running() {
			Skip("the sim is running")
		}
		last := simFolder()
		Expect(rememberSim(ctx, st, sim.Install{Root: last}, false)).To(Succeed())
		Expect(LocateSim(ctx, st, "")).To(Equal(sim.Install{Root: last}))
	})

	It("says when there's no sim to find", func() {
		if sim.Running() {
			Skip("the sim is running")
		}
		_, err := LocateSim(ctx, st, "")
		Expect(err).To(MatchError(sim.ErrNotFound))
	})
})
