package gui

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
)

// leaderColor is one of OPTCG's card colors. Card is the color of the card frame, measured
// from the sim's card images. Glow lights the banner's dark panel: the card color, except black,
// which would vanish there.
type leaderColor struct {
	Card, Glow ui.Color
}

var (
	neutralLeader = leaderColor{Card: ui.Hex("#888780"), Glow: ui.Hex("#888780")}

	leaderColors = map[cards.Color]leaderColor{
		cards.Red:    {Card: ui.Hex("#D2171D"), Glow: ui.Hex("#D2171D")},
		cards.Green:  {Card: ui.Hex("#048C68"), Glow: ui.Hex("#048C68")},
		cards.Blue:   {Card: ui.Hex("#0084B8"), Glow: ui.Hex("#0084B8")},
		cards.Purple: {Card: ui.Hex("#94398A"), Glow: ui.Hex("#94398A")},
		cards.Black:  {Card: ui.Hex("#232120"), Glow: ui.Hex("#8C8782")},
		cards.Yellow: {Card: ui.Hex("#F6E836"), Glow: ui.Hex("#F6E836")},
	}

	// colorOrder is the order the game prints a multicolor card's colors in, left to right.
	colorOrder = []cards.Color{cards.Red, cards.Green, cards.Blue, cards.Purple, cards.Black, cards.Yellow}

	bannerBase      = ui.Hex("#1C1C1B")
	bannerText      = ui.Hex("#FCEBEB")
	bannerTextMuted = ui.RGBA(252, 235, 235, 0.72)

	winColor     = ui.Hex("#3B9B45")
	lossColor    = ui.Hex("#E24B4A")
	unknownColor = ui.Hex("#888780")
	firstColor   = ui.Hex("#378ADD")
	secondColor  = ui.Hex("#BA7517")
)

// colorsOf is a card's colors in the game's order. The catalog lists them alphabetically.
func colorsOf(c cards.Card) []leaderColor {
	var out []leaderColor
	for _, col := range colorOrder {
		if slices.Contains(c.Colors, col) {
			out = append(out, leaderColors[col])
		}
	}
	if len(out) == 0 {
		out = append(out, neutralLeader)
	}
	return out
}

func resultColor(r stats.Result) ui.Color {
	switch r {
	case stats.Win:
		return winColor
	case stats.Loss:
		return lossColor
	default:
		return unknownColor
	}
}

func resultLetter(r stats.Result) string {
	switch r {
	case stats.Win:
		return "W"
	case stats.Loss:
		return "L"
	default:
		return "?"
	}
}
