package cards

import (
	"io"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// LimitlessBase is Limitless's One Piece site. It has no card API. Its set pages list every
// printing with all of the card's data when asked for ?display=full.
const LimitlessBase = "https://onepiece.limitlesstcg.com"

// LimitlessArt is where Limitless keeps card images, as <set>/<key>_EN.webp.
const LimitlessArt = "https://limitlesstcg.nyc3.cdn.digitaloceanspaces.com/one-piece"

// ArtURL is the Limitless image of a printing, by its key ("OP14-020" or "OP14-020_p1").
func ArtURL(key string) string {
	set, _, _ := strings.Cut(key, "-")
	return LimitlessArt + "/" + set + "/" + key + "_EN.webp"
}

// SetIndex is what the card index page lists: every set's page, and the sets its "Latest
// Updates" line names, which are the ones that still change.
type SetIndex struct {
	Sets   []string
	Latest []string
}

// ParseSetIndex reads Limitless's /cards page.
func ParseSetIndex(r io.Reader) (SetIndex, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return SetIndex{}, err
	}
	var idx SetIndex
	seen := make(map[string]bool)
	news := find(doc, byClass("site-news"))
	for a := range doc.Descendants() {
		if a.Type != html.ElementNode || a.Data != "a" {
			continue
		}
		slug, ok := strings.CutPrefix(attr(a, "href"), "/cards/")
		if !ok || !setSlug.MatchString(slug) {
			continue
		}
		if news != nil && isUnder(a, news) {
			idx.Latest = append(idx.Latest, slug)
		}
		if !seen[slug] {
			seen[slug] = true
			idx.Sets = append(idx.Sets, slug)
		}
	}
	return idx, nil
}

// setSlug matches a set page's name, such as "op14-the-azure-seas-seven" or "promos", and
// not a card's ("OP14-001") or the search page.
var setSlug = regexp.MustCompile(`^(?:[a-z]+\d+-[A-Za-z0-9-]+|promos)$`)

func isUnder(n, parent *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p == parent {
			return true
		}
	}
	return false
}

// limitlessPrint is one printing as a set page shows it, with the card's data.
type limitlessPrint struct {
	card     Card
	print    Print
	standard string
	extra    string
	// setCode is the code of the set the page shows the printing in, such as "OP01".
	setCode string
}

// parseSetPage reads a set page shown with ?display=full. A set page also lists reprints of
// cards first printed elsewhere.
func parseSetPage(r io.Reader) ([]limitlessPrint, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	var out []limitlessPrint
	for _, main := range findAll(doc, byClass("card-page-main")) {
		if p, ok := parsePrint(main); ok {
			out = append(out, p)
		}
	}
	return out, nil
}

var (
	costRe    = regexp.MustCompile(`(\d+)\s*Cost`)
	lifeRe    = regexp.MustCompile(`(\d+)\s*Life`)
	powerRe   = regexp.MustCompile(`(\d+)\s*Power`)
	counterRe = regexp.MustCompile(`\+(\d+)\s*Counter`)
	blockRe   = regexp.MustCompile(`Block\s+(\w+)`)
	keyRe     = regexp.MustCompile(`/([A-Z0-9]+-\d+(?:_p\d+)?)_EN\.webp$`)
	setCodeRe = regexp.MustCompile(`\s*\(([^)]*)\)$`)
)

func parsePrint(main *html.Node) (limitlessPrint, bool) {
	code := text(find(main, byClass("card-text-id")))
	if code == "" {
		return limitlessPrint{}, false
	}
	c := Card{Code: code, Detailed: true, Cost: -1}
	c.Name = CleanName(text(find(main, byClass("card-text-name"))))
	if t := find(main, byClass("card-text-type")); t != nil {
		c.Type = text(find(t, byTooltip("Category")))
		for col := range strings.SplitSeq(text(find(t, byTooltip("Color"))), "/") {
			if col = strings.TrimSpace(col); col != "" {
				c.Colors = append(c.Colors, Color(col))
			}
		}
		line := text(t)
		c.Cost = number(costRe, line, -1)
		c.Life = number(lifeRe, line, 0)
	}
	for _, s := range findAll(main, byClass("card-text-section")) {
		readSection(&c, s)
	}
	c.Block = submatch(blockRe, text(find(main, byClass("regulation-mark"))))
	p := Print{Artist: text(find(main, byClass("card-text-artist")))}
	p.Artist = strings.TrimSpace(strings.TrimPrefix(p.Artist, "Illustrated by"))
	if img := find(main, func(n *html.Node) bool { return n.Data == "img" && hasClass(n, "card") }); img != nil {
		p.Key = submatch(keyRe, attr(img, "src"))
	}
	if p.Key == "" {
		p.Key = code
	}
	setCode := ""
	if cur := find(main, byClass("prints-current-details")); cur != nil {
		var spans []*html.Node
		for s := range cur.Descendants() {
			if s.Type == html.ElementNode && s.Data == "span" {
				spans = append(spans, s)
			}
		}
		if len(spans) > 0 {
			p.Set = setCodeRe.ReplaceAllString(text(spans[0]), "")
			setCode = submatch(setCodeRe, text(spans[0]))
		}
		if len(spans) > 1 {
			p.Rarity = text(spans[1])
		}
	}
	if row := find(main, func(n *html.Node) bool { return n.Data == "tr" && hasClass(n, "current") }); row != nil {
		p.Kind = text(find(row, byClass("prints-table-card-number")))
	}
	c.Rarity, c.Set = p.Rarity, p.Set
	p.Block = c.Block
	return limitlessPrint{card: c, print: p, standard: legality(main, "Standard"), extra: legality(main, "Extra"), setCode: setCode}, true
}

// readSection reads one of the card text's sections: the power line, the effect or the
// traits. The title and type line are read before.
func readSection(c *Card, s *html.Node) {
	switch {
	case find(s, byClass("card-text-name")) != nil, hasClass(s, "card-text-type"), hasClass(s, "card-text-artist"):
	case find(s, byTooltip("Type")) != nil:
		for t := range strings.SplitSeq(text(find(s, byTooltip("Type"))), "/") {
			if t = strings.TrimSpace(t); t != "" {
				c.Traits = append(c.Traits, t)
			}
		}
	case s.Data == "p":
		line := text(s)
		c.Power = number(powerRe, line, 0)
		c.Counter = number(counterRe, line, 0)
		for a := range strings.SplitSeq(text(find(s, byTooltip("Attribute"))), "/") {
			// Unknown attributes come through as an untranslated key, "card.attribute.?".
			if a = strings.TrimSpace(a); a != "" && !strings.HasPrefix(a, "card.") {
				c.Attributes = append(c.Attributes, a)
			}
		}
	default:
		// Attribute names come in full-width brackets, as "＜Strike＞".
		t := strings.NewReplacer("＜", "<", "＞", ">").Replace(text(s))
		if c.Effect == "" {
			c.Effect = t
		} else if t != "" {
			c.Effect += "\n" + t
		}
	}
}

// legality reads a format's badge: "legal", "not-legal" or "banned".
func legality(main *html.Node, format string) string {
	for _, b := range findAll(main, byClass("card-legality-badge")) {
		var divs []*html.Node
		for d := b.FirstChild; d != nil; d = d.NextSibling {
			if d.Type == html.ElementNode && d.Data == "div" {
				divs = append(divs, d)
			}
		}
		if len(divs) == 2 && text(divs[0]) == format {
			return attr(divs[1], "class")
		}
	}
	return ""
}

func number(re *regexp.Regexp, s string, none int) int {
	n, err := strconv.Atoi(submatch(re, s))
	if err != nil {
		return none
	}
	return n
}

func submatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}
