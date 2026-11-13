package decks

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/VTGare/logpose/internal/gui/matches"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

// The list's stacks fan each card's copies down and to the right.
const (
	stackW      = 60
	stackStepX  = 4
	stackStepY  = 6
	shownTraits = 6
	// statColumnW fits the cost curve's 11 bars, and a trait name on one line.
	statColumnW = 200
)

// listTab is the lists of the chosen build, or of every game under Overall. The games in no
// build, and a leader without builds, get the suggested builds, as does asking for them.
func (pg *Page) listTab(c *ui.Context, e *page.Env, d *deckData) {
	if pg.suggesting || d.scope.kind == scopeNoBuild || len(d.builds) == 0 {
		pg.suggestionsPanel(c, e, d)
		return
	}
	pg.scopeList(c, e, d)
}

// scopeList is the lists the games ran, the latest first in a rail, and the chosen one in full.
func (pg *Page) scopeList(c *ui.Context, e *page.Env, d *deckData) {
	t := c.Theme()
	changes := d.changes
	if len(changes) == 0 {
		kit.Panel(c).Padding(16).Children(func() { noGames(c, d) })
		return
	}
	i := pg.chosenChange(changes)
	ch := changes[i]
	list := ch.Games[0].Facts.List
	ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
		pg.versionRail(c, e, changes, i)
		kit.Panel(c).Grow(1).Basis(0).MinWidth(0).Gap(14).Padding(16).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				title := "List from " + e.Date(ch.At) + ", " + ch.At.Format("15:04")
				if i == len(changes)-1 {
					title = "Latest list · " + e.Date(ch.At) + ", " + ch.At.Format("15:04")
				}
				ui.Text(c, title).FontWeight(600)
				ui.Text(c, kit.Games(len(ch.Games))+" · "+kit.Record(stats.RecordOf(ch.Games))).FontSize(13).TextColor(t.TextMuted)
				ui.Spacer(c)
				on := e.Prefs.DeckChanges
				if ui.Switch(c, &on).Label("Show changes").Changed() {
					page.PrefDeckChanges.Set(e, on)
				}
				ui.Text(c, "Show changes").FontSize(13).TextColor(t.TextMuted)
			})
			var marks map[string]int
			if e.Prefs.DeckChanges && i > 0 {
				marks = ch.Swaps
			}
			stacks(c, e, d.leader, list, marks)
			ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
			listStats(c, d.lists[i])
		})
	})
}

// listOrder is the cards as a decklist orders them: by type, then cost, then name.
func listOrder(e *page.Env, list map[string]int) []string {
	codes := slices.Collect(func(yield func(string) bool) {
		for code := range list {
			if !yield(code) {
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

// stacks draws the leader and each card as its copies fanned out, with the count on top. With
// marks, each copy swapped in gets a ring and the stack +N, and cards taken out show as faded
// stacks of the copies removed at the end.
func stacks(c *ui.Context, e *page.Env, leader string, list, marks map[string]int) {
	ui.Row(c).Gap(10).Wrap().AlignItems(ui.Start).Children(func() {
		e.Card(c.Key("leader"), leader, stackW).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, 0.45))
		for _, code := range listOrder(e, list) {
			stack(c.Key(code), e, code, list[code], marks[code], false)
		}
		for _, code := range swapOrder(marks) {
			if marks[code] < 0 && list[code] == 0 {
				stack(c.Key("out"+code), e, code, -marks[code], marks[code], true)
			}
		}
	})
}

func stack(c *ui.Context, e *page.Env, code string, n, swapped int, gone bool) {
	h := page.CardHeight(stackW)
	box := ui.Box(c).Size(stackW+float32(n-1)*stackStepX, h+float32(n-1)*stackStepY)
	box.Children(func() {
		for i := range n {
			card := e.Card(c.Key(i), code, stackW).Absolute().
				Left(float32(i)*stackStepX).Top(float32(i)*stackStepY).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, 0.45))
			if swapped > 0 && i >= n-swapped {
				card.Border(2, kit.WinColor(c.Theme()))
			}
			if gone {
				card.Opacity(0.35)
			}
		}
		if !gone {
			matches.Pill(c, fmt.Sprintf("×%d", n), 11).Absolute().Right(4).Bottom(4)
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

// listStats is the curve with the counters, then the types and colors with the traits, two to a row so
// they fit beside the versions rail.
func listStats(c *ui.Context, st stats.ListStats) {
	pair := func(key string, left, right func()) {
		ui.Row(c.Key(key)).Gap(32).AlignItems(ui.Start).Children(func() {
			left()
			right()
		})
	}
	ui.Column(c).Gap(20).Children(func() {
		pair("curve", func() {
			statColumn(c, "Cost curve", func() { curveBars(c, st.Curve) })
		}, func() {
			statColumn(c, "Counters", func() {
				total := 0
				for _, cv := range []int{0, 1000, 2000} {
					statLine(c, kit.CounterText(cv, false, "No counter"), fmt.Sprint(st.Counters[cv]))
					total += cv * st.Counters[cv]
				}
				statLine(c, "Total", fmt.Sprintf("%dk", total/1000))
			})
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
			statColumn(c, "Traits", func() {
				for _, tr := range st.Traits[:min(len(st.Traits), shownTraits)] {
					statLine(c.Key(tr.Trait), tr.Trait, fmt.Sprint(tr.Copies))
				}
				if more := len(st.Traits) - shownTraits; more > 0 {
					ui.Text(c, fmt.Sprintf("%d more", more)).FontSize(12).TextColor(kit.FaintColor(c.Theme()))
				}
			})
		})
	})
}

func statColumn(c *ui.Context, title string, body func()) {
	ui.Column(c).Gap(4).Grow(1).Basis(statColumnW).MinWidth(statColumnW).Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(600).Margin(0, 0, 4, 0)
		body()
	})
}

func statLine(c *ui.Context, label, value string) {
	ui.Row(c).Gap(24).Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(c.Theme().TextMuted)
		ui.Spacer(c)
		ui.Text(c, value).FontSize(13).FontWeight(600)
	})
}

// curveBars is a bar per cost from 0 to 10, the copies over each.
func curveBars(c *ui.Context, curve [11]int) {
	t := c.Theme()
	most := max(slices.Max(curve[:]), 1)
	ui.Row(c).Gap(2).AlignItems(ui.End).Height(96).Children(func() {
		for cost, n := range curve {
			ui.Column(c.Key(cost)).Width(14).AlignItems(ui.Center).Gap(2).Children(func() {
				if n > 0 {
					ui.Text(c, fmt.Sprint(n)).FontSize(11).TextColor(t.TextMuted)
				}
				ui.Box(c).Width(10).Height(float32(n) / float32(most) * 56).Radius(3).Background(t.Accent)
				ui.Text(c, fmt.Sprint(cost)).FontSize(11).TextColor(kit.FaintColor(t))
			})
		}
	})
}

// suggestionsPanel lists the suggested builds, each with its searchers, to make into a build or
// dismiss. Groups of fewer than minSuggested games fold away.
func (pg *Page) suggestionsPanel(c *ui.Context, e *page.Env, d *deckData) {
	t := c.Theme()
	s := pg
	kit.Panel(c).Gap(12).Padding(16).Children(func() {
		if s.suggesting && d.scope.kind != scopeNoBuild && len(d.builds) > 0 && linkButton(c, "Back to "+d.tag) {
			s.suggesting = false
		}
		if len(d.split.Suggestions) == 0 {
			ui.Text(c, "No suggested builds for "+e.Snap.Card(d.leader).Name).FontWeight(600)
			ui.Text(c, "Suggestions group your games by the searchers their lists run.").
				FontSize(13).TextColor(t.TextMuted).Wrap()
			return
		}
		ui.Text(c, fmt.Sprintf("Suggested builds · %d", shownSuggestions(d.leaderDecks))).FontWeight(600)
		ui.Text(c, "Grouped by the searchers each list runs. Make one a build to see its list, cards and changes.").
			FontSize(13).TextColor(t.TextMuted).Wrap()
		for _, sg := range d.split.Suggestions {
			if smallSuggestion(sg) && !s.showSmall {
				continue
			}
			pg.suggestionCard(c.Key(sg.Key), e, d.leader, sg)
		}
		if n := smallSuggestions(d.leaderDecks); n > 0 && !s.showSmall {
			label := fmt.Sprintf("Show %d small groups", n)
			if n == 1 {
				label = "Show 1 small group"
			}
			if linkButton(c, label) {
				s.showSmall = true
			}
		}
	})
}

func (pg *Page) suggestionCard(c *ui.Context, e *page.Env, leader string, sg stats.Suggestion) {
	t := c.Theme()
	s := pg
	key := sg.Key
	title := sg.Name
	ui.Column(c).Gap(10).Padding(12, 14).Radius(10).Border(1, t.Border.Alpha(0.6)).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontWeight(600)
			first, last := sg.Games[len(sg.Games)-1].PlayedAt, sg.Games[0].PlayedAt
			played := kit.Span(e.Date(first), e.Date(last))
			ui.Text(c, ScopeMeta(len(sg.Games), stats.RecordOf(sg.Games))+" · "+played).FontSize(13).TextColor(t.TextMuted)
			ui.Spacer(c)
			if s.naming == key {
				return
			}
			if ui.PrimaryButton(c, "Make this a build").Clicked() {
				s.naming, s.name = key, title
			}
			if ui.Button(c, "Not a build").Clicked() {
				pg.do(func(a Actions) { a.DismissSuggestion(store.Dismissal{Leader: leader, Core: sg.Core}) })
			}
		})
		if s.naming == key {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c, &s.name).Grow(1).Label("Build name")
				if (in.Submitted() || ui.PrimaryButton(c, "Save").Clicked()) && strings.TrimSpace(s.name) != "" {
					b := store.Build{Leader: leader, Name: strings.TrimSpace(s.name), Core: sg.Core}
					pg.do(func(a Actions) { a.SaveBuild(b, pg.chooseSaved) })
					s.naming = ""
				}
				if ui.Button(c, "Cancel").Clicked() {
					s.naming = ""
				}
			})
		}
		ui.Row(c).Gap(10).Wrap().Children(func() {
			for _, code := range sg.Core {
				searcherTile(c.Key(code), e, code)
			}
		})
	})
}

// searcherTile is a searcher's card with what it looks for.
func searcherTile(c *ui.Context, e *page.Env, code string) {
	t := c.Theme()
	card := e.Snap.Card(code)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(4, 10, 4, 4).Radius(8).Background(t.Surface).Children(func() {
		e.Card(c, code, 40)
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, card.Name).FontSize(13)
			if targets := cards.SearchTargets(card.Effect); len(targets) > 0 {
				ui.Text(c, "Searches "+strings.Join(targets, ", ")).FontSize(12).TextColor(kit.FaintColor(t))
			}
		})
	})
}

// linkButton is text in the accent color that reports a click.
func linkButton(c *ui.Context, label string) bool {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Background(ui.Transparent).Padding(0).AlignSelf(ui.Start)
	col := t.Accent
	if b.Hovered() {
		col = t.AccentHover
	}
	b.Children(func() { ui.Text(c, label).FontSize(13).TextColor(col) })
	return b.Clicked()
}
