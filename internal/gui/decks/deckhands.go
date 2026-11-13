package decks

import (
	"cmp"
	"fmt"
	"math"
	"strings"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

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
func (pg *Page) handsTab(c *ui.Context, e *page.Env, d *deckData) {
	mu := d.mulligans
	all := d.stats.Record
	if all.Games() == 0 {
		kit.Panel(c).Padding(16).Children(func() { noGames(c, d) })
		return
	}
	avg := all.Share()
	keepTile(c, e, mu, avg)
	pg.dealtPanel(c, e, mu.Cards, avg)
	pg.handTester(c, e, d.deckGames, mu.Cards)
}

// keepTile compares kept and mulliganed hands going first, going second and in all games.
func keepTile(c *ui.Context, e *page.Env, mu stats.Mulligans, avg float64) {
	t := c.Theme()
	sum := fmt.Sprintf("Kept %s · mulliganed %s", page.RateRecord(mu.All.Kept), page.RateRecord(mu.All.Mulliganed))
	e.FoldTile(c, "hands.keep", kit.FoldHead{Label: "Keep or mulligan", Summary: sum}, func() {
		ui.Text(c, KeepFinding(mu)).FontSize(14).Wrap()
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
					ui.Text(c, fmt.Sprintf("%d of %d sent back", s.k.Mulliganed.Games(), n)).FontSize(12).TextColor(kit.FaintColor(t))
				})
				ui.Box(c).Grow(1).Basis(0).Children(func() { rateBar(c.Key("kept"), s.k.Kept, avg, keepBarW) })
				ui.Box(c).Grow(1).Basis(0).Children(func() { rateBar(c.Key("mull"), s.k.Mulliganed, avg, keepBarW) })
			})
		}
		if line := oppLine(mu.Opp); line != "" {
			ui.Text(c, line).FontSize(13).TextColor(t.TextMuted).Wrap().Padding(6, 0, 0, 0)
		}
		ui.Text(c, fmt.Sprintf("You send back hands you think are weak, so this compares the hands, not the choice. "+
			"The tick is the %s these games win, and the bar is where each rate likely is.", kit.Pct(avg))).
			FontSize(12).TextColor(kit.FaintColor(t)).Wrap().Padding(4, 0, 0, 0)
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
		kit.Pct(mull), kit.Record(k.Mulliganed), kit.Pct(kept), kit.Record(k.Kept))
}

// KeepFinding says in words where keeping and mulliganing differ by turn order, or in all games
// when neither turn order has enough games of both.
func KeepFinding(mu stats.Mulligans) string {
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
			told = append(told, fmt.Sprintf("%s, hands you kept won %s and mulligans won %s.", s.name, kit.Pct(kept), kit.Pct(mull)))
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
		return fmt.Sprintf("Hands you kept won %s and mulligans won %s.", kit.Pct(kept), kit.Pct(mull))
	}
	return strings.Join(told, " ")
}

// dealtPanel is a row for each card dealt in the first five. A row opens the hands it was
// dealt in.
func (pg *Page) dealtPanel(c *ui.Context, e *page.Env, cs []stats.DealtCard, avg float64) {
	t := c.Theme()
	tv := kit.ArrangeTable(e, "hands.dealt", 12, dealtCols(e, t, avg), "dealt", true)
	tv.Trail = chevronColW
	tv.Tie = func(a, b stats.DealtCard) int { return cmp.Compare(b.Dealt(), a.Dealt()) }
	rows := tv.Sorted(cs)
	e.FoldTile(c, "hands.dealt", kit.FoldHead{Label: "Cards in your first five"}, func() {
		ui.Text(c, "Each row counts the games the card was in the five you were dealt. Click a row to see those hands.").
			FontSize(12).TextColor(t.TextMuted).Wrap()
		ui.Column(c).Children(func() {
			tv.Header(c, 8)
			for _, d := range rows {
				pg.dealtRow(c.Key(d.Card), e, tv, d)
			}
		})
	})
}

func dealtCols(e *page.Env, t *ui.Theme, avg float64) []kit.TableCol[stats.DealtCard] {
	won := func(id, title, tip, key string, rec func(stats.DealtCard) stats.Record) kit.TableCol[stats.DealtCard] {
		return kit.TableCol[stats.DealtCard]{
			ID: id, Title: title, Width: rateColW, Tip: tip,
			By:   func(a, b stats.DealtCard) int { return cmp.Compare(page.SortRate(rec(a)), page.SortRate(rec(b))) },
			Few:  func(d stats.DealtCard) bool { return !rec(d).Enough() },
			Cell: func(c *ui.Context, d stats.DealtCard) { rateBar(c.Key(key), rec(d), avg, 0) },
		}
	}
	return []kit.TableCol[stats.DealtCard]{
		{
			ID: "card", Title: "Card", Asc: true,
			By: func(a, b stats.DealtCard) int { return cmp.Compare(e.Snap.Card(a.Card).Name, e.Snap.Card(b.Card).Name) },
			Cell: func(c *ui.Context, d stats.DealtCard) {
				meta := d.Card
				if card := e.Snap.Card(d.Card); !card.IsLeader() {
					meta += fmt.Sprintf(" · cost %d", card.Cost)
				}
				cardNameCell(c, e, d.Card, meta)
			},
		},
		{
			ID: "dealt", Title: "Dealt", Width: dealtColW,
			By:   func(a, b stats.DealtCard) int { return cmp.Compare(a.Dealt(), b.Dealt()) },
			Cell: func(c *ui.Context, d stats.DealtCard) { ui.Text(c, fmt.Sprint(d.Dealt())).FontSize(13) },
		},
		{
			ID: "kept", Title: "Kept that hand", Width: keptColW,
			By: func(a, b stats.DealtCard) int {
				return cmp.Compare(float64(a.Kept.Games())/float64(a.Dealt()), float64(b.Kept.Games())/float64(b.Dealt()))
			},
			Cell: func(c *ui.Context, d stats.DealtCard) {
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

func (pg *Page) dealtRow(c *ui.Context, e *page.Env, tv kit.TableView[stats.DealtCard], d stats.DealtCard) {
	t := c.Theme()
	s := pg
	card := e.Snap.Card(d.Card)
	open := s.HandsCard == d.Card
	verb := "Show"
	if open {
		verb = "Hide"
	}
	row := ui.ButtonBase(c).Row().Gap(tv.Gap).Padding(6, 8).AlignItems(ui.Center).Radius(6).
		Label(verb + " the hands dealt " + card.Name).Transition(ui.ElementTransition{Duration: kit.HoverTime, Colors: true})
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
			s.HandsCard = ""
		} else {
			s.HandsCard, s.AllHands = d.Card, false
		}
	}
	row.Children(func() {
		tv.Cells(c, d)
		icon := kit.IconChevron
		if open {
			icon = kit.IconChevronUp
		}
		ui.Icon(c, icon).FontSize(12).TextColor(kit.FaintColor(t)).Width(chevronColW)
	})
	if open {
		pg.dealtHands(c.Key("hands"), e, d)
	}
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.3))
}

// dealtHands are the games the card was dealt in, newest first, each with the five sent back
// before the five kept, and the card outlined.
func (pg *Page) dealtHands(c *ui.Context, e *page.Env, d stats.DealtCard) {
	t := c.Theme()
	s := pg
	card, ms := d.Card, d.Games
	shown := ms
	if !s.AllHands && len(ms) > shownHands {
		shown = ms[:shownHands]
	}
	ui.Column(c).Gap(2).Padding(4, 16, 12, 16).Background(t.Accent.Alpha(0.06)).Children(func() {
		ui.Text(c, fmt.Sprintf("The %s you were dealt %s, newest first.", kit.Games(len(ms)), e.Snap.Card(card).Name)).
			FontSize(12).TextColor(t.TextMuted).Padding(0, 0, 4, 0)
		for _, m := range shown {
			handLine(c.Key(m.Fingerprint), e, m, card)
		}
		if len(shown) < len(ms) && linkButton(c, fmt.Sprintf("Show all %d", len(ms))) {
			s.AllHands = true
		}
	})
}

// handLine is one game's opening: the result, the opponent, turn order and date, then the hand
// sent back and the hand kept. It opens the match.
func handLine(c *ui.Context, e *page.Env, m stats.Match, card string) {
	t := c.Theme()
	opp := "Opponent unknown"
	if m.Opp.Leader != "" {
		opp = "vs " + e.Snap.Card(m.Opp.Leader).Name
	}
	kept, sent := m.Me.Kept()
	e.GameLine(c, m).Gap(14).Padding(6, 4).Children(func() {
		page.ResultBadge(c, m.Result)
		ui.Column(c).Width(handMetaW).Gap(1).Children(func() {
			ui.Text(c, opp).FontSize(13).FontWeight(600).SingleLine()
			when := e.DateTime(m.PlayedAt)
			if o := page.TurnOrderShort(m); o != "" {
				when = o + " · " + when
			}
			ui.Text(c, when).FontSize(12).TextColor(kit.FaintColor(t)).SingleLine()
		})
		ui.Row(c).Width(handSize * (sentThumbW + 8)).Gap(4).AlignItems(ui.Center).Children(func() {
			if len(sent) == 0 {
				ui.Text(c, "Kept").FontSize(12).TextColor(kit.FaintColor(t))
				return
			}
			handCards(c, e, sent, card, sentThumbW, true)
		})
		ui.Icon(c, kit.IconNextStep).FontSize(12).TextColor(kit.FaintColor(t)).Opacity(map[bool]float32{true: 1, false: 0}[len(sent) > 0])
		handCards(c.Key("kept"), e, kept, card, keptThumbW, false)
	})
}

// handCards draws the hand with every copy of card outlined.
func handCards(c *ui.Context, e *page.Env, hand []string, card string, w float32, dim bool) {
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
			box.Children(func() { e.Card(c, code, w) })
		}
	})
}
