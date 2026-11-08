package cards

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// BandaiSite is Bandai's English card list. Every series page also lists every series.
const BandaiSite = "https://en.onepiece-cardgame.com"

// BandaiRestrictions is Bandai's page of the cards banned or restricted today. It sends no
// ETag or Last-Modified, so every fetch is the whole page, about 48 KB.
const BandaiRestrictions = BandaiSite + "/rules/restriction/"

// Restrictions is Bandai's list. Banned cards can't be in a deck, a restricted card can be in
// one once, and the two cards of a pair can't share a deck.
type Restrictions struct {
	Banned     []string   `json:"banned"`
	Restricted []string   `json:"restricted"`
	Pairs      [][]string `json:"pairs"`
}

var cardCode = regexp.MustCompile(`\b[A-Z]+\d*-\d+\b`)

// ParseRestrictions reads Bandai's page. The page opens with the latest announcement, which
// names cards too, so only what follows its "Cards with Active Restrictions" heading counts.
// Each list under "Banned Pair Cards" is one pair.
func ParseRestrictions(r io.Reader) (Restrictions, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return Restrictions{}, err
	}
	var out Restrictions
	active := false
	section := ""
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		switch n.Data {
		case "h3":
			active = strings.Contains(text(n), "Active Restrictions")
		case "h4":
			section = text(n)
		case "ul", "ol":
			if !active {
				continue
			}
			var codes []string
			for _, li := range findAll(n, func(n *html.Node) bool { return n.Data == "li" && n.Type == html.ElementNode }) {
				if code := cardCode.FindString(text(li)); code != "" {
					codes = append(codes, code)
				}
			}
			switch {
			case len(codes) == 0:
			case strings.HasPrefix(section, "Banned Pair"):
				out.Pairs = append(out.Pairs, codes)
			case strings.HasPrefix(section, "Banned"):
				out.Banned = append(out.Banned, codes...)
			case strings.HasPrefix(section, "Restricted"):
				out.Restricted = append(out.Restricted, codes...)
			}
		}
	}
	return out, nil
}

// BandaiArtURL is a printing's image on Bandai's card list, a 600×838 PNG with 256 colours.
// The page adds a "?261016" query that only busts caches.
func BandaiArtURL(key string) string { return BandaiSite + "/images/cardlist/card/" + key + ".png" }

// BandaiPage is one card list page: the series its select names, the one it shows, and that
// series' printings.
type BandaiPage struct {
	Series []BandaiSeries
	Shown  string
	Prints []BandaiPrint
}

type BandaiSeries struct {
	ID string
	// Name is the select's label, such as "Promotion card" or "BOOSTER PACK -ROYAL BLOOD- [OP-10]".
	Name string
}

// BandaiPrint is one printing. Bandai marks no kind, so the product is all that tells a stamp
// from another.
type BandaiPrint struct {
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

// ParseBandaiPage reads a card list page, /cardlist/ or /cardlist/?series=<id>.
func ParseBandaiPage(r io.Reader) (BandaiPage, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return BandaiPage{}, err
	}
	var page BandaiPage
	if sel := find(doc, func(n *html.Node) bool { return n.Data == "select" && attr(n, "id") == "series" }); sel != nil {
		for _, opt := range findAll(sel, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "option" }) {
			id := attr(opt, "value")
			if id == "" {
				continue
			}
			name := strings.Join(strings.Fields(labelMarkup.ReplaceAllString(text(opt), " ")), " ")
			page.Series = append(page.Series, BandaiSeries{ID: id, Name: name})
			if hasAttr(opt, "selected") {
				page.Shown = id
			}
		}
	}
	for _, dl := range findAll(doc, func(n *html.Node) bool { return n.Data == "dl" && hasClass(n, "modalCol") }) {
		p := BandaiPrint{Key: attr(dl, "id")}
		if info := find(dl, byClass("infoCol")); info != nil {
			spans := findAll(info, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "span" })
			if len(spans) > 0 {
				p.Code = text(spans[0])
			}
			if len(spans) > 1 {
				p.Rarity = text(spans[1])
			}
		}
		p.Product = textAfterHeading(find(dl, byClass("getInfo")))
		if p.Key != "" && p.Code != "" {
			page.Prints = append(page.Prints, p)
		}
	}
	return page, nil
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
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
		if t := text(c); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}

// FetchBandaiPage reads a series' card list page from site, or the list's own page, which
// shows the newest booster, when series is empty. Bandai sends no gzip, so a page is up to
// about 920 KB.
func FetchBandaiPage(ctx context.Context, client *http.Client, site, series string) (BandaiPage, error) {
	url := site + "/cardlist/"
	if series != "" {
		url += "?series=" + series
	}
	body, err := getPage(ctx, client, url)
	if err != nil {
		return BandaiPage{}, err
	}
	return ParseBandaiPage(bytes.NewReader(body))
}
