package kit

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
)

// LeaderColor is one of OPTCG's card colors. Card is the color of the card frame, measured
// from the sim's card images. Glow lights the banner's dark panel: the card color, except black,
// which would vanish there.
type LeaderColor struct {
	Card, Glow ui.Color
}

var (
	neutralLeader = LeaderColor{Card: ui.Hex("#888780"), Glow: ui.Hex("#888780")}

	LeaderColors = map[cards.Color]LeaderColor{
		cards.Red:    {Card: ui.Hex("#D2171D"), Glow: ui.Hex("#D2171D")},
		cards.Green:  {Card: ui.Hex("#048C68"), Glow: ui.Hex("#048C68")},
		cards.Blue:   {Card: ui.Hex("#0084B8"), Glow: ui.Hex("#0084B8")},
		cards.Purple: {Card: ui.Hex("#94398A"), Glow: ui.Hex("#94398A")},
		cards.Black:  {Card: ui.Hex("#232120"), Glow: ui.Hex("#8C8782")},
		cards.Yellow: {Card: ui.Hex("#F6E836"), Glow: ui.Hex("#F6E836")},
	}

	// colorOrder is the order the game prints a multicolor card's colors in, left to right.
	colorOrder = []cards.Color{cards.Red, cards.Green, cards.Blue, cards.Purple, cards.Black, cards.Yellow}

	WarnColor     = ui.Hex("#E3A048")
	unknownColor  = ui.Hex("#888780")
	YouColor      = ui.Hex("#5AA9F0")
	OppColor      = ui.Hex("#E3A048")
	BookmarkColor = ui.Hex("#E8B83A")
)

// ResultColors are the win and loss colors, blue and orange for the colorblind setting.
func ResultColors(colorblind bool) (win, loss ui.Color) {
	if colorblind {
		return ui.Hex("#3D9BE9"), ui.Hex("#F08A24")
	}
	return ui.Hex("#4CB66A"), ui.Hex("#E5584F")
}

type Look struct{ Dark, Colorblind bool }

// Looks are the themes build picks from. They're built once, since build sets one every frame.
// The light one is mygo's with the bundled font. The result colors are the theme's Success and
// Danger, which mygo uses only in forms and meters.
var Looks = func() map[Look]*ui.Theme {
	out := make(map[Look]*ui.Theme)
	for _, l := range []Look{{false, false}, {false, true}, {true, false}, {true, true}} {
		t := Graphite()
		if !l.Dark {
			t = ui.LightTheme()
			t.Font = fontFamily
		}
		t.Success, t.Danger = ResultColors(l.Colorblind)
		out[l] = t
	}
	return out
}()

func WinColor(t *ui.Theme) ui.Color  { return t.Success }
func LossColor(t *ui.Theme) ui.Color { return t.Danger }

// Graphite is the dark theme: a near-black page, panels a step lighter and controls a step
// above them.
func Graphite() *ui.Theme {
	t := ui.DarkTheme()
	t.Background = ui.Hex("#111113")
	t.Surface = ui.Hex("#232327")
	t.SurfaceHover = ui.Hex("#2C2C31")
	t.SurfacePressed = ui.Hex("#35353B")
	t.Border = ui.RGBA(255, 255, 255, 0.10)
	t.Text = ui.Hex("#ECECEE")
	t.TextMuted = ui.Hex("#9B9BA4")
	t.Accent = YouColor
	t.AccentHover = ui.Hex("#7BBCF4")
	t.AccentPressed = ui.Hex("#3F95E3")
	t.AccentText = ui.Hex("#08111C")
	t.Radius = 8
	t.Font = fontFamily
	return t
}

var (
	PanelDark = ui.Hex("#1A1A1D")
	// FaintDark keeps 4.5:1 contrast on the page, the panels and the controls, so small text
	// stays readable.
	FaintDark  = ui.Hex("#8A8A94")
	PanelShade = ui.RGBA(0, 0, 0, 0.30)
)

// PanelColor is the face of a tile: a step above the page in the dark theme, and the control
// surface in the light one.
func PanelColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return PanelDark
	}
	return t.Surface
}

// SidebarColor is a shade between the page and the panels.
func SidebarColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#151517")
	}
	return t.Surface
}

// FaintColor is for the least important text, below TextMuted.
func FaintColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return FaintDark
	}
	return t.TextMuted
}

// ColorsOf is a card's colors in the game's order. The catalog lists them alphabetically.
func ColorsOf(c cards.Card) []LeaderColor {
	var out []LeaderColor
	for _, col := range colorOrder {
		if slices.Contains(c.Colors, col) {
			out = append(out, LeaderColors[col])
		}
	}
	if len(out) == 0 {
		out = append(out, neutralLeader)
	}
	return out
}

func ResultColor(t *ui.Theme, r stats.Result) ui.Color {
	switch r {
	case stats.Win:
		return WinColor(t)
	case stats.Loss:
		return LossColor(t)
	default:
		return unknownColor
	}
}

func ResultLetter(r stats.Result) string {
	switch r {
	case stats.Win:
		return "W"
	case stats.Loss:
		return "L"
	default:
		return "?"
	}
}

// OnAccent is text on a fill of the accent color.
var OnAccent = ui.RGB(8, 17, 28)

var CardGold = ui.Hex("#F0C05A")
