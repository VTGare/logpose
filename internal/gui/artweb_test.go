package gui

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VTGare/logpose/internal/arts"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// toServer sends a request to a test server, keeping its path.
type toServer struct{ srv *httptest.Server }

func (t toServer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme, r.URL.Host = "http", strings.TrimPrefix(t.srv.URL, "http://")
	return t.srv.Client().Transport.RoundTrip(r)
}

var _ = Describe("webArt", func() {
	var (
		dir   string
		srv   *httptest.Server
		hits  atomic.Int32
		size  int64
		paths chan string
		// While holding is set, Bandai's downloads wait for hold to close.
		holding atomic.Bool
		hold    chan struct{}
	)

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		var buf bytes.Buffer
		Expect(png.Encode(&buf, solid(600, 838, color.RGBA{200, 40, 40, 255}))).To(Succeed())
		body := buf.Bytes()
		size = int64(len(body))
		hits.Store(0)
		paths = make(chan string, 100)
		holding.Store(false)
		hold = make(chan struct{})
		srv = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if holding.Load() && strings.HasPrefix(r.URL.Path, "/images/") {
				<-hold
			}
			hits.Add(1)
			paths <- r.URL.Path
			_, _ = rw.Write(body)
		}))
		DeferCleanup(srv.Close)
	})

	// open sends every download to the test server, whichever site its key names.
	open := func(limit int64) *webArt {
		w := newWebArt(dir)
		w.client = &http.Client{Transport: toServer{srv}}
		w.limit = limit
		return w
	}
	exists := func(key string) bool {
		_, err := os.Stat(filepath.Join(dir, key+".webp"))
		return err == nil
	}
	usedAt := func(key string, ago time.Duration) {
		t := time.Now().Add(-ago)
		Expect(os.Chtimes(filepath.Join(dir, key+".webp"), t, t)).To(Succeed())
	}

	It("makes a thumb 300 px wide once and reads it without the full image", func() {
		w := open(webFullCap)
		img, err := w.thumb("OP01-001")
		Expect(err).NotTo(HaveOccurred())
		Expect(img.Bounds().Dx()).To(Equal(300))
		Expect(filepath.Join(dir, "thumb", "OP01-001.jpg")).To(BeAnExistingFile())

		Expect(os.Remove(filepath.Join(dir, "OP01-001.webp"))).To(Succeed())
		img, err = w.thumb("OP01-001")
		Expect(err).NotTo(HaveOccurred())
		Expect(img.Bounds().Dx()).To(Equal(300))
		Expect(hits.Load()).To(Equal(int32(1)))
	})

	It("keeps the full image at its own size", func() {
		img, err := open(webFullCap).full("OP01-001")
		Expect(err).NotTo(HaveOccurred())
		Expect(img.Bounds().Dx()).To(Equal(600))
	})

	It("deletes the least recently used full images past the limit", func() {
		w := open(size*5/2 + 1)
		for _, key := range []string{"A-1", "B-1"} {
			_, err := w.full(key)
			Expect(err).NotTo(HaveOccurred())
		}
		usedAt("A-1", 3*time.Hour)
		usedAt("B-1", 2*time.Hour)
		_, err := w.full("A-1")
		Expect(err).NotTo(HaveOccurred())

		_, err = w.full("C-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(exists("A-1")).To(BeTrue())
		Expect(exists("B-1")).To(BeFalse())
		Expect(exists("C-1")).To(BeTrue())
	})

	It("leaves room past an eviction, so the next download deletes nothing", func() {
		w := open(10*size + 1)
		keys := []string{"A-1", "B-1", "C-1", "D-1", "E-1", "F-1", "G-1", "H-1", "I-1", "J-1", "K-1"}
		for i, key := range keys {
			_, err := w.full(key)
			Expect(err).NotTo(HaveOccurred())
			usedAt(key, time.Duration(len(keys)-i)*time.Hour)
		}
		kept := func() int {
			n := 0
			for _, key := range append(keys, "L-1") {
				if exists(key) {
					n++
				}
			}
			return n
		}
		Expect(kept()).To(Equal(9))
		_, err := w.full("L-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(kept()).To(Equal(10))
	})

	It("keeps opened cards when browsing thumbs goes past the limit", func() {
		w := open(3*size + 1)
		_, err := w.full("OPEN-1")
		Expect(err).NotTo(HaveOccurred())
		usedAt("OPEN-1", time.Hour)
		for _, key := range []string{"A-1", "B-1", "C-1", "D-1", "E-1"} {
			_, err := w.thumb(key)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(exists("OPEN-1")).To(BeTrue())
	})

	It("keeps the image it just downloaded when that alone is over the limit", func() {
		w := open(size / 2)
		for _, key := range []string{"A-1", "B-1"} {
			_, err := w.full(key)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(exists("A-1")).To(BeFalse())
		Expect(exists("B-1")).To(BeTrue())
	})

	It("counts the images already in the folder and ignores a crashed download", func() {
		first := open(webFullCap)
		for _, key := range []string{"A-1", "B-1"} {
			_, err := first.full(key)
			Expect(err).NotTo(HaveOccurred())
		}
		usedAt("A-1", 3*time.Hour)
		usedAt("B-1", 2*time.Hour)
		stray := filepath.Join(dir, "dl-123")
		Expect(os.WriteFile(stray, make([]byte, 4*size), 0o600)).To(Succeed())
		old := time.Now().Add(-2 * staleTemp)
		Expect(os.Chtimes(stray, old, old)).To(Succeed())

		_, err := open(size*5/2 + 1).full("C-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(exists("A-1")).To(BeFalse())
		Expect(exists("B-1")).To(BeTrue())
		Expect(exists("C-1")).To(BeTrue())
		Expect(stray).NotTo(BeAnExistingFile())
	})

	fill := func(w *webArt, keys ...string) {
		GinkgoHelper()
		for i, key := range keys {
			_, err := w.full(key)
			Expect(err).NotTo(HaveOccurred())
			usedAt(key, time.Duration(len(keys)-i)*time.Hour)
		}
	}
	kept := func(keys ...string) []string {
		var out []string
		for _, key := range keys {
			if exists(key) {
				out = append(out, key)
			}
		}
		return out
	}

	It("reads and downloads nothing while off", func() {
		w := open(webFullCap)
		_, err := w.full("A-1")
		Expect(err).NotTo(HaveOccurred())
		w.setOff(true)
		_, err = w.full("A-1")
		Expect(err).To(MatchError(errNoWebArt))
		_, err = w.thumb("B-1")
		Expect(err).To(MatchError(errNoWebArt))
		Expect(hits.Load()).To(Equal(int32(1)))
		Expect(exists("B-1")).To(BeFalse())

		w.setOff(false)
		_, err = w.thumb("B-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(hits.Load()).To(Equal(int32(2)))
	})

	It("deletes down to nine tenths of a lower limit", func() {
		w := open(webFullCap)
		fill(w, "A-1", "B-1", "C-1", "D-1", "E-1")
		w.setLimit(3*size + 1)
		Expect(kept("A-1", "B-1", "C-1", "D-1", "E-1")).To(Equal([]string{"D-1", "E-1"}))
	})

	It("shrinks a folder that was already past the limit when trimmed", func() {
		fill(open(webFullCap), "A-1", "B-1", "C-1", "D-1", "E-1")
		w := open(3*size + 1)
		Expect(kept("A-1", "B-1", "C-1", "D-1", "E-1")).To(HaveLen(5))
		w.trim()
		Expect(kept("A-1", "B-1", "C-1", "D-1", "E-1")).To(Equal([]string{"D-1", "E-1"}))
		Expect(hits.Load()).To(Equal(int32(5)))
	})

	It("counts full images and thumbs, leaving out unfinished downloads", func() {
		w := open(webFullCap)
		_, err := w.thumb("A-1")
		Expect(err).NotTo(HaveOccurred())
		_, err = w.full("B-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(dir, "dl-1"), make([]byte, size), 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "thumb", "dl-2"), make([]byte, size), 0o600)).To(Succeed())
		fi, err := os.Stat(filepath.Join(dir, "thumb", "A-1.jpg"))
		Expect(err).NotTo(HaveOccurred())

		Expect(w.usage()).To(Equal(artUsage{FullBytes: 2 * size, FullCount: 2, ThumbBytes: fi.Size(), ThumbCount: 1}))
	})

	It("clears every image, and a thumb downloads again after", func() {
		w := open(webFullCap)
		_, err := w.thumb("A-1")
		Expect(err).NotTo(HaveOccurred())
		_, err = w.full("B-1")
		Expect(err).NotTo(HaveOccurred())
		before := w.usage()

		freed, err := w.clear()
		Expect(err).NotTo(HaveOccurred())
		Expect(freed).To(Equal(before.total()))
		Expect(w.usage()).To(Equal(artUsage{}))
		freed, err = w.clear()
		Expect(err).NotTo(HaveOccurred())
		Expect(freed).To(BeZero())

		_, err = w.thumb("A-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(hits.Load()).To(Equal(int32(3)))
	})

	It("downloads each site's picture from its own address into its own format", func() {
		w := open(webFullCap)
		_, err := w.full("OP07-021_p1")
		Expect(err).NotTo(HaveOccurred())
		Expect(<-paths).To(Equal("/one-piece/OP07/OP07-021_p1_EN.webp"))
		Expect(filepath.Join(dir, "OP07-021_p1.webp")).To(BeAnExistingFile())

		_, err = w.thumb("OP07-021_p3_bd")
		Expect(err).NotTo(HaveOccurred())
		Expect(<-paths).To(Equal("/images/cardlist/card/OP07-021_p3.png"))
		Expect(filepath.Join(dir, "OP07-021_p3_bd.png")).To(BeAnExistingFile())
		Expect(filepath.Join(dir, "thumb", "OP07-021_p3_bd.jpg")).To(BeAnExistingFile())
	})

	It("counts, evicts and clears Bandai's pictures with Limitless's", func() {
		w := open(webFullCap)
		fill(w, "A-1")
		_, err := w.full("OP07-021_p3_bd")
		Expect(err).NotTo(HaveOccurred())
		Expect(w.usage().FullCount).To(Equal(2))
		Expect(w.usage().FullBytes).To(Equal(2 * size))

		w.setLimit(2*size - 1)
		Expect(filepath.Join(dir, "A-1.webp")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(dir, "OP07-021_p3_bd.png")).To(BeAnExistingFile())
		_, err = w.clear()
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Join(dir, "OP07-021_p3_bd.png")).NotTo(BeAnExistingFile())
	})

	It("reads a picture for a grid without counting it as use", func() {
		w := open(webFullCap)
		k := arts.Key{Source: arts.Bandai, Print: "OP07-021_p3"}
		_, err := w.Picture(k)
		Expect(err).NotTo(HaveOccurred())
		path := filepath.Join(dir, "OP07-021_p3_bd.png")
		fi, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(fi.ModTime()).To(BeTemporally("==", unused))

		opened := time.Now().Add(-time.Hour).Truncate(time.Second)
		Expect(os.Chtimes(path, opened, opened)).To(Succeed())
		_, err = w.Picture(k)
		Expect(err).NotTo(HaveOccurred())
		fi, err = os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(fi.ModTime()).To(BeTemporally("==", opened))
		Expect(hits.Load()).To(Equal(int32(1)))
	})

	It("downloads nothing for a key that isn't a printing's", func() {
		w := open(webFullCap)
		for _, key := range []string{"sim:OP07-021", "OP07-021_r1", ""} {
			_, err := w.full(key)
			Expect(err).To(MatchError(errNoWebArt), key)
			_, err = w.thumb(key)
			Expect(err).To(MatchError(errNoWebArt), key)
		}
		Expect(hits.Load()).To(BeZero())
	})

	It("keeps Limitless's downloads going while Bandai's fill their slots", func() {
		w := open(webFullCap)
		holding.Store(true)
		var wg sync.WaitGroup
		for i := range webArtSlots + 1 {
			wg.Go(func() {
				defer GinkgoRecover()
				_, err := w.full(fmt.Sprintf("OP07-021_p%d_bd", i+1))
				Expect(err).NotTo(HaveOccurred())
			})
		}
		done := make(chan error)
		go func() {
			_, err := w.full("OP07-021")
			done <- err
		}()
		Eventually(done).Should(Receive(BeNil()))
		close(hold)
		wg.Wait()
	})

	It("returns every picture it downloads though the next download evicts it", func() {
		w := open(size / 2)
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				defer GinkgoRecover()
				img, err := w.full(fmt.Sprintf("OP07-0%02d", i))
				Expect(err).NotTo(HaveOccurred())
				Expect(img.Bounds().Dx()).To(Equal(600))
			})
		}
		wg.Wait()
	})
})
