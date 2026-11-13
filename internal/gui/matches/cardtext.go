package matches

import (
	"regexp"
	"strings"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

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
func cardText(c *ui.Context, e *page.Env, size float32, pieces []textPiece) ui.Element {
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
			e.CardPreview(c, el, p.code)
		}
	})
}

// logPieces is a log line with its player in their color and card names in gold.
func logPieces(sv *stepView, l simlog.Line, text ui.Color) []textPiece {
	w := newWords(sv.g, text)
	w.cardCol, w.cardWeight = kit.CardGold, 600
	if l.Player != "" {
		s := sv.side(l.Player)
		w.out = append(w.out, textPiece{text: s.Name() + "  ", weight: 700, color: s.Color()})
	}
	w.linked(l.Text)
	return w.out
}

// codeTag is a card link's code, or another bracketed tag the sim writes, such as "[Don]".
var codeTag = regexp.MustCompile(` ?\[[A-Za-z][A-Za-z0-9-]*\]`)

// words builds a paragraph of pieces, where card names show the card.
type words struct {
	g          *simlog.Game
	col        ui.Color
	cardCol    ui.Color
	cardWeight int
	out        []textPiece
}

func newWords(g *simlog.Game, col ui.Color) *words {
	return &words{g: g, col: col, cardCol: col}
}

func (w *words) text(s string) {
	if s == "" {
		return
	}
	if n := len(w.out); n > 0 && w.out[n-1].code == "" && w.out[n-1].color == w.col && w.out[n-1].weight == 0 {
		w.out[n-1].text += s
		return
	}
	w.out = append(w.out, textPiece{text: s, color: w.col})
}

func (w *words) named(name, code string) {
	w.out = append(w.out, textPiece{text: name, code: code, color: w.cardCol, weight: w.cardWeight})
}

func (w *words) Card(code string) { w.named(cardName(w.g, code), code) }

func (w *words) cards(codes []string) {
	for i, code := range codes {
		if i > 0 {
			w.text(", ")
		}
		w.Card(code)
	}
}

// linked writes log text with its card links as "Name [CODE]", dropping the codes and the
// sim's other tags.
func (w *words) linked(s string) {
	start := 0
	for _, m := range codeTag.FindAllStringIndex(s, -1) {
		seg := s[start:m[0]]
		code := strings.TrimPrefix(s[m[0]:m[1]], " ")
		code = code[1 : len(code)-1]
		if name := w.g.CardNames[code]; name != "" && strings.HasSuffix(seg, name) {
			w.text(seg[:len(seg)-len(name)])
			w.named(name, code)
		} else {
			w.text(seg)
		}
		start = m[1]
	}
	w.text(s[start:])
}

// cardName is the card's name in the game's log, or its code when the log doesn't name it.
func cardName(g *simlog.Game, code string) string {
	if name := g.CardNames[code]; name != "" {
		return name
	}
	return code
}
