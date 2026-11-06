package gui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// leaderCache keeps the Leaders page's stats until the scope or the leader changes.
type leaderCache struct {
	snap    *snapshot
	filters filters
	leader  string
	stats   stats.Leader
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
	}
	return lc.stats, all
}

func (v *view) leadersPage(c *ui.Context) {
	l, all := v.leaderData()
	label := "Leader"
	if l.Leader == v.snap.Overview.Deck {
		label = "Current deck"
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			ui.Text(c, "Leaders").FontSize(20).Bold()
			v.leaderPicker(c, all, l.Leader)
			v.leaderBanner(c, l.Leader, label, l.Record)
			ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
				turnOrderTile(c, l.TurnOrder).Grow(2).Basis(0)
				v.playedTile(c, l).Grow(1).Basis(0)
			})
			v.rollingChart(c, "Win rate", l.Rolling, &v.leaderPoint)
			v.matchupWall(c, l)
			v.leaderRecent(c, l)
		})
	})
}

// leaderPicker lists every leader played as a card to choose from.
func (v *view) leaderPicker(c *ui.Context, all []stats.LeaderRecord, chosen string) {
	t := c.Theme()
	ui.Row(c).Gap(12).Wrap().Children(func() {
		for i, lr := range all {
			name := v.snap.card(lr.Leader).Name
			card := ui.Column(c.Key(i)).Width(96).Gap(4).Padding(6).Radius(8).Label("Show " + name)
			if lr.Leader == chosen {
				card = card.Background(t.Surface).Border(2, t.Accent)
			}
			if card.Clicked() {
				v.openLeader(lr.Leader)
			}
			card.Children(func() {
				v.cardThumb(c, lr.Leader, 84)
				ui.Text(c, name).FontSize(12).Bold().SingleLine()
				ui.Text(c, fmt.Sprintf("%s · %s · %d", setOf(lr.Leader), rateOrTooFew(lr.Record), lr.Record.Games())).FontSize(11).TextColor(t.TextMuted)
			})
		}
	})
}

func (*view) playedTile(c *ui.Context, l stats.Leader) ui.Element {
	t := c.Theme()
	return tile(c, "Played", func() {
		if l.Record.Games() == 0 {
			ui.Text(c, "No games").TextColor(t.TextMuted)
			return
		}
		ui.Text(c, games(l.Record.Games())).FontSize(24).Bold()
		ui.Text(c, fmt.Sprintf("%s to %s", l.First.Format("Jan 2"), l.Last.Format("Jan 2"))).FontSize(12).TextColor(t.TextMuted)
	})
}

var matchupOrderLabels = []string{"Most played", "Best", "Worst"}

// matchupWall is the card wall of the leaders faced at least stats.MinGames times, with the
// rest as small chips under it, which the switch hides.
func (v *view) matchupWall(c *ui.Context, l stats.Leader) {
	t := c.Theme()
	sorted := stats.SortMatchups(l.Matchups, stats.MatchupOrder(v.matchupOrder))
	enough, few := stats.SplitBySample(sorted)
	tile(c, "Matchups", func() {
		ui.Row(c).Gap(16).Children(func() {
			ui.Segmented(c, &v.matchupOrder, matchupOrderLabels...).Label("Sort matchups")
			ui.Spacer(c)
			if ui.Text(c, fmt.Sprintf("Hide under %d games", stats.MinGames)).FontSize(12).TextColor(t.TextMuted).Clicked() {
				v.hideFew = !v.hideFew
			}
			ui.Switch(c, &v.hideFew).Label(fmt.Sprintf("Hide under %d games", stats.MinGames))
		})
		if len(enough) == 0 {
			ui.Text(c, fmt.Sprintf("No leader faced %d or more times yet.", stats.MinGames)).TextColor(t.TextMuted)
		}
		ui.Row(c).Gap(12).Wrap().Children(func() {
			for i, mu := range enough {
				ui.Column(c.Key(i)).Width(108).Gap(3).Children(func() {
					ui.Box(c).Size(108, 108*7/5).Children(func() {
						v.cardThumb(c, mu.Leader, 108)
						rateBadge(c, mu.Record).Absolute().Left(6).Right(6).Bottom(6)
					})
					ui.Text(c, v.snap.card(mu.Leader).Name).FontSize(12).Bold().SingleLine()
					ui.Text(c, setOf(mu.Leader)+" · "+record(mu.Record)).FontSize(11).TextColor(t.TextMuted)
				})
			}
		})
		if len(few) == 0 || v.hideFew {
			return
		}
		ui.Text(c, fmt.Sprintf("Also faced, under %d games", stats.MinGames)).FontSize(12).TextColor(t.TextMuted).Margin(6, 0, 0, 0)
		ui.Row(c).Gap(8).Wrap().Children(func() {
			for i, mu := range few {
				ui.Row(c.Key(i)).Gap(6).Padding(5, 10).Radius(6).Background(t.Background).Children(func() {
					v.leaderChip(c, mu.Leader, 10)
					ui.Text(c, v.snap.card(mu.Leader).Name).FontSize(12)
					ui.Text(c, setOf(mu.Leader)+" · "+record(mu.Record)).FontSize(11).TextColor(t.TextMuted)
				})
			}
		})
	})
}

// setOf is a card code's set, "OP13" for OP13-001, which tells leaders with one name apart.
func setOf(code string) string {
	set, _, _ := strings.Cut(code, "-")
	return set
}

// rateBadge is a win rate on the heatmap scale's color, or a grey game count under the minimum.
func rateBadge(c *ui.Context, r stats.Record) ui.Element {
	rate, ok := r.Rate()
	if !ok {
		t := c.Theme()
		return ui.Text(c, games(r.Games())).FontSize(11).TextAlign(ui.Center).Padding(3, 0).Radius(4).
			Background(t.Surface).TextColor(t.TextMuted)
	}
	bg, fg := heat(rate)
	return ui.Text(c, pct(rate)).FontSize(12).Bold().TextAlign(ui.Center).Padding(3, 0).Radius(4).Background(bg).TextColor(fg)
}

// heat is the matchup scale from ui.md: under 40%, 40–47%, 48–52%, 53–60% and over 60%.
func heat(rate float64) (bg, fg ui.Color) {
	switch {
	case rate < 0.40:
		return ui.Hex("#F7C1C1"), ui.Hex("#791F1F")
	case rate < 0.475:
		return ui.Hex("#FCEBEB"), ui.Hex("#791F1F")
	case rate <= 0.525:
		return ui.Hex("#E6E4DC"), ui.Hex("#2C2C2A")
	case rate <= 0.60:
		return ui.Hex("#EAF3DE"), ui.Hex("#27500A")
	default:
		return ui.Hex("#C0DD97"), ui.Hex("#27500A")
	}
}

func (v *view) leaderRecent(c *ui.Context, l stats.Leader) {
	t := c.Theme()
	tile(c, "Recent games", func() {
		for i, m := range l.Recent {
			row := ui.Row(c.Key(i)).Gap(8).Label("Open the match against " + m.Opp.Name)
			if row.Clicked() {
				v.openMatch(m)
			}
			row.Children(func() {
				ui.Text(c, resultLetter(m.Result)).Bold().TextColor(resultColor(m.Result)).Width(14)
				v.leaderChip(c, m.Opp.Leader, 12)
				ui.Text(c, v.snap.card(m.Opp.Leader).Name).SingleLine().Width(180)
				ui.Text(c, m.Opp.Name).SingleLine().Grow(1).TextColor(t.TextMuted)
				ui.Text(c, turnOrderShort(m)).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, m.PlayedAt.Format("Jan 2 15:04")).FontSize(12).TextColor(t.TextMuted).Width(90).TextAlign(ui.End)
			})
		}
	})
}
