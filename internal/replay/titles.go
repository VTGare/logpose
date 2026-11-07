package replay

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/VTGare/logpose/internal/simlog"
)

// countText is an effect the sim writes once per card or DON, such as "Activate 1 Don" five
// times for five DON.
var countText = regexp.MustCompile(`^(Activate|Rest|Minus|Draw|Trash) (\d+) (Don|Rested Don|Card|Cards from Deck|Remaining Cards)$`)

// counted reads a counted effect as its phrase and number. "Rest Don [Don]" is one DON.
func counted(rest string) (phrase string, n int, ok bool) {
	if rest == "Rest Don [Don]" {
		return "Rest Don", 1, true
	}
	m := countText.FindStringSubmatch(rest)
	if m == nil {
		return "", 0, false
	}
	n, _ = strconv.Atoi(m[2])
	return m[1] + " " + m[3], n, true
}

// countedText writes a counted effect's total.
func countedText(phrase string, n int) string {
	num := strconv.Itoa(n)
	switch phrase {
	case "Activate Don":
		return "Set " + num + " DON active"
	case "Rest Don":
		return "Rest " + num + " DON"
	case "Minus Don":
		return "Return " + num + " DON"
	case "Draw Don":
		return "Add " + num + " DON"
	case "Draw Rested Don":
		return "Add " + num + " rested DON"
	case "Draw Card":
		if n == 1 {
			return "Draw 1 card"
		}
		return "Draw " + num + " cards"
	}
	verb, obj, _ := strings.Cut(phrase, " ")
	return verb + " " + num + " " + obj
}

// sourceLines are the step's lines from the effect of its first card.
func (b *builder) sourceLines(s *Step) []simlog.Line {
	var out []simlog.Line
	for _, li := range s.Lines {
		l := b.g.Lines[li]
		if src, ok := effectSource(l); ok && src == s.Cards[0] {
			out = append(out, l)
		}
	}
	return out
}

// effectTitle is the first thing the step's effect does, as "Nami: Set 5 DON active". An
// effect written once per card or DON is added up over the step.
func (b *builder) effectTitle(s *Step, name func(string) string) string {
	lines := b.sourceLines(s)
	if len(lines) == 0 {
		return name(s.Cards[0])
	}
	head, rest, _ := strings.Cut(LineText(lines[0]), ": ")
	phrase, n, ok := counted(rest)
	if !ok {
		return head + ": " + rest
	}
	for _, l := range lines[1:] {
		_, r, _ := strings.Cut(l.Text, ": ")
		if p, k, ok := counted(r); ok && p == phrase {
			n += k
		}
	}
	return head + ": " + countedText(phrase, n)
}

// battleEffectTitle is what an effect during an attack did, with what it cost in brackets, as
// "Charlotte Linlin: +1000 to Stussy (trashed Edward Newgate)". Power for one card and effects
// written once per card or DON are added up.
func (b *builder) battleEffectTitle(s *Step, name func(string) string) string {
	var did, trashed []string
	power := make(map[string]int)
	counts := make(map[string]int)
	seen := make(map[string]bool)
	for _, l := range b.sourceLines(s) {
		_, rest, _ := strings.Cut(l.Text, ": ")
		key, text := rest, ""
		if phrase, n, ok := counted(rest); ok {
			key = phrase
			counts[phrase] += n
		} else if e, ok := parseEffect(rest); ok && e.kind == StatusPower {
			if e.target == "" {
				e.target = s.Cards[0]
			}
			key = "power " + e.target
			power[e.target] += e.n
		} else if strings.HasPrefix(rest, "Trash ") && len(l.Cards) == 2 {
			trashed = append(trashed, name(l.Cards[1]))
			continue
		} else {
			text = LineText(simlog.Line{Text: rest})
		}
		if !seen[key] {
			seen[key] = true
			did = append(did, key+"\x00"+text)
		}
	}
	var effects, paid []string
	for _, d := range did {
		key, text, _ := strings.Cut(d, "\x00")
		target, isPower := strings.CutPrefix(key, "power ")
		switch {
		case isPower:
			effects = append(effects, signedPower(power[target])+" to "+name(target))
		case key == "Rest Don":
			paid = append(paid, "rested "+strconv.Itoa(counts[key])+" DON")
		case key == "Minus Don":
			paid = append(paid, "returned "+strconv.Itoa(counts[key])+" DON")
		case counts[key] > 0:
			effects = append(effects, countedText(key, counts[key]))
		default:
			effects = append(effects, text)
		}
	}
	if len(trashed) > 0 {
		paid = append(paid, "trashed "+strings.Join(trashed, ", "))
	}
	title := name(s.Cards[0])
	switch {
	case len(effects) > 0 && len(paid) > 0:
		return title + ": " + strings.Join(effects, ", ") + " (" + strings.Join(paid, ", ") + ")"
	case len(effects) > 0:
		return title + ": " + strings.Join(effects, ", ")
	case len(paid) > 0:
		return title + ": " + strings.Join(paid, ", ")
	}
	return title
}

func signedPower(n int) string {
	if n < 0 {
		return strconv.Itoa(n)
	}
	return "+" + strconv.Itoa(n)
}
