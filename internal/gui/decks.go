package gui

import (
	"fmt"
	"slices"
	"strings"
	"time"

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
)

var deckTabLabels = []string{"Results", "List", "Cards"}

// decksState is what the Decks page shows of the leader.
type decksState struct {
	tab int
	// build is the chosen build's ID, 0 for none. List then shows the suggestions.
	build int64
	// naming is the core key of the suggestion being made a build, and name the name typed.
	naming, name string
	// saved is the name of a build just saved, chosen once the next snapshot has it.
	saved     string
	showSmall bool
	// version is when the shown list of the build was first played, zero for the latest.
	version time.Time
	// suggesting shows the suggestions on List though a build is chosen.
	suggesting bool
	// edit is the core being edited, nil when none is.
	edit *buildEdit
	// renaming and deleting show their bar under the build row for the chosen build.
	renaming, deleting bool
	rename             string
	// editOnOpen opens the core editor on the chosen build once the page has one.
	editOnOpen bool
}

// suggestionKey tells suggestions apart by their searchers.
func suggestionKey(s stats.Suggestion) string { return strings.Join(s.Core, ",") }

// leaderDecks are the leader's games in scope with what the deck did, how they split into the
// user's builds, and the suggested builds among the rest.
type leaderDecks struct {
	leader      string
	games       []stats.DeckGame
	staples     []string
	builds      []store.Build
	assignment  stats.Assignment
	suggestions []stats.Suggestion
	// unbuilt counts the games in no build.
	unbuilt int
}

// chosen is the build with the ID among the leader's.
func (d leaderDecks) chosen(id int64) (store.Build, bool) {
	for _, b := range d.builds {
		if b.ID == id {
			return b, true
		}
	}
	return store.Build{}, false
}

func (v *view) deckData(leader string, ms []stats.Match) leaderDecks {
	d := leaderDecks{leader: leader}
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

// minSuggested is the fewest games a suggested build is shown for. Smaller groups fold away.
const minSuggested = 3

// buildRow is the leader's builds, or one chip for the suggestions, and how many games are in no build.
func (v *view) buildRow(c *ui.Context, d leaderDecks) {
	t := c.Theme()
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Grow(1).Basis(0).Gap(8).AlignItems(ui.Center).Wrap().Children(func() { v.buildChips(c, d) })
		if len(d.games) == 0 {
			return
		}
		note := games(d.unbuilt) + " in no build"
		if len(d.builds) == 0 {
			note = "No builds yet · " + games(len(d.games))
		}
		ui.Text(c, note).FontSize(12).TextColor(faintColor(t)).SingleLine()
	})
	if b, ok := d.chosen(v.decks.build); ok {
		v.renameBar(c, b)
		v.deleteBar(c, d, b)
	}
}

// buildChips choose the build the tabs show, then "Builds" opens the build menu. Without
// builds, one chip opens List on the suggestions, next to "New build".
func (v *view) buildChips(c *ui.Context, d leaderDecks) {
	s := &v.decks
	for _, b := range d.builds {
		if buildChip(c.Key(fmt.Sprint("b", b.ID)), b.Name, buildMeta(d.assignment.Builds[b.ID]), false, s.build == b.ID) {
			s.build, s.version, s.suggesting = b.ID, time.Time{}, false
		}
	}
	if len(d.builds) > 0 {
		filterPill(c.Key("menu"), "Builds", false).PopoverMenuButton(func(m *ui.Menu) { v.buildMenu(m, d) })
		return
	}
	if len(d.suggestions) > 0 {
		if buildChip(c.Key("suggested"), fmt.Sprintf("%d suggested", shownSuggestions(d)), "Builds to make", true,
			s.tab == tabList && s.edit == nil) {
			s.tab, s.edit = tabList, nil
		}
	}
	if len(d.games) > 0 && ui.Button(c.Key("new"), "New build").Clicked() {
		v.startEdit(store.Build{Leader: d.leader})
	}
}

// buildMenu shows the chosen build's core and what can be done to it, then a new build and the
// suggestions.
func (v *view) buildMenu(m *ui.Menu, d leaderDecks) {
	s := &v.decks
	if b, ok := d.chosen(s.build); ok {
		var names []string
		for _, code := range b.Core {
			names = append(names, v.snap.card(code).Name)
		}
		m.Item("Core: " + strings.Join(names, ", ")).Disabled(true)
		if m.Item("Edit core").Chosen() {
			v.startEdit(b)
		}
		if m.Item("Rename").Chosen() {
			s.renaming, s.deleting, s.rename = true, false, b.Name
		}
		if m.Item("Delete build").Chosen() {
			s.deleting, s.renaming = true, false
		}
		m.Separator()
	}
	if m.Item("New build").Chosen() {
		v.startEdit(store.Build{Leader: d.leader})
	}
	label := "Suggested builds"
	if n := shownSuggestions(d); n > 0 {
		label = fmt.Sprintf("Suggested builds · %d", n)
	}
	if m.Item(label).Disabled(len(d.suggestions) == 0).Chosen() {
		s.suggesting, s.tab, s.edit = true, tabList, nil
	}
}

func (v *view) renameBar(c *ui.Context, b store.Build) {
	s := &v.decks
	if !s.renaming {
		return
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		in := ui.TextInput(c, &s.rename).Width(280).Label("Build name")
		if (in.Submitted() || ui.PrimaryButton(c, "Save").Clicked()) && strings.TrimSpace(s.rename) != "" {
			nb := b
			nb.Name = strings.TrimSpace(s.rename)
			v.do(func(a actions) { a.SaveBuild(nb) })
			s.renaming = false
		}
		if ui.Button(c, "Cancel").Clicked() {
			s.renaming = false
		}
	})
}

func (v *view) deleteBar(c *ui.Context, d leaderDecks, b store.Build) {
	t := c.Theme()
	s := &v.decks
	if !s.deleting {
		return
	}
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(8, 12).Radius(8).Background(t.Surface).Children(func() {
		ui.Text(c, fmt.Sprintf("Delete %s? Its %s go back to no build.", b.Name, games(len(d.assignment.Builds[b.ID])))).
			FontSize(13)
		ui.Spacer(c)
		if ui.Button(c, "Delete").Clicked() {
			v.do(func(a actions) { a.DeleteBuild(b.ID) })
			s.deleting, s.build = false, 0
		}
		if ui.Button(c, "Cancel").Clicked() {
			s.deleting = false
		}
	})
}

// chooseBuild keeps the chosen build to the leader's, chooses the first when none is, and
// chooses a build just saved once it's in.
func (v *view) chooseBuild(d leaderDecks) {
	s := &v.decks
	if s.saved != "" {
		for _, b := range d.builds {
			if b.Name == s.saved {
				s.build, s.saved, s.tab, s.suggesting = b.ID, "", tabList, false
			}
		}
	}
	if !slices.ContainsFunc(d.builds, func(b store.Build) bool { return b.ID == s.build }) {
		s.build = 0
	}
	if s.build == 0 && len(d.builds) > 0 {
		s.build = d.builds[0].ID
	}
	if b, ok := d.chosen(s.build); ok && s.editOnOpen {
		s.editOnOpen = false
		v.startEdit(b)
	}
	if s.edit != nil {
		s.tab = tabList
	}
	if s.build == 0 && s.tab == tabCards {
		s.tab = tabResults
	}
}

// shownSuggestions counts the suggestions of minSuggested games or more, or every one when all
// are smaller.
func shownSuggestions(d leaderDecks) int {
	if n := len(d.suggestions) - smallSuggestions(d); n > 0 {
		return n
	}
	return len(d.suggestions)
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

// buildChip reports a click.
func buildChip(c *ui.Context, name, meta string, suggested, on bool) bool {
	t := c.Theme()
	chip := ui.ButtonBase(c).Label(name).Column().Gap(1).Padding(7, 12).Radius(10).Border(1, t.Border.Alpha(0.6)).
		Background(ui.Transparent).AlignItems(ui.Start)
	nameCol := t.Text
	switch {
	case on:
		chip.Background(t.Surface).Border(1, t.Accent)
	case chip.Hovered():
		chip.Background(t.SurfaceHover)
	}
	if suggested {
		chip.BorderStyle(ui.BorderDashed)
		if !on {
			nameCol = t.TextMuted
		}
	}
	chip.Children(func() {
		ui.Text(c, name).FontSize(13).FontWeight(600).TextColor(nameCol).SingleLine()
		ui.Text(c, meta).FontSize(12).TextColor(faintColor(t)).SingleLine()
	})
	return chip.Clicked()
}

// pageTabs are the page's tabs. Cards needs a chosen build.
func (v *view) pageTabs(c *ui.Context) {
	t := c.Theme()
	s := &v.decks
	if s.tab == tabCards && s.build == 0 {
		s.tab = tabResults
	}
	ui.Column(c).Children(func() {
		tabs := ui.TabsBase(c, &s.tab, len(deckTabLabels))
		tabs.List.Gap(4).Children(func() {
			for i, label := range deckTabLabels {
				on := i == tabResults || i == tabList || s.build != 0
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
