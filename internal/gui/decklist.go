package gui

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

// The list's stacks fan each card's copies down and to the right.
const (
	stackW      = 80
	stackStepX  = 7
	stackStepY  = 9
	shownTraits = 6
)

// listTab is the chosen build's latest list, or the suggested builds when none is chosen.
func (v *view) listTab(c *ui.Context, d leaderDecks) {
	for _, b := range d.builds {
		if b.ID == v.decks.build {
			v.buildList(c, d, b)
			return
		}
	}
	v.suggestionsPanel(c, d)
}

func (v *view) buildList(c *ui.Context, d leaderDecks, b store.Build) {
	t := c.Theme()
	gs := d.assignment.Builds[b.ID]
	panel(c).Gap(16).Padding(16).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Text(c, "Core").FontSize(13).TextColor(t.TextMuted)
			for _, code := range b.Core {
				v.cardChip(c.Key(code), code)
			}
		})
		if len(gs) == 0 {
			ui.Text(c, "No games of "+b.Name+" match these filters.").TextColor(t.TextMuted)
			return
		}
		latest := gs[len(gs)-1]
		ui.Text(c, "Latest list · "+latest.PlayedAt.Format("Jan 2")).FontSize(12).TextColor(faintColor(t))
		v.stacks(c, d.leader, latest.Facts.List)
		ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
		listStats(c, stats.List(latest.Facts.List, v.snap.card))
	})
}

// cardChip is a card's small art and name.
func (v *view) cardChip(c *ui.Context, code string) {
	t := c.Theme()
	ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(2, 8, 2, 2).Radius(6).Background(t.Surface).Children(func() {
		v.cardArt(c, code, 22, 31)
		ui.Text(c, v.snap.card(code).Name).FontSize(13)
	})
}

// listOrder is the cards as a decklist orders them: by type, then cost, then name.
func (v *view) listOrder(list map[string]int) []string {
	codes := slices.Collect(func(yield func(string) bool) {
		for code := range list {
			if !yield(code) {
				return
			}
		}
	})
	slices.SortFunc(codes, func(a, b string) int {
		ca, cb := v.snap.card(a), v.snap.card(b)
		return cmp.Or(
			cmp.Compare(typeRank(ca.Type), typeRank(cb.Type)),
			cmp.Compare(ca.Cost, cb.Cost),
			cmp.Compare(ca.Name, cb.Name),
			cmp.Compare(a, b),
		)
	})
	return codes
}

// stacks draws the leader and each card as its copies fanned out, with the count on top.
func (v *view) stacks(c *ui.Context, leader string, list map[string]int) {
	h := float32(stackW * 7 / 5)
	ui.Row(c).Gap(14).Wrap().AlignItems(ui.Start).Children(func() {
		ui.Box(c.Key("leader")).Size(stackW, h).Radius(6).Border(2, c.Theme().Accent).Children(func() {
			v.cardArt(c, leader, stackW-4, h-4).Margin(2)
		})
		for _, code := range v.listOrder(list) {
			n := list[code]
			ui.Box(c.Key(code)).Size(stackW+float32(n-1)*stackStepX, h+float32(n-1)*stackStepY).Children(func() {
				for i := range n {
					v.cardArt(c.Key(i), code, stackW, h).Absolute().
						Left(float32(i)*stackStepX).Top(float32(i)*stackStepY).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, 0.45))
				}
				pill(c, fmt.Sprintf("×%d", n), 11).Absolute().Right(4).Bottom(4)
			})
		}
	})
}

func listStats(c *ui.Context, st stats.ListStats) {
	ui.Row(c).Gap(32).AlignItems(ui.Start).Children(func() {
		statColumn(c, "Cost curve", func() { curveBars(c, st.Curve) })
		statColumn(c, "Counters", func() {
			total := 0
			for _, cv := range []int{0, 1000, 2000} {
				label := "No counter"
				if cv > 0 {
					label = fmt.Sprint(cv)
				}
				statLine(c, label, fmt.Sprint(st.Counters[cv]))
				total += cv * st.Counters[cv]
			}
			statLine(c, "Total", fmt.Sprintf("%dk", total/1000))
		})
		statColumn(c, "Types", func() {
			for _, ty := range []string{"Character", "Event", "Stage"} {
				if n := st.Types[ty]; n > 0 {
					statLine(c, ty+"s", fmt.Sprint(n))
				}
			}
			for _, col := range slices.Sorted(maps.Keys(st.Colors)) {
				statLine(c.Key(string(col)), string(col), fmt.Sprint(st.Colors[col]))
			}
		})
		statColumn(c, "Traits", func() {
			for _, tr := range st.Traits[:min(len(st.Traits), shownTraits)] {
				statLine(c.Key(tr.Trait), tr.Trait, fmt.Sprint(tr.Copies))
			}
			if more := len(st.Traits) - shownTraits; more > 0 {
				ui.Text(c, fmt.Sprintf("%d more", more)).FontSize(12).TextColor(faintColor(c.Theme()))
			}
		})
	})
}

func statColumn(c *ui.Context, title string, body func()) {
	ui.Column(c).Gap(4).Grow(1).Basis(0).MinWidth(0).Children(func() {
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
	ui.Row(c).Gap(3).AlignItems(ui.End).Height(96).Children(func() {
		for cost, n := range curve {
			ui.Column(c.Key(cost)).Width(16).AlignItems(ui.Center).Gap(2).Children(func() {
				if n > 0 {
					ui.Text(c, fmt.Sprint(n)).FontSize(11).TextColor(t.TextMuted)
				}
				ui.Box(c).Width(12).Height(float32(n) / float32(most) * 56).Radius(3).Background(t.Accent)
				ui.Text(c, fmt.Sprint(cost)).FontSize(11).TextColor(faintColor(t))
			})
		}
	})
}

// suggestionsPanel lists the suggested builds, each with its searchers, to make into a build or
// dismiss. Groups of fewer than minSuggested games fold away.
func (v *view) suggestionsPanel(c *ui.Context, d leaderDecks) {
	t := c.Theme()
	s := &v.decks
	panel(c).Gap(12).Padding(16).Children(func() {
		if len(d.suggestions) == 0 {
			ui.Text(c, "No suggested builds for "+v.snap.card(d.leader).Name).FontWeight(600)
			ui.Text(c, "Suggestions group your games by the searchers their lists run.").
				FontSize(13).TextColor(t.TextMuted).Wrap()
			return
		}
		ui.Text(c, fmt.Sprintf("Suggested builds · %d", shownSuggestions(d))).FontWeight(600)
		ui.Text(c, "Grouped by the searchers each list runs. Make one a build to see its list, cards and changes.").
			FontSize(13).TextColor(t.TextMuted).Wrap()
		small := 0
		for _, sg := range d.suggestions {
			if len(sg.Games) < minSuggested {
				small++
				if !s.showSmall {
					continue
				}
			}
			v.suggestionCard(c.Key(suggestionKey(sg)), d, sg)
		}
		if small > 0 && !s.showSmall {
			label := fmt.Sprintf("Show %d small groups", small)
			if small == 1 {
				label = "Show 1 small group"
			}
			if linkButton(c, label) {
				s.showSmall = true
			}
		}
	})
}

func (v *view) suggestionCard(c *ui.Context, d leaderDecks, sg stats.Suggestion) {
	t := c.Theme()
	s := &v.decks
	key := suggestionKey(sg)
	title := v.suggestionName(sg, d.suggestions)
	ui.Column(c).Gap(10).Padding(12, 14).Radius(10).Border(1, t.Border.Alpha(0.6)).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontWeight(600)
			first, last := sg.Games[0].PlayedAt, sg.Games[len(sg.Games)-1].PlayedAt
			span := first.Format("Jan 2")
			if last.Format("Jan 2") != span {
				span += " – " + last.Format("Jan 2")
			}
			ui.Text(c, buildMeta(sg.Games)+" · "+span).FontSize(13).TextColor(t.TextMuted)
			ui.Spacer(c)
			if s.naming == key {
				return
			}
			if ui.PrimaryButton(c, "Make this a build").Clicked() {
				s.naming, s.name = key, title
			}
			if ui.Button(c, "Not a build").Clicked() {
				v.do(func(a actions) { a.DismissSuggestion(store.Dismissal{Leader: d.leader, Core: sg.Core}) })
			}
		})
		if s.naming == key {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c, &s.name).Grow(1).Label("Build name")
				if (in.Submitted() || ui.PrimaryButton(c, "Save").Clicked()) && strings.TrimSpace(s.name) != "" {
					b := store.Build{Leader: d.leader, Name: strings.TrimSpace(s.name), Core: sg.Core}
					v.do(func(a actions) { a.SaveBuild(b) })
					s.saved, s.naming = b.Name, ""
				}
				if ui.Button(c, "Cancel").Clicked() {
					s.naming = ""
				}
			})
		}
		ui.Row(c).Gap(10).Wrap().Children(func() {
			for _, code := range sg.Core {
				v.searcherTile(c.Key(code), code)
			}
		})
	})
}

// searcherTile is a searcher's card with what it looks for.
func (v *view) searcherTile(c *ui.Context, code string) {
	t := c.Theme()
	card := v.snap.card(code)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(4, 10, 4, 4).Radius(8).Background(t.Surface).Children(func() {
		v.cardArt(c, code, 40, 56)
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, card.Name).FontSize(13)
			if targets := cards.SearchTargets(card.Effect); len(targets) > 0 {
				ui.Text(c, "Searches "+strings.Join(targets, ", ")).FontSize(12).TextColor(faintColor(t))
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
