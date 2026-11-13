package matches

import (
	"fmt"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
)

// With no card clicked, inspector shows the step's own cards: the attacker and the card it
// attacks with their powers, or what the step played.
func inspector(c *ui.Context, e *page.Env, sv *stepView, width float32) {
	t := c.Theme()
	rs := sv.rs
	if rs.pile.kind != pileNone {
		pileView(c, e, sv, width)
		return
	}
	if card, mine, ok := findCard(sv, rs.selected); ok {
		ui.Row(c).Children(func() {
			ui.Text(c, "Selected card").FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Text(c, "Show this step").FontSize(12).TextColor(t.Accent).Clicked() {
				rs.selected = 0
			}
		})
		inspectCard(c, e, card, mine)
		return
	}
	ui.Text(c, "This step").FontSize(12).TextColor(t.TextMuted)
	bt := sv.step.Battle
	if bt != nil {
		ui.Row(c).Gap(8).Justify(ui.Center).AlignItems(ui.Start).Margin(8, 0, 0, 0).Children(func() {
			side := func(code string, p replay.Power, col ui.Color) {
				ui.Column(c.Key(code)).Grow(1).Basis(0).Gap(6).AlignItems(ui.Center).Children(func() {
					e.Card(c, code, 92).Shadow(0, 6, 18, 0, kit.PanelShade).Border(2, col)
					if len(p.Parts) > 0 {
						ui.Text(c, powerText(p)).FontSize(22).Bold().TextColor(col).Tooltip(powerSum(e, p))
					}
					ui.Text(c, e.Snap.Card(code).Name).FontSize(12).TextColor(t.TextMuted).TextAlign(ui.Center).Wrap()
				})
			}
			attacker := sv.attacker()
			side(bt.Attacker, bt.Attack, attacker.Color())
			ui.Text(c, "vs").FontWeight(600).TextColor(kit.FaintColor(t)).Margin(60, 0, 0, 0)
			side(bt.Defender(), bt.Defense, attacker.Other().Color())
		})
	} else if len(sv.step.Cards) > 0 {
		ui.Row(c).Gap(8).Justify(ui.Center).Margin(8, 0, 0, 0).Children(func() {
			for i, code := range sv.step.Cards[:min(len(sv.step.Cards), 2)] {
				ui.Box(c.Key(i)).Children(func() { e.Card(c, code, 120).Shadow(0, 6, 18, 0, kit.PanelShade) })
			}
		})
	}
	ui.Text(c, "Click a card, deck or trash on the mat to see it here.").FontSize(12).TextColor(kit.FaintColor(t)).Margin(10, 0, 0, 0).Wrap()
}

func inspectCard(c *ui.Context, e *page.Env, card replay.Card, mine bool) {
	t := c.Theme()
	ui.Row(c).Justify(ui.Center).Margin(8, 0, 4, 0).Children(func() {
		e.Card(c, card.Code, 170).Shadow(0, 10, 24, 0, kit.PanelShade)
	})
	whose := "Your card"
	if !mine {
		whose = "Their card"
	}
	if card.Rested {
		whose += " · rested"
	}
	if card.Don > 0 {
		whose += fmt.Sprintf(" · DON!! ×%d", card.Don)
	}
	ui.Text(c, e.Snap.Card(card.Code).Name).FontWeight(700)
	ui.Text(c, whose).FontSize(12).TextColor(t.TextMuted)
	notes := statusNotes(e, card)
	for i, note := range notes {
		ui.Text(c.Key(i), note).FontSize(12).Wrap().Margin(4, 0, 0, 0)
	}
}

// findCard is the card with an ID on either side's board or in their trash, and whether it's
// the user's.
func findCard(sv *stepView, id int) (replay.Card, bool, bool) {
	if id == 0 {
		return replay.Card{}, false, false
	}
	for _, s := range []*replay.Side{sv.me, sv.them} {
		zone := append(append(append([]replay.Card{s.Leader}, s.Characters...), s.Stage...), s.Trash...)
		for _, card := range zone {
			if card.ID == id {
				return card, s == sv.me, true
			}
		}
	}
	return replay.Card{}, false, false
}
