package decks

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
)

// leaderCache keeps the Decks page's numbers. The split into builds lasts until the snapshot
// or the leader changes. The games in scope also go when the filters change, and the scope's
// numbers when the scope does.
type leaderCache struct {
	snap  *page.Snapshot
	gen   int
	decks leaderDecks
	shown *deckData
}

// deckData is what the Decks tabs show: the leader's games split into builds, and the numbers
// of the games the chosen scope counts.
type deckData struct {
	leaderDecks
	scope deckScope
	// tag names the scope in panel titles, and is empty for a leader without builds, where every
	// game counts anyway. name names the games in messages, the leader's when tag is empty.
	tag, name string
	stats     stats.Leader
	// games are the scope's games, days its days played and deckGames the games with deck
	// facts, latest first.
	games     []stats.Match
	days      []stats.Day
	deckGames []stats.DeckGame
	mulligans stats.Mulligans
	cards     []stats.CardStats
	// changes are the lists the games ran, oldest first, and lists the stats of each.
	changes []stats.Change
	lists   []stats.ListStats
}

// leaderDecks is the leader the deck filter names, or the most played one in scope, with its
// games split into builds.
func (pg *Page) leaderDecks(e *page.Env) leaderDecks {
	lc := &pg.leaderStats
	sc := e.Scoped()
	if lc.gen == sc.Gen {
		return lc.decks
	}
	leader := e.Filter.Deck
	if leader == "" && len(sc.Decks) > 0 {
		leader = sc.Decks[0].Leader
	}
	var split *leaderDecks
	if lc.snap == e.Snap && lc.decks.leader == leader {
		split = &lc.decks
	}
	*lc = leaderCache{snap: e.Snap, gen: sc.Gen, decks: splitLeader(e, leader, split)}
	return lc.decks
}

func (pg *Page) deckData(e *page.Env, ld leaderDecks) *deckData {
	lc := &pg.leaderStats
	sc := pg.scope
	if lc.shown != nil && lc.shown.scope == sc {
		return lc.shown
	}
	d := &deckData{leaderDecks: ld, scope: sc, tag: ld.scopeName(sc)}
	d.name = d.tag
	if d.name == "" {
		d.name = e.Snap.Card(ld.leader).Name
	}
	d.stats = stats.ForLeader(ld.scopedMatches(sc, ld.anyTurn), ld.leader, e.Filter.Turn)
	d.games = ld.scopedMatches(sc, ld.matches)
	d.days = stats.Days(d.games)
	d.mulligans = stats.ForMulligans(d.games)
	d.deckGames = ld.scopedGames(sc)
	if len(d.deckGames) > 0 {
		d.cards = stats.Cards(d.deckGames, e.Snap.Card)
		d.changes = stats.Changes(d.deckGames)
		for _, ch := range d.changes {
			d.lists = append(d.lists, stats.List(ch.Games[0].Facts.List, e.Snap.Card))
		}
	}
	lc.shown = d
	return d
}

func (pg *Page) Draw(c *ui.Context, e *page.Env) {
	ld := pg.leaderDecks(e)
	// Choosing a build changes the scope, so it comes before the scope's numbers.
	pg.chooseBuild(ld)
	d := pg.deckData(e, ld)
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			if pg.Edit != nil {
				pg.coreEditor(c, e, ld)
				return
			}
			e.DeckTabs(c, e.Scoped().Decks, ld.leader, "Show ", e.OpenLeader)
			leaderHead(c, e, d)
			pg.buildRow(c, e, ld)
			pg.pageTabs(c)
			DeckPageTabs[pg.Tab].Draw(pg, c, e, ld, d)
		})
	})
}

// deckTab is a tab of the Decks page. sub names it for -page, and Results, the default, has none.
type deckTab struct {
	label, Sub string
	Draw       func(pg *Page, c *ui.Context, e *page.Env, ld leaderDecks, d *deckData)
}

const (
	tabResults = iota
	tabList
	tabCards
	tabHands
)

var DeckPageTabs = [...]deckTab{
	tabResults: {label: "Results", Draw: (*Page).resultsTab},
	tabList:    {label: "List", Sub: "list", Draw: func(pg *Page, c *ui.Context, e *page.Env, _ leaderDecks, d *deckData) { pg.listTab(c, e, d) }},
	tabCards:   {label: "Cards", Sub: "cards", Draw: func(_ *Page, c *ui.Context, e *page.Env, _ leaderDecks, d *deckData) { cardsTab(c, e, d) }},
	tabHands:   {label: "Hands", Sub: "hands", Draw: func(pg *Page, c *ui.Context, e *page.Env, _ leaderDecks, d *deckData) { pg.handsTab(c, e, d) }},
}

func (pg *Page) resultsTab(c *ui.Context, e *page.Env, ld leaderDecks, d *deckData) {
	l := d.stats
	if d.scope.kind == scopeOverall && len(ld.builds) > 0 {
		pg.buildsTable(c, e, ld)
	}
	ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Grow(1).Basis(0).Gap(16).Children(func() {
			rollingChart(c, e, d.tag, l.Rolling, d.games, pg.LeaderPoint.at(e.Scoped().Gen, d.scope))
			leaderDays(c, e, d)
		})
		ui.Column(c).Width(320).Gap(16).Children(func() {
			o := l.TurnOrder
			sum := fmt.Sprintf("1st %s · 2nd %s", page.RateText(o.First), page.RateText(o.Second))
			e.FoldTile(c, "decks.turns", kit.FoldHead{Label: "Turn order", Tag: d.tag, Summary: sum}, func() {
				e.TurnRows(c, o, e.Filter.Turn, e.SetTurn)
			})
			matchupLists(c, e, d.tag, l)
		})
	})
}

// leaderHead is the leader's art and numbers. With a build or No build chosen, it names that
// over the leader and counts only its games.
func leaderHead(c *ui.Context, e *page.Env, d *deckData) {
	t := c.Theme()
	l := d.stats
	leader := e.Snap.Card(l.Leader).Name
	title, note := leader, cards.SetOf(l.Leader)
	if d.scope.kind != scopeOverall {
		title, note = d.tag, ""
	}
	if !l.First.IsZero() {
		note = strings.TrimPrefix(note+" · since "+e.Date(l.First), " · ")
	}
	height := float32(150)
	if title != leader {
		height = 166
	}
	e.Banner(c, l.Leader, height, func() {
		if title != leader {
			ui.Text(c, leader+" ›").FontSize(12).TextColor(t.TextMuted)
		}
		ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
			ui.Text(c, title).FontSize(22).Bold()
			ui.Text(c, note).FontSize(13).TextColor(t.TextMuted).Margin(0, 0, 3, 0)
		})
		ui.Row(c).Gap(32).Margin(12, 0, 0, 0).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Win rate").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, page.RateText(l.Record)).FontSize(28).Bold()
				ui.Text(c, fmt.Sprintf("%s · %s", kit.Record(l.Record), kit.Games(l.Record.Games()))).FontSize(12).TextColor(t.TextMuted)
			})
			if len(l.Rolling) == 0 {
				return
			}
			last := l.Rolling[len(l.Rolling)-1]
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, fmt.Sprintf("Last %d", stats.RollingWindow)).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, kit.Pct(last.Rate)).FontSize(28).Bold()
				overall, _ := l.Record.Rate()
				d := page.PointsApart(last.Rate, overall)
				text := "same as overall"
				switch {
				case d > 0:
					text = fmt.Sprintf("%.0f pts above overall", d)
				case d < 0:
					text = fmt.Sprintf("%.0f pts below overall", -d)
				}
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() { page.Trend(c, d, text) })
			})
		})
	})
}

// leaderDays is the leader's latest days played, like the Overview's.
func leaderDays(c *ui.Context, e *page.Env, d *deckData) {
	days := d.days
	sum := ""
	if len(days) > 0 {
		sum = fmt.Sprintf("Latest %s, %s", e.Date(days[0].Date), kit.Record(days[0].Record))
	}
	e.FoldTile(c, "decks.days", kit.FoldHead{Label: "By day", Tag: d.tag, Summary: sum}, func() {
		for i, day := range days[:min(len(days), 5)] {
			e.DayRow(c.Key(i), day)
		}
	})
}

// matchupLists are the best and worst matchups with enough games, split at 50%, then the even
// ones and a count of the rest, which the Matchups page lists in full.
func matchupLists(c *ui.Context, e *page.Env, tag string, l stats.Leader) {
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
	list := func(id, title string, mus []stats.LeaderRecord) {
		if len(mus) == 0 {
			return
		}
		e.FoldTile(c, id, kit.FoldHead{Label: title, Tag: tag}, func() {
			for i, mu := range mus[:min(len(mus), 3)] {
				facedLine(c.Key(i), e, mu)
			}
		})
	}
	list("decks.best", "Best matchups", best)
	list("decks.worst", "Worst matchups", worst)
	if len(best)+len(worst)+len(even) == 0 {
		kit.TagTile(c, "Matchups", tag, "", func() {
			ui.Text(c, fmt.Sprintf("No leader faced %d or more times yet.", stats.MinGames)).FontSize(13).TextColor(t.TextMuted)
		})
	}
	if len(even) > 0 {
		names := make([]string, len(even))
		for i, mu := range even {
			names[i] = fmt.Sprintf("%s %s", shortName(e.Snap.Card(mu.Leader).Name), kit.Record(mu.Record))
		}
		kit.TitledTile(c, "Even", strings.Join(names, " · "), func() {})
	}
	if len(few) > 0 {
		el := ui.Row(c).Label("Show every matchup")
		if el.Clicked() {
			e.Page = page.Matchups
		}
		el.Children(func() {
			kit.TitledTile(c, "Rarely faced", fmt.Sprintf("%d under %d games", len(few), stats.MinGames), func() {}).Grow(1)
		})
	}
}

// matchupLine is one opposing leader with its record as a split bar, the record and the rate.
func facedLine(c *ui.Context, e *page.Env, mu stats.LeaderRecord) {
	t := c.Theme()
	name := e.Snap.Card(mu.Leader).Name
	el := ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(3, 6).Margin(0, -6).Radius(8).Label("Show games vs " + name).
		PopoverMenu(e.OpponentMenu("", mu.Leader))
	if el.Hovered() {
		el.Background(t.Surface)
	}
	if el.Clicked() {
		e.ShowOpponentGames("", mu.Leader)
	}
	el.Children(func() {
		e.ArtStrip(c, mu.Leader, 42, 30)
		ui.Text(c, shortName(name)).FontWeight(500).SingleLine().Grow(1).Basis(0)
		ui.Box(c).Width(40).Children(func() { e.SplitBar(c, mu.Record.Share()) })
		ui.Text(c, kit.Record(mu.Record)).Bold().Width(40).TextAlign(ui.End)
		ui.Text(c, page.RateText(mu.Record)).FontSize(12).TextColor(t.TextMuted).Width(34).TextAlign(ui.End)
	})
}

// shortName drops the "Monkey D." and like from a leader's name, where a column is narrow.
func shortName(name string) string {
	if i := strings.LastIndex(name, " "); i > 0 && len(name) > 12 {
		return name[i+1:]
	}
	return name
}

// chartPick is the point under the pointer on a rolling chart. It holds only for the games and
// the deck scope it was picked in, since the same index is another game in another series.
type chartPick struct {
	I     int
	gen   int
	scope deckScope
}

// at is the pick for the games of scope generation gen in scope, -1 when either changed.
func (p *chartPick) at(gen int, scope deckScope) *int {
	if p.gen != gen || p.scope != scope {
		*p = chartPick{I: -1, gen: gen, scope: scope}
	}
	return &p.I
}

// rollingChart is the rolling win rate on an axis that fits it, with 50% marked, over every game
// as a win or loss square, grouped by day, and sel is the point under the pointer.
func rollingChart(c *ui.Context, e *page.Env, tag string, pts []stats.Point, newest []stats.Match, sel *int) {
	t := c.Theme()
	games := slices.Clone(newest)
	slices.Reverse(games)
	label := fmt.Sprintf("Win rate, last %d games", stats.RollingWindow)
	e.FoldTile(c, "decks.rolling", kit.FoldHead{Label: label, Tag: tag}, func() {
		if len(pts) < 2 || len(games) == 0 {
			ui.Text(c, fmt.Sprintf("Shows up after %d games.", stats.RollingWindow+1)).TextColor(t.TextMuted)
			return
		}
		n := len(games)
		gx := func(i int) float32 { return (float32(i) + 0.5) / float32(n) }
		lo := 1.0
		for _, p := range pts {
			lo = min(lo, p.Rate)
		}
		lo = max(0, math.Floor((lo-0.05)*5)/5)
		const height, band = 190, 26
		// yAt is a rate's height up the plot as a fraction, above the band of squares.
		yAt := func(rate float64) float32 {
			return (band + float32((rate-lo)/(1-lo))*(height-band-8)) / height
		}
		xs := make([]float32, len(pts))
		ys := make([]float32, len(pts))
		for i, p := range pts {
			xs[i], ys[i] = gx(p.Game-1), yAt(p.Rate)
		}
		var left []chart.Tick
		for r := lo; r <= 1.0001; r += 0.2 {
			left = append(left, chart.Tick{At: yAt(r), Label: kit.Pct(r)})
		}
		days := dayRuns(games)
		var bottom []chart.Tick
		for _, d := range days {
			at := (gx(d.start) + gx(d.end-1)) / 2
			// Labels closer than this would run into each other.
			if len(bottom) > 0 && at-bottom[len(bottom)-1].At < 0.09 {
				continue
			}
			bottom = append(bottom, chart.Tick{At: at, Label: d.date.Format("Jan 2")})
		}
		describe := func(i int) string {
			p := pts[i]
			return fmt.Sprintf("%s · game %d · %s, %d of the last %d won", e.DateTime(p.At), p.Game, kit.Pct(p.Rate), p.Wins, stats.RollingWindow)
		}
		chart.Plot(c, chart.Axes{Left: left, Bottom: bottom, Label: page.AxisLabel(c)}, func(area ui.Element) {
			drawn := e.Grow(area, "draw", 1, kit.DrawTime)
			area.Label(label).Draw(func(p *ui.Painter, r ui.Rect) {
				for k, d := range days {
					if k%2 == 1 {
						x0, x1 := r.X+r.W*float32(d.start)/float32(n), r.X+r.W*float32(d.end)/float32(n)
						p.Fill(ui.Rect{X: x0, Y: r.Y, W: x1 - x0, H: r.H}, ui.RGBA(255, 255, 255, 0.025), 0)
					}
				}
				for _, tick := range left {
					p.Line(r.X, chart.Y(r, tick.At), r.X+r.W, chart.Y(r, tick.At), 1, t.Border.Alpha(0.6))
				}
				if lo < 0.5 {
					y := chart.Y(r, yAt(0.5))
					for x := r.X; x < r.X+r.W; x += 8 {
						p.Line(x, y, min(x+4, r.X+r.W), y, 1, t.TextMuted)
					}
				}
				p.Clip(ui.Rect{X: r.X - 4, Y: r.Y - 4, W: (r.W + 8) * drawn, H: r.H + 8}, 0, func() {
					p.StrokePath(chart.Line(r, xs, ys, chart.CurveLinear), 2.25, kit.YouColor)
				})
				size := min(11, r.W/float32(n)-2)
				for i, m := range games {
					cx := r.X + r.W*gx(i)
					p.Fill(ui.Rect{X: cx - size/2, Y: r.Y + r.H - 4 - size, W: size, H: size}, kit.ResultColor(t, m.Result), 2)
				}
			})
			last := len(pts) - 1
			chart.Point(c, xs[last], ys[last], 8, 8).Radius(4).Background(kit.YouColor)
			chart.Anchor(c, xs[last], ys[last], ui.End, ui.End, func() {
				ui.Text(c, kit.Pct(pts[last].Rate)).FontSize(12).Bold().Margin(0, 6, 4, 0)
			})
			chart.Select(c, area, sel, xs, describe)
			if i := *sel; i >= 0 && i < len(pts) {
				chart.RuleX(c, xs[i], 1).Background(t.TextMuted)
				chart.Point(c, xs[i], ys[i], 10, 10).Radius(5).Background(kit.YouColor)
				align := ui.Center
				switch {
				case xs[i] < 0.2:
					align = ui.Start
				case xs[i] > 0.8:
					align = ui.End
				}
				chart.Anchor(c, xs[i], 1, align, ui.Start, func() {
					ui.Text(c, describe(i)).NoWrap().FontSize(12).Padding(4, 8).Radius(6).Background(t.Surface).Border(1, t.Border)
				})
			}
		}).Height(height).Gap(6).Margin(6, 0, 0, 0)
	})
}

// dayRun is the games of one day, as indexes into the games oldest first.
type dayRun struct {
	date       time.Time
	start, end int
}

func dayRuns(games []stats.Match) []dayRun {
	var out []dayRun
	for i, m := range games {
		day := stats.DayStart(m.PlayedAt)
		if len(out) == 0 || !out[len(out)-1].date.Equal(day) {
			out = append(out, dayRun{date: day, start: i})
		}
		out[len(out)-1].end = i + 1
	}
	return out
}

func (pg *Page) Open(e *page.Env, sub string) {
	switch sub {
	case "nobuild":
		pg.scope = deckScope{kind: scopeNoBuild}
	case "build", "edit":
		if d := pg.leaderDecks(e); len(d.builds) > 0 {
			pg.scope = buildScope(d.builds[0].ID)
			if sub == "edit" {
				pg.startEdit(d.builds[0])
			}
		}
	}
	for i, tab := range DeckPageTabs {
		if tab.Sub != "" && tab.Sub == sub {
			pg.Tab = i
		}
	}
}

// ShowsHands reports whether the tab shown is one where "Show hands with" fits a card.
func (pg *Page) ShowsHands() bool { return pg.Tab != tabResults }

// ShowHands opens the Hands tab on the hands the card was dealt in.
func (pg *Page) ShowHands(code string) {
	pg.Tab, pg.handsCard, pg.AllHands, pg.Edit = tabHands, code, false, nil
}
