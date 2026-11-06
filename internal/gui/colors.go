package gui

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
)

// leaderColor is one of OPTCG's card colors: Mid for chips and bars, Deep for banners, which
// carry light text.
type leaderColor struct {
	Mid, Deep ui.Color
}

var (
	neutralLeader = leaderColor{Mid: ui.Hex("#888780"), Deep: ui.Hex("#444441")}

	leaderColors = map[cards.Color]leaderColor{
		cards.Red:    {Mid: ui.Hex("#E24B4A"), Deep: ui.Hex("#A32D2D")},
		cards.Green:  {Mid: ui.Hex("#639922"), Deep: ui.Hex("#3B6D11")},
		cards.Blue:   {Mid: ui.Hex("#378ADD"), Deep: ui.Hex("#185FA5")},
		cards.Purple: {Mid: ui.Hex("#7F77DD"), Deep: ui.Hex("#534AB7")},
		cards.Black:  {Mid: ui.Hex("#5F5E5A"), Deep: ui.Hex("#2C2C2A")},
		cards.Yellow: {Mid: ui.Hex("#EF9F27"), Deep: ui.Hex("#854F0B")},
	}

	bannerText      = ui.Hex("#FCEBEB")
	bannerTextMuted = ui.RGBA(252, 235, 235, 0.72)

	winColor     = ui.Hex("#3B9B45")
	lossColor    = ui.Hex("#E24B4A")
	unknownColor = ui.Hex("#888780")
	firstColor   = ui.Hex("#378ADD")
	secondColor  = ui.Hex("#BA7517")
)

func colorsOf(c cards.Card) []leaderColor {
	var out []leaderColor
	for _, col := range c.Colors {
		if lc, ok := leaderColors[col]; ok {
			out = append(out, lc)
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
