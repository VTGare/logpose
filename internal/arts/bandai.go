package arts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/fetch"
	"github.com/VTGare/logpose/internal/htmlq"
)

// bandaiArtURL is a printing's image on Bandai's card list, a 600×838 PNG with 256 colours.
// The page adds a "?261016" query that only busts caches.
func bandaiArtURL(key string) string {
	return cards.BandaiSite + "/images/cardlist/card/" + key + ".png"
}

// bandaiPage is one card list page: the series its select names, the one it shows, and that
// series' printings.
type bandaiPage struct {
	Series []bandaiSeries
	Shown  string
	Prints []bandaiPrint
}

type bandaiSeries struct {
	ID string
	// Name is the select's label, such as "Promotion card" or "BOOSTER PACK -ROYAL BLOOD- [OP-10]".
	Name string
}

// bandaiPrint is one printing. Bandai marks no kind, so the product is all that tells a stamp
// from another.
type bandaiPrint struct {
	// Key is "OP07-021_p3", or "OP07-021_r1" for a reprint.
	Key  string
	Code string
	// Rarity is the card's own rarity, or "SP CARD" for an SP.
	Rarity string
	// Product is the "Card Set(s)" line, such as "Offline Regional Finalist Card Set 25-26 Season 1".
	Product string
}

// The select's labels carry escaped markup, which parses to text such as `<br class="spInline">`.
var labelMarkup = regexp.MustCompile(`<[^>]*>`)

// parseBandaiPage reads a card list page, /cardlist/ or /cardlist/?series=<id>.
func parseBandaiPage(r io.Reader) (bandaiPage, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return bandaiPage{}, err
	}
	var page bandaiPage
	if sel := htmlq.Find(doc, func(n *html.Node) bool { return n.Data == "select" && htmlq.Attr(n, "id") == "series" }); sel != nil {
		for _, opt := range htmlq.FindAll(sel, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "option" }) {
			id := htmlq.Attr(opt, "value")
			if id == "" {
				continue
			}
			name := strings.Join(strings.Fields(labelMarkup.ReplaceAllString(htmlq.Text(opt), " ")), " ")
			page.Series = append(page.Series, bandaiSeries{ID: id, Name: name})
			if htmlq.Has(opt, "selected") {
				page.Shown = id
			}
		}
	}
	for _, dl := range htmlq.FindAll(doc, func(n *html.Node) bool { return n.Data == "dl" && htmlq.HasClass(n, "modalCol") }) {
		p := bandaiPrint{Key: htmlq.Attr(dl, "id")}
		if info := htmlq.Find(dl, htmlq.ByClass("infoCol")); info != nil {
			spans := htmlq.FindAll(info, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "span" })
			if len(spans) > 0 {
				p.Code = htmlq.Text(spans[0])
			}
			if len(spans) > 1 {
				p.Rarity = htmlq.Text(spans[1])
			}
		}
		p.Product = textAfterHeading(htmlq.Find(dl, htmlq.ByClass("getInfo")))
		if p.Key != "" && p.Code != "" {
			page.Prints = append(page.Prints, p)
		}
	}
	return page, nil
}

// textAfterHeading is n's text without its <h3>, which runs into the text after it.
func textAfterHeading(n *html.Node) string {
	if n == nil {
		return ""
	}
	var parts []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "h3" {
			continue
		}
		if t := htmlq.Text(c); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}

// fetchBandaiPage reads a series' card list page from site, or the list's own page, which
// shows the newest booster, when series is empty. Bandai sends no gzip, so a page is up to
// about 920 KB.
func fetchBandaiPage(ctx context.Context, client *http.Client, site, series string) (bandaiPage, error) {
	url := site + "/cardlist/"
	if series != "" {
		url += "?series=" + series
	}
	body, err := fetch.Get(ctx, client, url, 16<<20)
	if err != nil {
		return bandaiPage{}, fmt.Errorf("arts: %w", err)
	}
	return parseBandaiPage(bytes.NewReader(body))
}
