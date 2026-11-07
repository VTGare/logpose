package gui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/live"
	"github.com/VTGare/logpose/internal/stats"
)

// liveStrip is the game running in the sim right now: the lobby while it waits, then the
// opponent, your history against them, and both players' life and hand.
func (v *view) liveStrip(c *ui.Context) {
	if v.detail.replay != nil {
		return
	}
	s := v.live
	t := c.Theme()
	ui.Row(c).Margin(16, 20, 0, 20).Padding(12, 16).Gap(20).Radius(12).Background(panelColor(t)).Border(1, t.Border.Alpha(0.6)).
		Shadow(0, 6, 20, 0, panelShade).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(8, 8).Radius(4).Background(winColor)
		if s.Phase == live.Lobby {
			ui.Text(c, lobbyStatus(s)).Bold()
			ui.Box(c).Grow(1)
			lobbyTag(c, s)
			return
		}
		v.livePlayers(c, s)
		ui.Box(c).Grow(1)
		v.liveCounts(c, s)
		lobbyTag(c, s)
	})
}

func lobbyStatus(s live.State) string {
	switch {
	case s.Ranked():
		return "Match found"
	case s.Hosting:
		return "Hosting a lobby"
	default:
		return "Joining a lobby"
	}
}

func lobbyTag(c *ui.Context, s live.State) {
	kind := "Casual"
	if s.Ranked() {
		kind = "Ranked"
	}
	if s.Lobby != "" {
		kind += " · " + s.Lobby
	}
	ui.Text(c, kind).FontSize(12).TextColor(c.Theme().TextMuted)
}

func (v *view) livePlayers(c *ui.Context, s live.State) {
	t := c.Theme()
	me, _ := s.Local()
	opp, ok := s.Opponent()
	if !ok {
		ui.Text(c, "Starting a game").Bold()
		return
	}
	mine := me.Leader
	if mine == "" {
		mine = v.snap.Overview.Deck
	}
	vs := stats.VersusFor(v.snap.Matches, mine, opp.Leader, opp.ID)

	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, "vs "+nameOf(opp.ID)).Bold().SingleLine()
		if opp.Leader != "" {
			v.leaderName(c, opp.Leader)
		} else {
			ui.Text(c, "Leader not in the sim's log").FontSize(12).TextColor(t.TextMuted)
		}
	})
	if opp.Leader != "" && mine != "" {
		ui.Column(c).Gap(4).Children(func() {
			label := fmt.Sprintf("With %s vs %s", v.snap.card(mine).Name, v.snap.card(opp.Leader).Name)
			ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).SingleLine()
			ui.Text(c, matchupLine(vs.Matchup)).Bold()
			if line := turnOrderLine(s, me, vs.TurnOrder); line != "" {
				ui.Text(c, line).FontSize(12).TextColor(t.TextMuted)
			}
		})
	}
	if vs.Player.Games() > 0 {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "Against "+nameOf(opp.ID)).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, record(vs.Player)).Bold()
		})
	}
}

func matchupLine(r stats.Record) string {
	if r.Games() == 0 {
		return "First time"
	}
	if rate, ok := r.Rate(); ok {
		return fmt.Sprintf("%s · %s", record(r), pct(rate))
	}
	return record(r)
}

// turnOrderLine is your record in this game's turn order once it's known, or both before.
func turnOrderLine(s live.State, me live.Player, o stats.TurnOrder) string {
	switch s.First {
	case "":
		if o.First.Games()+o.Second.Games() == 0 {
			return ""
		}
		return fmt.Sprintf("First %s · Second %s", record(o.First), record(o.Second))
	case me.ID:
		return "Going first · " + record(o.First)
	default:
		return "Going second · " + record(o.Second)
	}
}

func (*view) liveCounts(c *ui.Context, s live.State) {
	t := c.Theme()
	me, _ := s.Local()
	opp, _ := s.Opponent()
	side := func(label string, p live.Player) {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, fmt.Sprintf("%d life · %d hand", p.Life, p.Hand)).SingleLine()
		})
	}
	side("You", me)
	side("Them", opp)
}

func nameOf(id string) string {
	name, _, _ := strings.Cut(id, "#")
	return name
}
