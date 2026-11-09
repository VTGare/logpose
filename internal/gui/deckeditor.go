package gui

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

// buildEdit is a build's core and name being edited. A new build has ID 0.
type buildEdit struct {
	build  store.Build
	core   []string
	name   string
	search string
}

// newBuildID stands in for the ID of a build not saved yet when the games are assigned.
const newBuildID = -1

func (v *view) startEdit(b store.Build) {
	s := &v.decks
	s.edit = &buildEdit{build: b, core: slices.Clone(b.Core), name: b.Name}
	s.tab, s.renaming, s.suggesting = tabList, false, false
}

// leaderGames are every game of the leader with what the deck did, oldest first, whatever the
// filters, since a build's core is about every list played.
func (v *view) leaderGames(leader string) []stats.DeckGame {
	var all []stats.DeckGame
	for _, g := range v.snap.Decks {
		if g.Me.Leader == leader {
			all = append(all, g)
		}
	}
	slices.SortFunc(all, func(a, b stats.DeckGame) int { return a.PlayedAt.Compare(b.PlayedAt) })
	return all
}

// editPreview is where the leader's games go now and with the edited core.
type editPreview struct {
	all          []stats.DeckGame
	now, after   map[string]bool
	others       map[string]bool
	conflicts    int
	before, with stats.Record
}

func (v *view) preview(d leaderDecks, e *buildEdit) editPreview {
	p := editPreview{all: v.leaderGames(d.leader), now: make(map[string]bool), after: make(map[string]bool), others: make(map[string]bool)}
	id := e.build.ID
	if id == 0 {
		id = newBuildID
	}
	var builds []store.Build
	for _, b := range d.builds {
		if b.ID != e.build.ID {
			builds = append(builds, b)
		}
	}
	for _, g := range stats.Assign(p.all, d.builds).Builds[e.build.ID] {
		p.now[g.Fingerprint] = true
		p.before.Add(g.Result)
	}
	edited := e.build
	edited.ID, edited.Core = id, e.core
	a := stats.Assign(p.all, append(builds, edited))
	for bid, gs := range a.Builds {
		for _, g := range gs {
			if bid == id {
				p.after[g.Fingerprint] = true
				p.with.Add(g.Result)
			} else {
				p.others[g.Fingerprint] = true
			}
		}
	}
	p.conflicts = len(a.Conflicts)
	return p
}

// coreEditor takes the page while a build's core is edited: the cards of its latest list, then
// the cards of the leader's other lists, with the games it gets as they change.
func (v *view) coreEditor(c *ui.Context, d leaderDecks) {
	t := c.Theme()
	e := v.decks.edit
	p := v.preview(d, e)
	if len(p.all) == 0 {
		v.decks.edit = nil
		return
	}
	freq := make(map[string]int)
	for _, g := range p.all {
		for code := range g.Facts.List {
			freq[code]++
		}
	}
	staples := stats.Staples(p.all)
	base := p.all[len(p.all)-1].Facts.List
	for i := len(p.all) - 1; i >= 0; i-- {
		if p.now[p.all[i].Fingerprint] {
			base = p.all[i].Facts.List
			break
		}
	}
	root := panel(c).Gap(16).Padding(16, 20, 20, 20)
	if root.OverlayShortcut(0, ui.KeyEscape) {
		v.decks.edit = nil
		return
	}
	root.Children(func() {
		v.editorHeader(c, d, e)
		editorPreview(c, e, p)
		ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
		ui.Column(c).Gap(12).Children(func() {
			ui.Text(c, "Latest list").FontWeight(600)
			ui.Row(c).Gap(10).Wrap().AlignItems(ui.Start).Children(func() {
				for _, code := range v.listOrder(base) {
					v.editStack(c.Key(code), e, code, base[code], freq[code], len(p.all), slices.Contains(staples, code))
				}
			})
		})
		v.otherCards(c, e, base, freq, len(p.all))
	})
}

func (v *view) editorHeader(c *ui.Context, d leaderDecks, e *buildEdit) {
	t := c.Theme()
	leader := v.snap.card(d.leader).Name
	ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
		v.artStrip(c, d.leader, 48, 34)
		ui.Column(c).Gap(2).Children(func() {
			title := "New " + leader + " build"
			if e.build.ID != 0 {
				title = "Edit " + e.build.Name
			}
			ui.Text(c, title).FontSize(18).FontWeight(600)
			ui.Text(c, "A game is in the build when its list runs every core card.").FontSize(12).TextColor(t.TextMuted)
		})
		ui.Spacer(c)
		name := strings.TrimSpace(e.name)
		var missing string
		switch {
		case name == "" && len(e.core) == 0:
			missing = "Name it and pick a core card"
		case name == "":
			missing = "Name it to save"
		case len(e.core) == 0:
			missing = "Pick a core card to save"
		}
		if missing != "" {
			ui.Text(c, missing).FontSize(12).TextColor(faintColor(t))
		}
		if ui.Button(c, "Cancel").Tooltip("Esc").Clicked() {
			v.decks.edit = nil
		}
		save := ui.PrimaryButton(c, "Save").Disabled(missing != "")
		if save.Clicked() && missing == "" {
			b := e.build
			b.Name, b.Core = name, slices.Clone(e.core)
			v.do(func(a actions) { a.SaveBuild(b) })
			v.decks.saved, v.decks.edit = name, nil
		}
	})
	if e.build.ID == 0 {
		ui.TextInput(c, &e.name).Width(280).Label("Build name").Placeholder("Build name").AutoFocus()
	}
}

// editorPreview is the games the core gets, and how that differs from the saved core once it
// does.
func editorPreview(c *ui.Context, e *buildEdit, p editPreview) {
	t := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.End).Children(func() {
			joins, leaves := 0, 0
			for fp := range p.after {
				if !p.now[fp] {
					joins++
				}
			}
			for fp := range p.now {
				if !p.after[fp] {
					leaves++
				}
			}
			switch {
			case len(e.core) == 0:
				ui.Text(c, "Pick a core card to see its games").FontSize(14).FontWeight(600).TextColor(t.TextMuted)
			case e.build.ID == 0 || joins+leaves == 0:
				ui.Text(c, games(len(p.after))+" · "+record(p.with)).FontSize(14).FontWeight(600)
			default:
				ui.Text(c, fmt.Sprintf("%s → %s · %s → %s", games(len(p.now)), games(len(p.after)), record(p.before), record(p.with))).
					FontSize(14).FontWeight(600)
				ui.Text(c, fmt.Sprintf("+%d joining, −%d leaving", joins, leaves)).FontSize(12).TextColor(t.TextMuted)
			}
			if p.conflicts > 0 {
				ui.Text(c, games(p.conflicts)+" fit two builds").FontSize(12).TextColor(warnColor)
			}
		})
		gameStrip(c, p)
	})
}

// gameStrip is a mark per game of the leader, oldest first, with a gap between days: in the
// build, joining or leaving it with the edit, in another build, or in none.
func gameStrip(c *ui.Context, p editPreview) {
	t := c.Theme()
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Gap(2).Wrap().AlignItems(ui.Center).Children(func() {
			last := ""
			for i, g := range p.all {
				day := g.PlayedAt.Format("Jan 2")
				if last != "" && day != last {
					ui.Box(c.Key(fmt.Sprint("gap", i))).Width(5)
				}
				last = day
				mark := ui.Box(c.Key(i)).Size(8, 14).Radius(2).Tooltip(g.PlayedAt.Format("Jan 2 15:04"))
				fill, edge := stripMark(t, p, g.Fingerprint)
				mark.Background(fill).Border(1.5, edge)
			}
		})
		ui.Row(c).Gap(14).Children(func() {
			legend := func(label string, fill, edge ui.Color) {
				ui.Row(c.Key(label)).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(8, 12).Radius(2).Background(fill).Border(1.5, edge)
					ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
				})
			}
			legend("in this build", t.Accent, t.Accent)
			legend("joins", ui.Transparent, t.Accent)
			legend("leaves", ui.Transparent, lossColor)
			legend("in another build", warnColor, warnColor)
			legend("in no build", t.Border, t.Border)
		})
	})
}

func stripMark(t *ui.Theme, p editPreview, fp string) (fill, edge ui.Color) {
	switch {
	case p.after[fp] && p.now[fp]:
		return t.Accent, t.Accent
	case p.after[fp]:
		return ui.Transparent, t.Accent
	case p.now[fp]:
		return ui.Transparent, lossColor
	case p.others[fp]:
		return warnColor, warnColor
	}
	return t.Border, t.Border
}

func (e *buildEdit) toggle(code string) {
	if i := slices.Index(e.core, code); i >= 0 {
		e.core = slices.Delete(e.core, i, i+1)
		return
	}
	e.core = append(e.core, code)
	slices.Sort(e.core)
}

// coreMark is what a card's toggle says under it: Core when it's in, + Core on hover, and
// Staple for a card in nearly every list.
func coreMark(c *ui.Context, core, hovered, staple bool) {
	t := c.Theme()
	label, col := "+ Core", t.TextMuted
	switch {
	case core:
		label, col = "Core", t.Accent
	case hovered:
	case staple:
		label, col = "Staple", faintColor(t)
	default:
		col = ui.Transparent
	}
	ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(col)
}

// toggleLook styles a card toggle by whether the card is in the core.
func toggleLook(c *ui.Context, b ui.Element, core bool) {
	t := c.Theme()
	b.Radius(8).Border(1, ui.Transparent).Background(ui.Transparent)
	switch {
	case core:
		b.Background(t.Accent.Alpha(0.16)).Border(1, t.Accent)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
}

// editStack is a card of the list to click into the core or out, with how many of the leader's
// games ran it. Staples are faded, since they can't tell builds apart.
func (v *view) editStack(c *ui.Context, e *buildEdit, code string, n, in, total int, staple bool) {
	core := slices.Contains(e.core, code)
	h := float32(stackW * 7 / 5)
	b := ui.ButtonBase(c).Label(v.snap.card(code).Name).Column().Gap(4).Padding(4)
	toggleLook(c, b, core)
	if b.Clicked() {
		e.toggle(code)
	}
	hovered := b.Hovered()
	b.Children(func() {
		art := ui.Box(c).Size(stackW+float32(n-1)*stackStepX, h+float32(n-1)*stackStepY)
		if staple && !core && !hovered {
			art.Opacity(0.7)
		}
		art.Children(func() {
			for i := range n {
				v.cardArt(c.Key(i), code, stackW, h).Absolute().Left(float32(i) * stackStepX).Top(float32(i) * stackStepY)
			}
			pill(c, fmt.Sprintf("×%d", n), 11).Absolute().Right(4).Bottom(4)
		})
		freqBar(c, in, total, stackW+float32(n-1)*stackStepX)
		coreMark(c, core, hovered, staple)
	})
}

// freqBar is how many of the games ran a card, as a bar and "N of M".
func freqBar(c *ui.Context, in, total int, w float32) {
	t := c.Theme()
	ui.Column(c).Gap(2).Width(w).Children(func() {
		ui.Box(c).Size(w, 4).Radius(2).Background(t.Surface).Children(func() {
			ui.Box(c).Size(w*float32(in)/float32(max(total, 1)), 4).Radius(2).Background(t.TextMuted)
		})
		ui.Text(c, fmt.Sprintf("%d of %d", in, total)).FontSize(11).TextColor(faintColor(t))
	})
}

// otherCards are the cards of the leader's other lists, most played first, to search and click
// into the core as the stacks are.
func (v *view) otherCards(c *ui.Context, e *buildEdit, base, freq map[string]int, total int) {
	t := c.Theme()
	var codes []string
	for code := range maps.Keys(freq) {
		if base[code] == 0 {
			codes = append(codes, code)
		}
	}
	if len(codes) == 0 {
		return
	}
	slices.SortFunc(codes, func(a, b string) int { return cmp.Or(cmp.Compare(freq[b], freq[a]), cmp.Compare(a, b)) })
	q := strings.ToLower(strings.TrimSpace(e.search))
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Cards from your other "+v.snap.card(e.build.Leader).Name+" lists").FontWeight(600)
		ui.TextInput(c, &e.search).Width(240).Label("Search cards").Placeholder("Search cards")
	})
	ui.Row(c).Wrap().Gap(6).Children(func() {
		for _, code := range codes {
			card := v.snap.card(code)
			if q != "" && !strings.Contains(strings.ToLower(card.Name), q) && !strings.Contains(strings.ToLower(code), q) {
				continue
			}
			core := slices.Contains(e.core, code)
			b := ui.ButtonBase(c.Key(code)).Label(card.Name).Row().Width(330).Gap(10).AlignItems(ui.Center).Padding(4, 8, 4, 4)
			toggleLook(c, b, core)
			if b.Clicked() {
				e.toggle(code)
			}
			hovered := b.Hovered()
			b.Children(func() {
				v.cardArt(c, code, 24, 34)
				ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, card.Name).FontSize(13).SingleLine()
					ui.Text(c, code).FontSize(11).TextColor(faintColor(t))
				})
				freqBar(c, freq[code], total, 80)
				ui.Box(c).Width(44).Children(func() { coreMark(c, core, hovered, false) })
			})
		}
	})
}
