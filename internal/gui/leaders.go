package gui

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// leaderCache keeps the Decks page's stats until the scope or the leader changes.
type leaderCache struct {
	snap    *snapshot
	filters filters
	leader  string
	stats   stats.Leader
	// turnOrder ignores the turn order filter, so the tile's other half still shows its games.
	turnOrder stats.TurnOrder
	// games are the leader's games, oldest first, and days its days played, latest first.
	games []stats.Match
	days  []stats.Day
	decks leaderDecks
}

// leaderData is the leader the deck filter names, or the most played one in scope, and every
// leader in scope to choose from.
func (v *view) leaderData() (stats.Leader, []stats.LeaderRecord) {
	sc := v.scoped()
	all := sc.decks
	leader := v.filter.deck
	if leader == "" && len(all) > 0 {
		leader = all[0].Leader
	}
	lc := &v.leaderStats
	if lc.snap != v.snap || lc.filters != v.filter || lc.leader != leader || lc.stats.Leader == "" {
		lc.snap, lc.filters, lc.leader = v.snap, v.filter, leader
		lc.stats = stats.ForLeader(sc.anyDeck, leader)
		lc.turnOrder = stats.ForLeader(sc.anyTurn, leader).TurnOrder
		lc.games = lc.games[:0]
		var newest []stats.Match
		for _, m := range sc.anyDeck {
			if m.Me.Leader == leader {
				newest = append(newest, m)
			}
		}
		for i := len(newest) - 1; i >= 0; i-- {
			lc.games = append(lc.games, newest[i])
		}
		lc.days = stats.BuildOverview(newest, v.snap.Now).Days
		lc.decks = v.deckData(leader, lc.games)
	}
	return lc.stats, all
}

func (v *view) decksPage(c *ui.Context) {
	l, all := v.leaderData()
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			d := v.leaderStats.decks
			v.chooseBuild(d)
			if v.decks.edit != nil {
				v.coreEditor(c, d)
				return
			}
			v.leaderPicker(c, all, l.Leader)
			v.leaderHead(c, l)
			v.buildRow(c, d)
			v.pageTabs(c)
			switch v.decks.tab {
			case tabList:
				v.listTab(c, d)
				return
			case tabCards:
				v.cardsTab(c, d)
				return
			}
			ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Gap(16).Children(func() {
					v.rollingChart(c, l.Rolling, v.leaderStats.games, &v.leaderPoint)
					v.leaderDays(c)
				})
				ui.Column(c).Width(320).Gap(16).Children(func() {
					turnOrderTile(c, v.leaderStats.turnOrder, v.filter.turn, v.setTurn)
					v.matchupLists(c, l)
				})
			})
		})
	})
}

// leaderPicker is a tab for each leader played, with its art, set, games and rate, which opens
// it on the Decks page.
func (v *view) leaderPicker(c *ui.Context, all []stats.LeaderRecord, chosen string) {
	v.deckTabs(c, all, chosen, "Show ", v.openLeader)
}

// deckTabs is a tab for each deck, calling pick with the one clicked. label starts each tab's
// name for assistive technology.
func (v *view) deckTabs(c *ui.Context, all []stats.LeaderRecord, chosen, label string, pick func(string)) {
	t := c.Theme()
	ui.Row(c).Gap(8).Wrap().Children(func() {
		for i, lr := range all {
			name := v.snap.card(lr.Leader).Name
			on := lr.Leader == chosen
			tab := ui.Row(c.Key(i)).Gap(10).AlignItems(ui.Center).Padding(6, 12, 6, 6).
				Border(1, t.Border.Alpha(0.6)).Label(label + name).Transition(ui.ElementTransition{Duration: hoverTime, Colors: true})
			if on {
				tab = tab.Background(t.Surface).Border(1, t.Border)
			} else if tab.Hovered() {
				tab = tab.Background(t.Surface.Alpha(0.5))
			}
			lift(tab, 10)
			if tab.Clicked() {
				pick(lr.Leader)
			}
			tab.Children(func() {
				v.artStrip(c, lr.Leader, 48, 34)
				ui.Column(c).Gap(1).Children(func() {
					nameText := ui.Text(c, name).FontSize(13).FontWeight(600).SingleLine()
					if !on {
						nameText.TextColor(t.TextMuted)
					}
					meta := fmt.Sprintf("%s · %s", setOf(lr.Leader), games(lr.Record.Games()))
					if rate, ok := lr.Record.Rate(); ok {
						meta += " · " + pct(rate)
					}
					ui.Text(c, meta).FontSize(12).TextColor(faintColor(t))
				})
			})
		}
	})
}

// leaderHead is the leader's banner with its win rate and the rate over its latest games.
func (v *view) leaderHead(c *ui.Context, l stats.Leader) {
	t := c.Theme()
	since := ""
	if !l.First.IsZero() {
		since = " · since " + l.First.Format("Jan 2")
	}
	v.banner(c, l.Leader, 150, func() {
		ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
			ui.Text(c, v.snap.card(l.Leader).Name).FontSize(22).Bold()
			ui.Text(c, setOf(l.Leader)+since).FontSize(13).TextColor(t.TextMuted).Margin(0, 0, 3, 0)
		})
		ui.Row(c).Gap(32).Margin(12, 0, 0, 0).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Win rate").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, rateOrTooFew(l.Record)).FontSize(28).Bold()
				ui.Text(c, fmt.Sprintf("%s · %s", record(l.Record), games(l.Record.Games()))).FontSize(12).TextColor(t.TextMuted)
			})
			if len(l.Rolling) == 0 {
				return
			}
			last := l.Rolling[len(l.Rolling)-1]
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, fmt.Sprintf("Last %d", stats.RollingWindow)).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, pct(last.Rate)).FontSize(28).Bold()
				overall, _ := l.Record.Rate()
				d := math.Round((last.Rate - overall) * 100)
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					switch {
					case d > 0:
						ui.Icon(c, iconTrendUp).FontSize(12).TextColor(t.TextMuted).Label("Up")
						ui.Text(c, fmt.Sprintf("%.0f pts above overall", d)).FontSize(12).TextColor(t.TextMuted)
					case d < 0:
						ui.Icon(c, iconTrendDown).FontSize(12).TextColor(t.TextMuted).Label("Down")
						ui.Text(c, fmt.Sprintf("%.0f pts below overall", -d)).FontSize(12).TextColor(t.TextMuted)
					default:
						ui.Text(c, "same as overall").FontSize(12).TextColor(t.TextMuted)
					}
				})
			})
		})
	})
}

// leaderDays is the leader's latest days played, like the Overview's.
func (v *view) leaderDays(c *ui.Context) {
	days := v.leaderStats.days
	tile(c, "By day", func() {
		for i, d := range days[:min(len(days), 5)] {
			v.dayRow(c.Key(i), d)
		}
	})
}

// matchupLists are the best and worst matchups with enough games, split at 50%, then the even
// ones and a count of the rest, which the Matchups page lists in full.
func (v *view) matchupLists(c *ui.Context, l stats.Leader) {
	t := c.Theme()
	var best, worst, even, few []stats.LeaderRecord
	for _, mu := range stats.SortMatchups(l.Matchups, stats.BestFirst) {
		rate, ok := mu.Record.Rate()
		switch {
		case !ok:
			few = append(few, mu)
		case rate > 0.5:
			best = append(best, mu)
		case rate < 0.5:
			worst = append(worst, mu)
		default:
			even = append(even, mu)
		}
	}
	slices.Reverse(worst)
	list := func(title string, mus []stats.LeaderRecord) {
		if len(mus) == 0 {
			return
		}
		tile(c, title, func() {
			for i, mu := range mus[:min(len(mus), 3)] {
				v.matchupLine(c.Key(i), mu)
			}
		})
	}
	list("Best matchups", best)
	list("Worst matchups", worst)
	if len(best)+len(worst)+len(even) == 0 {
		tile(c, "Matchups", func() {
			ui.Text(c, fmt.Sprintf("No leader faced %d or more times yet.", stats.MinGames)).FontSize(13).TextColor(t.TextMuted)
		})
	}
	if len(even) > 0 {
		names := make([]string, len(even))
		for i, mu := range even {
			names[i] = fmt.Sprintf("%s %s", shortName(v.snap.card(mu.Leader).Name), record(mu.Record))
		}
		titledTile(c, "Even", strings.Join(names, " · "), func() {})
	}
	if len(few) > 0 {
		el := ui.Row(c).Label("Show every matchup")
		if el.Clicked() {
			v.page = pageMatchups
		}
		el.Children(func() {
			titledTile(c, "Rarely faced", fmt.Sprintf("%d under %d games", len(few), stats.MinGames), func() {}).Grow(1)
		})
	}
}

// matchupLine is one opposing leader with its record as a split bar, the record and the rate.
func (v *view) matchupLine(c *ui.Context, mu stats.LeaderRecord) {
	t := c.Theme()
	name := v.snap.card(mu.Leader).Name
	el := ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(3, 6).Margin(0, -6).Radius(8).Label("Show games vs " + name)
	if el.Hovered() {
		el.Background(t.Surface)
	}
	if el.Clicked() {
		v.filter.oppLeader = mu.Leader
		v.page = pageMatches
	}
	el.Children(func() {
		v.artStrip(c, mu.Leader, 42, 30)
		ui.Text(c, shortName(name)).FontWeight(500).SingleLine().Grow(1).Basis(0)
		ui.Box(c).Width(40).Children(func() { splitBar(c, winShare(mu.Record)) })
		ui.Text(c, record(mu.Record)).Bold().Width(40).TextAlign(ui.End)
		ui.Text(c, rateOrTooFew(mu.Record)).FontSize(12).TextColor(t.TextMuted).Width(34).TextAlign(ui.End)
	})
}

// shortName drops the "Monkey D." and like from a leader's name, where a column is narrow.
func shortName(name string) string {
	if i := strings.LastIndex(name, " "); i > 0 && len(name) > 12 {
		return name[i+1:]
	}
	return name
}

// setOf is a card code's set, "OP13" for OP13-001, which tells leaders with one name apart.
func setOf(code string) string {
	set, _, _ := strings.Cut(code, "-")
	return set
}
