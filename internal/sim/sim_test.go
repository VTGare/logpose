package sim_test

import (
	"os"
	"path/filepath"

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
