package cards

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
)

func hasClass(n *html.Node, class string) bool {
	return n.Type == html.ElementNode && slices.Contains(strings.Fields(attr(n, "class")), class)
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// findAll returns the nodes under n, in document order, that match.
func findAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	for d := range n.Descendants() {
		if match(d) {
			out = append(out, d)
		}
	}
	return out
}

func find(n *html.Node, match func(*html.Node) bool) *html.Node {
	for d := range n.Descendants() {
		if match(d) {
			return d
		}
	}
	return nil
}

func byClass(class string) func(*html.Node) bool {
	return func(n *html.Node) bool { return hasClass(n, class) }
}

func byTooltip(tip string) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && attr(n, "data-tooltip") == tip }
}

// text is n's text with runs of spaces collapsed. A <br> ends a line.
func text(n *html.Node) string {
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
