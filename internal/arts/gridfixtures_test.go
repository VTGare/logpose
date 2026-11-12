//go:build gridfixtures

package arts

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// This writes testdata/grids from a folder of real images, lim-<key>.webp from Limitless and
// bd-<key>.png from Bandai, which the repo doesn't hold. Run it after changing the sampling:
// ARTS_GRID_SOURCES=<folder> go test -tags gridfixtures ./internal/arts/
var _ = Describe("grid fixtures", func() {
	It("are made from the real images", func() {
		dir := os.Getenv("ARTS_GRID_SOURCES")
		Expect(dir).NotTo(BeEmpty(), "ARTS_GRID_SOURCES names no folder of real images")
		entries, err := os.ReadDir(dir)
		Expect(err).NotTo(HaveOccurred())
		n := 0
		for _, e := range entries {
			name := e.Name()
			var k Key
			switch {
			case strings.HasPrefix(name, "lim-") && strings.HasSuffix(name, ".webp"):
				k = Key{Limitless, strings.TrimSuffix(strings.TrimPrefix(name, "lim-"), ".webp")}
			case strings.HasPrefix(name, "bd-") && strings.HasSuffix(name, ".png"):
				k = Key{Bandai, strings.TrimSuffix(strings.TrimPrefix(name, "bd-"), ".png")}
			default:
				continue
			}
			g := gridOf(decodeFile(filepath.Join(dir, name)))
			Expect(os.WriteFile(filepath.Join("testdata", "grids", k.String()+".grid"), g[:], 0o600)).To(Succeed())
			n++
		}
		Expect(n).To(BeNumerically(">", 0))
	})
})
