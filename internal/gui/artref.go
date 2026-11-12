package gui

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

// artRef is what a picture is. The art cache and the panels keep it in its string form, which
// String writes and parseArtRef reads.
type artRef struct {
	kind refKind
	// code is the card's code or the printing's key, or the path of a file.
	code string
}

const (
	simOwnPrefix  = "sim:"
	ownFilePrefix = "file:"
	donCode       = "Don"
	donBackCode   = "DonBack"
)

func parseArtRef(s string) artRef {
	if code, ok := strings.CutPrefix(s, simOwnPrefix); ok {
		return artRef{refSimOwn, code}
	}
	if path, ok := strings.CutPrefix(s, ownFilePrefix); ok {
		return artRef{refFile, path}
	}
	switch s {
	case donCode:
		return artRef{kind: refDon}
	case donBackCode:
		return artRef{kind: refDonBack}
	}
	return artRef{refPrint, s}
}

func (r artRef) String() string {
	switch r.kind {
	case refSimOwn:
		return simOwnPrefix + r.code
	case refFile:
		return ownFilePrefix + r.code
	case refDon:
		return donCode
	case refDonBack:
		return donBackCode
	}
	return r.code
}

func simOwnRef(code string) string { return artRef{refSimOwn, code}.String() }

func fileRef(path string) string { return artRef{refFile, path}.String() }

// card is the code of the card the picture shows, for its placeholder. A file's is unknown.
func (r artRef) card() string {
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

// fileName is the name a saved picture gets: the printing's key, or the card's code.
func (r artRef) fileName() string {
	switch r.kind {
	case refSimOwn:
		return r.code + "_sim.png"
	case refFile:
		base := filepath.Base(r.code)
		return strings.TrimSuffix(base, filepath.Ext(base)) + ".png"
	}
	return r.String() + ".png"
}

// read is the picture at the size kind is made from. A printing comes from the web art, and
// from the sim when the web has none and simFallback is set.
func (r artRef) read(in *sim.Install, web *arts.Store, kind artKind, simFallback bool) (image.Image, error) {
	small := kind == artFull
	switch r.kind {
	case refSimOwn:
		if in == nil {
			return nil, sim.ErrNotFound
		}
		return decode(in.CardImage(r.code, small))
	case refFile:
		img, err := decode(r.code)
		if err == nil && small {
			img = arts.ScaleToWidth(img, webSmallWidth)
		}
		return img, err
	case refPrint:
		img, err := r.web(web, kind.fromThumb())
		if err == nil {
			if small {
				img = arts.ScaleToWidth(img, webSmallWidth)
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
		return decode(in.DonImage())
	case refDonBack:
		return decode(in.DonBack())
	}
	return decode(in.CardImage(r.code, small))
}

func (r artRef) web(web *arts.Store, thumb bool) (image.Image, error) {
	k, err := arts.ParseKey(r.code)
	if web == nil || err != nil {
		return nil, arts.ErrNoPicture
	}
	if thumb {
		return web.Thumb(k)
	}
	return web.Full(k)
}
