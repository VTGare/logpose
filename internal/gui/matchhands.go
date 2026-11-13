package gui

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

func verdict(p store.GamePlayer) string {
	if p.Mulliganed {
		return "mulliganed"
	}
	return "kept"
}

func handsTile(c *ui.Context, e *env, m stats.Match) ui.Element {
	t := c.Theme()
	side := func(key, who string, p store.GamePlayer) {
		ui.Column(c.Key(key)).Grow(1).Basis(0).Gap(6).Children(func() {
			hand, sent := p.Kept()
			if len(hand) == 0 {
				ui.Text(c, who).FontSize(12).Bold()
				ui.Text(c, "The log doesn't show this hand.").FontSize(12).TextColor(t.TextMuted)
				return
			}
			ui.Text(c, who+" "+verdict(p)).FontSize(12).FontWeight(600)
			thumbRow(c, e, hand, 60, false)
			if len(sent) > 0 {
				ui.Text(c, "Sent back").FontSize(11).TextColor(t.TextMuted)
				thumbRow(c, e, sent, 34, true)
			}
		})
	}
	you := mySide(m)
	sum := you.name() + " " + verdict(m.Me) + " · " + you.other().name() + " " + verdict(m.Opp)
	return e.foldTile(c, "match.hands", foldHead{label: "Opening hands", summary: sum}, func() {
		ui.Row(c).Gap(24).AlignItems(ui.Start).Children(func() {
			side("me", you.name(), m.Me)
			side("opp", you.other().name(), m.Opp)
		})
	})
}

func thumbRow(c *ui.Context, e *env, codes []string, width float32, dim bool) {
	ui.Row(c).Gap(4).Wrap().Children(func() {
		for i, code := range codes {
			box := ui.Box(c.Key(i)).Label(e.snap.card(code).Name).Children(func() { lift(e.card(c, code, width), 5) })
			if dim {
				box.Grayscale().Opacity(0.85)
			}
		}
	})
}
