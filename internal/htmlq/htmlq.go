// Package htmlq finds nodes and reads text in parsed HTML pages.
package htmlq

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
)

func HasClass(n *html.Node, class string) bool {
	return n.Type == html.ElementNode && slices.Contains(strings.Fields(Attr(n, "class")), class)
}

func Attr(n *html.Node, key string) string {
	v, _ := lookup(n, key)
	return v
}

// Has reports whether n has the attribute, even with an empty value, as <option selected> does.
func Has(n *html.Node, key string) bool {
	_, ok := lookup(n, key)
	return ok
}

func lookup(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func FindAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	for d := range n.Descendants() {
		if match(d) {
			out = append(out, d)
		}
	}
	return out
}

func Find(n *html.Node, match func(*html.Node) bool) *html.Node {
	for d := range n.Descendants() {
		if match(d) {
			return d
		}
	}
	return nil
}

func ByClass(class string) func(*html.Node) bool {
	return func(n *html.Node) bool { return HasClass(n, class) }
}

func ByTooltip(tip string) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && Attr(n, "data-tooltip") == tip }
}

// Text is n's text with runs of spaces collapsed. A <br> ends a line.
func Text(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			b.WriteString(n.Data)
		case n.Type == html.ElementNode && n.Data == "br":
			b.WriteString("\n")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	var lines []string
	for line := range strings.SplitSeq(b.String(), "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
