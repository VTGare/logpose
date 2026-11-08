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

// handsTile shows both opening hands side by side. A mulligan's first hand sits under the kept
// one, small and greyed like removed cards.
func (v *view) handsTile(c *ui.Context, m stats.Match) ui.Element {
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
			v.thumbRow(c, hand, 60, false)
			if len(sent) > 0 {
				ui.Text(c, "Sent back").FontSize(11).TextColor(t.TextMuted)
				v.thumbRow(c, sent, 34, true)
			}
		})
	}
	return tile(c, "Opening hands", func() {
		ui.Row(c).Gap(24).AlignItems(ui.Start).Children(func() {
			side("me", "You", m.Me)
			side("opp", m.Opp.Name, m.Opp)
		})
	})
}

func (v *view) thumbRow(c *ui.Context, codes []string, width float32, dim bool) {
	ui.Row(c).Gap(4).Wrap().Children(func() {
		for i, code := range codes {
			box := ui.Box(c.Key(i)).Label(v.snap.card(code).Name).Children(func() { v.cardThumb(c, code, width) })
			if dim {
				box.Grayscale().Opacity(0.85)
			}
		}
	})
}
