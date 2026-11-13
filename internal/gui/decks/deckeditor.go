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

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

// buildEdit is a build's core and name being edited. A new build has ID 0.
type buildEdit struct {
	build  store.Build
	core   []string
	name   string
	search string
	// shown is the preview for the leader and core in shownFor, over shownSnap's games.
	shown     editPreview
	shownFor  string
	shownSnap *page.Snapshot
}

func (pg *Page) startEdit(b store.Build) {
	s := pg
	s.Edit = &buildEdit{build: b, core: slices.Clone(b.Core), name: b.Name}
	s.Tab, s.renaming, s.suggesting = tabList, false, false
}

// editPreview is where the leader's games go now and with the edited core. base is the list the
// editor lays out: the build's latest, or the leader's.
type editPreview struct {
	stats.BuildEdit
	all  []stats.DeckGame
	base map[string]int
}

// preview is made again only when the leader, the core or the games change.
func preview(e *page.Env, d leaderDecks, ed *buildEdit) editPreview {
	key := d.leader + " " + strings.Join(ed.core, ",")
	if ed.shownSnap == e.Snap && ed.shownFor == key {
		return ed.shown
	}
	edited := ed.build
	edited.Core = ed.core
	p := editPreview{BuildEdit: stats.EditBuild(d.all, d.builds, edited), all: d.all}
	if len(p.all) > 0 {
		p.base = p.all[0].Facts.List
	}
	for _, g := range p.all {
		if p.Now[g.Fingerprint] {
			p.base = g.Facts.List
			break
		}
	}
	ed.shown, ed.shownFor, ed.shownSnap = p, key, e.Snap
	return p
}

// coreEditor takes the page while a build's core is edited: the cards of its latest list, then
// the cards of the leader's other lists, with the games it gets as they change.
func (pg *Page) coreEditor(c *ui.Context, e *page.Env, d leaderDecks) {
	t := c.Theme()
	ed := pg.Edit
	p := preview(e, d, ed)
	if len(p.all) == 0 {
		pg.Edit = nil
		return
	}
	freq, base := p.Games, p.base
	root := kit.Panel(c).Gap(16).Padding(16, 20, 20, 20)
	if root.OverlayShortcut(0, ui.KeyEscape) {
		pg.Edit = nil
		return
	}
	root.Children(func() {
		pg.editorHeader(c, e, d, ed)
		editorPreview(c, e, ed, p)
		ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
		ui.Column(c).Gap(12).Children(func() {
			ui.Text(c, "Latest list").FontWeight(600)
			ui.Row(c).Gap(10).Wrap().AlignItems(ui.Start).Children(func() {
				for _, code := range listOrder(e, base) {
					editStack(c.Key(code), e, ed, code, base[code], freq[code], len(p.all), slices.Contains(p.Staples, code))
				}
			})
		})
		otherCards(c, e, ed, base, freq, len(p.all))
	})
}

func (pg *Page) editorHeader(c *ui.Context, e *page.Env, d leaderDecks, ed *buildEdit) {
	t := c.Theme()
	leader := e.Snap.Card(d.leader).Name
	ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
		e.ArtStrip(c, d.leader, 48, 34)
		ui.Column(c).Gap(2).Children(func() {
			title := "New " + leader + " build"
			if ed.build.ID != 0 {
				title = "Edit " + ed.build.Name
			}
			ui.Text(c, title).FontSize(18).FontWeight(600)
			ui.Text(c, "A game is in the build when its list runs every core card.").FontSize(12).TextColor(t.TextMuted)
		})
		ui.Spacer(c)
		name := strings.TrimSpace(ed.name)
		var missing string
		switch {
		case name == "" && len(ed.core) == 0:
			missing = "Name it and pick a core card"
		case name == "":
			missing = "Name it to save"
		case len(ed.core) == 0:
			missing = "Pick a core card to save"
		}
		if missing != "" {
			ui.Text(c, missing).FontSize(12).TextColor(kit.FaintColor(t))
		}
		if ui.Button(c, "Cancel").Tooltip("Esc").Clicked() {
			pg.Edit = nil
		}
		save := ui.PrimaryButton(c, "Save").Disabled(missing != "")
		if save.Clicked() && missing == "" {
			b := ed.build
			b.Name, b.Core = name, slices.Clone(ed.core)
			pg.do(func(a Actions) { a.SaveBuild(b, pg.chooseSaved) })
			pg.Edit = nil
		}
	})
	if ed.build.ID == 0 {
		ui.TextInput(c, &ed.name).Width(280).Label("Build name").Placeholder("Build name").AutoFocus()
	}
}

// editorPreview is the games the core gets, and how that differs from the saved core once it
// does.
func editorPreview(c *ui.Context, e *page.Env, ed *buildEdit, p editPreview) {
	t := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.End).Children(func() {
			joins, leaves := 0, 0
			for fp := range p.After {
				if !p.Now[fp] {
					joins++
				}
			}
			for fp := range p.Now {
				if !p.After[fp] {
					leaves++
				}
			}
			switch {
			case len(ed.core) == 0:
				ui.Text(c, "Pick a core card to see its games").FontSize(14).FontWeight(600).TextColor(t.TextMuted)
			case ed.build.ID == 0 || joins+leaves == 0:
				ui.Text(c, kit.Games(len(p.After))+" · "+kit.Record(p.With)).FontSize(14).FontWeight(600)
			default:
				ui.Text(c, fmt.Sprintf("%s → %s · %s → %s", kit.Games(len(p.Now)), kit.Games(len(p.After)), kit.Record(p.Before), kit.Record(p.With))).
					FontSize(14).FontWeight(600)
				ui.Text(c, fmt.Sprintf("+%d joining, −%d leaving", joins, leaves)).FontSize(12).TextColor(t.TextMuted)
			}
			if p.Conflicts > 0 {
				ui.Text(c, kit.Games(p.Conflicts)+" fit two builds").FontSize(12).TextColor(kit.WarnColor)
			}
		})
		gameStrip(c, e, p)
	})
}

// gameStrip is a mark per game of the leader, oldest first, with a gap between days: in the
// build, joining or leaving it with the edit, in another build, or in none.
func gameStrip(c *ui.Context, e *page.Env, p editPreview) {
	t := c.Theme()
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Gap(2).Wrap().AlignItems(ui.Center).Children(func() {
			last := ""
			for i, g := range slices.Backward(p.all) {
				day := g.PlayedAt.Format("Jan 2")
				if last != "" && day != last {
					ui.Box(c.Key(fmt.Sprint("gap", i))).Width(5)
				}
				last = day
				mark := ui.Box(c.Key(i)).Size(8, 14).Radius(2).Tooltip(e.DateTime(g.PlayedAt))
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
			legend("leaves", ui.Transparent, kit.LossColor(t))
			legend("in another build", kit.WarnColor, kit.WarnColor)
			legend("in no build", t.Border, t.Border)
		})
	})
}

func stripMark(t *ui.Theme, p editPreview, fp string) (fill, edge ui.Color) {
	switch {
	case p.After[fp] && p.Now[fp]:
		return t.Accent, t.Accent
	case p.After[fp]:
		return ui.Transparent, t.Accent
	case p.Now[fp]:
		return ui.Transparent, kit.LossColor(t)
	case p.Others[fp]:
		return kit.WarnColor, kit.WarnColor
	}
	return t.Border, t.Border
}

func (ed *buildEdit) toggle(code string) {
	if i := slices.Index(ed.core, code); i >= 0 {
		ed.core = slices.Delete(ed.core, i, i+1)
		return
	}
	ed.core = append(ed.core, code)
	slices.Sort(ed.core)
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
		label, col = "Staple", kit.FaintColor(t)
	default:
		col = ui.Transparent
	}
	ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(col)
}

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
func editStack(c *ui.Context, e *page.Env, ed *buildEdit, code string, n, in, total int, staple bool) {
	core := slices.Contains(ed.core, code)
	h := page.CardHeight(stackW)
	b := ui.ButtonBase(c).Label(e.Snap.Card(code).Name).Column().Gap(4).Padding(4)
	toggleLook(c, b, core)
	if b.Clicked() {
		ed.toggle(code)
	}
	hovered := b.Hovered()
	b.Children(func() {
		art := ui.Box(c).Size(stackW+float32(n-1)*stackStepX, h+float32(n-1)*stackStepY)
		if staple && !core && !hovered {
			art.Opacity(0.7)
		}
		art.Children(func() {
			for i := range n {
				e.Card(c.Key(i), code, stackW).Absolute().Left(float32(i) * stackStepX).Top(float32(i) * stackStepY)
			}
			matches.Pill(c, fmt.Sprintf("×%d", n), 11).Absolute().Right(4).Bottom(4)
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
		ui.Text(c, fmt.Sprintf("%d of %d", in, total)).FontSize(11).TextColor(kit.FaintColor(t))
	})
}

// otherCards are the cards of the leader's other lists, most played first, to search and click
// into the core as the stacks are.
func otherCards(c *ui.Context, e *page.Env, ed *buildEdit, base, freq map[string]int, total int) {
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
	q := strings.ToLower(strings.TrimSpace(ed.search))
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Cards from your other "+e.Snap.Card(ed.build.Leader).Name+" lists").FontWeight(600)
		ui.TextInput(c, &ed.search).Width(240).Label("Search cards").Placeholder("Search cards")
	})
	ui.Row(c).Wrap().Gap(6).Children(func() {
		for _, code := range codes {
			card := e.Snap.Card(code)
			if q != "" && !strings.Contains(strings.ToLower(card.Name), q) && !strings.Contains(strings.ToLower(code), q) {
				continue
			}
			core := slices.Contains(ed.core, code)
			b := ui.ButtonBase(c.Key(code)).Label(card.Name).Row().Width(330).Gap(10).AlignItems(ui.Center).Padding(4, 8, 4, 4)
			toggleLook(c, b, core)
			if b.Clicked() {
				ed.toggle(code)
			}
			hovered := b.Hovered()
			b.Children(func() {
				e.Card(c, code, 24)
				ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, card.Name).FontSize(13).SingleLine()
					ui.Text(c, code).FontSize(11).TextColor(kit.FaintColor(t))
				})
				freqBar(c, freq[code], total, 80)
				ui.Box(c).Width(44).Children(func() { coreMark(c, core, hovered, false) })
			})
		}
	})
}
