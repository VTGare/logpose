package gui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
)

type matchupsState struct {
	// pick is the opposing leader shown in full, or "" for the most played one.
	pick string

	snap    *snapshot
	filters filters
	deck    string
	list    []stats.Matchup
	// turns is each matchup's turn order without the turn order filter, for the turn order tile.
	turns map[string]stats.TurnOrder
}

// matchupData is the deck the filter names, or the most played one in scope, and its matchups.
func (v *view) matchupData() (string, []stats.Matchup) {
	sc := v.scoped()
	deck := v.filter.deck
	if deck == "" && len(sc.decks) > 0 {
		deck = sc.decks[0].Leader
	}
	s := &v.matchups
	if s.snap != v.snap || s.filters != v.filter || s.deck != deck {
		s.snap, s.filters, s.deck = v.snap, v.filter, deck
		s.list = stats.Matchups(sc.matches, deck)
		s.turns = make(map[string]stats.TurnOrder)
		for _, mu := range stats.Matchups(sc.anyTurn, deck) {
			s.turns[mu.Leader] = mu.TurnOrder
		}
	}
	return deck, s.list
}

// matchupNone is the pick when the user closed the open row.
const matchupNone = "-"

func (v *view) matchupsPage(c *ui.Context) {
	t := c.Theme()
	deck, list := v.matchupData()
	var decks []stats.LeaderRecord
	for _, d := range v.scoped().decks {
		decks = append(decks, stats.LeaderRecord{Leader: d.Leader, Record: d.Record})
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(20).Gap(16).Children(func() {
			v.deckTabs(c, decks, deck, "Show matchups with ", v.setDeck)
			if len(list) == 0 {
				ui.Text(c, "No opposing leaders in the logs for these games.").TextColor(t.TextMuted)
				return
			}
			open := list[0].Leader
			if v.matchups.pick != "" {
				open = v.matchups.pick
			}
			panel(c).Padding(4, 0, 8, 0).Children(func() {
				v.matchupHeader(c, len(list))
				for _, mu := range list {
					v.matchupRow(c.Key(mu.Leader), deck, mu, mu.Leader == open)
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

// Matchup table columns, in DIPs. The leader's name takes the rest.
const (
	muArtW   = 40
	muGamesW = 56
	muRateW  = 64
	muBarW   = 80
	muTurnW  = 52
	muSqW    = 200
	muLastW  = 84
)

func (*view) matchupHeader(c *ui.Context, n int) {
	t := c.Theme()
	head := func(label string, w float32, end bool) {
		el := ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).Width(w)
		if end {
			el.TextAlign(ui.End)
		}
	}
	ui.Row(c).Gap(12).Padding(10, 16).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border.Alpha(0.6)).Children(func() {
		ui.Box(c).Width(muArtW)
		ui.Text(c, leadersFaced(n)).FontSize(12).TextColor(t.TextMuted).Grow(1).Basis(0)
		head("Games", muGamesW, true)
		head("Win rate", muRateW, true)
		head("", muBarW, false)
		head("1st", muTurnW, true)
		head("2nd", muTurnW, true)
		head("Games, oldest first", muSqW, false)
		head("Last faced", muLastW, true)
	})
}

// matchupRow is one opposing leader's line, and under it, when it's open, its turn order, how
// its games went and its latest games. Clicking the line opens it or closes it.
func (v *view) matchupRow(c *ui.Context, deck string, mu stats.Matchup, open bool) {
	t := c.Theme()
	few := mu.Record.Games() < stats.MinGames
	name := v.snap.card(mu.Leader).Name
	turns := v.matchups.turns[mu.Leader]
	ui.Column(c).Transition(slide).Children(func() {
		line := ui.Row(c).Gap(12).Padding(6, 16).AlignItems(ui.Center).Cursor(ui.CursorPointer).
			Label("Show the matchup vs " + name + " " + setOf(mu.Leader))
		if open {
			line.Background(t.Surface).BorderStartWidth(3).BorderColor(t.Accent)
		} else if line.Hovered() {
			line.Background(t.Surface.Alpha(0.5))
		}
		if line.Clicked() {
			if open {
				v.matchups.pick = matchupNone
			} else {
				v.matchups.pick = mu.Leader
			}
		}
		line.Children(func() {
			v.artStrip(c, mu.Leader, muArtW, 28)
			ui.Row(c).Gap(6).Grow(1).Basis(0).AlignItems(ui.End).Children(func() {
				nameText := ui.Text(c, name).FontWeight(500).SingleLine()
				if open {
					nameText.FontWeight(600)
				}
				ui.Text(c, setOf(mu.Leader)).FontSize(11).TextColor(faintColor(t))
			})
			ui.Text(c, fmt.Sprint(mu.Record.Games())).Width(muGamesW).TextAlign(ui.End)
			if rate, ok := mu.Record.Rate(); ok {
				ui.Text(c, pct(rate)).Bold().Width(muRateW).TextAlign(ui.End)
			} else {
				ui.Text(c, "–").TextColor(faintColor(t)).Width(muRateW).TextAlign(ui.End)
			}
			ui.Box(c).Width(muBarW).Opacity(map[bool]float32{true: 0.35, false: 1}[few]).Children(func() {
				splitBar(c, winShare(mu.Record))
			})
			turnText := func(r stats.Record) {
				s := "–"
				if r.Games() > 0 {
					s = record(r)
				}
				ui.Text(c, s).TextColor(t.TextMuted).Width(muTurnW).TextAlign(ui.End)
			}
			turnText(turns.First)
			turnText(turns.Second)
			ui.Row(c).Width(muSqW).Gap(2).Wrap().Children(func() {
				for i := len(mu.Games) - 1; i >= 0; i-- {
					ui.Box(c.Key(i)).Size(9, 9).Radius(2).Background(resultColor(mu.Games[i].Result))
				}
			})
			ui.Text(c, ago(v.snap.Now, mu.Games[0].PlayedAt)).FontSize(12).TextColor(faintColor(t)).Width(muLastW).TextAlign(ui.End)
		})
		if open {
			v.matchupOpen(c, deck, mu, turns)
		}
	})
}

// matchupOpen is the open row's detail: turn order, how the games went and the latest games.
func (v *view) matchupOpen(c *ui.Context, deck string, mu stats.Matchup, turns stats.TurnOrder) {
	t := c.Theme()
	label := func(text string) { ui.Text(c, text).FontSize(12).TextColor(t.TextMuted) }
	ui.Row(c.Key("open")).Gap(28).Padding(8, 16, 16, 16+muArtW+12).Background(t.Surface).BorderStartWidth(3).BorderColor(t.Accent).
		AlignItems(ui.Start).Clip().Transition(unfold).Children(func() {
		ui.Column(c).Grow(1).Basis(0).Gap(8).Children(func() {
			label("Turn order")
			turnRows(c, turns, v.filter.turn, v.setTurn)
		})
		ui.Column(c).Grow(1).Basis(0).Gap(8).Children(func() {
			label("How the games went")
			howGamesWent(c, mu.Games)
		})
		ui.Column(c).Grow(1.3).Basis(0).Gap(4).Children(func() {
			ui.Row(c).Children(func() {
				label("Latest games")
				ui.Spacer(c)
				if ui.Text(c, "Show in Matches").FontSize(12).TextColor(t.Accent).Clicked() {
					v.filter.deck, v.filter.oppLeader = deck, mu.Leader
					v.page = pageMatches
					v.resetPoints()
				}
			})
			for i, m := range mu.Games[:min(len(mu.Games), 4)] {
				v.gameLine(c.Key(i), m)
			}
		})
	})
}

// howGamesWent is the average turns of the wins and of the losses, and how many games ended by
// lethal and by concession.
func howGamesWent(c *ui.Context, ms []stats.Match) {
	t := c.Theme()
	var winTurns, lossTurns, wins, losses, lethal, concede int
	for _, m := range ms {
		switch m.Result {
		case stats.Win:
			wins++
			winTurns += m.Turns
		case stats.Loss:
			losses++
			lossTurns += m.Turns
		}
		switch m.End.Kind {
		case simlog.EndLethal:
			lethal++
		case simlog.EndConcede:
			concede++
		}
	}
	avg := func(sum, n int) string {
		if n == 0 {
			return "–"
		}
		return fmt.Sprint((sum + n/2) / n)
	}
	cell := func(label, value string) {
		ui.Column(c.Key(label)).Grow(1).Basis(0).Gap(1).Children(func() {
			ui.Text(c, label).FontSize(12).TextColor(faintColor(t))
			ui.Text(c, value).FontSize(18).Bold()
		})
	}
	ui.Column(c).Gap(10).Children(func() {
		ui.Row(c).Children(func() {
			cell("Turns, wins", avg(winTurns, wins))
			cell("Turns, losses", avg(lossTurns, losses))
		})
		ui.Row(c).Children(func() {
			cell("Lethal", fmt.Sprint(lethal))
			cell("Concession", fmt.Sprint(concede))
		})
	})
}

// gameLine is one game in short: the result, the opponent, turn order, turns and the date. It
// opens the match.
func (v *view) gameLine(c *ui.Context, m stats.Match) {
	t := c.Theme()
	row := ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(3, 0).Label("Open the match against " + m.Opp.Name)
	if row.Clicked() {
		v.openMatch(m)
	}
	row.Children(func() {
		resultBadge(c, m.Result)
		ui.Text(c, m.Opp.Name).FontSize(13).FontWeight(500).SingleLine().Grow(1).Basis(0)
		turnMark(c, m)
		ui.Text(c, fmt.Sprintf("%d turns", m.Turns)).FontSize(12).TextColor(t.TextMuted).Width(54).TextAlign(ui.End)
		ui.Text(c, m.PlayedAt.Format("Jan 2")).FontSize(12).TextColor(faintColor(t)).Width(44).TextAlign(ui.End)
	})
}

// resultBadge is W or L on the result's color.
func resultBadge(c *ui.Context, r stats.Result) {
	ui.Text(c, resultLetter(r)).FontSize(11).Bold().TextColor(ui.Hex("#0B1209")).TextAlign(ui.Center).
		Size(20, 20).Radius(5).Background(resultColor(r)).Padding(2, 0)
}

// turnMark is 1st with a filled mark or 2nd with a hollow one, or nothing when the log doesn't
// say.
func turnMark(c *ui.Context, m stats.Match) {
	t := c.Theme()
	ui.Row(c).Gap(5).AlignItems(ui.Center).Width(40).Children(func() {
		if !m.TurnOrderKnown {
			return
		}
		mark := ui.Box(c).Size(8, 8).Radius(4)
		if m.WentFirst {
			mark.Background(t.TextMuted)
		} else {
			mark.Border(1.5, t.TextMuted)
		}
		ui.Text(c, turnOrderShort(m)).FontSize(12).TextColor(t.TextMuted)
	})
}
