package arts

import (
	"image"
	"os"
	"path/filepath"
	"strings"

	// Registers the decoders for the two sites' images.
	_ "image/png"

	_ "golang.org/x/image/webp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// gridSources is a folder of real images, lim-<key>.webp from Limitless and bd-<key>.png from
// Bandai. With it set, the spec below writes testdata/grids from them.
const gridSources = "ARTS_GRID_SOURCES"

func decodeFile(path string) image.Image {
	GinkgoHelper()
	f, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = f.Close() }()
	img, _, err := image.Decode(f)
	Expect(err).NotTo(HaveOccurred())
	return img
}

// fixtureGrid is a grid made from a real image by the spec below.
func fixtureGrid(key string) grid {
	GinkgoHelper()
	data, err := os.ReadFile(filepath.Join("testdata", "grids", key+".grid"))
	Expect(err).NotTo(HaveOccurred())
	var g grid
	Expect(data).To(HaveLen(len(g)))
	copy(g[:], data)
	return g
}

var _ = Describe("grid fixtures", func() {
	It("are made from the real images", func() {
		dir := os.Getenv(gridSources)
		if dir == "" {
			Skip(gridSources + " names no folder of real images")
		}
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
