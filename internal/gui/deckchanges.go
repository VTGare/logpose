package gui

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

const (
	railW = 190
	// railSquares is the most games a rail entry draws a square for.
	railSquares = 12
)

// chosenChange is the shown list among the build's: the one first played at pg.version, or
// the latest. It returns its index.
func (pg *decksPage) chosenChange(changes []stats.Change) int {
	for i, ch := range changes {
		if ch.At.Equal(pg.version) {
			return i
		}
	}
	return len(changes) - 1
}

// versionRail lists the build's lists, newest first, each with its date and time, the copies
// swapped in and out, and a square per game. Clicking one shows it. With Show changes on, the
// chosen one lists the cards swapped.
func (pg *decksPage) versionRail(c *ui.Context, e *env, changes []stats.Change, chosen int) {
	t := c.Theme()
	panel(c).Width(railW).Gap(2).Padding(6).Children(func() {
		for i := len(changes) - 1; i >= 0; i-- {
			ch := changes[i]
			b := ui.ButtonBase(c.Key(i)).Label(e.dateTime(ch.At)).Column().Gap(3).Padding(7, 8).Radius(8).
				Background(ui.Transparent).AlignItems(ui.Stretch)
			switch {
			case i == chosen:
				b.Background(t.Surface).Border(1, t.Accent)
			case b.Hovered():
				b.Background(t.SurfaceHover)
			}
			if b.Clicked() {
				pg.version = ch.At
				if i == len(changes)-1 {
					pg.version = time.Time{}
				}
			}
			b.Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
					ui.Text(c, e.date(ch.At)).FontSize(13).FontWeight(600)
					ui.Text(c, ch.At.Format("15:04")).FontSize(12).TextColor(faintColor(t))
					if i == len(changes)-1 {
						ui.Text(c, "Latest").FontSize(11).TextColor(t.Accent)
					}
				})
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					in, out := swapCounts(ch.Swaps)
					if i == 0 {
						ui.Text(c, "First list").FontSize(12).TextColor(t.TextMuted)
					} else {
						ui.Text(c, fmt.Sprintf("+%d", in)).FontSize(12).FontWeight(600).TextColor(winColor(t))
						ui.Text(c, fmt.Sprintf("−%d", out)).FontSize(12).FontWeight(600).TextColor(lossColor(t))
					}
					ui.Spacer(c)
					gs := ch.Games[:min(len(ch.Games), railSquares)]
					rs := make([]stats.Result, len(gs))
					for i, g := range gs {
						rs[i] = g.Result
					}
					resultSquares(c, rs, 7)
				})
				if i == chosen && i > 0 && e.prefs.deckChanges {
					ui.Box(c).Height(1).Background(t.Border.Alpha(0.6)).Margin(4, 0, 2, 0)
					for _, code := range swapOrder(ch.Swaps) {
						n := ch.Swaps[code]
						sign, col := "+", winColor(t)
						if n < 0 {
							sign, col = "−", lossColor(t)
						}
						ui.Row(c.Key(code)).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, fmt.Sprintf("%s%d", sign, max(n, -n))).FontSize(12).FontWeight(700).TextColor(col).Width(24)
							ui.Text(c, e.snap.card(code).Name).FontSize(12).SingleLine()
						})
					}
				}
			})
		}
	})
}

func swapCounts(swaps map[string]int) (in, out int) {
	for _, n := range swaps {
		if n > 0 {
			in += n
		} else {
			out -= n
		}
	}
	return in, out
}

// swapOrder puts the cards in before the cards out, the bigger swaps first.
func swapOrder(swaps map[string]int) []string {
	codes := slices.Collect(maps.Keys(swaps))
	slices.SortFunc(codes, func(a, b string) int {
		return cmp.Or(cmp.Compare(swaps[b], swaps[a]), cmp.Compare(a, b))
	})
	return codes
}
