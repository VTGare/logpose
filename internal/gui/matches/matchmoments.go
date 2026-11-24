package matches

import (
	"fmt"
	"strings"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
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
			pg.momentRow(c.Key(i), e, m, l, i, mo, i == len(l.Moments)-1)
		}
	})
}

func (pg *Page) momentRow(c *ui.Context, e *page.Env, m stats.Match, l *MatchTimeline, i int, mo replay.Moment, last bool) {
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
		ui.Column(c).Grow(1).Gap(12).Padding(0, 0, 16, 0).Children(func() {
			ui.Row(c).Gap(20).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(2).Basis(0).MinWidth(200).Gap(3).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("Turn %d", mo.Turn)).Bold()
						if mo.Player != "" {
							ui.Text(c, s.Name()).FontSize(12).TextColor(s.Color())
						}
						for j, k := range momentKinds(m, mo) {
							ui.Text(c.Key(j), k).FontSize(11).FontWeight(600).TextColor(t.TextMuted).Padding(1, 8).Radius(9).
								Border(1, t.Border)
						}
					})
					for j, line := range momentLines(e, m, mo, l) {
						ui.Text(c.Key(j), line).Wrap()
					}
					for j, line := range momentNotes(e, m, mo, l) {
						ui.Text(c.Key(fmt.Sprintf("note%d", j)), line).FontSize(13).TextColor(t.TextMuted).Wrap()
					}
				})
				ui.Column(c).Grow(3).Basis(0).Children(func() { momentStrip(c, e, m, mo) })
			})
			if finished(mo) && len(l.Finish) > 0 {
				lethalTurn(c, e, m, mo.Player, l.Finish)
			}
		})
	})
}

// momentCardWidth keeps a strip of 8 cards beside the text at the default window size.
const momentCardWidth = 44

func momentStrip(c *ui.Context, e *page.Env, m stats.Match, mo replay.Moment) {
	t := c.Theme()
	s := page.SideOf(m, mo.Player)
	is := func(k replay.ReasonKind) bool {
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
		ringColor ui.Color
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
						box.Border(2, cd.ringColor)
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
	ringed := func(codes []string, color ui.Color) []card {
		out := make([]card, len(codes))
		for k, code := range codes {
			out[k] = card{code: code, ring: true, ringColor: color}
		}
		return out
	}
	removed := func() {
		group("removed", "Removed from "+s.Other().Object(), plain(mo.Removed, true))
		group("back", "Came back", ringed(mo.CameBack, s.Other().Color()))
	}
	battles := func() {
		if is(replay.ReasonHandDrain) {
			group("countered", s.Other().Name()+" countered with", plain(mo.Countered, false))
			group("blocked", s.Other().Name()+" blocked with", plain(mo.Blocked, true))
		}
		for k, lk := range mo.Locks {
			label := "Can't attack"
			if lk.Rested {
				label = "Can't attack or block"
			}
			group(fmt.Sprintf("lock%d", k), label, ringed(lk.Cards, s.Color()))
		}
	}
	ui.Row(c).Gap(18).Wrap().Children(func() {
		switch {
		case is(replay.ReasonBoard):
			board := make([]card, len(mo.Board))
			for k, code := range mo.Board {
				board[k] = card{code: code, ring: k >= len(mo.Board)-mo.New, ringColor: s.Color()}
			}
			group("board", s.Possessive()+" board after the turn", board)
			group("gone", "Also played", plain(mo.Gone, false))
			removed()
		default:
			group("played", s.Name()+" played", plain(mo.Played, false))
			removed()
		}
		battles()
	})
}

func (pg *Page) momentPin(c *ui.Context, e *page.Env, m stats.Match, i int, mo replay.Moment) {
	p := ui.Text(c, fmt.Sprint(i+1)).FontSize(11).Bold().TextColor(ui.Hex("#FFFFFF")).Size(18, 18).Radius(9).
		Background(page.SideOf(m, mo.Player).Color()).TextAlign(ui.Center).Label(fmt.Sprintf("Replay moment %d", i+1)).
		Tooltip("Watch from here").Cursor(ui.CursorPointer)
	if p.Clicked() {
		pg.openReplay(e, m, mo.Line)
	}
}

// momentLines words a moment's reasons as table talk, as in "emog removed Garp, Oden and Law"
// or "emog took 2 life · You're at 1". Life lost and life left share a line.
func momentLines(e *page.Env, m stats.Match, mo replay.Moment, l *MatchTimeline) []string {
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
		case replay.ReasonRemoved:
			lines = append(lines, name(r.Player)+" removed "+cardList(e, mo.Removed))
		case replay.ReasonBoard:
			lines = append(lines, fmt.Sprintf("%s played %s", name(r.Player), cardList(e, mo.Characters)))
		case replay.ReasonLifeLost:
			addLife(r.Player, hitText(m, r))
		case replay.ReasonAtOne:
			addLife(r.Player, at(r.Player, 1))
		case replay.ReasonAtZero:
			addLife(r.Player, at(r.Player, 0))
		case replay.ReasonFinish:
			loser := page.SideOf(m, r.Player)
			finish = finishText(m, loser.Name(), loser.Other().Name())
		case replay.ReasonHandDrain:
			lines = append(lines, drainText(m, mo, r, l))
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
func hitText(m stats.Match, r replay.Reason) string {
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

func finished(mo replay.Moment) bool {
	for _, r := range mo.Reasons {
		if r.Kind == replay.ReasonFinish {
			return true
		}
	}
	return false
}

// momentKinds labels what kind of moment a turn is, for the moments the reasons' words don't
// name.
func momentKinds(m stats.Match, mo replay.Moment) []string {
	var out []string
	for _, r := range mo.Reasons {
		if r.Kind == replay.ReasonHandDrain {
			out = append(out, "Hand drain")
		}
	}
	if len(mo.Locks) > 0 {
		out = append(out, "Lock")
	}
	if len(mo.CameBack) > 0 {
		out = append(out, "Came back")
	}
	if len(mo.Defended) > 0 {
		out = append(out, "Defended and lost")
	}
	if finished(mo) && m.End.Kind == simlog.EndLethal {
		out = append(out, "Lethal")
	}
	return out
}

// drainText words a hand drain, as "Zeta spent 4 cards to stay at 4 life".
func drainText(m stats.Match, mo replay.Moment, r replay.Reason, l *MatchTimeline) string {
	d := page.SideOf(m, r.Player)
	life := l.Opp
	if d.Mine {
		life = l.Me
	}
	text := fmt.Sprintf("%s spent %s", d.Name(), kit.Cards(r.N))
	if t := mo.Turn; t < len(life) && life[t] == life[t-1] {
		text += fmt.Sprintf(" to stay at %d life", life[t])
	}
	return text
}

// handsText is who holds what after a hand drain, as "Zeta holds 3 cards, you hold 5".
func handsText(m stats.Match, mo replay.Moment, r replay.Reason, l *MatchTimeline) string {
	d := page.SideOf(m, r.Player)
	hand, other := l.OppHand, l.MeHand
	if d.Mine {
		hand, other = l.MeHand, l.OppHand
	}
	t := mo.Turn
	if t >= len(hand) || t >= len(other) {
		return ""
	}
	first, second := "holds", "hold"
	if d.Mine {
		first, second = "hold", "holds"
	}
	return fmt.Sprintf("%s %s %s, %s %s %d", d.Name(), first, kit.Cards(hand[t]), d.Other().Object(), second, other[t])
}

// momentNotes are the smaller things a moment's turn did: locks, cards that came back and
// characters defended and lost anyway.
func momentNotes(e *page.Env, m stats.Match, mo replay.Moment, l *MatchTimeline) []string {
	var out []string
	for _, r := range mo.Reasons {
		if r.Kind == replay.ReasonHandDrain {
			if h := handsText(m, mo, r, l); h != "" {
				out = append(out, h)
			}
		}
	}
	for _, lk := range mo.Locks {
		what := "attacking next turn"
		if lk.Rested {
			what = "blocking this turn and attacking next turn"
		}
		out = append(out, fmt.Sprintf("%s stops %s %s", e.Snap.Card(lk.Source).Name, cardList(e, lk.Cards), what))
	}
	if len(mo.CameBack) > 0 {
		out = append(out, cardList(e, mo.CameBack)+" came back from the trash")
	}
	for _, d := range mo.Defended {
		owner := page.SideOf(m, mo.Player).Other()
		out = append(out, fmt.Sprintf("%s spent %s on %s and lost it", owner.Name(), kit.Cards(d.Cards), e.Snap.Card(d.Card).Name))
	}
	return out
}
