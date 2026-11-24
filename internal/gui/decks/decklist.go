package decks

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/deck"
	"github.com/VTGare/logpose/internal/gui/kit"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
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
		pg.versionRail(c, e, d.leader, changes, i)
		kit.Panel(c).Grow(1).Basis(0).MinWidth(0).Gap(14).Padding(16).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				title, artTitle := listTitle(e, ch, i == len(changes)-1)
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, title).FontWeight(600).SingleLine()
					ui.Text(c, kit.Games(len(ch.Games))+" · "+kit.Record(stats.RecordOf(ch.Games))).FontSize(12.5).TextColor(t.TextMuted).SingleLine()
				})
				ui.Spacer(c)
				if kit.IconButton(c, kit.IconArt, "Customize art").Clicked() {
					pg.openDeckArt(d.leader, artTitle, list)
				}
				if kit.IconButton(c, kit.IconCopy, "Copy for sim").Clicked() {
					page.CopyDeckForSim(e, d.leader, e.Snap.Card(d.leader).Name, list)
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					on := e.Prefs.DeckChanges
					if ui.Switch(c, &on).Label("Show changes").Changed() {
						page.PrefDeckChanges.Set(e, on)
					}
					ui.Text(c, "Show changes").FontSize(13).TextColor(t.TextMuted).SingleLine()
				})
			})
			if note := page.DeckPartialNote(deck.Count(list)); note != "" {
				page.DeckPartialBanner(c, note)
			}
			var marks map[string]int
			if e.Prefs.DeckChanges && i > 0 {
				marks = ch.Swaps
			}
			page.DeckStacks(c, e, d.leader, list, marks)
			ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
			page.DeckListStatsGrid(c, d.lists[i])
		})
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

func listTitle(e *page.Env, ch stats.Change, latest bool) (heading, short string) {
	when := e.Date(ch.At) + ", " + ch.At.Format("15:04")
	if latest {
		return "Latest list · " + when, "Latest list"
	}
	return "List from " + when, when
}
