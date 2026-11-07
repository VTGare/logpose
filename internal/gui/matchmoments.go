package gui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
)

func (v *view) timeline(c *ui.Context, m stats.Match) {
	v.flowTile(c, m)
	v.momentsTile(c, m)
}

func (v *view) momentsTile(c *ui.Context, m stats.Match) ui.Element {
	t := c.Theme()
	return tile(c, "Key moments", func() {
		l := v.detail.timeline
		switch {
		case l == nil:
			ui.Text(c, "Reading the log").TextColor(t.TextMuted)
			return
		case len(l.Moments) == 0:
			ui.Text(c, "Nothing stood out in this game.").TextColor(t.TextMuted)
			return
		}
		for i, mo := range l.Moments {
			v.momentRow(c.Key(i), m, i, mo, i == len(l.Moments)-1)
		}
	})
}

// momentRow is one moment as a ledger line: its pin on a spine that joins the moments, the turn
// and what happened, then every card of the turn in one strip.
func (v *view) momentRow(c *ui.Context, m stats.Match, i int, mo simlog.Moment, last bool) {
	t := c.Theme()
	col := playerColor(m, mo.Player)
	row := ui.Row(c).Gap(12).AlignItems(ui.Stretch).Cursor(ui.CursorPointer).Tooltip("Watch from here")
	defer func() {
		if row.Clicked() {
			v.openReplay(m, mo.Line)
		}
	}()
	row.Children(func() {
		ui.Column(c).Width(20).AlignItems(ui.Center).Gap(4).Padding(1, 0, 0, 0).Children(func() {
			v.momentPin(c, m, i, mo)
			if !last {
				ui.Box(c).Width(2).Grow(1).MinHeight(12).Background(t.Border)
			}
		})
		ui.Row(c).Grow(1).Gap(20).AlignItems(ui.Start).Padding(0, 0, 16, 0).Children(func() {
			ui.Column(c).Grow(2).Basis(0).MinWidth(200).Gap(3).Children(func() {
				ui.Row(c).Gap(6).Children(func() {
					ui.Text(c, fmt.Sprintf("Turn %d", mo.Turn)).Bold()
					ui.Text(c, turnOwner(m, mo.Player)).FontSize(12).TextColor(col)
				})
				for j, line := range v.momentLines(m, mo) {
					ui.Text(c.Key(j), line).Wrap()
				}
			})
			ui.Column(c).Grow(3).Basis(0).Children(func() { v.momentStrip(c, m, mo) })
		})
	})
}

// momentCardWidth keeps a strip of 8 cards beside the text at the default window size.
const momentCardWidth = 44

// momentStrip shows a moment's cards in one row that wraps when narrow: what the turn's player
// played, then what the other player lost, greyed out. On a big board the board leads, with the
// cards new this turn outlined in the player's color, and played cards that didn't stay come
// after.
func (v *view) momentStrip(c *ui.Context, m stats.Match, mo simlog.Moment) {
	t := c.Theme()
	col := playerColor(m, mo.Player)
	player, other := turnOwner(m, mo.Player), "you"
	if mo.Player == m.Me.ID {
		other = m.Opp.Name
	}
	is := func(k simlog.ReasonKind) bool {
		for _, r := range mo.Reasons {
			if r.Kind == k {
				return true
			}
		}
		return false
	}
	type card struct {
		code      string
		dim, ring bool
	}
	group := func(key, label string, cards []card) {
		if len(cards) == 0 {
			return
		}
		ui.Column(c.Key(key)).Gap(4).Children(func() {
			ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
			ui.Row(c).Gap(4).Wrap().Children(func() {
				for k, cd := range cards {
					box := ui.Box(c.Key(k)).Label(v.snap.card(cd.code).Name).Radius(6).Padding(2).Children(func() {
						v.cardThumb(c, cd.code, momentCardWidth)
					})
					if cd.ring {
						box.Border(2, col)
					} else {
						box.Border(2, ui.Color{})
					}
					if cd.dim {
						box.Grayscale().Opacity(0.85)
					}
				}
			})
		})
	}
	plain := func(codes []string, dim bool) []card {
		out := make([]card, len(codes))
		for k, code := range codes {
			out[k] = card{code: code, dim: dim}
		}
		return out
	}
	removed := func() { group("removed", "Removed from "+other, plain(mo.Removed, true)) }
	ui.Row(c).Gap(18).Wrap().Children(func() {
		switch {
		case is(simlog.ReasonBoard):
			left := make(map[string]int)
			for _, code := range mo.Played {
				left[code]++
			}
			var board []card
			for _, code := range mo.Board {
				board = append(board, card{code: code, ring: left[code] > 0})
				if left[code] > 0 {
					left[code]--
				}
			}
			var gone []string
			for _, code := range mo.Played {
				if left[code] > 0 {
					gone = append(gone, code)
					left[code]--
				}
			}
			group("board", possessive(player)+" board after the turn", board)
			group("gone", "Also played", plain(gone, false))
			removed()
		default:
			group("played", player+" played", plain(mo.Played, false))
			removed()
		}
	})
}

func possessive(name string) string {
	if name == "You" {
		return "Your"
	}
	return name + "'s"
}

func pin(c *ui.Context, n int, col ui.Color) ui.Element {
	return ui.Text(c, fmt.Sprint(n)).FontSize(11).Bold().TextColor(ui.Hex("#FFFFFF")).
		Size(18, 18).Radius(9).Background(col).TextAlign(ui.Center)
}

// momentPin is a key moment's numbered pin, which opens the replay at the moment's turn.
func (v *view) momentPin(c *ui.Context, m stats.Match, i int, mo simlog.Moment) {
	p := pin(c, i+1, playerColor(m, mo.Player)).Label(fmt.Sprintf("Replay moment %d", i+1)).
		Tooltip("Watch from here").Cursor(ui.CursorPointer)
	if p.Clicked() {
		v.openReplay(m, mo.Line)
	}
}

func playerColor(m stats.Match, id string) ui.Color {
	if id == m.Me.ID {
		return youColor
	}
	return oppColor
}

func turnOwner(m stats.Match, id string) string {
	switch id {
	case "":
		return ""
	case m.Me.ID:
		return "You"
	}
	return m.Opp.Name
}

// momentLines words a moment's reasons as table talk, as in "emog removed Garp, Oden and Law"
// or "emog took 2 life · You're at 1". Life lost and life left share a line.
func (v *view) momentLines(m stats.Match, mo simlog.Moment) []string {
	name := func(id string) string {
		if id == m.Me.ID {
			return "You"
		}
		return m.Opp.Name
	}
	other := func(id string) string {
		if id == m.Me.ID {
			return m.Opp.Name
		}
		return "You"
	}
	at := func(id string, n int) string {
		if id == m.Me.ID {
			return fmt.Sprintf("You're at %d", n)
		}
		return fmt.Sprintf("%s is at %d", m.Opp.Name, n)
	}
	var lines []string
	life := make(map[string][]string)
	var lifeOrder []string
	addLife := func(id, text string) {
		if _, ok := life[id]; !ok {
			lifeOrder = append(lifeOrder, id)
		}
		life[id] = append(life[id], text)
	}
	var finish string
	for _, r := range mo.Reasons {
		switch r.Kind {
		case simlog.ReasonRemoved:
			lines = append(lines, name(r.Player)+" removed "+v.cardList(mo.Removed))
		case simlog.ReasonBoard:
			lines = append(lines, fmt.Sprintf("%s played %s", name(r.Player), v.cardList(characters(v, mo.Played))))
		case simlog.ReasonLifeLost:
			addLife(r.Player, hitText(m, r))
		case simlog.ReasonAtOne:
			addLife(r.Player, at(r.Player, 1))
		case simlog.ReasonAtZero:
			addLife(r.Player, at(r.Player, 0))
		case simlog.ReasonFinish:
			finish = finishText(m, name(r.Player), other(r.Player))
		}
	}
	for _, id := range lifeOrder {
		lines = append(lines, strings.Join(life[id], " · "))
	}
	if finish != "" {
		lines = append(lines, finish)
	}
	return lines
}

// hitText words life lost from the attacker's side, as "You hit emog for 2" or "emog hit you
// for 2".
func hitText(m stats.Match, r simlog.Reason) string {
	if r.Player == m.Me.ID {
		return fmt.Sprintf("%s hit you for %d", m.Opp.Name, r.N)
	}
	return fmt.Sprintf("You hit %s for %d", m.Opp.Name, r.N)
}

func characters(v *view, codes []string) []string {
	var out []string
	for _, code := range codes {
		if v.snap.card(code).Type != "Stage" {
			out = append(out, code)
		}
	}
	return out
}

// cardList names cards as "Garp, Oden ×2 and Law".
func (v *view) cardList(codes []string) string {
	var names []string
	count := make(map[string]int)
	for _, code := range codes {
		n := v.snap.card(code).Name
		if count[n] == 0 {
			names = append(names, n)
		}
		count[n]++
	}
	for i, n := range names {
		if count[n] > 1 {
			names[i] = fmt.Sprintf("%s ×%d", n, count[n])
		}
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

func finishText(m stats.Match, loser, winner string) string {
	switch m.End.Kind {
	case simlog.EndLethal:
		return winner + " dealt lethal"
	case simlog.EndConcede:
		return loser + " conceded"
	case simlog.EndDeckOut:
		return loser + " decked out"
	}
	return "The game ended"
}
