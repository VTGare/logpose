package page

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/deck"
	"github.com/VTGare/logpose/internal/gui/kit"
	"github.com/VTGare/logpose/internal/stats"
)

const (
	stackW      = 60
	stackStepX  = 4
	stackStepY  = 6
	maxStackFan = 4
	shownTraits = 6
	railTraits  = 4
	// statColumnW fits the cost curve's 11 bars, and a trait name on one line.
	statColumnW = 200
	statsRailW  = 228
)

var pillBack = ui.RGBA(0, 0, 0, 0.72)

// Pill draws a small dark badge over a card or pile.
func Pill(c *ui.Context, text string, size float32) ui.Element {
	return ui.Text(c, text).FontSize(size).Bold().TextColor(PillText).Padding(1, 6).Radius(8).Background(pillBack)
}

// DeckListOrder orders a decklist the way lists are read: by type, then cost, then name.
func DeckListOrder(e *Env, list map[string]int) []string {
	codes := slices.Collect(func(yield func(string) bool) {
		for code, n := range list {
			if n > 0 && !yield(code) {
				return
			}
		}
	})
	slices.SortFunc(codes, func(a, b string) int {
		ca, cb := e.Snap.Card(a), e.Snap.Card(b)
		return cmp.Or(
			cmp.Compare(kit.TypeRank(ca.Type), kit.TypeRank(cb.Type)),
			cmp.Compare(ca.Cost, cb.Cost),
			cmp.Compare(ca.Name, cb.Name),
			cmp.Compare(a, b),
		)
	})
	return codes
}

// DeckStacks draws the leader and each card as its copies fanned out, capped at four fanned
// layers while the count pill keeps the real number. With marks, added copies get a ring and a +N
// badge, and cards removed in full show faded at the end.
func DeckStacks(c *ui.Context, e *Env, leader string, list, marks map[string]int) {
	ui.Row(c).Gap(10).Wrap().AlignItems(ui.Start).Children(func() {
		if leader != "" {
			e.Card(c.Key("leader"), leader, stackW).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, 0.45))
		}
		for _, code := range DeckListOrder(e, list) {
			deckStack(c.Key(code), e, code, list[code], marks[code], false)
		}
		for _, code := range swapOrder(marks) {
			if marks[code] < 0 && list[code] == 0 {
				deckStack(c.Key("out"+code), e, code, -marks[code], marks[code], true)
			}
		}
	})
}

func deckStack(c *ui.Context, e *Env, code string, n, swapped int, gone bool) {
	fan := min(max(n, 1), maxStackFan)
	h := CardHeight(stackW)
	box := ui.Box(c).Size(stackW+float32(fan-1)*stackStepX, h+float32(fan-1)*stackStepY)
	box.Children(func() {
		for i := range fan {
			card := e.Card(c.Key(i), code, stackW).Absolute().
				Left(float32(i)*stackStepX).Top(float32(i)*stackStepY).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, 0.45))
			if swapped > 0 && i >= fan-swapped {
				card.Border(2, kit.WinColor(c.Theme()))
			}
			if gone {
				card.Opacity(0.35)
			}
		}
		if !gone {
			Pill(c, fmt.Sprintf("×%d", n), 11).Absolute().Right(4).Bottom(4)
		}
		switch {
		case swapped > 0:
			ui.Text(c, fmt.Sprintf("+%d", swapped)).FontSize(11).Bold().TextColor(kit.WinColor(c.Theme())).Padding(0, 6).Radius(8).
				Background(kit.PanelDark).Border(1, kit.WinColor(c.Theme())).Absolute().Right(-4).Top(-8)
		case swapped < 0:
			ui.Text(c, fmt.Sprintf("−%d", -swapped)).FontSize(11).Bold().TextColor(kit.LossColor(c.Theme())).Padding(0, 6).Radius(8).
				Background(kit.PanelDark).Border(1, kit.LossColor(c.Theme())).Absolute().Right(-4).Top(-8)
		}
	})
}

func swapOrder(swaps map[string]int) []string {
	codes := slices.Collect(maps.Keys(swaps))
	slices.Sort(codes)
	return codes
}

// DeckListStatsGrid draws a decklist's curve, counters, types, colors and traits in two rows of
// two columns.
func DeckListStatsGrid(c *ui.Context, st stats.ListStats) {
	pair := func(key string, left, right func()) {
		ui.Row(c.Key(key)).Gap(32).AlignItems(ui.Start).Children(func() {
			left()
			right()
		})
	}
	ui.Column(c).Gap(20).Children(func() {
		pair("curve", func() {
			statColumn(c, "Cost curve", func() { curveBars(c, st.Curve, 56) })
		}, func() {
			statColumn(c, "Counters", func() { counterLines(c, st.Counters) })
		})
		pair("types", func() {
			statColumn(c, "Types", func() {
				for _, ty := range []string{cards.Character, cards.Event, cards.Stage} {
					if n := st.Types[ty]; n > 0 {
						statLine(c, ty+"s", fmt.Sprint(n))
					}
				}
				if len(st.Colors) > 0 {
					ui.Text(c, "Colors").FontSize(13).FontWeight(600).Margin(14, 0, 4, 0)
				}
				for _, col := range slices.Sorted(maps.Keys(st.Colors)) {
					statLine(c.Key(string(col)), string(col), fmt.Sprint(st.Colors[col]))
				}
			})
		}, func() {
			statColumn(c, "Traits", func() { traitLines(c, st.Traits, shownTraits) })
		})
	})
}

// DeckListStatsRail draws a decklist's stats in a narrow vertical column beside card stacks.
func DeckListStatsRail(c *ui.Context, st stats.ListStats, partialNote string) {
	t := c.Theme()
	ui.Column(c).Width(statsRailW).Gap(14).BorderStartWidth(1).BorderColor(t.Border.Alpha(0.7)).Padding(0, 0, 0, 18).Children(func() {
		if partialNote != "" {
			DeckPartialBanner(c, partialNote)
		}
		railSection(c, "Cost curve", func() { curveBars(c, st.Curve, 44) })
		railSection(c, "Counters", func() { counterLines(c, st.Counters) })
		if line := compactTypes(st.Types); line != "" {
			railSection(c, "Types", func() {
				ui.Text(c, line).FontSize(12).TextColor(t.TextMuted).Wrap()
			})
		}
		if len(st.Traits) > 0 {
			railSection(c, "Traits", func() { traitLines(c, st.Traits, railTraits) })
		}
	})
}

// DeckPartialBanner draws a muted amber note when a game's log named fewer than 50 cards.
func DeckPartialBanner(c *ui.Context, note string) {
	ui.Box(c).Padding(7, 10).Radius(7).Background(kit.WarnColor.Alpha(0.10)).Border(1, kit.WarnColor.Alpha(0.28)).Children(func() {
		ui.Text(c, note).FontSize(12).TextColor(kit.WarnColor).Wrap()
	})
}

// DeckPartialNote describes missing cards when total is under DeckSize, or "" for a full deck.
func DeckPartialNote(total int) string {
	if total <= 0 || total >= stats.DeckSize {
		return ""
	}
	miss := stats.DeckSize - total
	if miss == 1 {
		return fmt.Sprintf("%d of %d cards · 1 card wasn't named in the log.", total, stats.DeckSize)
	}
	return fmt.Sprintf("%d of %d cards · %d cards weren't named in the log.", total, stats.DeckSize, miss)
}

// CopyDeckForSim formats a decklist in OPTCGSim clipboard format and asks the app to copy it.
func CopyDeckForSim(e *Env, leader, label string, list map[string]int) {
	if e.Act == nil || len(list) == 0 {
		return
	}
	text := deck.FormatSim(leader, DeckListOrder(e, list), list)
	total := deck.Count(list)
	toast := ui.Toast{ID: "copy-deck"}
	if total < stats.DeckSize {
		miss := stats.DeckSize - total
		toast.Title = fmt.Sprintf("Copied %d of %d cards for the sim.", total, stats.DeckSize)
		if miss == 1 {
			toast.Description = "1 card wasn't named in the log."
		} else {
			toast.Description = fmt.Sprintf("%d cards weren't named in the log.", miss)
		}
	} else {
		toast.Title = "Copied decklist for the sim."
		toast.Description = fmt.Sprintf("%d cards", total)
		if label != "" {
			toast.Description += " · " + label
		}
	}
	e.Act.CopyText(text, toast)
}

func statColumn(c *ui.Context, title string, body func()) {
	ui.Column(c).Gap(4).Grow(1).Basis(statColumnW).MinWidth(statColumnW).Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(600).Margin(0, 0, 4, 0)
		body()
	})
}

func railSection(c *ui.Context, title string, body func()) {
	ui.Column(c).Gap(3).Children(func() {
		ui.Text(c, title).FontSize(12).FontWeight(600).Margin(0, 0, 3, 0)
		body()
	})
}

func statLine(c *ui.Context, label, value string) {
	ui.Row(c).Gap(16).Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(c.Theme().TextMuted)
		ui.Spacer(c)
		ui.Text(c, value).FontSize(13).FontWeight(600)
	})
}

func counterLines(c *ui.Context, counters map[int]int) {
	total := 0
	for _, cv := range []int{0, 1000, 2000} {
		statLine(c, kit.CounterText(cv, false, "No counter"), fmt.Sprint(counters[cv]))
		total += cv * counters[cv]
	}
	statLine(c, "Total", fmt.Sprintf("%dk", total/1000))
}

func traitLines(c *ui.Context, traits []stats.TraitCount, limit int) {
	for _, tr := range traits[:min(len(traits), limit)] {
		statLine(c.Key(tr.Trait), tr.Trait, fmt.Sprint(tr.Copies))
	}
	if more := len(traits) - limit; more > 0 {
		ui.Text(c, fmt.Sprintf("%d more", more)).FontSize(12).TextColor(kit.FaintColor(c.Theme()))
	}
}

func compactTypes(types map[string]int) string {
	var parts []string
	for _, ty := range []string{cards.Character, cards.Event, cards.Stage} {
		if n := types[ty]; n > 0 {
			name := strings.ToLower(ty)
			if n != 1 {
				name += "s"
			}
			parts = append(parts, fmt.Sprintf("%d %s", n, name))
		}
	}
	return strings.Join(parts, " · ")
}

func curveBars(c *ui.Context, curve [11]int, maxBarH float32) {
	t := c.Theme()
	most := max(slices.Max(curve[:]), 1)
	ui.Row(c).Gap(2).AlignItems(ui.End).Height(maxBarH + 36).Children(func() {
		for cost, n := range curve {
			ui.Column(c.Key(cost)).Width(14).AlignItems(ui.Center).Gap(2).Children(func() {
				if n > 0 {
					ui.Text(c, fmt.Sprint(n)).FontSize(11).TextColor(t.TextMuted)
				}
				ui.Box(c).Width(10).Height(float32(n) / float32(most) * maxBarH).Radius(3).Background(t.Accent)
				ui.Text(c, fmt.Sprint(cost)).FontSize(11).TextColor(kit.FaintColor(t))
			})
		}
	})
}
