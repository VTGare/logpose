package gui

import (
	"bytes"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("webArt", func() {
	var (
		dir  string
		srv  *httptest.Server
		hits atomic.Int32
		size int64
	)

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		var buf bytes.Buffer
		Expect(png.Encode(&buf, solid(600, 838, color.RGBA{200, 40, 40, 255}))).To(Succeed())
		body := buf.Bytes()
		size = int64(len(body))
		hits.Store(0)
		srv = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			_, _ = rw.Write(body)
		}))
		DeferCleanup(srv.Close)
	})

	open := func(limit int64) *webArt {
		w := newWebArt(dir)
		w.url = func(key string) string { return srv.URL + "/" + key }
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
		for _, key := range []string{"A", "B"} {
			_, err := w.full(key)
			Expect(err).NotTo(HaveOccurred())
		}
		usedAt("A", 3*time.Hour)
		usedAt("B", 2*time.Hour)
		_, err := w.full("A")
		Expect(err).NotTo(HaveOccurred())

		_, err = w.full("C")
		Expect(err).NotTo(HaveOccurred())
		Expect(exists("A")).To(BeTrue())
		Expect(exists("B")).To(BeFalse())
		Expect(exists("C")).To(BeTrue())
	})

	It("leaves room past an eviction, so the next download deletes nothing", func() {
		w := open(10*size + 1)
		keys := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K"}
		for i, key := range keys {
			_, err := w.full(key)
			Expect(err).NotTo(HaveOccurred())
			usedAt(key, time.Duration(len(keys)-i)*time.Hour)
		}
		kept := func() int {
			n := 0
			for _, key := range append(keys, "L") {
				if exists(key) {
					n++
				}
			}
			return n
		}
		Expect(kept()).To(Equal(9))
		_, err := w.full("L")
		Expect(err).NotTo(HaveOccurred())
		Expect(kept()).To(Equal(10))
	})

	It("keeps opened cards when browsing thumbs goes past the limit", func() {
		w := open(3*size + 1)
		_, err := w.full("opened")
		Expect(err).NotTo(HaveOccurred())
		usedAt("opened", time.Hour)
		for _, key := range []string{"A", "B", "C", "D", "E"} {
			_, err := w.thumb(key)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(exists("opened")).To(BeTrue())
	})

	It("keeps the image it just downloaded when that alone is over the limit", func() {
		w := open(size / 2)
		for _, key := range []string{"A", "B"} {
			_, err := w.full(key)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(exists("A")).To(BeFalse())
		Expect(exists("B")).To(BeTrue())
	})

	It("counts the images already in the folder and ignores a crashed download", func() {
		first := open(webFullCap)
		for _, key := range []string{"A", "B"} {
			_, err := first.full(key)
			Expect(err).NotTo(HaveOccurred())
		}
		usedAt("A", 3*time.Hour)
		usedAt("B", 2*time.Hour)
		stray := filepath.Join(dir, "dl-123")
		Expect(os.WriteFile(stray, make([]byte, 4*size), 0o600)).To(Succeed())
		old := time.Now().Add(-2 * staleTemp)
		Expect(os.Chtimes(stray, old, old)).To(Succeed())

		_, err := open(size*5/2 + 1).full("C")
		Expect(err).NotTo(HaveOccurred())
		Expect(exists("A")).To(BeFalse())
		Expect(exists("B")).To(BeTrue())
		Expect(exists("C")).To(BeTrue())
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
		_, err := w.full("A")
		Expect(err).NotTo(HaveOccurred())
		w.setOff(true)
		_, err = w.full("A")
		Expect(err).To(MatchError(errNoWebArt))
		_, err = w.thumb("B")
		Expect(err).To(MatchError(errNoWebArt))
		Expect(hits.Load()).To(Equal(int32(1)))
		Expect(exists("B")).To(BeFalse())

		w.setOff(false)
		_, err = w.thumb("B")
		Expect(err).NotTo(HaveOccurred())
		Expect(hits.Load()).To(Equal(int32(2)))
	})

	It("deletes down to nine tenths of a lower limit", func() {
		w := open(webFullCap)
		fill(w, "A", "B", "C", "D", "E")
		w.setLimit(3*size + 1)
		Expect(kept("A", "B", "C", "D", "E")).To(Equal([]string{"D", "E"}))
	})

	It("shrinks a folder that was already past the limit when trimmed", func() {
		fill(open(webFullCap), "A", "B", "C", "D", "E")
		w := open(3*size + 1)
		Expect(kept("A", "B", "C", "D", "E")).To(HaveLen(5))
		w.trim()
		Expect(kept("A", "B", "C", "D", "E")).To(Equal([]string{"D", "E"}))
		Expect(hits.Load()).To(Equal(int32(5)))
	})

	It("counts full images and thumbs, leaving out unfinished downloads", func() {
		w := open(webFullCap)
		_, err := w.thumb("A")
		Expect(err).NotTo(HaveOccurred())
		_, err = w.full("B")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(dir, "dl-1"), make([]byte, size), 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "thumb", "dl-2"), make([]byte, size), 0o600)).To(Succeed())
		fi, err := os.Stat(filepath.Join(dir, "thumb", "A.jpg"))
		Expect(err).NotTo(HaveOccurred())

		Expect(w.usage()).To(Equal(artUsage{FullBytes: 2 * size, FullCount: 2, ThumbBytes: fi.Size(), ThumbCount: 1}))
	})

	It("clears every image, and a thumb downloads again after", func() {
		w := open(webFullCap)
		_, err := w.thumb("A")
		Expect(err).NotTo(HaveOccurred())
		_, err = w.full("B")
		Expect(err).NotTo(HaveOccurred())
		before := w.usage()

		freed, err := w.clear()
		Expect(err).NotTo(HaveOccurred())
		Expect(freed).To(Equal(before.total()))
		Expect(w.usage()).To(Equal(artUsage{}))
		freed, err = w.clear()
		Expect(err).NotTo(HaveOccurred())
		Expect(freed).To(BeZero())

		_, err = w.thumb("A")
		Expect(err).NotTo(HaveOccurred())
		Expect(hits.Load()).To(Equal(int32(3)))
	})
})
