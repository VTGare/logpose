package arts

import (
	"image"
	"path/filepath"
	"strings"

	"github.com/VTGare/logpose/internal/sim"
)

type refKind int

const (
	// refPrint is a card code or printing key, resolved from web art and then the sim.
	refPrint refKind = iota
	// refExactPrint names a specific printing without mirroring the sim's override.
	refExactPrint
	// refSimOwn is the sim's own picture of a card.
	refSimOwn
	// refFile is a local image file by path.
	refFile
	refDon
	refDonBack
)

const (
	exactPrintPrefix = "print:"
	simOwnPrefix     = "sim:"
	ownFilePrefix    = "file:"
	DonCode          = "Don"
	DonBackCode      = "DonBack"
)

const (
	// SmallWidth is how wide SizeSmall images are kept when scaled from a web thumb, local file
	// or mirrored override.
	SmallWidth = 150
	// LargeWidth caps mirrored custom overrides so huge PNGs do not bloat previews.
	LargeWidth = 600
)

// Size selects which resolution Resolve returns.
type Size int

const (
	SizeFull Size = iota
	SizeThumb
	SizeSmall
)

// Ref names a card picture source in either typed or string form.
type Ref struct {
	kind refKind
	// code is the card code, printing key, or file path depending on kind.
	code string
}

func ParseRef(s string) Ref {
	if code, ok := strings.CutPrefix(s, exactPrintPrefix); ok {
		return Ref{kind: refExactPrint, code: code}
	}
	if code, ok := strings.CutPrefix(s, simOwnPrefix); ok {
		return Ref{kind: refSimOwn, code: code}
	}
	if path, ok := strings.CutPrefix(s, ownFilePrefix); ok {
		return Ref{kind: refFile, code: path}
	}
	switch s {
	case DonCode:
		return Ref{kind: refDon}
	case DonBackCode:
		return Ref{kind: refDonBack}
	}
	return Ref{kind: refPrint, code: s}
}

func PrintRef(key string) Ref { return Ref{kind: refPrint, code: key} }

// ExactPrintRef keeps a printing key on its own art even when mirroring sim overrides.
// Non-print references and empty keys are returned as parsed.
func ExactPrintRef(key string) Ref {
	r := ParseRef(key)
	if r.kind != refPrint || key == "" {
		return r
	}
	return Ref{kind: refExactPrint, code: r.code}
}

func SimOwnRef(code string) Ref { return Ref{kind: refSimOwn, code: code} }

func FileRef(path string) Ref { return Ref{kind: refFile, code: path} }

func DonRef() Ref { return Ref{kind: refDon} }

func DonBackRef() Ref { return Ref{kind: refDonBack} }

func (r Ref) IsExact() bool { return r.kind == refExactPrint }

func (r Ref) String() string {
	switch r.kind {
	case refExactPrint:
		return exactPrintPrefix + r.code
	case refSimOwn:
		return simOwnPrefix + r.code
	case refFile:
		return ownFilePrefix + r.code
	case refDon:
		return DonCode
	case refDonBack:
		return DonBackCode
	}
	return r.code
}

// Card is the card code for a placeholder, or "" for files and DON!! cards.
func (r Ref) Card() string {
	switch r.kind {
	case refSimOwn:
		return r.code
	case refPrint, refExactPrint:
		if k, err := ParseKey(r.code); err == nil {
			return k.Code()
		}
		return r.code
	}
	return ""
}

// FileName is the default PNG file name when saving the picture.
func (r Ref) FileName() string {
	switch r.kind {
	case refExactPrint:
		return r.code + ".png"
	case refSimOwn:
		return r.code + "_sim.png"
	case refFile:
		base := filepath.Base(r.code)
		return strings.TrimSuffix(base, filepath.Ext(base)) + ".png"
	}
	return r.String() + ".png"
}

// Resolve loads ref at size. When mirror is true, plain card codes use the sim's override first.
// When simFallback is true, printing lookups fall back to the sim's own image if web art fails.
func Resolve(in *sim.Install, web *Store, ref Ref, size Size, mirror, simFallback bool) (image.Image, error) {
	if mirror && in != nil && ref.kind == refPrint && ref.code != "" {
		if img, ok := resolveMirror(in, ref.code, size); ok {
			return img, nil
		}
	}
	small := size == SizeSmall
	switch ref.kind {
	case refSimOwn:
		if in == nil {
			return nil, sim.ErrNotFound
		}
		return Decode(in.CardImage(ref.code, small))
	case refFile:
		img, err := Decode(ref.code)
		if err == nil && small {
			img = ScaleToWidth(img, SmallWidth)
		}
		return img, err
	case refDon:
		if in == nil {
			return nil, ErrNoPicture
		}
		// DON!! cards have no small image.
		return Decode(in.DonImage())
	case refDonBack:
		if in == nil {
			return nil, ErrNoPicture
		}
		return Decode(in.DonBack())
	case refPrint, refExactPrint:
		img, err := readWeb(web, ref.code, size != SizeFull)
		if err == nil {
			if small {
				img = ScaleToWidth(img, SmallWidth)
			}
			return img, nil
		}
		if !simFallback {
			return nil, err
		}
	}
	if in == nil {
		return nil, ErrNoPicture
	}
	return Decode(in.CardImage(ref.code, small))
}

func resolveMirror(in *sim.Install, code string, size Size) (image.Image, bool) {
	path := in.Override(code)
	if path == "" {
		return nil, false
	}
	img, err := Decode(path)
	if err != nil {
		return nil, false
	}
	switch size {
	case SizeSmall:
		img = ScaleToWidth(img, SmallWidth)
	case SizeFull:
		img = ScaleToWidth(img, LargeWidth)
	}
	return img, true
}

func readWeb(web *Store, code string, thumb bool) (image.Image, error) {
	k, err := ParseKey(code)
	if web == nil || err != nil {
		return nil, ErrNoPicture
	}
	if thumb {
		return web.Thumb(k)
	}
	return web.Full(k)
}
