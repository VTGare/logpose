package sim_test

import (
	"os"
	"path/filepath"
	"time"

	"github.com/VTGare/logpose/internal/sim"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CardImage", func() {
	It("finds a full image as a JPEG or a PNG, and a small one as a JPEG", func() {
		root := GinkgoT().TempDir()
		cards := filepath.Join(root, "OPTCGSim_Data", "StreamingAssets", "Cards")
		for _, name := range []string{"OP14/OP14-037.jpg", "EB01/EB01-013.png"} {
			path := filepath.Join(cards, name)
			Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
			Expect(os.WriteFile(path, nil, 0o600)).To(Succeed())
		}
		in := sim.Install{Root: root}
		Expect(in.CardImage("OP14-037", false)).To(Equal(filepath.Join(cards, "OP14", "OP14-037.jpg")))
		Expect(in.CardImage("EB01-013", false)).To(Equal(filepath.Join(cards, "EB01", "EB01-013.png")))
		Expect(in.CardImage("EB01-013", true)).To(Equal(filepath.Join(cards, "EB01", "EB01-013_small.jpg")))
	})
})

var _ = Describe("Install", func() {
	It("checks a folder for the sim and counts its logs", func() {
		root := GinkgoT().TempDir()
		_, err := sim.Open(root)
		Expect(err).To(MatchError(sim.ErrNotFound))

		logs := filepath.Join(root, "CombatLogs", "AutoSaved")
		Expect(os.MkdirAll(filepath.Join(root, "OPTCGSim_Data"), 0o700)).To(Succeed())
		Expect(os.MkdirAll(logs, 0o700)).To(Succeed())
		for _, name := range []string{"2026-11-05T15.52.10.log", "2026-11-06T20.01.44.log", "notes.txt"} {
			Expect(os.WriteFile(filepath.Join(logs, name), nil, 0o600)).To(Succeed())
		}
		in, err := sim.Open(root)
		Expect(err).NotTo(HaveOccurred())
		Expect(in.LogCount()).To(Equal(2))
		Expect(in.HasExe()).To(BeFalse())
		Expect(os.WriteFile(filepath.Join(root, "OPTCGSim.exe"), nil, 0o600)).To(Succeed())
		Expect(in.HasExe()).To(BeTrue())
	})
})

var _ = Describe("Version", func() {
	It("is the version folder next to Player.log changed last", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("USERPROFILE", home)
		dir := filepath.Join(home, "AppData", "LocalLow", "Batsu", "OPTCGSim")
		Expect(sim.Version()).To(BeEmpty())
		old := time.Now().Add(-time.Hour)
		for i, name := range []string{"1.44a", "1.43a", "Unity"} {
			Expect(os.MkdirAll(filepath.Join(dir, name), 0o700)).To(Succeed())
			at := old.Add(time.Duration(i) * time.Minute)
			Expect(os.Chtimes(filepath.Join(dir, name), at, at)).To(Succeed())
		}
		Expect(sim.Version()).To(Equal("1.43a"))
	})
})

var _ = Describe("OpenShared", func() {
	It("lets the sim rename a log while it's open", func() {
		dir := GinkgoT().TempDir()
		partial := filepath.Join(dir, "game.partial.log")
		Expect(os.WriteFile(partial, []byte("RZ1|HDR|1.44a|2|RZ1\n"), 0o600)).To(Succeed())
		f, err := sim.OpenShared(partial)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(f.Close)

		Expect(os.Rename(partial, filepath.Join(dir, "game.log"))).To(Succeed())
	})
})
