package gui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

// Decks page tabs.
const (
	tabResults = iota
	tabList
	tabCards
	tabChanges
)

var deckTabLabels = []string{"Results", "List", "Cards", "Changes"}

// decksState is what the Decks page shows of the leader: which tab.
type decksState struct {
	tab int
}

// leaderDecks are the leader's games in scope with what the deck did, how they split into the
// user's builds, and the suggested builds among the rest.
type leaderDecks struct {
	games       []stats.DeckGame
	staples     []string
	builds      []store.Build
	assignment  stats.Assignment
	suggestions []stats.Suggestion
	// unbuilt counts the games in no build.
	unbuilt int
}

func (v *view) deckData(leader string, ms []stats.Match) leaderDecks {
	var d leaderDecks
	for _, m := range ms {
		if g, ok := v.snap.Decks[m.Fingerprint]; ok {
			d.games = append(d.games, g)
		}
	}
	for _, b := range v.snap.Builds {
		if b.Leader == leader {
			d.builds = append(d.builds, b)
		}
	}
	d.staples = stats.Staples(d.games)
	d.assignment = stats.Assign(d.games, d.builds)
	built := make(map[string]bool)
	for _, gs := range d.assignment.Builds {
		for _, g := range gs {
			built[g.Fingerprint] = true
		}
	}
	for _, g := range d.assignment.Conflicts {
		built[g.Fingerprint] = true
	}
	for _, g := range d.games {
		if !built[g.Fingerprint] {
			d.unbuilt++
		}
	}
	d.suggestions = v.suggestions(leader)
	return d
}

// suggestions come from every game of the leader whatever the filters, since a build is about
// the lists played, not when. Games in a build aren't suggested again.
func (v *view) suggestions(leader string) []stats.Suggestion {
	var all []stats.DeckGame
	for _, g := range v.snap.Decks {
		if g.Me.Leader == leader {
			all = append(all, g)
		}
	}
	slices.SortFunc(all, func(a, b stats.DeckGame) int { return a.PlayedAt.Compare(b.PlayedAt) })
	var builds []store.Build
	for _, b := range v.snap.Builds {
		if b.Leader == leader {
			builds = append(builds, b)
		}
	}
	a := stats.Assign(all, builds)
	built := make(map[string]bool)
	for _, gs := range a.Builds {
		for _, g := range gs {
			built[g.Fingerprint] = true
		}
	}
	var rest []stats.DeckGame
	for _, g := range all {
		if !built[g.Fingerprint] {
			rest = append(rest, g)
		}
	}
	return stats.Suggest(rest, v.snap.Dismissed[leader])
}

// minSuggested is the fewest games a suggested build gets its own chip for. Smaller groups are
// only counted, in the note at the row's end.
const minSuggested = 3

// buildRow is the leader's builds, then the suggested ones, and how many games are in none.
func (v *view) buildRow(c *ui.Context, d leaderDecks) {
	t := c.Theme()
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Grow(1).Basis(0).Gap(8).AlignItems(ui.Center).Wrap().Children(func() { v.buildChips(c, d) })
		if len(d.games) == 0 {
			return
		}
		ui.Column(c).Gap(2).AlignItems(ui.End).Children(func() {
			note := games(d.unbuilt) + " in no build"
			if len(d.builds) == 0 {
				note = "No builds yet · " + games(len(d.games))
			}
			ui.Text(c, note).FontSize(12).TextColor(faintColor(t)).SingleLine()
			switch small := smallSuggestions(d); small {
			case 0:
			case 1:
				ui.Text(c, "+1 small group").FontSize(12).TextColor(faintColor(t)).SingleLine()
			default:
				ui.Text(c, fmt.Sprintf("+%d small groups", small)).FontSize(12).TextColor(faintColor(t)).SingleLine()
			}
		})
	})
}

func (v *view) buildChips(c *ui.Context, d leaderDecks) {
	for _, b := range d.builds {
		buildChip(c.Key(fmt.Sprint("b", b.ID)), b.Name, buildMeta(d.assignment.Builds[b.ID]), false)
	}
	for i, s := range d.suggestions {
		if len(s.Games) >= minSuggested {
			buildChip(c.Key(fmt.Sprint("s", i)), v.suggestionName(s, d.suggestions)+"?", "Suggested · "+games(len(s.Games)), true)
		}
	}
}

func smallSuggestions(d leaderDecks) int {
	n := 0
	for _, s := range d.suggestions {
		if len(s.Games) < minSuggested {
			n++
		}
	}
	return n
}

func buildMeta(gs []stats.DeckGame) string {
	var r stats.Record
	for _, g := range gs {
		r.Add(g.Result)
	}
	meta := games(len(gs)) + " · " + record(r)
	if rate, ok := r.Rate(); ok {
		meta += " · " + pct(rate)
	}
	return meta
}

// suggestionName is what the suggestion's searchers look for, such as "Land of Wano", or a
// searcher's name when its effect doesn't say. Searchers every suggestion runs are left out,
// since they don't tell the suggestions apart.
func (v *view) suggestionName(s stats.Suggestion, all []stats.Suggestion) string {
	shared := func(c string) bool {
		return !slices.ContainsFunc(all, func(o stats.Suggestion) bool { return !slices.Contains(o.Core, c) })
	}
	telling := slices.DeleteFunc(slices.Clone(s.Core), func(c string) bool { return len(all) > 1 && shared(c) })
	if len(telling) == 0 {
		telling = s.Core
	}
	var names []string
	for _, c := range telling {
		card := v.snap.card(c)
		targets := cards.SearchTargets(card.Effect)
		if len(targets) == 0 {
			targets = []string{card.Name}
		}
		for _, t := range targets {
			if !slices.Contains(names, t) {
				names = append(names, t)
			}
		}
	}
	return strings.Join(names, " + ")
}

func buildChip(c *ui.Context, name, meta string, suggested bool) {
	t := c.Theme()
	chip := ui.Column(c).Gap(1).Padding(7, 12).Radius(10).Border(1, t.Border.Alpha(0.6))
	nameCol := t.Text
	if suggested {
		chip.BorderStyle(ui.BorderDashed)
		nameCol = t.TextMuted
	}
	chip.Children(func() {
		ui.Text(c, name).FontSize(13).FontWeight(600).TextColor(nameCol).SingleLine()
		ui.Text(c, meta).FontSize(12).TextColor(faintColor(t)).SingleLine()
	})
}

// pageTabs are the page's tabs. Only Results opens. List, Cards and Changes need a build.
func (v *view) pageTabs(c *ui.Context) {
	t := c.Theme()
	s := &v.decks
	if s.tab != tabResults {
		s.tab = tabResults
	}
	ui.Column(c).Children(func() {
		tabs := ui.TabsBase(c, &s.tab, len(deckTabLabels))
		tabs.List.Gap(4).Children(func() {
			for i, label := range deckTabLabels {
				on := i == tabResults
				tab := tabs.Tab(i).Column().Gap(6).Padding(7, 4, 0, 4)
				col, bar := faintColor(t), ui.Transparent
				switch {
				case i == s.tab:
					col, bar = t.Text, t.Accent
				case on && tab.Hovered():
					col = t.Text
				case on:
					col = t.TextMuted
				}
				tab.Children(func() {
					ui.Text(c, label).FontSize(13).TextColor(col).Padding(0, 8)
					ui.Box(c).Height(2).Radius(1).Background(bar).AlignSelf(ui.Stretch)
				})
			}
		})
		ui.Box(c).Height(1).Background(t.Border)
	})
}
