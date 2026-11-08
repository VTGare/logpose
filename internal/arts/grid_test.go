package arts

import (
	"context"
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakePictures serves pictures by key and counts the reads.
type fakePictures struct {
	mu    sync.Mutex
	pics  map[Key]image.Image
	reads map[Key]int
	total atomic.Int32
	// gate, when set, holds every read until it closes.
	gate chan struct{}
}

func newFakePictures() *fakePictures {
	return &fakePictures{pics: make(map[Key]image.Image), reads: make(map[Key]int)}
}

func (f *fakePictures) Picture(k Key) (image.Image, error) {
	if f.gate != nil {
		<-f.gate
	}
	f.total.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads[k]++
	img, ok := f.pics[k]
	if !ok {
		return nil, errors.New("no such picture")
	}
	return img, nil
}

func (f *fakePictures) readsOf(k Key) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads[k]
}

func solidImage(c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 60, 84))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

var _ = Describe("grids", func() {
	It("put one printing from both sites within matchMax, and another art past it", func() {
		lim := gridOf(decodeFile(filepath.Join("testdata", "lim-OP07-021-small.png")))
		bd := gridOf(decodeFile(filepath.Join("testdata", "bd-OP07-021-small.png")))
		Expect(distance(lim, bd)).To(BeNumerically("<=", matchMax))
		Expect(distance(lim, fixtureGrid("OP07-021_p3"))).To(BeNumerically(">", matchMax))
		Expect(distance(bd, fixtureGrid("OP07-021_p3"))).To(BeNumerically(">", matchMax))
	})

	It("measure the mean squared difference of the channels", func() {
		black := gridOf(solidImage(color.RGBA{0, 0, 0, 255}))
		gray := gridOf(solidImage(color.RGBA{10, 20, 30, 255}))
		Expect(distance(black, black)).To(BeZero())
		Expect(distance(black, gray)).To(BeNumerically("~", (100+400+900)/3.0))
	})

	Describe("store", func() {
		var (
			dir  string
			pics *fakePictures
			k    = Key{Bandai, "OP07-021_p3"}
		)
		BeforeEach(func() {
			dir = filepath.Join(GinkgoT().TempDir(), "grids-1")
			pics = newFakePictures()
			pics.pics[k] = solidImage(color.RGBA{200, 40, 40, 255})
		})

		It("makes a grid from the picture once and finds it on disk after", func() {
			s := newGridStore(dir)
			_, ok := s.known(k)
			Expect(ok).To(BeFalse())
			g, err := s.get(context.Background(), k, pics)
			Expect(err).NotTo(HaveOccurred())
			Expect(g).To(Equal(gridOf(pics.pics[k])))
			Expect(filepath.Join(dir, "OP07-021_p3_bd.grid")).To(BeAnExistingFile())

			again := newGridStore(dir)
			got, ok := again.known(k)
			Expect(ok).To(BeTrue())
			Expect(got).To(Equal(g))
			_, err = again.get(context.Background(), k, pics)
			Expect(err).NotTo(HaveOccurred())
			Expect(pics.readsOf(k)).To(Equal(1))
		})

		It("reads a picture once while several callers want its grid", func() {
			s := newGridStore(dir)
			pics.gate = make(chan struct{})
			var wg sync.WaitGroup
			errs := make(chan error, 3)
			for range 3 {
				wg.Go(func() {
					_, err := s.get(context.Background(), k, pics)
					errs <- err
				})
			}
			time.Sleep(20 * time.Millisecond)
			close(pics.gate)
			wg.Wait()
			close(errs)
			for err := range errs {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(pics.readsOf(k)).To(Equal(1))
		})

		It("writes nothing when the picture can't be read", func() {
			s := newGridStore(dir)
			missing := Key{Bandai, "OP07-021_p4"}
			_, err := s.get(context.Background(), missing, pics)
			Expect(err).To(HaveOccurred())
			_, ok := s.known(missing)
			Expect(ok).To(BeFalse())
			entries, _ := os.ReadDir(dir)
			Expect(entries).To(BeEmpty())
		})
	})
})
