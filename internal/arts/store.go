package arts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"

	// Registers the PNG decoder for Bandai's card images.
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"

	"github.com/VTGare/logpose/internal/fetch"
	// Registers the WebP decoder for Limitless's card images.
	_ "golang.org/x/image/webp"
)

// storeSlots is how many images download at once from each site. Bandai's take about 1.9 s
// each, so they get slots of their own and never hold up Limitless's.
const storeSlots = 4

// ThumbWidth is how wide a thumb is kept. It covers the app's tiles narrower than its large
// art at 150% scaling, and the GUI's card width. Thumbs already kept stay at the width they
// were made with.
const ThumbWidth = 300

const thumbQuality = 85

// DefaultCap is how many bytes of full images are kept until SetLimit says otherwise. Thumbs
// don't count.
const DefaultCap = 150 << 20

// staleTemp is how old a download's temp file must be before it counts as left by a crash.
// It is well past the client's timeout, so a running download's file is never removed.
const staleTemp = time.Hour

const tempPrefix = "dl-"

// Store keeps card images from Limitless and Bandai in dir. Full images are downloaded as
// <key><Key.Ext>, in the site's own format, and the least recently used are deleted past limit.
// Thumbs are thumb/<key>.jpg, made from the full image once and never deleted by the limit.
type Store struct {
	dir    string
	client *http.Client
	slots  map[Source]chan struct{}

	mu     sync.Mutex
	limit  int64
	off    bool
	failed map[Key]bool
	// fullBytes is the size of every full image, valid once counted is set.
	fullBytes int64
	counted   bool
}

func NewStore(dir string) *Store {
	if dir == "" {
		return nil
	}
	return &Store{
		dir:    dir,
		limit:  DefaultCap,
		client: fetch.NewClient(),
		slots: map[Source]chan struct{}{
			Limitless: make(chan struct{}, storeSlots),
			Bandai:    make(chan struct{}, storeSlots),
		},
		failed: make(map[Key]bool),
	}
}

// ErrNoPicture is what reading an image gives when there's none to download or downloads are
// off.
var ErrNoPicture = errors.New("arts: no downloaded image")

func (w *Store) fullPath(k Key) string { return filepath.Join(w.dir, k.String()+k.Ext()) }

func (w *Store) thumbPath(k Key) string {
	return filepath.Join(w.dir, "thumb", k.String()+".jpg")
}

// Full is the printing's image as its site has it. A key that failed to download isn't tried
// again until logpose restarts.
func (w *Store) Full(k Key) (image.Image, error) { return w.load(k, true) }

// Picture is the full image for a grid. Reading it never counts as using it.
func (w *Store) Picture(k Key) (image.Image, error) { return w.load(k, false) }

func (w *Store) Thumb(k Key) (image.Image, error) {
	if w.Off() {
		return nil, ErrNoPicture
	}
	path := w.thumbPath(k)
	if img, err := decode(path); err == nil {
		return img, nil
	}
	img, err := w.load(k, false)
	if err != nil {
		return nil, err
	}
	img = ScaleToWidth(img, ThumbWidth)
	// A thumb that can't be written is made again next time, which only costs time.
	_ = writeThumb(path, img)
	return img, nil
}

// unused is the time a full image gets when it was only read to make a thumb or a grid, so
// browsing the grid and comparing pictures never push out the cards that were opened.
var unused = time.Unix(1, 0)

// load reads or downloads the full image. Its modification time is when it was last used,
// which eviction goes by, and only a use moves it forward.
func (w *Store) load(k Key, use bool) (image.Image, error) {
	if w.Off() {
		return nil, ErrNoPicture
	}
	path := w.fullPath(k)
	if img, err := decode(path); err == nil {
		if use {
			now := time.Now()
			_ = os.Chtimes(path, now, now)
		}
		return img, nil
	}
	w.mu.Lock()
	failed := w.failed[k]
	w.mu.Unlock()
	if failed {
		return nil, ErrNoPicture
	}
	slots := w.slots[k.Source]
	slots <- struct{}{}
	img, err := w.download(k, path, use)
	<-slots
	if err != nil {
		w.mu.Lock()
		w.failed[k] = true
		w.mu.Unlock()
		return nil, err
	}
	return img, nil
}

func (w *Store) Off() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.off
}

func (w *Store) SetOff(off bool) {
	w.mu.Lock()
	w.off = off
	w.mu.Unlock()
}

// SetLimit sets how many bytes of full images are kept. It deletes down to the new limit only
// once the folder has been counted, so it never lists the folder itself.
func (w *Store) SetLimit(limit int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.limit = limit
	if w.counted && w.fullBytes > w.limit {
		w.evict("")
	}
}

// Trim counts the folder and deletes down to the limit. Eviction otherwise only runs after a
// download, so a folder already past the limit would stay that way.
func (w *Store) Trim() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.count()
	if w.fullBytes > w.limit {
		w.evict("")
	}
}

func (w *Store) count() {
	if !w.counted {
		_, w.fullBytes = w.fulls()
		w.counted = true
	}
}

// Usage is what the store keeps on disk.
type Usage struct {
	FullBytes  int64
	FullCount  int
	ThumbBytes int64
	ThumbCount int
}

func (u Usage) Total() int64 { return u.FullBytes + u.ThumbBytes }

func (w *Store) Usage() Usage {
	w.mu.Lock()
	defer w.mu.Unlock()
	files, total := w.fulls()
	w.fullBytes, w.counted = total, true
	thumbs, thumbBytes := w.thumbs()
	return Usage{FullBytes: total, FullCount: len(files), ThumbBytes: thumbBytes, ThumbCount: len(thumbs)}
}

func (w *Store) thumbs() ([]string, int64) {
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

func (w *Store) Clear() (int64, error) {
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
		return jpeg.Encode(f, img, &jpeg.Options{Quality: thumbQuality})
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

// download fetches the image and keeps it as the site sent it. It decodes the bytes before
// keeping them, so another download's eviction can't delete the file before it is read. A
// download only made for a thumb or a grid gets the unused time.
func (w *Store) download(k Key, path string, use bool) (image.Image, error) {
	url := k.URL()
	body, err := fetch.Get(context.Background(), w.client, url, 8<<20)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	tmp, n, err := writeTemp(w.dir, func(f io.Writer) error {
		_, err := f.Write(body)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := w.keep(tmp, path, n, use); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}
	return img, nil
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
func (w *Store) keep(tmp, path string, n int64, use bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.count()
	if fi, err := os.Stat(path); err == nil {
		w.fullBytes -= fi.Size()
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if !use {
		_ = os.Chtimes(path, unused, unused)
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
func (w *Store) fulls() ([]fullFile, int64) {
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
		case slices.Contains(fullExts, filepath.Ext(e.Name())):
			files = append(files, fullFile{path, fi.Size(), fi.ModTime()})
			total += fi.Size()
		}
	}
	return files, total
}

// evict deletes down to nine tenths of the limit, so the folder isn't listed again on every
// download that follows.
func (w *Store) evict(keep string) {
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

func decode(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	img, _, err := image.Decode(f)
	return img, err
}

func ScaleToWidth(img image.Image, w int) image.Image {
	b := img.Bounds()
	if b.Dx() <= w {
		return img
	}
	h := b.Dy() * w / b.Dx()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(out, out.Bounds(), img, b, xdraw.Src, nil)
	return out
}
