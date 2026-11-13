package gui

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/stats"
)

type matchupsPage struct {
	// pick is the opposing leader shown in full, or "" for the most played one.
	pick string

	gen  int
	list []stats.Matchup
}

// matchupData is the deck the filter names, or the most played one in scope, and its matchups.
func (pg *matchupsPage) matchupData(e *env) (string, []stats.Matchup) {
	sc := e.scoped()
	deck := e.filter.deck
	if deck == "" && len(sc.decks) > 0 {
		deck = sc.decks[0].Leader
	}
	if pg.gen != sc.gen {
		pg.gen = sc.gen
		pg.list = stats.Matchups(sc.anyTurn, deck, e.filter.turn)
	}
	return deck, pg.list
}

// matchupNone is the pick when the user closed the open row.
const matchupNone = "-"

func (pg *matchupsPage) draw(c *ui.Context, e *env) {
	t := c.Theme()
	deck, list := pg.matchupData(e)
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			e.deckTabs(c, e.scoped().decks, deck, "Show matchups with ", e.setDeck)
			if len(list) == 0 {
				ui.Text(c, "No opposing leaders in the logs for these games.").TextColor(t.TextMuted)
				return
			}
			open := list[0].Leader
			if pg.pick != "" {
				open = pg.pick
			}
			tv := arrangeTable(e, "matchups", muGap, matchupCols(e, t, len(list), open), "games", true)
			tv.tie = func(a, b stats.Matchup) int { return cmp.Compare(b.Record.Games(), a.Record.Games()) }
			rows := tv.sorted(list)
			panel(c).Padding(4, 0, 8, 0).Children(func() {
				tv.header(c, 16)
				for _, mu := range rows {
					pg.matchupRow(c.Key(mu.Leader), e, tv, deck, mu, mu.Leader == open)
				}
			})
		})
	})
}

func leadersFaced(n int) string {
	if n == 1 {
		return "1 leader faced"
	}
	return fmt.Sprintf("%d leaders faced", n)
}

const (
	muArtW   = 40
	muGamesW = 68
	muRateW  = 64
	muBarW   = 80
	muTurnW  = 52
	muSqW    = 200
	muLastW  = 84
	muGap    = 12
)

func matchupCols(e *env, t *ui.Theme, n int, open string) []tableCol[stats.Matchup] {
	name := func(mu stats.Matchup) string { return e.snap.card(mu.Leader).Name + " " + mu.Leader }
	turn := func(id, title, tip string, rec func(stats.Matchup) stats.Record) tableCol[stats.Matchup] {
		return tableCol[stats.Matchup]{
			id: id, title: title, width: muTurnW, end: true, tip: tip,
			by:  func(a, b stats.Matchup) int { return cmp.Compare(rec(a).Share(), rec(b).Share()) },
			few: func(mu stats.Matchup) bool { return rec(mu).Games() == 0 },
			cell: func(c *ui.Context, mu stats.Matchup) {
				s := "–"
				if r := rec(mu); r.Games() > 0 {
					s = record(r)
				}
				ui.Text(c, s).TextColor(t.TextMuted)
			},
		}
	}
	return []tableCol[stats.Matchup]{
		{
			id: "leader", title: leadersFaced(n), asc: true,
			by: func(a, b stats.Matchup) int { return cmp.Compare(name(a), name(b)) },
			cell: func(c *ui.Context, mu stats.Matchup) {
				ui.Row(c).Gap(12).AlignItems(ui.Center).MinWidth(0).Children(func() {
					e.artStrip(c, mu.Leader, muArtW, 28)
					ui.Row(c).Gap(6).AlignItems(ui.End).MinWidth(0).Children(func() {
						nameText := ui.Text(c, e.snap.card(mu.Leader).Name).FontWeight(500).SingleLine().Tooltip(e.snap.card(mu.Leader).Name)
						if mu.Leader == open {
							nameText.FontWeight(600)
						}
						ui.Text(c, cards.SetOf(mu.Leader)).FontSize(11).TextColor(faintColor(t))
					})
				})
			},
		},
		{
			id: "games", title: "Games", width: muGamesW, end: true,
			by:   func(a, b stats.Matchup) int { return cmp.Compare(a.Record.Games(), b.Record.Games()) },
			cell: func(c *ui.Context, mu stats.Matchup) { ui.Text(c, fmt.Sprint(mu.Record.Games())) },
		},
		{
			id: "rate", title: "Win rate", width: muRateW + muGap + muBarW, tip: fmt.Sprintf("Rates show from %d games.", stats.MinGames),
			by:  func(a, b stats.Matchup) int { return cmp.Compare(sortRate(a.Record), sortRate(b.Record)) },
			few: func(mu stats.Matchup) bool { return !mu.Record.Enough() },
			cell: func(c *ui.Context, mu stats.Matchup) {
				ui.Row(c).Gap(muGap).AlignItems(ui.Center).Grow(1).MinWidth(0).Children(func() {
					if rate, ok := mu.Record.Rate(); ok {
						ui.Text(c, pct(rate)).Bold().Width(muRateW).TextAlign(ui.End)
					} else {
						ui.Text(c, tooFew).TextColor(faintColor(t)).Width(muRateW).TextAlign(ui.End)
					}
					few := !mu.Record.Enough()
					ui.Box(c).Grow(1).Basis(0).Opacity(map[bool]float32{true: 0.35, false: 1}[few]).Children(func() {
						e.splitBar(c, mu.Record.Share())
					})
				})
			},
		},
		turn("first", "1st", "Going first.", func(mu stats.Matchup) stats.Record { return mu.TurnOrder.First }),
		turn("second", "2nd", "Going second.", func(mu stats.Matchup) stats.Record { return mu.TurnOrder.Second }),
		{
			id: "squares", title: "Games, oldest first", width: muSqW,
			cell: func(c *ui.Context, mu stats.Matchup) {
				rs := make([]stats.Result, 0, len(mu.Games))
				for _, m := range slices.Backward(mu.Games) {
					rs = append(rs, m.Result)
				}
				resultSquares(c, rs, 9).Grow(1)
			},
		},
		{
			id: "last", title: "Last faced", width: muLastW, end: true,
			by: func(a, b stats.Matchup) int { return a.Games[0].PlayedAt.Compare(b.Games[0].PlayedAt) },
			cell: func(c *ui.Context, mu stats.Matchup) {
				ui.Text(c, ago(e.now(), mu.Games[0].PlayedAt)).FontSize(12).TextColor(faintColor(t)).SingleLine()
			},
		},
	}
}

// matchupRow is one opposing leader's line, and under it, when it's open, its turn order, how
// its games went and its latest games. Clicking the line opens it or closes it.
func (pg *matchupsPage) matchupRow(c *ui.Context, e *env, tv tableView[stats.Matchup], deck string, mu stats.Matchup, open bool) {
	t := c.Theme()
	name := e.snap.card(mu.Leader).Name
	ui.Column(c).Transition(slide).Children(func() {
		line := ui.Row(c).Gap(tv.gap).Padding(6, 16).AlignItems(ui.Center).Cursor(ui.CursorPointer).
			Label("Show the matchup vs " + name + " " + cards.SetOf(mu.Leader)).PopoverMenu(e.opponentMenu(deck, mu.Leader))
		if open {
			line.Background(t.Surface).BorderStartWidth(3).BorderColor(t.Accent)
		} else if line.Hovered() {
			line.Background(t.Surface.Alpha(0.5))
		}
		if line.Clicked() {
			if open {
				pg.pick = matchupNone
			} else {
				pg.pick = mu.Leader
			}
		}
		line.Children(func() { tv.cells(c, mu) })
		if open {
			matchupOpen(c, e, deck, mu)
		}
	})
}

// matchupOpen is the open row's detail: turn order, how the games went and the latest games,
// then links to the matchup's games and to a filter on the leader.
func matchupOpen(c *ui.Context, e *env, deck string, mu stats.Matchup) {
	t := c.Theme()
	label := func(text string) { ui.Text(c, text).FontSize(12).TextColor(t.TextMuted) }
	name := e.snap.card(mu.Leader).Name
	ui.Column(c.Key("open")).Gap(12).Padding(8, 16, 14, 16+muArtW+12).Background(t.Surface).BorderStartWidth(3).
		BorderColor(t.Accent).Clip().Transition(unfold).Children(func() {
		ui.Row(c).Gap(28).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Gap(8).Children(func() {
				label("Turn order")
				e.turnRows(c, mu.TurnOrder, e.filter.turn, e.setTurn)
			})
			ui.Column(c).Grow(1).Basis(0).Gap(8).Children(func() {
				label("How the games went")
				howGamesWent(c, mu)
			})
			ui.Column(c).Grow(1.3).Basis(0).Gap(4).Children(func() {
				label("Latest games")
				for i, m := range mu.Games[:min(len(mu.Games), 4)] {
					matchupGame(c.Key(i), e, m)
				}
			})
		})
		ui.Row(c).Gap(24).AlignItems(ui.Center).Children(func() {
			if iconLink(c.Key("show"), iconMatches, "Show in Matches") {
				e.showOpponentGames(deck, mu.Leader)
			}
			if iconLink(c.Key("filter"), iconFilter, "Filter to games vs "+name) {
				e.filterOpponent(mu.Leader)
			}
		})
	})
}

func iconLink(c *ui.Context, icon *ui.SVG, label string) bool {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Background(ui.Transparent).Padding(2, 0).Gap(6).AlignItems(ui.Center)
	col := t.Accent
	if b.Hovered() {
		col = t.AccentHover
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(14, 14).TextColor(col)
		ui.Text(c, label).FontSize(13).TextColor(col).SingleLine()
	})
	return b.Clicked()
}

// howGamesWent is the average turns of the wins and of the losses, and how many games ended by
// lethal and by concession.
func howGamesWent(c *ui.Context, mu stats.Matchup) {
	t := c.Theme()
	avg := func(turns, n int) string {
		if n == 0 {
			return "–"
		}
		return fmt.Sprint(turns)
	}
	cell := func(label, value string) {
		ui.Column(c.Key(label)).Grow(1).Basis(0).Gap(1).Children(func() {
			ui.Text(c, label).FontSize(12).TextColor(faintColor(t))
			ui.Text(c, value).FontSize(18).Bold()
		})
	}
	ui.Column(c).Gap(10).Children(func() {
		ui.Row(c).Children(func() {
			cell("Turns, wins", avg(mu.WinTurns, mu.Record.Wins))
			cell("Turns, losses", avg(mu.LossTurns, mu.Record.Losses))
		})
		ui.Row(c).Children(func() {
			cell("Lethal", fmt.Sprint(mu.Lethal))
			cell("Concession", fmt.Sprint(mu.Conceded))
		})
	})
}

// matchupGame is one game in short: the result, the opponent, turn order, turns and the date.
func matchupGame(c *ui.Context, e *env, m stats.Match) {
	t := c.Theme()
	e.gameLine(c, m).Gap(10).Padding(3, 0).Children(func() {
		resultBadge(c, m.Result)
		ui.Text(c, m.Opp.Name).FontSize(13).FontWeight(500).SingleLine().Grow(1).Basis(0)
		turnMark(c, m)
		ui.Text(c, turns(m.Turns)).FontSize(12).TextColor(t.TextMuted).Width(54).TextAlign(ui.End)
		// The column has no room for a year.
		ui.Text(c, m.PlayedAt.Format("Jan 2")).FontSize(12).TextColor(faintColor(t)).Width(44).TextAlign(ui.End)
	})
}
