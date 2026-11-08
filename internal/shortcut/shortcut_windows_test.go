package shortcut

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("shortcuts on Windows", func() {
	It("writes a shortcut file that names its target", func() {
		path := filepath.Join(GinkgoT().TempDir(), "logpose test.lnk")
		exe, err := os.Executable()
		Expect(err).NotTo(HaveOccurred())
		Expect(save(path, Link{Target: exe, Args: []string{"-db", `C:\a folder\x.db`}, Dir: filepath.Dir(exe)})).To(Succeed())
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		// A .lnk starts with its header size, 0x4C, and stores the target's name in UTF-16.
		Expect(data[0]).To(BeEquivalentTo(0x4c))
		Expect(string(data)).To(ContainSubstring(utf16Bytes(filepath.Base(exe))))
		Expect(string(data)).To(ContainSubstring(utf16Bytes(`-db "C:\a folder\x.db"`)))
	})

	It("finds the Start menu and desktop folders", func() {
		for _, place := range Places {
			path, err := Path(place, "logpose")
			Expect(err).NotTo(HaveOccurred())
			Expect(filepath.Dir(path)).To(BeADirectory())
		}
	})
})

func utf16Bytes(s string) string {
	b := make([]byte, 0, 2*len(s))
	for _, r := range s {
		b = append(b, byte(r), byte(r>>8))
	}
	return string(b)
}
