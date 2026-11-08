package gui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

// webArt downloads card images from Limitless into dir once, and reads them from there after.
type webArt struct {
	dir    string
	client *http.Client
	slots  chan struct{}

	mu     sync.Mutex
	failed map[string]bool
}

func newWebArt(dir string) *webArt {
	if dir == "" {
		return nil
	}
	return &webArt{
		dir:    dir,
		client: &http.Client{Timeout: 30 * time.Second},
		slots:  make(chan struct{}, webArtSlots),
		failed: make(map[string]bool),
	}
}

var errNoWebArt = errors.New("no Limitless image")

// image is the printing's image, by its key ("OP14-020" or "OP14-020_p1"). A key that failed
// to download isn't tried again until logpose restarts.
func (w *webArt) image(key string) (image.Image, error) {
	path := filepath.Join(w.dir, key+".webp")
	if img, err := decode(path); err == nil {
		return img, nil
	}
	w.mu.Lock()
	failed := w.failed[key]
	w.mu.Unlock()
	if failed {
		return nil, errNoWebArt
	}
	w.slots <- struct{}{}
	err := w.download(cards.ArtURL(key), path)
	<-w.slots
	if err != nil {
		w.mu.Lock()
		w.failed[key] = true
		w.mu.Unlock()
		return nil, err
	}
	return decode(path)
}

func (w *webArt) download(url, path string) error {
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
	if err := os.MkdirAll(w.dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(w.dir, "dl-*")
	if err != nil {
		return err
	}
	_, err = io.Copy(tmp, io.LimitReader(resp.Body, 8<<20))
	err = errors.Join(err, tmp.Close())
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}
