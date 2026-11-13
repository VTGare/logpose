package matches

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

func verdict(p store.GamePlayer) string {
	if p.Mulliganed {
		return "mulliganed"
	}
	return "kept"
}

func handsTile(c *ui.Context, e *page.Env, m stats.Match) ui.Element {
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
	you := page.MySide(m)
	sum := you.Name() + " " + verdict(m.Me) + " · " + you.Other().Name() + " " + verdict(m.Opp)
	return e.FoldTile(c, "match.hands", kit.FoldHead{Label: "Opening hands", Summary: sum}, func() {
		ui.Row(c).Gap(24).AlignItems(ui.Start).Children(func() {
			side("me", you.Name(), m.Me)
			side("opp", you.Other().Name(), m.Opp)
		})
	})
}

func thumbRow(c *ui.Context, e *page.Env, codes []string, width float32, dim bool) {
	ui.Row(c).Gap(4).Wrap().Children(func() {
		for i, code := range codes {
			box := ui.Box(c.Key(i)).Label(e.Snap.Card(code).Name).Children(func() { kit.Lift(e.Card(c, code, width), 5) })
			if dim {
				box.Grayscale().Opacity(0.85)
			}
		}
	})
}
