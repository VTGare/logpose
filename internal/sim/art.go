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
	"slices"
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

// fullSuffixes are the full image overrides in the order the sim looks for them.
var fullSuffixes = []string{overrideJPG, overridePNG}

// The sim's small images are 120×167 JPEGs.
const (
	smallWidth  = 120
	smallHeight = 167
)

// Override is the full image the sim shows in place of the card's own, or "" when it shows
// its own.
func (i Install) Override(code string) string {
	dir := i.cardDir(code)
	for _, suffix := range fullSuffixes {
		if p := filepath.Join(dir, code+suffix); fileExists(p) {
			return p
		}
	}
	return ""
}

func (i Install) Overrides() []string {
	var codes []string
	for _, suffix := range fullSuffixes {
		paths, _ := filepath.Glob(filepath.Join(i.streamingAssets(), "Cards", "*", "*"+suffix))
		for _, p := range paths {
			base := filepath.Base(p)
			if strings.HasSuffix(base, overrideSmall) {
				continue
			}
			code := strings.TrimSuffix(base, suffix)
			if !slices.Contains(codes, code) {
				codes = append(codes, code)
			}
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

// ArtState is what the sim shows for a card and any kept file of the user's.
type ArtState struct {
	Choice       ArtChoice
	OwnFile      string
	Override     string
	OverrideHash string
}

// SimArt changes which picture the sim shows for a card, through the sim's override files.
// Before it replaces or removes an override it didn't write, it keeps a copy in Dir.
type SimArt struct {
	Install Install
	// Dir holds the record of what logpose wrote and the user's own files it kept.
	Dir string

	mu        sync.Mutex
	record    *artRecord
	loadedDir string
}

// artRecord maps a file's SHA-256 to the printing it shows.
type artRecord struct {
	// Written holds the files logpose wrote.
	Written map[string]string `json:"written"`
	// Matches holds the user's own files that show a printing.
	Matches map[string]string `json:"matches"`
}

// lookup returns the printing the file with this hash shows, whether logpose wrote the file,
// and whether the file is in the record at all.
func (r artRecord) lookup(sum string) (key string, ours, known bool) {
	if key, ok := r.Written[sum]; ok {
		return key, true, true
	}
	key, known = r.Matches[sum]
	return key, false, known
}

func (s *SimArt) recordPath() string { return filepath.Join(s.Dir, "sim-art.json") }
func (s *SimArt) keptDir() string    { return filepath.Join(s.Dir, "kept") }

func (s *SimArt) load() (*artRecord, error) {
	if s.record != nil && s.loadedDir == s.Dir {
		return s.record, nil
	}
	r := artRecord{Written: make(map[string]string), Matches: make(map[string]string)}
	data, err := os.ReadFile(s.recordPath())
	if errors.Is(err, os.ErrNotExist) {
		s.record, s.loadedDir = &r, s.Dir
		return s.record, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	if r.Written == nil {
		r.Written = make(map[string]string)
	}
	if r.Matches == nil {
		r.Matches = make(map[string]string)
	}
	s.record, s.loadedDir = &r, s.Dir
	return s.record, nil
}

func (s *SimArt) save(r *artRecord) error {
	data, err := json.Marshal(r)
	if err != nil {
		s.record = nil
		return err
	}
	if err := writeFile(s.recordPath(), func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	}); err != nil {
		s.record = nil
		return err
	}
	s.record, s.loadedDir = r, s.Dir
	return nil
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

// Inspect reads the card's current choice, override file, override hash and any kept user file
// in one pass.
func (s *SimArt) Inspect(code string) (ArtState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.load()
	if err != nil {
		return ArtState{}, err
	}
	override := s.Install.Override(code)
	if override == "" {
		return ArtState{
			Choice:  ArtChoice{Kind: ArtOwn},
			OwnFile: s.keptOwnFile(code, r),
		}, nil
	}
	sum, err := fileHash(override)
	if err != nil {
		return ArtState{}, err
	}
	st := ArtState{Override: override, OverrideHash: sum}
	key, ours, known := r.lookup(sum)
	switch {
	case ours:
		st.Choice = ArtChoice{Kind: ArtPrint, Key: key}
		st.OwnFile = s.keptOwnFile(code, r)
	case known:
		st.Choice = ArtChoice{Kind: ArtPrint, Key: key}
	default:
		st.Choice = ArtChoice{Kind: ArtFile}
		st.OwnFile = override
	}
	return st, nil
}

func (s *SimArt) keptOwnFile(code string, r *artRecord) string {
	p := s.keptFile(code)
	if p == "" {
		return ""
	}
	sum, err := fileHash(p)
	if err != nil {
		return ""
	}
	if _, _, known := r.lookup(sum); known {
		return ""
	}
	return p
}

// keptFile is the full image kept last for the card, or "".
func (s *SimArt) keptFile(code string) string {
	var newest string
	var at time.Time
	for _, suffix := range fullSuffixes {
		p := filepath.Join(s.keptDir(), code+suffix)
		if info, err := os.Stat(p); err == nil && (newest == "" || info.ModTime().After(at)) {
			newest, at = p, info.ModTime()
		}
	}
	return newest
}

// RecordMatch records that the file with SHA-256 sum is the printing key.
func (s *SimArt) RecordMatch(sum, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sum == "" {
		return nil
	}
	r, err := s.load()
	if err != nil {
		return err
	}
	if _, ours, _ := r.lookup(sum); ours {
		return nil
	}
	if r.Matches[sum] == key {
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
	fullSum, err := writeHashed(full, func(w io.Writer) error { return png.Encode(w, img) })
	if err != nil {
		return err
	}
	small := filepath.Join(dir, code+overrideSmall)
	smallSum, err := writeHashed(small, func(w io.Writer) error {
		return jpeg.Encode(w, smallImage(img), &jpeg.Options{Quality: 90})
	})
	if err != nil {
		return err
	}
	r.Written[fullSum] = key
	r.Written[smallSum] = key
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
func (s *SimArt) keepAll(code string, r *artRecord) error {
	dir := s.Install.cardDir(code)
	for _, suffix := range append(slices.Clone(fullSuffixes), overrideSmall) {
		path := filepath.Join(dir, code+suffix)
		if !fileExists(path) {
			continue
		}
		sum, err := fileHash(path)
		if err != nil {
			return err
		}
		if _, ours, _ := r.lookup(sum); !ours {
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

func writeHashed(path string, write func(io.Writer) error) (string, error) {
	h := sha256.New()
	if err := writeFile(path, func(w io.Writer) error {
		return write(io.MultiWriter(w, h))
	}); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
