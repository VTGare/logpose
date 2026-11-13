package page

import (
	"image"
	"path/filepath"
	"strings"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/sim"
)

type refKind int

const (
	// refPrint is a card's code or a printing's key, read from the web art and then the sim.
	refPrint refKind = iota
	// refSimOwn is the sim's own picture of a card.
	refSimOwn
	// refFile is a file of the user's by its path.
	refFile
	refDon
	refDonBack
)

// ArtRef is what a picture is. The art cache and the panels keep it in its string form, which
// String writes and parseArtRef reads.
type ArtRef struct {
	kind refKind
	// code is the card's code or the printing's key, or the path of a file.
	code string
}

const (
	simOwnPrefix  = "sim:"
	ownFilePrefix = "file:"
	DonCode       = "Don"
	DonBackCode   = "DonBack"
)

func ParseArtRef(s string) ArtRef {
	if code, ok := strings.CutPrefix(s, simOwnPrefix); ok {
		return ArtRef{refSimOwn, code}
	}
	if path, ok := strings.CutPrefix(s, ownFilePrefix); ok {
		return ArtRef{refFile, path}
	}
	switch s {
	case DonCode:
		return ArtRef{kind: refDon}
	case DonBackCode:
		return ArtRef{kind: refDonBack}
	}
	return ArtRef{refPrint, s}
}

func (r ArtRef) String() string {
	switch r.kind {
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

// PrintRef is the art of a card or printing, by its code or key.
func PrintRef(key string) ArtRef { return ArtRef{refPrint, key} }

func SimOwnRef(code string) string { return ArtRef{refSimOwn, code}.String() }

func FileRef(path string) string { return ArtRef{refFile, path}.String() }

// Card is the code of the Card the picture shows, for its placeholder. A file's is unknown.
func (r ArtRef) Card() string {
	switch r.kind {
	case refSimOwn:
		return r.code
	case refPrint:
		if k, err := arts.ParseKey(r.code); err == nil {
			return k.Code()
		}
		return r.code
	}
	return ""
}

// FileName is the name a saved picture gets: the printing's key, or the card's code.
func (r ArtRef) FileName() string {
	switch r.kind {
	case refSimOwn:
		return r.code + "_sim.png"
	case refFile:
		base := filepath.Base(r.code)
		return strings.TrimSuffix(base, filepath.Ext(base)) + ".png"
	}
	return r.String() + ".png"
}

// Read is the picture at the size kind is made from. A printing comes from the web art, and
// from the sim when the web has none and simFallback is set.
func (r ArtRef) Read(in *sim.Install, web *arts.Store, kind artKind, simFallback bool) (image.Image, error) {
	small := kind == ArtFull
	switch r.kind {
	case refSimOwn:
		if in == nil {
			return nil, sim.ErrNotFound
		}
		return Decode(in.CardImage(r.code, small))
	case refFile:
		img, err := Decode(r.code)
		if err == nil && small {
			img = arts.ScaleToWidth(img, WebSmallWidth)
		}
		return img, err
	case refPrint:
		img, err := r.web(web, kind.fromThumb())
		if err == nil {
			if small {
				img = arts.ScaleToWidth(img, WebSmallWidth)
			}
			return img, nil
		}
		if !simFallback {
			return nil, err
		}
	}
	if in == nil {
		return nil, arts.ErrNoPicture
	}
	// DON!! cards have no small image, so every size comes from the full one.
	switch r.kind {
	case refDon:
		return Decode(in.DonImage())
	case refDonBack:
		return Decode(in.DonBack())
	}
	return Decode(in.CardImage(r.code, small))
}

func (r ArtRef) web(web *arts.Store, thumb bool) (image.Image, error) {
	k, err := arts.ParseKey(r.code)
	if web == nil || err != nil {
		return nil, arts.ErrNoPicture
	}
	if thumb {
		return web.Thumb(k)
	}
	return web.Full(k)
}
