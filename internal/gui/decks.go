package gui

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

type deckActions interface {
	// SaveBuild calls saved with the build's ID on the main thread once it's stored.
	SaveBuild(b store.Build, saved func(id int64))
	DeleteBuild(id int64)
	DismissSuggestion(d store.Dismissal)
}

type decksPage struct {
	act deckActions

	leaderPoint chartPick
	leaderStats leaderCache

	tab   int
	scope deckScope
	// naming is the core key of the suggestion being made a build, and name the name typed.
	naming, name string
	// saved is the ID of a build just saved, chosen once a snapshot has it.
	saved     int64
	showSmall bool
	// version is when the shown list of the build was first played, zero for the latest.
	version time.Time
	// suggesting shows the suggestions on List though a build is chosen.
	suggesting bool
	// edit is the core being edited, nil when none is.
	edit *buildEdit
	// renaming shows the rename bar under the build row for the chosen build.
	renaming bool
	rename   string
	// handsCard is the card whose dealt hands the Hands tab shows, and allHands shows every one
	// of them rather than the latest few.
	handsCard string
	allHands  bool
	// tester is the Hands tab's practice deal, and hideNumbers keeps the cards' records hidden
	// until the hand is kept or sent back.
	tester      handTest
	hideNumbers bool
	rng         *rand.Rand
}

type scopeKind int

const (
	scopeOverall scopeKind = iota
	scopeNoBuild
	scopeBuild
)

// deckScope is which of the leader's games the Decks page counts: all of them, the ones in no
// build, or one build's.
type deckScope struct {
	kind scopeKind
	// build is the build's ID when kind is scopeBuild.
	build int64
}

func buildScope(id int64) deckScope { return deckScope{kind: scopeBuild, build: id} }

// leaderDecks are the leader's games in the filters' scope with what the deck did, how they
// split into the user's builds, and the suggested builds among the rest.
type leaderDecks struct {
	leader string
	// matches are the leader's games in scope, latest first, anyTurn the same whatever the turn
	// order, and games the matches with deck facts.
	matches, anyTurn []stats.Match
	games            []stats.DeckGame
	builds           []store.Build
	// all is every game of the leader with deck facts whatever the filters, latest first, and
	// split splits them, since a build is about the lists played, not when.
	all   []stats.DeckGame
	split stats.BuildSplit
	// unbuilt counts the games in no build, those without deck facts too.
	unbuilt int
	// rows are the builds table's, when the leader has builds.
	rows []scopeRow
}

// counts reports whether the scope counts the game. A game that fits two builds counts only
// for Overall.
func (d leaderDecks) counts(sc deckScope, fingerprint string) bool {
	id, built := d.split.In(fingerprint)
	switch sc.kind {
	case scopeBuild:
		return built && id == sc.build
	case scopeNoBuild:
		return !built
	}
	return true
}

func (d leaderDecks) scopedGames(sc deckScope) []stats.DeckGame {
	var out []stats.DeckGame
	for _, g := range d.games {
		if d.counts(sc, g.Fingerprint) {
			out = append(out, g)
		}
	}
	return out
}

func (d leaderDecks) scopedMatches(sc deckScope, ms []stats.Match) []stats.Match {
	var out []stats.Match
	for _, m := range ms {
		if d.counts(sc, m.Fingerprint) {
			out = append(out, m)
		}
	}
	return out
}

func (d leaderDecks) chosen(id int64) (store.Build, bool) {
	for _, b := range d.builds {
		if b.ID == id {
			return b, true
		}
	}
	return store.Build{}, false
}

// splitLeader splits the leader's games into builds, or takes the split from prev when it has one.
func splitLeader(e *env, leader string, prev *leaderDecks) leaderDecks {
	d := leaderDecks{leader: leader}
	sc := e.scoped()
	for _, m := range sc.anyDeck {
		if m.Me.Leader == leader {
			d.matches = append(d.matches, m)
		}
	}
	for _, m := range sc.anyTurn {
		if m.Me.Leader == leader {
			d.anyTurn = append(d.anyTurn, m)
		}
	}
	for _, m := range d.matches {
		if g, ok := e.snap.Decks[m.Fingerprint]; ok {
			d.games = append(d.games, g)
		}
	}
	for _, b := range e.snap.Builds {
		if b.Leader == leader {
			d.builds = append(d.builds, b)
		}
	}
	if prev != nil {
		d.all, d.split = prev.all, prev.split
	} else {
		for _, g := range e.snap.Decks {
			if g.Me.Leader == leader {
				d.all = append(d.all, g)
			}
		}
		slices.SortFunc(d.all, func(a, b stats.DeckGame) int { return b.PlayedAt.Compare(a.PlayedAt) })
		d.split = stats.Builds(d.all, d.builds, e.snap.Dismissed[leader], e.snap.card)
	}
	d.unbuilt = len(d.scopedMatches(deckScope{kind: scopeNoBuild}, d.matches))
	if len(d.builds) > 0 {
		d.rows = d.scopeRows()
	}
	return d
}

// noBuild names the scope of the games in no build.
const noBuild = "No build"

// minSuggested is the fewest games a suggested build is shown for. Smaller groups fold away.
const minSuggested = 3

// scopeName names the scope in panel titles. It's empty for a leader without builds, where
// every game counts anyway.
func (d leaderDecks) scopeName(s deckScope) string {
	switch {
	case len(d.builds) == 0:
		return ""
	case s.kind == scopeNoBuild:
		return noBuild
	case s.kind == scopeBuild:
		b, _ := d.chosen(s.build)
		return b.Name
	}
	return "Overall"
}

// buildRow chooses the scope: Overall, the leader's builds and No build, then a chip for a new
// build and a link to the suggestions. Without builds, the suggestions get a chip of their own.
func (pg *decksPage) buildRow(c *ui.Context, e *env, d leaderDecks) {
	t := c.Theme()
	s := pg
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Grow(1).Basis(0).Gap(8).AlignItems(ui.Center).Wrap().Children(func() { pg.buildChips(c, e, d) })
		if len(d.games) == 0 {
			return
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if len(d.builds) == 0 {
				ui.Text(c, "No builds yet · "+games(len(d.games))).FontSize(12).TextColor(faintColor(t)).SingleLine()
				return
			}
			if len(d.split.Suggestions) > 0 {
				if linkButton(c, fmt.Sprintf("%d suggested", shownSuggestions(d))) {
					s.suggesting, s.tab, s.edit = true, tabList, nil
				}
			}
		})
	})
	if b, ok := d.chosen(s.scope.build); ok && s.scope.kind == scopeBuild {
		pg.renameBar(c, b)
	}
}

// buildChips choose the scope the tabs show. Each build has its actions on right-click, and
// the chosen one on its ⋯ too. Without builds, one chip opens List on the suggestions.
func (pg *decksPage) buildChips(c *ui.Context, e *env, d leaderDecks) {
	t := c.Theme()
	s := pg
	if len(d.builds) > 0 {
		overall := deckScope{}
		if buildChip(c.Key("overall"), "Overall", matchesMeta(d.matches), false, s.scope == overall) {
			s.scope, s.suggesting = overall, false
		}
		ui.Box(c).Size(1, 34).Background(t.Border).Margin(0, 4)
	}
	for _, b := range d.builds {
		pg.savedBuildChip(c.Key(fmt.Sprint("b", b.ID)), e, d, b)
	}
	none := deckScope{kind: scopeNoBuild}
	if len(d.builds) > 0 && (d.unbuilt > 0 || s.scope == none) {
		if buildChip(c.Key("none"), noBuild, matchesMeta(d.scopedMatches(none, d.matches)), false, s.scope == none) {
			s.scope, s.suggesting = none, false
		}
	}
	if len(d.builds) == 0 && len(d.split.Suggestions) > 0 {
		if buildChip(c.Key("suggested"), fmt.Sprintf("%d suggested", shownSuggestions(d)), "Builds to make", true,
			s.tab == tabList && s.edit == nil) {
			s.tab, s.edit = tabList, nil
		}
	}
	if len(d.games) > 0 && buildChip(c.Key("new"), "New build", "Pick its core", true, false) {
		pg.startEdit(store.Build{Leader: d.leader})
	}
}

func (pg *decksPage) savedBuildChip(c *ui.Context, e *env, d leaderDecks, b store.Build) {
	t := c.Theme()
	s := pg
	on := s.scope == buildScope(b.ID)
	menu := func(m *ui.Menu) { pg.buildMenu(e, m, d, b) }
	group := ui.Row(c).AlignItems(ui.Stretch).Radius(10).Clip().Border(1, t.Border.Alpha(0.6))
	if on {
		group.Background(t.Accent.Alpha(0.12)).Border(1, t.Accent)
	}
	group.Children(func() {
		chip := ui.ButtonBase(c).Label(b.Name).Column().Gap(1).Padding(7, 12).Background(ui.Transparent).
			AlignItems(ui.Start).Tooltip(coreText(e, b)).PopoverMenu(menu)
		if !on && chip.Hovered() {
			chip.Background(t.SurfaceHover)
		}
		if chip.Clicked() {
			s.scope, s.version, s.suggesting = buildScope(b.ID), time.Time{}, false
		}
		chip.Children(func() {
			ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
				coreThumbs(c, e, b.Core)
				ui.Text(c, b.Name).FontSize(13).FontWeight(600).SingleLine()
			})
			ui.Text(c, matchesMeta(d.scopedMatches(buildScope(b.ID), d.matches))).FontSize(12).TextColor(faintColor(t)).SingleLine()
		})
		if !on {
			return
		}
		ui.Box(c).Width(1).Background(t.Border.Alpha(0.6))
		more := ui.ButtonBase(c).Label(b.Name+" actions").Padding(0, 9).Background(ui.Transparent).Center().
			TextColor(t.TextMuted).PopoverMenuButton(menu)
		if more.Hovered() {
			more.Background(t.SurfaceHover).TextColor(t.Text)
		}
		more.Children(func() { ui.Icon(c, iconMore).FontSize(16) })
	})
}

func coreText(e *env, b store.Build) string {
	var names []string
	for _, code := range b.Core {
		names = append(names, e.snap.card(code).Name)
	}
	return "Core: " + strings.Join(names, ", ")
}

func (pg *decksPage) buildMenu(e *env, m *ui.Menu, d leaderDecks, b store.Build) {
	s := pg
	m.Item(coreText(e, b)).Disabled(true)
	m.Separator()
	if m.Item("Edit core").Chosen() {
		s.scope = buildScope(b.ID)
		pg.startEdit(b)
	}
	if m.Item("Rename").Chosen() {
		s.scope, s.renaming, s.rename = buildScope(b.ID), true, b.Name
	}
	m.Separator()
	if m.Item("Delete build").Chosen() {
		pg.deleteBuild(e, d, b)
	}
}

func (pg *decksPage) renameBar(c *ui.Context, b store.Build) {
	s := pg
	if !s.renaming {
		return
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		in := ui.TextInput(c, &s.rename).Width(280).Label("Build name")
		if (in.Submitted() || ui.PrimaryButton(c, "Save").Clicked()) && strings.TrimSpace(s.rename) != "" {
			nb := b
			nb.Name = strings.TrimSpace(s.rename)
			pg.do(func(a deckActions) { a.SaveBuild(nb, nil) })
			s.renaming = false
		}
		if ui.Button(c, "Cancel").Clicked() {
			s.renaming = false
		}
	})
}

// buildUndoTime is how long the message after a delete shows, with its Undo.
const buildUndoTime = 10 * time.Second

// deleteBuild deletes the build at once and offers Undo, which saves it again as a new build.
func (pg *decksPage) deleteBuild(e *env, d leaderDecks, b store.Build) {
	n := len(d.split.Builds[b.ID])
	pg.do(func(a deckActions) { a.DeleteBuild(b.ID) })
	pg.scope, pg.renaming = deckScope{}, false
	e.toast = &ui.Toast{
		ID: "build-deleted", Timeout: buildUndoTime, Title: "Deleted " + b.Name + ".",
		Description: "Its " + games(n) + " went back to no build.", Action: "Undo",
		OnAction: func() {
			nb := b
			nb.ID = 0
			pg.do(func(a deckActions) { a.SaveBuild(nb, pg.chooseSaved) })
		},
	}
}

func (pg *decksPage) chooseSaved(id int64) { pg.saved = id }

// chooseBuild keeps the chosen build to the leader's, chooses the first when none is, and
// chooses a build just saved once it's in.
func (pg *decksPage) chooseBuild(d leaderDecks) {
	s := pg
	if _, ok := d.chosen(s.saved); ok {
		s.scope, s.saved, s.tab, s.suggesting = buildScope(s.saved), 0, tabList, false
	}
	if _, ok := d.chosen(s.scope.build); s.scope.kind == scopeBuild && !ok {
		s.scope = deckScope{}
	}
	if len(d.builds) == 0 {
		s.scope = deckScope{}
	}
	if s.edit != nil {
		s.tab = tabList
	}
}

// shownSuggestions counts the suggestions of minSuggested games or more, or every one when all
// are smaller.
func shownSuggestions(d leaderDecks) int {
	if n := len(d.split.Suggestions) - smallSuggestions(d); n > 0 {
		return n
	}
	return len(d.split.Suggestions)
}

func smallSuggestions(d leaderDecks) int {
	n := 0
	for _, s := range d.split.Suggestions {
		if smallSuggestion(s) {
			n++
		}
	}
	return n
}

func smallSuggestion(s stats.Suggestion) bool { return len(s.Games) < minSuggested }

// coreThumbs are small overlapping art of the first core cards.
func coreThumbs(c *ui.Context, e *env, core []string) {
	// h is a little more than the cards are tall, and sets the build chip's height.
	const w, h, step, most = 12, 17, 8, 3
	n := min(len(core), most)
	if n == 0 {
		return
	}
	ui.Box(c).Size(w+float32(n-1)*step, h).Children(func() {
		for i, code := range core[:n] {
			e.card(c.Key(code), code, w).Absolute().Left(float32(i) * step).Top(0)
		}
	})
}

func matchesMeta(ms []stats.Match) string { return scopeMeta(len(ms), stats.RecordOf(ms)) }

// scopeMeta is n games with their record r and its rate.
func scopeMeta(n int, r stats.Record) string {
	return games(n) + " · " + record(r) + " · " + rateText(r)
}

func buildChip(c *ui.Context, name, meta string, suggested, on bool) bool {
	t := c.Theme()
	chip := ui.ButtonBase(c).Label(name).Column().Gap(1).Padding(7, 12).Radius(10).Border(1, t.Border.Alpha(0.6)).
		Background(ui.Transparent).AlignItems(ui.Start)
	nameCol := t.Text
	switch {
	case on:
		chip.Background(t.Accent.Alpha(0.12)).Border(1, t.Accent)
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

func (pg *decksPage) pageTabs(c *ui.Context) {
	t := c.Theme()
	s := pg
	ui.Column(c).Children(func() {
		tabs := ui.TabsBase(c, &s.tab, len(deckPageTabs))
		tabs.List.Gap(4).Children(func() {
			for i, dt := range deckPageTabs {
				tab := tabs.Tab(i).Column().Gap(6).Padding(7, 4, 0, 4)
				col, bar := t.TextMuted, ui.Transparent
				switch {
				case i == s.tab:
					col, bar = t.Text, t.Accent
				case tab.Hovered():
					col = t.Text
				}
				tab.Children(func() {
					ui.Text(c, dt.label).FontSize(13).TextColor(col).Padding(0, 8)
					ui.Box(c).Height(2).Radius(1).Background(bar).AlignSelf(ui.Stretch)
				})
			}
		})
		ui.Box(c).Height(1).Background(t.Border)
	})
}

func (pg *decksPage) do(fn func(deckActions)) {
	if pg.act != nil {
		fn(pg.act)
	}
}
