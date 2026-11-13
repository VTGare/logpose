package matches

import (
	"fmt"
	"strings"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
)

func (pg *Page) momentsTile(c *ui.Context, e *page.Env, m stats.Match) ui.Element {
	t := c.Theme()
	return e.FoldTile(c, "match.moments", kit.FoldHead{Label: "Key moments"}, func() {
		l := pg.Detail.timeline
		switch {
		case l == nil:
			ui.Text(c, "Reading the log").TextColor(t.TextMuted)
			return
		case len(l.Moments) == 0:
			ui.Text(c, "Nothing stood out in this game.").TextColor(t.TextMuted)
			return
		}
		for i, mo := range l.Moments {
			pg.momentRow(c.Key(i), e, m, i, mo, i == len(l.Moments)-1)
		}
	})
}

func (pg *Page) momentRow(c *ui.Context, e *page.Env, m stats.Match, i int, mo simlog.Moment, last bool) {
	t := c.Theme()
	s := page.SideOf(m, mo.Player)
	row := ui.Row(c).Gap(12).AlignItems(ui.Stretch).Cursor(ui.CursorPointer).Tooltip("Watch from here")
	defer func() {
		if row.Clicked() {
			pg.openReplay(e, m, mo.Line)
		}
	}()
	row.Children(func() {
		ui.Column(c).Width(20).AlignItems(ui.Center).Gap(4).Padding(1, 0, 0, 0).Children(func() {
			pg.momentPin(c, e, m, i, mo)
			if !last {
				ui.Box(c).Width(2).Grow(1).MinHeight(12).Background(t.Border)
			}
		})
		ui.Row(c).Grow(1).Gap(20).AlignItems(ui.Start).Padding(0, 0, 16, 0).Children(func() {
			ui.Column(c).Grow(2).Basis(0).MinWidth(200).Gap(3).Children(func() {
				ui.Row(c).Gap(6).Children(func() {
					ui.Text(c, fmt.Sprintf("Turn %d", mo.Turn)).Bold()
					if mo.Player != "" {
						ui.Text(c, s.Name()).FontSize(12).TextColor(s.Color())
					}
				})
				for j, line := range momentLines(e, m, mo) {
					ui.Text(c.Key(j), line).Wrap()
				}
			})
			ui.Column(c).Grow(3).Basis(0).Children(func() { momentStrip(c, e, m, mo) })
		})
	})
}

// momentCardWidth keeps a strip of 8 cards beside the text at the default window size.
const momentCardWidth = 44

func momentStrip(c *ui.Context, e *page.Env, m stats.Match, mo simlog.Moment) {
	t := c.Theme()
	s := page.SideOf(m, mo.Player)
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
					box := ui.Box(c.Key(k)).Label(e.Snap.Card(cd.code).Name).Radius(6).Padding(2).Children(func() {
						kit.Lift(e.Card(c, cd.code, momentCardWidth), 5)
					})
					if cd.ring {
						box.Border(2, s.Color())
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
	removed := func() { group("removed", "Removed from "+s.Other().Object(), plain(mo.Removed, true)) }
	ui.Row(c).Gap(18).Wrap().Children(func() {
		switch {
		case is(simlog.ReasonBoard):
			board := make([]card, len(mo.Board))
			for k, code := range mo.Board {
				board[k] = card{code: code, ring: k >= len(mo.Board)-mo.New}
			}
			group("board", s.Possessive()+" board after the turn", board)
			group("gone", "Also played", plain(mo.Gone, false))
			removed()
		default:
			group("played", s.Name()+" played", plain(mo.Played, false))
			removed()
		}
	})
}

func (pg *Page) momentPin(c *ui.Context, e *page.Env, m stats.Match, i int, mo simlog.Moment) {
	p := ui.Text(c, fmt.Sprint(i+1)).FontSize(11).Bold().TextColor(ui.Hex("#FFFFFF")).Size(18, 18).Radius(9).
		Background(page.SideOf(m, mo.Player).Color()).TextAlign(ui.Center).Label(fmt.Sprintf("Replay moment %d", i+1)).
		Tooltip("Watch from here").Cursor(ui.CursorPointer)
	if p.Clicked() {
		pg.openReplay(e, m, mo.Line)
	}
}

// momentLines words a moment's reasons as table talk, as in "emog removed Garp, Oden and Law"
// or "emog took 2 life · You're at 1". Life lost and life left share a line.
func momentLines(e *page.Env, m stats.Match, mo simlog.Moment) []string {
	name := func(id string) string { return page.SideOf(m, id).Name() }
	at := func(id string, n int) string {
		if s := page.SideOf(m, id); !s.Mine {
			return fmt.Sprintf("%s is at %d", s.Name(), n)
		}
		return fmt.Sprintf("You're at %d", n)
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
			lines = append(lines, name(r.Player)+" removed "+cardList(e, mo.Removed))
		case simlog.ReasonBoard:
			lines = append(lines, fmt.Sprintf("%s played %s", name(r.Player), cardList(e, mo.Characters)))
		case simlog.ReasonLifeLost:
			addLife(r.Player, hitText(m, r))
		case simlog.ReasonAtOne:
			addLife(r.Player, at(r.Player, 1))
		case simlog.ReasonAtZero:
			addLife(r.Player, at(r.Player, 0))
		case simlog.ReasonFinish:
			loser := page.SideOf(m, r.Player)
			finish = finishText(m, loser.Name(), loser.Other().Name())
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
	hit := page.SideOf(m, r.Player)
	return fmt.Sprintf("%s hit %s for %d", hit.Other().Name(), hit.Object(), r.N)
}

// cardList names cards as "Garp, Oden ×2 and Law".
func cardList(e *page.Env, codes []string) string {
	var names []string
	count := make(map[string]int)
	for _, code := range codes {
		n := e.Snap.Card(code).Name
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
