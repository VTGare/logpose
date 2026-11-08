package sim

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"
)

// The sim shows <code>_OVERRIDE.jpg, then <code>_OVERRIDE.png, in place of a card's own full
// image, and <code>_small_OVERRIDE.jpg in place of its small one, from the card's set folder.
const (
	overrideJPG   = "_OVERRIDE.jpg"
	overridePNG   = "_OVERRIDE.png"
	overrideSmall = "_small_OVERRIDE.jpg"
)

// The sim's small images are 120×167 JPEGs.
const (
	smallWidth  = 120
	smallHeight = 167
)

func (i Install) cardDir(code string) string {
	set, _, _ := strings.Cut(code, "-")
	return filepath.Join(i.streamingAssets(), "Cards", set)
}

// Override is the full image the sim shows in place of the card's own, or "" when it shows
// its own.
func (i Install) Override(code string) string {
	dir := i.cardDir(code)
	for _, suffix := range []string{overrideJPG, overridePNG} {
		if p := filepath.Join(dir, code+suffix); fileExists(p) {
			return p
		}
	}
	return ""
}

// Overrides lists the cards the sim shows another full image for.
func (i Install) Overrides() []string {
	var codes []string
	for _, suffix := range []string{overrideJPG, overridePNG} {
		paths, _ := filepath.Glob(filepath.Join(i.streamingAssets(), "Cards", "*", "*"+suffix))
		for _, p := range paths {
			codes = append(codes, strings.TrimSuffix(filepath.Base(p), suffix))
		}
	}
	return codes
}

// ArtKind is where the picture the sim shows for a card comes from.
type ArtKind int

const (
	// ArtOwn is the sim's own picture.
	ArtOwn ArtKind = iota
	// ArtPrint is a printing's picture, the one Key names.
	ArtPrint
	// ArtFile is a file logpose didn't write and doesn't know as a printing.
	ArtFile
)

type ArtChoice struct {
	Kind ArtKind
	Key  string
}

// SimArt changes which picture the sim shows for a card, through the sim's override files.
// Before it replaces or removes an override it didn't write, it keeps a copy in Dir.
type SimArt struct {
	Install Install
	// Dir holds the record of what logpose wrote and the user's own files it kept.
	Dir string

	mu sync.Mutex
}

// artRecord maps a file's SHA-256 to the printing it shows.
type artRecord struct {
	// Written holds the files logpose wrote.
	Written map[string]string `json:"written"`
	// Matches holds the user's own files that show a printing.
	Matches map[string]string `json:"matches"`
}

func (s *SimArt) recordPath() string { return filepath.Join(s.Dir, "sim-art.json") }
func (s *SimArt) keptDir() string    { return filepath.Join(s.Dir, "kept") }

func (s *SimArt) load() (artRecord, error) {
	r := artRecord{Written: make(map[string]string), Matches: make(map[string]string)}
	data, err := os.ReadFile(s.recordPath())
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, err
	}
	if r.Written == nil {
		r.Written = make(map[string]string)
	}
	if r.Matches == nil {
		r.Matches = make(map[string]string)
	}
	return r, nil
}

func (s *SimArt) save(r artRecord) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return writeFile(s.recordPath(), func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Current is what the sim shows for the card.
func (s *SimArt) Current(code string) (ArtChoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current(code)
}

func (s *SimArt) current(code string) (ArtChoice, error) {
	path := s.Install.Override(code)
	if path == "" {
		return ArtChoice{Kind: ArtOwn}, nil
	}
	r, err := s.load()
	if err != nil {
		return ArtChoice{}, err
	}
	sum, err := fileHash(path)
	if err != nil {
		return ArtChoice{}, err
	}
	if key, ok := r.Written[sum]; ok {
		return ArtChoice{Kind: ArtPrint, Key: key}, nil
	}
	if key, ok := r.Matches[sum]; ok {
		return ArtChoice{Kind: ArtPrint, Key: key}, nil
	}
	return ArtChoice{Kind: ArtFile}, nil
}

// OwnFile is the user's own file for the card: the override the sim shows when logpose didn't
// write it, or else the newest copy logpose kept. It's "" when there's none, or when the file
// is known to show a printing.
func (s *SimArt) OwnFile(code string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.load()
	if err != nil {
		return ""
	}
	candidates := []string{s.Install.Override(code), s.keptFile(code)}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		sum, err := fileHash(p)
		if err != nil {
			continue
		}
		if _, ours := r.Written[sum]; ours {
			continue
		}
		if _, known := r.Matches[sum]; known {
			return ""
		}
		return p
	}
	return ""
}

// keptFile is the newest full image kept for the card, or "".
func (s *SimArt) keptFile(code string) string {
	for _, suffix := range []string{overridePNG, overrideJPG} {
		if p := filepath.Join(s.keptDir(), code+suffix); fileExists(p) {
			return p
		}
	}
	return ""
}

// Matched records that the override the sim shows for the card is the printing key. Current
// then reports that printing.
func (s *SimArt) Matched(code, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.Install.Override(code)
	if path == "" {
		return nil
	}
	r, err := s.load()
	if err != nil {
		return err
	}
	sum, err := fileHash(path)
	if err != nil {
		return err
	}
	if _, ours := r.Written[sum]; ours {
		return nil
	}
	r.Matches[sum] = key
	return s.save(r)
}

// UsePrint makes the sim show img, the printing key, for the card. It writes the full image as
// a PNG and the small one as a JPEG.
func (s *SimArt) UsePrint(code, key string, img image.Image) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.load()
	if err != nil {
		return err
	}
	if err := s.keepAll(code, r); err != nil {
		return err
	}
	dir := s.Install.cardDir(code)
	full := filepath.Join(dir, code+overridePNG)
	if err := writeFile(full, func(w io.Writer) error { return png.Encode(w, img) }); err != nil {
		return err
	}
	small := filepath.Join(dir, code+overrideSmall)
	if err := writeFile(small, func(w io.Writer) error {
		return jpeg.Encode(w, smallImage(img), &jpeg.Options{Quality: 90})
	}); err != nil {
		return err
	}
	for _, p := range []string{full, small} {
		sum, err := fileHash(p)
		if err != nil {
			return err
		}
		r.Written[sum] = key
	}
	return s.save(r)
}

// UseOwn makes the sim show its own picture for the card.
func (s *SimArt) UseOwn(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.load()
	if err != nil {
		return err
	}
	return s.keepAll(code, r)
}

// UseFile makes the sim show the user's own file again, from the copy logpose kept.
func (s *SimArt) UseFile(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.keptFile(code)
	if kept == "" {
		return fmt.Errorf("sim: no file of your own kept for %s", code)
	}
	r, err := s.load()
	if err != nil {
		return err
	}
	if err := s.keepAll(code, r); err != nil {
		return err
	}
	dir := s.Install.cardDir(code)
	for _, suffix := range []string{filepath.Base(kept)[len(code):], overrideSmall} {
		from := filepath.Join(s.keptDir(), code+suffix)
		if !fileExists(from) {
			continue
		}
		if err := copyFile(from, filepath.Join(dir, code+suffix)); err != nil {
			return err
		}
	}
	return nil
}

// keepAll removes the card's override files. Each one logpose didn't write is kept first.
func (s *SimArt) keepAll(code string, r artRecord) error {
	dir := s.Install.cardDir(code)
	for _, suffix := range []string{overrideJPG, overridePNG, overrideSmall} {
		path := filepath.Join(dir, code+suffix)
		if !fileExists(path) {
			continue
		}
		sum, err := fileHash(path)
		if err != nil {
			return err
		}
		if _, ours := r.Written[sum]; !ours {
			if err := s.keep(path, sum); err != nil {
				return err
			}
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

// keep copies a file into the kept folder under its own name. An older, different file of
// that name gets the time it was kept added to its name, so nothing kept is lost.
func (s *SimArt) keep(path, sum string) error {
	dst := filepath.Join(s.keptDir(), filepath.Base(path))
	if fileExists(dst) {
		old, err := fileHash(dst)
		if err != nil {
			return err
		}
		if old == sum {
			return nil
		}
		ext := filepath.Ext(dst)
		aside := strings.TrimSuffix(dst, ext) + "." + strconv.FormatInt(time.Now().UnixMilli(), 10) + ext
		if err := os.Rename(dst, aside); err != nil {
			return err
		}
	}
	return copyFile(path, dst)
}

// smallImage scales a card to the sim's small size. Transparent corners become black, since
// JPEG has no alpha.
func smallImage(img image.Image) image.Image {
	out := image.NewRGBA(image.Rect(0, 0, smallWidth, smallHeight))
	draw.Draw(out, out.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(out, out.Bounds(), img, img.Bounds(), draw.Over, nil)
	return out
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	return writeFile(to, func(w io.Writer) error {
		_, err := io.Copy(w, src)
		return err
	})
}

// writeFile writes through a temporary file in the same folder, so a reader never sees half a
// file.
func writeFile(path string, write func(io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".logpose-*")
	if err != nil {
		return err
	}
	err = write(tmp)
	err = errors.Join(err, tmp.Close())
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}
