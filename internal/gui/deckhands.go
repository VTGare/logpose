package gui

import (
	"cmp"
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

const (
	keepLabelW  = 170
	keepBarW    = 200
	dealtColW   = 56
	keptColW    = 150
	keptBarW    = 90
	shownHands  = 5
	sentThumbW  = 26
	keptThumbW  = 36
	evenGap     = 0.1
	handMetaW   = 170
	chevronColW = 20
)

// handsTab is how the scope's games went by the mulligan, and by each card in the first five.
func (v *view) handsTab(c *ui.Context, d *deckData) {
	mu := d.mulligans
	all := d.stats.Record
	if all.Games() == 0 {
		panel(c).Padding(16).Children(func() { noGames(c, d) })
		return
	}
	avg := all.Share()
	v.keepTile(c, mu, avg)
	v.dealtPanel(c, mu.Cards, avg)
	v.handTester(c, d.deckGames, mu.Cards)
}

// keepTile compares kept and mulliganed hands going first, going second and in all games.
func (v *view) keepTile(c *ui.Context, mu stats.Mulligans, avg float64) {
	t := c.Theme()
	sum := fmt.Sprintf("Kept %s · mulliganed %s", rateRecord(mu.All.Kept), rateRecord(mu.All.Mulliganed))
	v.foldTile(c, "hands.keep", foldHead{label: "Keep or mulligan", summary: sum}, func() {
		ui.Text(c, keepFinding(mu)).FontSize(14).Wrap()
		ui.Row(c).Gap(12).Padding(6, 0, 0, 0).Children(func() {
			ui.Box(c).Width(keepLabelW)
			ui.Text(c, "Kept the first five").FontSize(12).TextColor(t.TextMuted).Grow(1).Basis(0)
			ui.Text(c, "Mulliganed").FontSize(12).TextColor(t.TextMuted).Grow(1).Basis(0)
		})
		for _, s := range []struct {
			label string
			k     stats.KeepSplit
		}{{"Going first", mu.First}, {"Going second", mu.Second}, {"All games", mu.All}} {
			n := s.k.Kept.Plus(s.k.Mulliganed).Games()
			if n == 0 {
				continue
			}
			ui.Box(c.Key(s.label + "-line")).Height(1).Background(t.Border.Alpha(0.3))
			ui.Row(c.Key(s.label)).Gap(12).AlignItems(ui.Center).Padding(4, 0).Children(func() {
				ui.Column(c).Width(keepLabelW).Gap(1).Children(func() {
					ui.Text(c, s.label).FontSize(13).FontWeight(600)
					ui.Text(c, fmt.Sprintf("%d of %d sent back", s.k.Mulliganed.Games(), n)).FontSize(12).TextColor(faintColor(t))
				})
				ui.Box(c).Grow(1).Basis(0).Children(func() { rateBar(c.Key("kept"), s.k.Kept, avg, keepBarW) })
				ui.Box(c).Grow(1).Basis(0).Children(func() { rateBar(c.Key("mull"), s.k.Mulliganed, avg, keepBarW) })
			})
		}
		if line := oppLine(mu.Opp); line != "" {
			ui.Text(c, line).FontSize(13).TextColor(t.TextMuted).Wrap().Padding(6, 0, 0, 0)
		}
		ui.Text(c, fmt.Sprintf("You send back hands you think are weak, so this compares the hands, not the choice. "+
			"The tick is the %s these games win, and the bar is where each rate likely is.", pct(avg))).
			FontSize(12).TextColor(faintColor(t)).Wrap().Padding(4, 0, 0, 0)
	})
}

// oppLine is how the games went by the opponent's mulligan, or nothing when either side has
// too few games.
func oppLine(k stats.KeepSplit) string {
	mull, ok1 := k.Mulliganed.Rate()
	kept, ok2 := k.Kept.Rate()
	if !ok1 || !ok2 {
		return ""
	}
	return fmt.Sprintf("When the opponent mulliganed you won %s (%s), and %s (%s) when they kept.",
		pct(mull), record(k.Mulliganed), pct(kept), record(k.Kept))
}

// keepFinding says in words where keeping and mulliganing differ by turn order, or in all games
// when neither turn order has enough games of both.
func keepFinding(mu stats.Mulligans) string {
	var told, even []string
	for _, s := range []struct {
		name string
		k    stats.KeepSplit
	}{{"Going first", mu.First}, {"Going second", mu.Second}} {
		kept, ok1 := s.k.Kept.Rate()
		mull, ok2 := s.k.Mulliganed.Rate()
		switch {
		case !ok1 || !ok2:
		case math.Abs(kept-mull) < evenGap:
			even = append(even, s.name)
		default:
			told = append(told, fmt.Sprintf("%s, hands you kept won %s and mulligans won %s.", s.name, pct(kept), pct(mull)))
		}
	}
	switch {
	case len(told) == 0 && len(even) == 2:
		return "Going first or second, keeping or mulliganing made little difference."
	case len(even) == 1:
		told = append(told, even[0]+" it made little difference.")
	case len(told) == 0:
		kept, ok1 := mu.All.Kept.Rate()
		mull, ok2 := mu.All.Mulliganed.Rate()
		if !ok1 || !ok2 {
			return "Too few games yet to compare kept hands with mulligans."
		}
		if math.Abs(kept-mull) < evenGap {
			return "Keeping or mulliganing made little difference."
		}
		return fmt.Sprintf("Hands you kept won %s and mulligans won %s.", pct(kept), pct(mull))
	}
	return strings.Join(told, " ")
}

// dealtPanel is a row for each card dealt in the first five. A row opens the hands it was
// dealt in.
func (v *view) dealtPanel(c *ui.Context, cs []stats.DealtCard, avg float64) {
	t := c.Theme()
	tv := arrangeTable(v, "hands.dealt", 12, v.dealtCols(t, avg), "dealt", true)
	tv.trail = chevronColW
	tv.tie = func(a, b stats.DealtCard) int { return cmp.Compare(b.Dealt(), a.Dealt()) }
	rows := tv.sorted(cs)
	v.foldTile(c, "hands.dealt", foldHead{label: "Cards in your first five"}, func() {
		ui.Text(c, "Each row counts the games the card was in the five you were dealt. Click a row to see those hands.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
		ui.Column(c).Children(func() {
			tv.header(c, 8)
			for _, d := range rows {
				v.dealtRow(c.Key(d.Card), tv, d)
			}
		})
	})
}

func (v *view) dealtCols(t *ui.Theme, avg float64) []tableCol[stats.DealtCard] {
	won := func(id, title, tip, key string, rec func(stats.DealtCard) stats.Record) tableCol[stats.DealtCard] {
		return tableCol[stats.DealtCard]{
			id: id, title: title, width: rateColW, tip: tip,
			by:   func(a, b stats.DealtCard) int { return cmp.Compare(sortRate(rec(a)), sortRate(rec(b))) },
			few:  func(d stats.DealtCard) bool { return !rec(d).Enough() },
			cell: func(c *ui.Context, d stats.DealtCard) { rateBar(c.Key(key), rec(d), avg, 0) },
		}
	}
	return []tableCol[stats.DealtCard]{
		{
			id: "card", title: "Card", asc: true,
			by: func(a, b stats.DealtCard) int { return cmp.Compare(v.snap.card(a.Card).Name, v.snap.card(b.Card).Name) },
			cell: func(c *ui.Context, d stats.DealtCard) {
				meta := d.Card
				if card := v.snap.card(d.Card); !card.IsLeader() {
					meta += fmt.Sprintf(" · cost %d", card.Cost)
				}
				v.cardNameCell(c, d.Card, meta)
			},
		},
		{
			id: "dealt", title: "Dealt", width: dealtColW,
			by:   func(a, b stats.DealtCard) int { return cmp.Compare(a.Dealt(), b.Dealt()) },
			cell: func(c *ui.Context, d stats.DealtCard) { ui.Text(c, fmt.Sprint(d.Dealt())).FontSize(13) },
		},
		{
			id: "kept", title: "Kept that hand", width: keptColW,
			by: func(a, b stats.DealtCard) int {
				return cmp.Compare(float64(a.Kept.Games())/float64(a.Dealt()), float64(b.Kept.Games())/float64(b.Dealt()))
			},
			cell: func(c *ui.Context, d stats.DealtCard) {
				dealt := d.Dealt()
				share := float32(d.Kept.Games()) / float32(dealt)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Grow(1).Basis(0).MinWidth(0).Children(func() {
					track, fill := t.Surface, t.Accent
					ui.Box(c).Height(6).Grow(1).Basis(0).MinWidth(20).Draw(func(p *ui.Painter, rc ui.Rect) {
						p.Fill(rc, track, 3)
						p.Fill(ui.Rect{X: rc.X, Y: rc.Y, W: max(share*rc.W, 2), H: rc.H}, fill, 3)
					})
					ui.Text(c, fmt.Sprintf("%d of %d", d.Kept.Games(), dealt)).FontSize(12).SingleLine()
				})
			},
		},
		won("wonKept", "Won, kept", "Win rate when you kept a hand with it.", "kept", func(d stats.DealtCard) stats.Record { return d.Kept }),
		won("wonSent", "Won, sent back", "Win rate when you sent back a hand with it.", "sent", func(d stats.DealtCard) stats.Record { return d.SentBack }),
	}
}

func (v *view) dealtRow(c *ui.Context, tv tableView[stats.DealtCard], d stats.DealtCard) {
	t := c.Theme()
	s := &v.decks
	card := v.snap.card(d.Card)
	open := s.handsCard == d.Card
	verb := "Show"
	if open {
		verb = "Hide"
	}
	row := ui.ButtonBase(c).Row().Gap(tv.gap).Padding(6, 8).AlignItems(ui.Center).Radius(6).
		Label(verb + " the hands dealt " + card.Name).Transition(ui.ElementTransition{Duration: hoverTime, Colors: true})
	switch {
	case open:
		row.Background(t.Accent.Alpha(0.06))
	case row.Hovered():
		row.Background(t.Surface.Alpha(0.5))
	default:
		row.Background(ui.Transparent)
	}
	if row.Clicked() {
		if open {
			s.handsCard = ""
		} else {
			s.handsCard, s.allHands = d.Card, false
		}
	}
	row.Children(func() {
		tv.cells(c, d)
		icon := iconChevron
		if open {
			icon = iconChevronUp
		}
		ui.Icon(c, icon).FontSize(12).TextColor(faintColor(t)).Width(chevronColW)
	})
	if open {
		v.dealtHands(c.Key("hands"), d)
	}
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.3))
}

// dealtHands are the games the card was dealt in, newest first, each with the five sent back
// before the five kept, and the card outlined.
func (v *view) dealtHands(c *ui.Context, d stats.DealtCard) {
	t := c.Theme()
	s := &v.decks
	card, ms := d.Card, d.Games
	shown := ms
	if !s.allHands && len(ms) > shownHands {
		shown = ms[:shownHands]
	}
	ui.Column(c).Gap(2).Padding(4, 16, 12, 16).Background(t.Accent.Alpha(0.06)).Children(func() {
		ui.Text(c, fmt.Sprintf("The %s you were dealt %s, newest first.", games(len(ms)), v.snap.card(card).Name)).
			FontSize(12).TextColor(t.TextMuted).Padding(0, 0, 4, 0)
		for _, m := range shown {
			v.handLine(c.Key(m.Fingerprint), m, card)
		}
		if len(shown) < len(ms) && linkButton(c, fmt.Sprintf("Show all %d", len(ms))) {
			s.allHands = true
		}
	})
}

// handLine is one game's opening: the result, the opponent, turn order and date, then the hand
// sent back and the hand kept. It opens the match.
func (v *view) handLine(c *ui.Context, m stats.Match, card string) {
	t := c.Theme()
	opp := "Opponent unknown"
	if m.Opp.Leader != "" {
		opp = "vs " + v.snap.card(m.Opp.Leader).Name
	}
	kept, sent := m.Me.Kept()
	v.gameLine(c, m).Gap(14).Padding(6, 4).Children(func() {
		resultBadge(c, m.Result)
		ui.Column(c).Width(handMetaW).Gap(1).Children(func() {
			ui.Text(c, opp).FontSize(13).FontWeight(600).SingleLine()
			when := v.dateTime(m.PlayedAt)
			if o := turnOrderShort(m); o != "" {
				when = o + " · " + when
			}
			ui.Text(c, when).FontSize(12).TextColor(faintColor(t)).SingleLine()
		})
		ui.Row(c).Width(handSize * (sentThumbW + 8)).Gap(4).AlignItems(ui.Center).Children(func() {
			if len(sent) == 0 {
				ui.Text(c, "Kept").FontSize(12).TextColor(faintColor(t))
				return
			}
			v.handCards(c, sent, card, sentThumbW, true)
		})
		ui.Icon(c, iconNextStep).FontSize(12).TextColor(faintColor(t)).Opacity(map[bool]float32{true: 1, false: 0}[len(sent) > 0])
		v.handCards(c.Key("kept"), kept, card, keptThumbW, false)
	})
}

// handCards draws the hand with every copy of card outlined.
func (v *view) handCards(c *ui.Context, hand []string, card string, w float32, dim bool) {
	t := c.Theme()
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		for i, code := range hand {
			edge := ui.Transparent
			if code == card {
				edge = t.Accent
			}
			box := ui.Box(c.Key(i)).Padding(2).Radius(6).Border(1.5, edge)
			if dim && code != card {
				box.Opacity(0.5)
			}
			box.Children(func() { v.card(c, code, w) })
		}
	})
}
