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
	s.tab, s.renaming, s.deleting, s.suggesting = tabList, false, false, false
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

// coreEditor edits a build's core on its latest list, then the cards of the leader's other lists,
// with a preview of the games it gets.
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
	panel(c).Gap(14).Padding(16).Children(func() {
		v.editorHeader(c, d, e, p)
		gameStrip(c, p)
		ui.Text(c, "Click a card to add it to the core or take it out. A game is in the build when its list runs "+
			"every core card. Cards in nearly every list can't tell builds apart, so they're dimmed.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
		ui.Row(c).Gap(10).Wrap().AlignItems(ui.Start).Children(func() {
			for _, code := range v.listOrder(base) {
				v.editStack(c.Key(code), e, code, base[code], freq[code], len(p.all), slices.Contains(staples, code))
			}
		})
		v.otherCards(c, e, base, freq, len(p.all))
	})
}

func (v *view) editorHeader(c *ui.Context, d leaderDecks, e *buildEdit, p editPreview) {
	t := c.Theme()
	ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			title := "New " + v.snap.card(d.leader).Name + " build"
			if e.build.ID != 0 {
				title = "Editing " + e.build.Name + "'s core"
			}
			ui.Text(c, title).FontWeight(600)
			ui.TextInput(c, &e.name).Width(260).Label("Build name").Placeholder("Name")
		})
		ui.Column(c).Gap(2).Padding(6, 12).Radius(8).Background(t.Surface).Children(func() {
			ui.Text(c, fmt.Sprintf("%s → %s · %s → %s", games(len(p.now)), games(len(p.after)), record(p.before), record(p.with))).
				FontSize(13).FontWeight(600)
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
			note := fmt.Sprintf("+%d joining, −%d leaving", joins, leaves)
			if p.conflicts > 0 {
				note += fmt.Sprintf(", %s in two builds", games(p.conflicts))
			}
			ui.Text(c, note).FontSize(12).TextColor(t.TextMuted)
		})
		ui.Spacer(c)
		if ui.Button(c, "Cancel").Clicked() {
			v.decks.edit = nil
		}
		name := strings.TrimSpace(e.name)
		save := ui.PrimaryButton(c, "Save").Disabled(name == "" || len(e.core) == 0)
		if save.Clicked() && name != "" && len(e.core) > 0 {
			b := e.build
			b.Name, b.Core = name, slices.Clone(e.core)
			v.do(func(a actions) { a.SaveBuild(b) })
			v.decks.saved, v.decks.edit = name, nil
		}
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
				switch fp := g.Fingerprint; {
				case p.after[fp] && p.now[fp]:
					mark.Background(t.Accent)
				case p.after[fp]:
					mark.Background(t.Accent.Alpha(0.35)).Border(1, t.Accent)
				case p.now[fp]:
					mark.Background(lossColor.Alpha(0.35)).Border(1, lossColor)
				case p.others[fp]:
					mark.Background(warnColor)
				default:
					mark.Background(t.Surface)
				}
			}
		})
		ui.Row(c).Gap(14).Children(func() {
			legend := func(label string, fill, edge ui.Color) {
				ui.Row(c.Key(label)).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(8, 12).Radius(2).Background(fill).Border(1, edge)
					ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
				})
			}
			legend("in this build", t.Accent, t.Accent)
			legend("joins", t.Accent.Alpha(0.35), t.Accent)
			legend("leaves", lossColor.Alpha(0.35), lossColor)
			legend("in another build", warnColor, warnColor)
		})
	})
}

func (e *buildEdit) toggle(code string) {
	if i := slices.Index(e.core, code); i >= 0 {
		e.core = slices.Delete(e.core, i, i+1)
		return
	}
	e.core = append(e.core, code)
	slices.Sort(e.core)
}

// editStack is a card of the list to click into the core or out, with how many of the leader's
// games ran it.
func (v *view) editStack(c *ui.Context, e *buildEdit, code string, n, in, total int, staple bool) {
	t := c.Theme()
	core := slices.Contains(e.core, code)
	h := float32(stackW * 7 / 5)
	b := ui.ButtonBase(c).Label(v.snap.card(code).Name).Column().Gap(4).Padding(4).Radius(8).Background(ui.Transparent)
	switch {
	case core:
		b.Background(t.Accent.Alpha(0.16)).Border(1, t.Accent)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	if staple && !core {
		b.Opacity(0.45)
	}
	if b.Clicked() {
		e.toggle(code)
	}
	b.Children(func() {
		ui.Box(c).Size(stackW+float32(n-1)*stackStepX, h+float32(n-1)*stackStepY).Children(func() {
			for i := range n {
				v.cardArt(c.Key(i), code, stackW, h).Absolute().Left(float32(i) * stackStepX).Top(float32(i) * stackStepY)
			}
			pill(c, fmt.Sprintf("×%d", n), 11).Absolute().Right(4).Bottom(4)
		})
		freqBar(c, in, total, stackW+float32(n-1)*stackStepX)
		label, col := "+ Core", faintColor(t)
		if core {
			label, col = "Core", t.Accent
		}
		ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(col)
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

// otherCards are the cards of the leader's other lists, most played first, to search and tick
// into the core.
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
	ui.Row(c).Wrap().Gap(4).Children(func() {
		for _, code := range codes {
			card := v.snap.card(code)
			if q != "" && !strings.Contains(strings.ToLower(card.Name), q) && !strings.Contains(strings.ToLower(code), q) {
				continue
			}
			on := slices.Contains(e.core, code)
			ui.Row(c.Key(code)).Width(330).Gap(10).AlignItems(ui.Center).Padding(4, 6).Children(func() {
				if ui.Checkbox(c, &on, "").Label(card.Name).Changed() {
					e.toggle(code)
				}
				v.cardArt(c, code, 24, 34)
				ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, card.Name).FontSize(13).SingleLine()
					ui.Text(c, code).FontSize(11).TextColor(faintColor(t))
				})
				freqBar(c, freq[code], total, 90)
			})
		}
	})
}
