package matches

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/deck"
	"github.com/VTGare/logpose/internal/gui/kit"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/stats"
)

const decksWideMin = 720

func (pg *Page) decksTile(c *ui.Context, e *page.Env, m stats.Match) ui.Element {
	t := c.Theme()
	l := pg.Detail.timeline
	you := page.MySide(m)
	oppList, myList := deckLists(l)
	oppTotal, myTotal := deck.Count(oppList), deck.Count(myList)
	sum := decksSummary(you, oppTotal, myTotal)

	return e.FoldTile(c, "match.decks", kit.FoldHead{Label: "Decks", Summary: sum}, func() {
		body := ui.Column(c.Key("body")).Gap(14).MinWidth(0)
		// Bounds is empty on the first frame, so the layout keeps its last width until measured.
		if b := body.Bounds(); b.W > 0 && b.W != pg.decksWidth {
			pg.decksWidth = b.W
			c.Invalidate()
		}
		body.Children(func() {
			switch {
			case oppTotal == 0 && myTotal == 0 && l == nil:
				ui.Text(c, "Reading the log").TextColor(t.TextMuted)
				return
			case oppTotal == 0 && myTotal == 0:
				ui.Text(c, "The log doesn't include decklists for this game.").TextColor(t.TextMuted)
				return
			}

			oppLeaderName := e.Snap.Card(m.Opp.Leader).Name
			myLeaderName := e.Snap.Card(m.Me.Leader).Name
			leader, list, total, copyLabel := m.Opp.Leader, oppList, oppTotal, oppCopyLabel(you, oppLeaderName)
			if pg.Detail.decksMine {
				leader, list, total, copyLabel = m.Me.Leader, myList, myTotal, myLeaderName
			}

			ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
				pg.deckSideSwitcher(c, you, oppLeaderName, myLeaderName, l != nil && oppTotal == 0, l != nil && myTotal == 0)
				ui.Spacer(c)
				copyBtn := kit.IconButton(c, kit.IconCopy, "Copy for sim").Label("Copy for sim")
				if leader == "" || total == 0 {
					copyBtn.Disabled(true)
				} else if copyBtn.Clicked() {
					page.CopyDeckForSim(e, leader, copyLabel, list)
				}
			})

			if total == 0 {
				msg := "The log doesn't include this decklist."
				if l == nil {
					msg = "Reading the log"
				}
				ui.Text(c, msg).TextColor(t.TextMuted)
				return
			}

			if note := page.DeckPartialNote(total); note != "" {
				page.DeckPartialBanner(c, note)
			}
			st := stats.List(list, e.Snap.Card)
			if pg.decksWidth == 0 || pg.decksWidth >= decksWideMin {
				ui.Row(c.Key("wide")).Gap(20).AlignItems(ui.Start).Children(func() {
					ui.Box(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
						page.DeckStacks(c, e, leader, list, nil)
					})
					page.DeckListStatsRail(c, st)
				})
				return
			}
			ui.Column(c.Key("narrow")).Gap(14).Children(func() {
				page.DeckStacks(c, e, leader, list, nil)
				ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
				page.DeckListStatsGrid(c, st)
			})
		})
	})
}

func (pg *Page) deckSideSwitcher(c *ui.Context, you page.Side, oppLeader, myLeader string, oppEmpty, myEmpty bool) {
	t := c.Theme()
	items := [2]struct {
		mine  bool
		label string
		color ui.Color
		empty bool
	}{
		{mine: false, label: you.Other().Name() + " · " + oppLeader, color: you.Other().Color(), empty: oppEmpty},
		{mine: true, label: you.Name() + " · " + myLeader, color: you.Color(), empty: myEmpty},
	}
	ui.Row(c).Background(t.Background).Border(1, t.Border).Radius(8).Padding(3).Gap(2).Children(func() {
		for i, item := range items {
			active := pg.Detail.decksMine == item.mine
			label := item.label
			if item.empty {
				label += " (no list)"
			}
			btn := ui.ButtonBase(c.Key(i)).Label(label).Row().Gap(6).AlignItems(ui.Center).
				Padding(4, 10).Radius(6).Background(ui.Transparent).
				Transition(ui.ElementTransition{Duration: kit.HoverTime, Colors: true})
			switch {
			case active:
				btn.Background(t.Surface)
			case btn.Hovered():
				btn.Background(t.Surface.Alpha(0.5))
			}
			if item.empty && !active {
				btn.Opacity(0.55)
			}
			if btn.Clicked() {
				pg.Detail.decksMine = item.mine
			}
			btn.Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(item.color)
				txt := ui.Text(c, label).FontSize(12).SingleLine()
				if active {
					txt.Bold().TextColor(t.Text)
				} else {
					txt.TextColor(t.TextMuted)
				}
			})
		}
	})
}

func deckLists(l *MatchTimeline) (opp, mine map[string]int) {
	if l == nil {
		return nil, nil
	}
	return l.OppList, l.MyList
}

func decksSummary(you page.Side, oppTotal, myTotal int) string {
	var parts []string
	if oppTotal > 0 {
		parts = append(parts, fmt.Sprintf("%s: %s", you.Other().Name(), deckCountText(oppTotal)))
	}
	if myTotal > 0 {
		parts = append(parts, fmt.Sprintf("%s: %s", you.Name(), deckCountText(myTotal)))
	}
	return strings.Join(parts, " · ")
}

func deckCountText(n int) string {
	if n < stats.DeckSize {
		return fmt.Sprintf("%d of %d cards", n, stats.DeckSize)
	}
	return kit.Cards(n)
}

func oppCopyLabel(you page.Side, leader string) string {
	if name := you.Other().Name(); name != "" {
		return name + " · " + leader
	}
	return leader
}
