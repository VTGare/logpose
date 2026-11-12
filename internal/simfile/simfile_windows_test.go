package simfile_test

import (
	"os"
	"path/filepath"

	"github.com/VTGare/logpose/internal/simfile"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Windows refuses to rename a file opened without FILE_SHARE_DELETE. Other systems always allow it.
var _ = Describe("Open", func() {
	It("lets the sim rename a log while it's open", func() {
		dir := GinkgoT().TempDir()
		partial := filepath.Join(dir, "game.partial.log")
		Expect(os.WriteFile(partial, []byte("RZ1|HDR|1.44a|2|RZ1\n"), 0o600)).To(Succeed())
		f, err := simfile.Open(partial)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(f.Close)

		Expect(os.Rename(partial, filepath.Join(dir, "game.log"))).To(Succeed())
	})
})
