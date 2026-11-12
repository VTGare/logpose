package arts

import (
	"image"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func decodeFile(path string) image.Image {
	GinkgoHelper()
	f, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = f.Close() }()
	img, _, err := image.Decode(f)
	Expect(err).NotTo(HaveOccurred())
	return img
}

// fixtureGrid is a grid made from a real image by gridfixtures_test.go.
func fixtureGrid(key string) grid {
	GinkgoHelper()
	data, err := os.ReadFile(filepath.Join("testdata", "grids", key+".grid"))
	Expect(err).NotTo(HaveOccurred())
	var g grid
	Expect(data).To(HaveLen(len(g)))
	copy(g[:], data)
	return g
}
