// Package arts groups a card's printings from Limitless and Bandai's English card list into
// the distinct pictures they show.
package arts

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/VTGare/logpose/internal/cards"
)

type Source uint8

const (
	Limitless Source = iota + 1
	Bandai
)

// Key is one printing's picture on one site. Bandai's _p8 and Limitless's _p8 are often
// different pictures, so a key always carries its site.
type Key struct {
	Source Source
	// Print is the site's own key: the code, with _pN for an alternate printing.
	Print string
}

// bandaiSuffix marks Bandai keys in their string form. A plain key stays Limitless's, which
// keeps the keys already saved in sim-art.json and the art folder meaning what they meant.
// The string has no ':', since it is a Windows file name, and cutting it at the first '_'
// still gives the card and at the first '-' the set.
const bandaiSuffix = "_bd"

var bandaiKey = regexp.MustCompile(`^(` + cards.PrintKeyPattern + `)` + bandaiSuffix + `$`)

// ParseKey reads a key's string form. "OP07-021_p1" is Limitless's, "OP07-021_p3_bd" Bandai's.
func ParseKey(s string) (Key, error) {
	if cards.IsPrintKey(s) {
		return Key{Limitless, s}, nil
	}
	if m := bandaiKey.FindStringSubmatch(s); m != nil {
		return Key{Bandai, m[1]}, nil
	}
	return Key{}, fmt.Errorf("arts: %q is not a printing's key", s)
}

func (k Key) String() string {
	if k.Source == Bandai {
		return k.Print + bandaiSuffix
	}
	return k.Print
}

func (k Key) Code() string {
	code, _, _ := strings.Cut(k.Print, "_")
	return code
}

func (k Key) URL() string {
	if k.Source == Bandai {
		return bandaiArtURL(k.Print)
	}
	return limitlessArtURL(k.Print)
}

// Ext is the full image's file extension in the art folder, the site's own format.
func (k Key) Ext() string {
	if k.Source == Bandai {
		return fullExts[1]
	}
	return fullExts[0]
}

var fullExts = []string{".webp", ".png"}

// limitlessArtURL is a printing's image on Limitless's image host, as <set>/<key>_EN.webp.
func limitlessArtURL(key string) string {
	return "https://limitlesstcg.nyc3.cdn.digitaloceanspaces.com/one-piece/" + cards.SetOf(key) + "/" + key + "_EN.webp"
}
