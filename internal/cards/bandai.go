package cards

import (
	"io"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"github.com/VTGare/logpose/internal/htmlq"
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
			active = strings.Contains(htmlq.Text(n), "Active Restrictions")
		case "h4":
			section = htmlq.Text(n)
		case "ul", "ol":
			if !active {
				continue
			}
			var codes []string
			for _, li := range htmlq.FindAll(n, func(n *html.Node) bool { return n.Data == "li" && n.Type == html.ElementNode }) {
				if code := cardCode.FindString(htmlq.Text(li)); code != "" {
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
