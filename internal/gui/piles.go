package gui

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
)

// pileName is what the inspector and the mat call a pile, such as "Your deck" or "emog's
// trash".
func (sv *stepView) pileName(p pileRef) string {
	what := "deck"
	switch p.kind {
	case pileTrash:
		what = "trash"
	case pileLife:
		what = "life"
	}
	return sv.seat(p.mine).possessive() + " " + what
}

func (sv *stepView) pileSide(p pileRef) *replay.Side {
	if p.mine {
		return sv.me
	}
	return sv.them
}

// pileView is a deck, trash or life in the inspector. The deck and life show in order, top
// first, since that's the order they come off in. The trash shows its last cards, then every
// card in it with how many copies.
func (v *view) pileView(c *ui.Context, sv *stepView, width float32) {
	rs := sv.rs
	t := c.Theme()
	s := sv.pileSide(rs.pile)
	var cards []replay.Card
	switch rs.pile.kind {
	case pileDeck:
		cards = s.Deck()
	case pileLife:
		cards = s.Life
	default:
		cards = s.Trash
	}
	ui.Column(c).Grow(1).Basis(0).Gap(10).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Baseline).Children(func() {
				ui.Text(c, sv.pileName(rs.pile)).FontSize(15).FontWeight(700)
				ui.Text(c, cardCount(len(cards))).FontSize(12).TextColor(t.TextMuted)
				ui.Spacer(c)
				if ui.Text(c, "Show this step").FontSize(12).TextColor(t.Accent).Cursor(ui.CursorPointer).Clicked() {
					rs.pile = pileRef{}
				}
			})
			switch {
			case len(cards) == 0:
			case rs.pile.kind == pileDeck:
				ui.Text(c, "Top first. 1 is the next draw.").FontSize(12).TextColor(faintColor(t))
			case rs.pile.kind == pileLife:
				ui.Text(c, "Top first. 1 is the next to be taken.").FontSize(12).TextColor(faintColor(t))
			}
		})
		v.pileSwitches(c, sv)
		// The scroll bar is drawn over the content's right edge, so the content stops short of it.
		const bar = 14
		width -= bar
		ui.Scroll(c).Grow(1).Basis(0).Children(func() {
			ui.Column(c).Padding(0, bar, 0, 0).Children(func() { v.pileCards(c, sv, cards, width) })
		})
	})
}

func (v *view) pileCards(c *ui.Context, sv *stepView, cards []replay.Card, width float32) {
	rs := sv.rs
	t := c.Theme()
	switch {
	case len(cards) == 0:
		ui.Text(c, "No cards.").FontSize(12).TextColor(faintColor(t))
	case rs.pile.kind == pileDeck || rs.pile.kind == pileLife:
		v.deckGrid(c, cards, width)
	default:
		v.trashList(c, cards, width)
	}
}

func (*view) pileSwitches(c *ui.Context, sv *stepView) {
	rs := sv.rs
	if rs.pile.kind == pileNone {
		return
	}
	whose := 1
	if rs.pile.mine {
		whose = 0
	}
	kinds := []pileKind{pileDeck, pileTrash, pileLife}
	kind := slices.Index(kinds, rs.pile.kind)
	s := sv.pileSide(rs.pile)
	ui.Row(c).Gap(8).Wrap().Children(func() {
		ui.Segmented(c, &whose, sv.seat(true).name(), sv.seat(false).name()).Label("Whose pile")
		ui.Segmented(c, &kind, fmt.Sprintf("Deck %d", len(s.Deck())), fmt.Sprintf("Trash %d", len(s.Trash)),
			fmt.Sprintf("Life %d", len(s.Life))).Label("Pile")
	})
	picked := pileRef{kind: kinds[kind], mine: whose == 0}
	// The pile above the switches was drawn before the change, so the frame is drawn again.
	if picked != rs.pile {
		rs.pile = picked
		c.Invalidate()
	}
}

func (v *view) deckGrid(c *ui.Context, deck []replay.Card, width float32) {
	const cols, gap = 4, 8
	w := (width - gap*(cols-1)) / cols
	h := cardHeight(w)
	n := len(deck)
	ui.Column(c).Gap(gap).Children(func() {
		for r := 0; r*cols < n; r++ {
			ui.Row(c.Key(r)).Gap(gap).Children(func() {
				for i := r * cols; i < min(n, (r+1)*cols); i++ {
					card := deck[n-1-i]
					ui.Box(c.Key(card.ID)).Size(w, h).Children(func() {
						if card.Code == "" {
							cardBack(c, w, h).Label("Not named in the log").Tooltip("Not named in the log")
						} else {
							v.card(c, card.Code, w)
						}
						drawBadge(c, i+1)
					})
				}
			})
		}
	})
}

func drawBadge(c *ui.Context, n int) {
	t := c.Theme()
	b := ui.Text(c, fmt.Sprint(n)).FontSize(11).Bold().Padding(1, 6).Radius(9).Absolute().Left(4).Top(4)
	if n == 1 {
		b.Background(t.Accent).TextColor(t.AccentText)
	} else {
		b.Background(ui.RGBA(10, 10, 12, 0.85)).TextColor(t.Text)
	}
}

func (v *view) trashList(c *ui.Context, trash []replay.Card, width float32) {
	t := c.Theme()
	const last, gap = 5, 6
	w := (width - gap*(last-1)) / last
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "Last trashed").FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Gap(gap).Margin(0, 0, 8, 0).Children(func() {
			for i := len(trash) - 1; i >= max(0, len(trash)-last); i-- {
				ui.Box(c.Key(trash[i].ID)).Children(func() { v.card(c, trash[i].Code, w) })
			}
		})
		ui.Text(c, fmt.Sprintf("All %d, by card", len(trash))).FontSize(12).TextColor(t.TextMuted)
		for i, g := range v.byCard(trash) {
			row := ui.Row(c.Key(g.code)).Gap(10).AlignItems(ui.Center).Padding(5, 6).Radius(7)
			if i%2 == 0 {
				row.Background(ui.RGBA(255, 255, 255, 0.025))
			}
			row.Children(func() {
				v.artStrip(c, g.code, 44, 30)
				ui.Column(c).Grow(1).Basis(0).Children(func() {
					card := v.snap.card(g.code)
					ui.Text(c, card.Name).FontSize(13).FontWeight(600).SingleLine()
					if card.Type != "" {
						ui.Text(c, card.Type).FontSize(12).TextColor(faintColor(t))
					}
				})
				ui.Text(c, fmt.Sprintf("×%d", g.n)).Bold().TextColor(t.TextMuted)
			})
		}
	})
}

type cardCopies struct {
	code string
	n    int
}

func (v *view) byCard(cards []replay.Card) []cardCopies {
	var out []cardCopies
	for _, card := range cards {
		if i := slices.IndexFunc(out, func(g cardCopies) bool { return g.code == card.Code }); i >= 0 {
			out[i].n++
		} else {
			out = append(out, cardCopies{card.Code, 1})
		}
	}
	slices.SortStableFunc(out, func(a, b cardCopies) int {
		ca, cb := v.snap.card(a.code), v.snap.card(b.code)
		return cmp.Or(cmp.Compare(typeRank(ca.Type), typeRank(cb.Type)), cmp.Compare(ca.Name, cb.Name))
	})
	return out
}
