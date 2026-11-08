package gui

import (
	"cmp"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/simlog"
)

// textPiece is a run of text, which names a card when code is set.
type textPiece struct {
	text   string
	code   string
	color  ui.Color
	weight int
}

// cardText draws pieces as one paragraph. A card's name underlines and shows the card while
// the pointer rests on it.
func (v *view) cardText(c *ui.Context, size float32, pieces []textPiece) ui.Element {
	return ui.RichText(c).FontSize(size).Children(func() {
		for i, p := range pieces {
			el := ui.Text(c.Key(i), p.text).FontSize(size).TextColor(p.color)
			if p.weight > 0 {
				el.FontWeight(p.weight)
			}
			if p.code == "" {
				continue
			}
			if el.Hovered() {
				el.Underline()
			}
			v.cardPreview(c, el, p.code)
		}
	})
}

// logPieces is a log line with its player in their color and card names in gold.
func logPieces(sv *stepView, l simlog.Line, text ui.Color) []textPiece {
	var out []textPiece
	if l.Player != "" && l.Player != "You" {
		out = append(out, textPiece{text: sv.who(l.Player) + "  ", weight: 700, color: sv.color(l.Player)})
	}
	rest := l.Text
	for _, code := range l.Cards {
		name := sv.g.CardNames[code]
		tag := name + " [" + code + "]"
		i := strings.Index(rest, tag)
		if i < 0 || name == "" {
			continue
		}
		out = append(out,
			textPiece{text: rest[:i], color: text},
			textPiece{text: name, code: code, weight: 600, color: cardGold})
		rest = rest[i+len(tag):]
	}
	return append(out, textPiece{text: replay.LineText(simlog.Line{Text: rest}), color: text})
}

// stepNames maps the names of the cards in a step's lines to their codes.
func stepNames(sv *stepView, s *replay.Step) map[string]string {
	names := make(map[string]string)
	add := func(code string) {
		if name := sv.g.CardNames[code]; name != "" && code != "" {
			names[name] = code
		}
	}
	for _, code := range s.Cards {
		add(code)
	}
	for _, li := range s.Lines {
		for _, code := range sv.g.Lines[li].Cards {
			add(code)
		}
	}
	return names
}

// namePieces splits text at the card names in names, the longest name first where two start
// at the same place.
func namePieces(text string, names map[string]string, col ui.Color) []textPiece {
	keys := make([]string, 0, len(names))
	for n := range names {
		keys = append(keys, n)
	}
	slices.SortFunc(keys, func(a, b string) int { return cmp.Compare(len(b), len(a)) })
	var out []textPiece
	start := 0
	for i := 0; i < len(text); {
		matched := ""
		for _, n := range keys {
			if strings.HasPrefix(text[i:], n) {
				matched = n
				break
			}
		}
		if matched == "" {
			i++
			continue
		}
		if i > start {
			out = append(out, textPiece{text: text[start:i], color: col})
		}
		out = append(out, textPiece{text: matched, code: names[matched], color: col})
		i += len(matched)
		start = i
	}
	if start < len(text) {
		out = append(out, textPiece{text: text[start:], color: col})
	}
	return out
}
