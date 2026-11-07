package gui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
)

// inspector shows the card clicked on the mat in full, with what is on it. With none clicked,
// it shows the step's own cards: the attacker and the card it attacks with their powers, or what
// the step played.
func (v *view) inspector(c *ui.Context, sv *stepView, width float32) {
	t := c.Theme()
	rs := v.detail.replay
	if rs == nil {
		return
	}
	if rs.pile.kind != pileNone {
		v.pileView(c, sv, rs, width)
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
		v.inspectCard(c, card, mine)
		return
	}
	ui.Text(c, "This step").FontSize(12).TextColor(t.TextMuted)
	bt := sv.step.Battle
	if bt != nil {
		ui.Row(c).Gap(8).Justify(ui.Center).AlignItems(ui.Start).Margin(8, 0, 0, 0).Children(func() {
			side := func(code string, p replay.Power, col ui.Color) {
				ui.Column(c.Key(code)).Grow(1).Basis(0).Gap(6).AlignItems(ui.Center).Children(func() {
					v.cardArt(c, code, 92, 129).Shadow(0, 6, 18, 0, panelShade).Border(2, col)
					if len(p.Parts) > 0 {
						ui.Text(c, powerText(p)).FontSize(22).Bold().TextColor(col).Tooltip(v.powerSum(p))
					}
					ui.Text(c, v.snap.card(code).Name).FontSize(12).TextColor(t.TextMuted).TextAlign(ui.Center).Wrap()
				})
			}
			attacker, defender := youColor, oppColor
			if sv.me.Owner != bt.AttackerOwner {
				attacker, defender = oppColor, youColor
			}
			side(bt.Attacker, bt.Attack, attacker)
			ui.Text(c, "vs").FontWeight(600).TextColor(faintColor(t)).Margin(60, 0, 0, 0)
			side(bt.Defender(), bt.Defense, defender)
		})
	} else if len(sv.step.Cards) > 0 {
		ui.Row(c).Gap(8).Justify(ui.Center).Margin(8, 0, 0, 0).Children(func() {
			for i, code := range sv.step.Cards[:min(len(sv.step.Cards), 2)] {
				ui.Box(c.Key(i)).Children(func() { v.cardArt(c, code, 120, 168).Shadow(0, 6, 18, 0, panelShade) })
			}
		})
	}
	ui.Text(c, "Click a card, deck or trash on the mat to see it here.").FontSize(12).TextColor(faintColor(t)).Margin(10, 0, 0, 0).Wrap()
}

// inspectCard is a card large, whose it is and where, and each status with its source and end.
func (v *view) inspectCard(c *ui.Context, card replay.Card, mine bool) {
	t := c.Theme()
	ui.Row(c).Justify(ui.Center).Margin(8, 0, 4, 0).Children(func() {
		v.cardArt(c, card.Code, 170, 238).Shadow(0, 10, 24, 0, panelShade)
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
	ui.Text(c, v.snap.card(card.Code).Name).FontWeight(700)
	ui.Text(c, whose).FontSize(12).TextColor(t.TextMuted)
	notes := v.statusNotes(card)
	for i, note := range notes {
		ui.Text(c.Key(i), note).FontSize(12).Wrap().Margin(4, 0, 0, 0)
	}
}

// findCard is the card with an ID on either side's board, and whether it's the user's.
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
