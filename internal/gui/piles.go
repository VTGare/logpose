package gui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
)

// pileName is what the inspector and the mat call a pile, such as "Your deck" or "emog's
// trash".
func (sv *stepView) pileName(p pileRef) string {
	what := "deck"
	if p.kind == pileTrash {
		what = "trash"
	}
	if p.mine {
		return "Your " + what
	}
	return sv.who(sv.them.Player) + "'s " + what
}

func (sv *stepView) pileSide(p pileRef) *replay.Side {
	if p.mine {
		return sv.me
	}
	return sv.them
}

// pileView is a deck or trash in the inspector. The deck shows in order, top first, since
// its order is the order of the draws to come. The trash shows its last cards, then every card
// in it with how many copies.
func (v *view) pileView(c *ui.Context, sv *stepView, rs *replayState, width float32) {
	t := c.Theme()
	s := sv.pileSide(rs.pile)
	var cards []replay.Card
	if rs.pile.kind == pileDeck {
		cards = s.Deck()
	} else {
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
			if rs.pile.kind == pileDeck && len(cards) > 0 {
				ui.Text(c, "Top first. 1 is the next draw.").FontSize(12).TextColor(faintColor(t))
			}
		})
		v.pileSwitches(c, sv, rs)
		// The scroll bar is drawn over the content's right edge, so the content stops short of it.
		const bar = 14
		width -= bar
		ui.Scroll(c).Grow(1).Basis(0).Children(func() {
			ui.Column(c).Padding(0, bar, 0, 0).Children(func() { v.pileCards(c, rs, cards, width) })
		})
	})
}

func (v *view) pileCards(c *ui.Context, rs *replayState, cards []replay.Card, width float32) {
	t := c.Theme()
	switch {
	case len(cards) == 0:
		ui.Text(c, "No cards.").FontSize(12).TextColor(faintColor(t))
	case rs.pile.kind == pileDeck:
		v.deckGrid(c, cards, width)
	default:
		v.trashList(c, cards, width)
	}
}

// pileSwitches pick whose pile and which one.
func (*view) pileSwitches(c *ui.Context, sv *stepView, rs *replayState) {
	if rs.pile.kind == pileNone {
		return
	}
	whose := 1
	if rs.pile.mine {
		whose = 0
	}
	kind := 0
	if rs.pile.kind == pileTrash {
		kind = 1
	}
	s := sv.pileSide(rs.pile)
	ui.Row(c).Gap(8).Wrap().Children(func() {
		ui.Segmented(c, &whose, "You", sv.who(sv.them.Player)).Label("Whose pile")
		ui.Segmented(c, &kind, fmt.Sprintf("Deck %d", s.DeckCount()), fmt.Sprintf("Trash %d", len(s.Trash))).Label("Pile")
	})
	picked := pileRef{kind: pileDeck, mine: whose == 0}
	if kind == 1 {
		picked.kind = pileTrash
	}
	// The pile above the switches was drawn before the change, so the frame is drawn again.
	if picked != rs.pile {
		rs.pile = picked
		c.Invalidate()
	}
}

// deckGrid is the deck four to a row, top first and numbered, with the next draw marked.
func (v *view) deckGrid(c *ui.Context, deck []replay.Card, width float32) {
	const cols, gap = 4, 8
	w := (width - gap*(cols-1)) / cols
	h := w * 7 / 5
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
							v.cardArt(c, card.Code, w, h)
						}
						drawBadge(c, i+1)
					})
				}
			})
		}
	})
}

// drawBadge numbers a deck card by how many draws away it is.
func drawBadge(c *ui.Context, n int) {
	t := c.Theme()
	b := ui.Text(c, fmt.Sprint(n)).FontSize(11).Bold().Padding(1, 6).Radius(9).Absolute().Left(4).Top(4)
	if n == 1 {
		b.Background(t.Accent).TextColor(t.AccentText)
	} else {
		b.Background(ui.RGBA(10, 10, 12, 0.85)).TextColor(t.Text)
	}
}

// trashList is the last five cards trashed, newest first, then every card in the trash with
// its copies, characters first and then by name.
func (v *view) trashList(c *ui.Context, trash []replay.Card, width float32) {
	t := c.Theme()
	const last, gap = 5, 6
	w := (width - gap*(last-1)) / last
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "Last trashed").FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Gap(gap).Margin(0, 0, 8, 0).Children(func() {
			for i := len(trash) - 1; i >= max(0, len(trash)-last); i-- {
				ui.Box(c.Key(trash[i].ID)).Children(func() { v.cardArt(c, trash[i].Code, w, w*7/5) })
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

// typeRank orders cards as a decklist does, with types the card data lacks last.
func typeRank(kind string) int {
	switch strings.ToLower(kind) {
	case "character":
		return 0
	case "event":
		return 1
	case "stage":
		return 2
	}
	return 3
}

func cardCount(n int) string {
	if n == 1 {
		return "1 card"
	}
	return fmt.Sprintf("%d cards", n)
}
