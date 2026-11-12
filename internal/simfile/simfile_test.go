package simfile_test

import (
	"os"
	"path/filepath"

	"github.com/VTGare/logpose/internal/simfile"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CombatLogs", func() {
	It("lists the logs oldest first and leaves other files out", func() {
		dir := GinkgoT().TempDir()
		for _, name := range []string{"2026-11-06T20.01.44.log", "2026-11-05T15.52.10.partial.log", "notes.txt"} {
			Expect(os.WriteFile(filepath.Join(dir, name), nil, 0o600)).To(Succeed())
		}
		Expect(simfile.CombatLogs(dir)).To(Equal([]string{
			filepath.Join(dir, "2026-11-05T15.52.10.partial.log"),
			filepath.Join(dir, "2026-11-06T20.01.44.log"),
		}))
	})
})
