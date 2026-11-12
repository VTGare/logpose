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

	warnColor     = ui.Hex("#E3A048")
	unknownColor  = ui.Hex("#888780")
	youColor      = ui.Hex("#5AA9F0")
	oppColor      = ui.Hex("#E3A048")
	bookmarkColor = ui.Hex("#E8B83A")
)

// resultColors are the win and loss colors, blue and orange for the colorblind setting.
func resultColors(colorblind bool) (win, loss ui.Color) {
	if colorblind {
		return ui.Hex("#3D9BE9"), ui.Hex("#F08A24")
	}
	return ui.Hex("#4CB66A"), ui.Hex("#E5584F")
}

type look struct{ dark, colorblind bool }

// looks are the themes build picks from. They're built once, since build sets one every frame.
// The light one is mygo's with the bundled font. The result colors are the theme's Success and
// Danger, which mygo uses only in forms and meters.
var looks = func() map[look]*ui.Theme {
	out := make(map[look]*ui.Theme)
	for _, l := range []look{{false, false}, {false, true}, {true, false}, {true, true}} {
		t := graphite()
		if !l.dark {
			t = ui.LightTheme()
			t.Font = fontFamily
		}
		t.Success, t.Danger = resultColors(l.colorblind)
		out[l] = t
	}
	return out
}()

func winColor(t *ui.Theme) ui.Color  { return t.Success }
func lossColor(t *ui.Theme) ui.Color { return t.Danger }

// graphite is the dark theme: a near-black page, panels a step lighter and controls a step
// above them.
func graphite() *ui.Theme {
	t := ui.DarkTheme()
	t.Background = ui.Hex("#111113")
	t.Surface = ui.Hex("#232327")
	t.SurfaceHover = ui.Hex("#2C2C31")
	t.SurfacePressed = ui.Hex("#35353B")
	t.Border = ui.RGBA(255, 255, 255, 0.10)
	t.Text = ui.Hex("#ECECEE")
	t.TextMuted = ui.Hex("#9B9BA4")
	t.Accent = youColor
	t.AccentHover = ui.Hex("#7BBCF4")
	t.AccentPressed = ui.Hex("#3F95E3")
	t.AccentText = ui.Hex("#08111C")
	t.Radius = 8
	t.Font = fontFamily
	return t
}

var (
	panelDark = ui.Hex("#1A1A1D")
	// faintDark keeps 4.5:1 contrast on the page, the panels and the controls, so small text
	// stays readable.
	faintDark  = ui.Hex("#8A8A94")
	panelShade = ui.RGBA(0, 0, 0, 0.30)
)

// panelColor is the face of a tile: a step above the page in the dark theme, and the control
// surface in the light one.
func panelColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return panelDark
	}
	return t.Surface
}

// sidebarColor is a shade between the page and the panels.
func sidebarColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#151517")
	}
	return t.Surface
}

// faintColor is for the least important text, below TextMuted.
func faintColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return faintDark
	}
	return t.TextMuted
}

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

func resultColor(t *ui.Theme, r stats.Result) ui.Color {
	switch r {
	case stats.Win:
		return winColor(t)
	case stats.Loss:
		return lossColor(t)
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

// onAccent is text on a fill of the accent color.
var onAccent = ui.RGB(8, 17, 28)

var cardGold = ui.Hex("#F0C05A")
