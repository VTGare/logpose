package gui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	// Registers the WebP decoder for Limitless's card images.
	_ "golang.org/x/image/webp"

	"github.com/VTGare/logpose/internal/cards"
)

// webArtSlots is how many images download at once.
const webArtSlots = 4

// webSmallWidth is how wide a Limitless image is kept for artFull, the size small cards and
// thumbnails use. Limitless only has 600 px images.
const webSmallWidth = 150

// webThumbWidth covers tiles narrower than largeArtMin at 150% scaling.
const webThumbWidth = 300

const webThumbQuality = 85

// webFullCap is how many bytes of full images are kept. Thumbs don't count.
const webFullCap = 150 << 20

// staleTemp is how old a download's temp file must be before it counts as left by a crash.
// It is well past the client's timeout, so a running download's file is never removed.
const staleTemp = time.Hour

const tempPrefix = "dl-"

// webArt keeps card images from Limitless in dir. Full images are downloaded as <key>.webp and
// the least recently used are deleted past limit. Thumbs are thumb/<key>.jpg, made from the
// full image once and never deleted by the limit.
type webArt struct {
	dir    string
	url    func(key string) string
	client *http.Client
	slots  chan struct{}

	mu     sync.Mutex
	limit  int64
	off    bool
	failed map[string]bool
	// fullBytes is the size of every full image, valid once counted is set.
	fullBytes int64
	counted   bool
}

func newWebArt(dir string) *webArt {
	if dir == "" {
		return nil
	}
	return &webArt{
		dir:    dir,
		url:    cards.ArtURL,
		limit:  webFullCap,
		client: &http.Client{Timeout: 30 * time.Second},
		slots:  make(chan struct{}, webArtSlots),
		failed: make(map[string]bool),
	}
}

var errNoWebArt = errors.New("no Limitless image")

func (w *webArt) fullPath(key string) string { return filepath.Join(w.dir, key+".webp") }

func (w *webArt) thumbPath(key string) string {
	return filepath.Join(w.dir, "thumb", key+".jpg")
}

// full is the printing's image as Limitless has it, by its key ("OP14-020" or "OP14-020_p1").
// A key that failed to download isn't tried again until logpose restarts.
func (w *webArt) full(key string) (image.Image, error) { return w.load(key, true) }

func (w *webArt) thumb(key string) (image.Image, error) {
	if w.isOff() {
		return nil, errNoWebArt
	}
	path := w.thumbPath(key)
	if img, err := decode(path); err == nil {
		return img, nil
	}
	img, err := w.load(key, false)
	if err != nil {
		return nil, err
	}
	img = scaleToWidth(img, webThumbWidth)
	// A thumb that can't be written is made again next time, which only costs time.
	_ = writeThumb(path, img)
	return img, nil
}

// unused is the time a full image gets when it was only read to make a thumb, so browsing
// the grid never pushes out the cards that were opened.
var unused = time.Unix(1, 0)

// load reads or downloads the full image. Its modification time is when it was last used,
// which eviction goes by, and only a use moves it forward.
func (w *webArt) load(key string, use bool) (image.Image, error) {
	if w.isOff() {
		return nil, errNoWebArt
	}
	path := w.fullPath(key)
	if img, err := decode(path); err == nil {
		if use {
			now := time.Now()
			_ = os.Chtimes(path, now, now)
		}
		return img, nil
	}
	w.mu.Lock()
	failed := w.failed[key]
	w.mu.Unlock()
	if failed {
		return nil, errNoWebArt
	}
	w.slots <- struct{}{}
	err := w.download(key, path)
	<-w.slots
	if err != nil {
		w.mu.Lock()
		w.failed[key] = true
		w.mu.Unlock()
		return nil, err
	}
	if !use {
		_ = os.Chtimes(path, unused, unused)
	}
	return decode(path)
}

func (w *webArt) isOff() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.off
}

func (w *webArt) setOff(off bool) {
	w.mu.Lock()
	w.off = off
	w.mu.Unlock()
}

// setLimit sets how many bytes of full images are kept. It deletes down to the new limit only
// once the folder has been counted, so it never lists the folder itself.
func (w *webArt) setLimit(limit int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.limit = limit
	if w.counted && w.fullBytes > w.limit {
		w.evict("")
	}
}

// trim counts the folder and deletes down to the limit. Eviction otherwise only runs after a
// download, so a folder already past the limit would stay that way.
func (w *webArt) trim() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.count()
	if w.fullBytes > w.limit {
		w.evict("")
	}
}

func (w *webArt) count() {
	if !w.counted {
		_, w.fullBytes = w.fulls()
		w.counted = true
	}
}

type artUsage struct {
	FullBytes  int64
	FullCount  int
	ThumbBytes int64
	ThumbCount int
}

func (u artUsage) total() int64 { return u.FullBytes + u.ThumbBytes }

func (w *webArt) usage() artUsage {
	w.mu.Lock()
	defer w.mu.Unlock()
	files, total := w.fulls()
	w.fullBytes, w.counted = total, true
	thumbs, thumbBytes := w.thumbs()
	return artUsage{FullBytes: total, FullCount: len(files), ThumbBytes: thumbBytes, ThumbCount: len(thumbs)}
}

func (w *webArt) thumbs() ([]string, int64) {
	dir := filepath.Join(w.dir, "thumb")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0
	}
	var paths []string
	var total int64
	for _, e := range entries {
		if !e.Type().IsRegular() || filepath.Ext(e.Name()) != ".jpg" {
			continue
		}
		if fi, err := e.Info(); err == nil {
			paths = append(paths, filepath.Join(dir, e.Name()))
			total += fi.Size()
		}
	}
	return paths, total
}

func (w *webArt) clear() (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	files, _ := w.fulls()
	thumbs, _ := w.thumbs()
	paths := make([]string, 0, len(files)+len(thumbs))
	for _, f := range files {
		paths = append(paths, f.path)
	}
	paths = append(paths, thumbs...)
	var freed int64
	var errs []error
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if err := os.Remove(p); err != nil {
			errs = append(errs, err)
			continue
		}
		freed += fi.Size()
	}
	_, w.fullBytes = w.fulls()
	w.counted = true
	return freed, errors.Join(errs...)
}

func writeThumb(path string, img image.Image) error {
	tmp, _, err := writeTemp(filepath.Dir(path), func(f io.Writer) error {
		return jpeg.Encode(f, img, &jpeg.Options{Quality: webThumbQuality})
	})
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (w *webArt) download(key, path string) error {
	url := w.url(key)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "logpose (+https://github.com/VTGare/logpose)")
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	tmp, n, err := writeTemp(w.dir, func(f io.Writer) error {
		_, err := io.Copy(f, io.LimitReader(resp.Body, 8<<20))
		return err
	})
	if err != nil {
		return err
	}
	if err := w.keep(tmp, path, n); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func writeTemp(dir string, fill func(io.Writer) error) (string, int64, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", 0, err
	}
	f, err := os.CreateTemp(dir, tempPrefix+"*")
	if err != nil {
		return "", 0, err
	}
	err = fill(f)
	var n int64
	if err == nil {
		n, err = f.Seek(0, io.SeekCurrent)
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		_ = os.Remove(f.Name())
		return "", 0, err
	}
	return f.Name(), n, nil
}

// keep moves a downloaded full image of n bytes into place. Past the limit, it deletes the
// least recently used full images, never the one it just moved.
func (w *webArt) keep(tmp, path string, n int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.count()
	if fi, err := os.Stat(path); err == nil {
		w.fullBytes -= fi.Size()
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	w.fullBytes += n
	if w.fullBytes > w.limit {
		w.evict(path)
	}
	return nil
}

type fullFile struct {
	path string
	size int64
	used time.Time
}

// fulls lists the full images and their total size. It removes temp files left by a crash.
func (w *webArt) fulls() ([]fullFile, int64) {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return nil, 0
	}
	var files []fullFile
	var total int64
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(w.dir, e.Name())
		switch {
		case strings.HasPrefix(e.Name(), tempPrefix):
			if time.Since(fi.ModTime()) > staleTemp {
				_ = os.Remove(path)
			}
		case filepath.Ext(e.Name()) == ".webp":
			files = append(files, fullFile{path, fi.Size(), fi.ModTime()})
			total += fi.Size()
		}
	}
	return files, total
}

// evict deletes down to nine tenths of the limit, so the folder isn't listed again on every
// download that follows.
func (w *webArt) evict(keep string) {
	files, total := w.fulls()
	slices.SortFunc(files, func(a, b fullFile) int { return a.used.Compare(b.used) })
	target := w.limit / 10 * 9
	for _, f := range files {
		if total <= target {
			break
		}
		if f.path == keep {
			continue
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
	w.fullBytes = total
}
