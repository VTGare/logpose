package matches

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/gui/kit"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/stats"
)

const (
	lethalAttackerWidth = 40
	lethalAnswerWidth   = 26
	lethalStepWidth     = 112
)

// lethalTurn lays out the attacks of the turn that ended the game, with what the attacking
// player did between them.
func lethalTurn(c *ui.Context, e *page.Env, m stats.Match, player string, attacks []replay.TurnAttack) {
	s := page.SideOf(m, player)
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Stretch).Children(func() {
		for i, a := range attacks {
			for j, act := range a.Before {
				lethalAction(c.Key(fmt.Sprintf("act%d.%d", i, j)), e, act)
			}
			lethalAttack(c.Key(i), e, s, i+1, a)
		}
	})
}

func lethalAttack(c *ui.Context, e *page.Env, s page.Side, n int, a replay.TurnAttack) {
	t := c.Theme()
	ui.Column(c).Width(lethalStepWidth).Gap(6).Padding(8, 9).Radius(10).Background(t.Surface).Children(func() {
		ui.Column(c).Children(func() {
			ui.Text(c, fmt.Sprintf("Attack %d", n)).FontSize(12).FontWeight(600)
			ui.Text(c, fmt.Sprintf("%d DON open", a.DonActive)).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
			e.Card(c, a.Attacker, lethalAttackerWidth)
			ui.Text(c, kilo(a.Power)).FontSize(18).Bold()
		})
		ui.Row(c).Gap(4).MinHeight(lethalAnswerWidth * 1.4).Children(func() {
			if len(a.Answers) == 0 {
				ui.Text(c, "—").FontSize(18).TextColor(kit.FaintColor(t))
			}
			for k, an := range a.Answers {
				card := e.Card(c.Key(k), an.Card, lethalAnswerWidth)
				if an.Kind == replay.AnswerBlock {
					card.Tooltip("Blocked")
				}
			}
		})
		ui.Text(c, answerText(e, a)).FontSize(12).TextColor(t.TextMuted).Wrap()
		word, color := "Stopped", t.TextMuted
		switch {
		case a.Lethal:
			word, color = "Lethal", s.Color()
		case a.Outcome == replay.OutcomeHit:
			word, color = "Hit", s.Color()
		case a.Outcome == replay.OutcomeKO:
			word = "K.O."
		}
		ui.Text(c, word).FontSize(14).Bold().TextColor(color)
		if a.Outcome == replay.OutcomeHit && !a.Lethal {
			ui.Text(c, fmt.Sprintf("%s at %d", isAt(s.Other()), a.Life)).FontSize(11).TextColor(t.TextMuted)
		}
	})
}

func isAt(s page.Side) string {
	if s.Mine {
		return "You're"
	}
	return s.Name() + " is"
}

// answerText says how the defender met an attack, as "Saint Gunko blocks +2k" or "Countered
// +3k".
func answerText(e *page.Env, a replay.TurnAttack) string {
	power := 0
	for _, an := range a.Answers {
		power += an.Power
	}
	plus := ""
	if power > 0 {
		plus = " +" + kilo(power)
	}
	switch {
	case a.Blocker != "":
		return e.Snap.Card(a.Blocker).Name + " blocks" + plus
	case power > 0:
		return "Countered" + plus
	}
	return "No answer"
}

func lethalAction(c *ui.Context, e *page.Env, act replay.Action) {
	t := c.Theme()
	ui.Column(c).Width(84).Gap(6).Padding(8, 6).Radius(10).Border(1, t.Border).AlignItems(ui.Center).
		Justify(ui.Center).Children(func() {
		e.Card(c, act.Card, lethalAnswerWidth+4)
		if act.Played {
			ui.Text(c, "Played").FontSize(11).TextColor(t.TextMuted)
		}
		for k, eff := range act.Effects {
			ui.Text(c.Key(k), eff.Text).FontSize(12).TextAlign(ui.Center).Wrap()
		}
	})
}

// kilo writes power as "7k", or "7.5k" off the thousand.
func kilo(n int) string {
	if n%1000 == 0 {
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}
