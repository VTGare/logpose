package gui

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/stats"
)

// Opening hand size, the most DON!! a player has, and the last turn the tester draws to.
const (
	handSize  = 5
	maxDon    = 10
	lastTurn  = 8
	firstDons = 1
)

// handTest is a practice deal from a list: the hand, what's left of the deck, the hand sent back
// by the one mulligan, and the player's own turn, 0 before the first.
type handTest struct {
	list       string
	deck, hand []string
	sent       []string
	mulliganed bool
	turn       int
	second     bool
}

// listKey names a list by its cards and copies, so a new list deals a new hand.
func listKey(list map[string]int) string {
	codes := make([]string, 0, len(list))
	for c, n := range list {
		codes = append(codes, fmt.Sprintf("%s×%d", c, n))
	}
	slices.Sort(codes)
	return strings.Join(codes, ",")
}

// deal shuffles the list and draws five, keeping the turn order.
func (h *handTest) deal(list map[string]int, r *rand.Rand) {
	deck := make([]string, 0, 50)
	for c, n := range list {
		for range n {
			deck = append(deck, c)
		}
	}
	slices.Sort(deck)
	r.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	n := min(handSize, len(deck))
	*h = handTest{list: listKey(list), deck: deck[n:], hand: deck[:n:n], second: h.second}
}

// mulligan puts the hand back, shuffles and draws five. There's one, before the first turn.
func (h *handTest) mulligan(r *rand.Rand) {
	if !h.canMulligan() {
		return
	}
	deck := append(slices.Clone(h.deck), h.hand...)
	r.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	n := min(handSize, len(deck))
	h.sent, h.hand, h.deck, h.mulliganed = h.hand, deck[:n:n], deck[n:], true
}

func (h *handTest) canMulligan() bool { return !h.mulliganed && h.turn == 0 }

// next starts the player's next turn and draws, except on the first turn going first.
func (h *handTest) next() {
	if h.turn >= lastTurn {
		return
	}
	h.turn++
	if (h.second || h.turn > 1) && len(h.deck) > 0 {
		h.hand = append(h.hand, h.deck[0])
		h.deck = h.deck[1:]
	}
}

// don is the DON!! the player has on their turn: 1 then 2 more a turn going first, 2 more a turn
// going second, up to 10.
func (h *handTest) don() int {
	if h.turn == 0 {
		return 0
	}
	d := 2 * h.turn
	if !h.second {
		d -= firstDons
	}
	return min(d, maxDon)
}

// testerCardW is a card's width in the tester, and testerSmallW its width once the hand has
// grown past what fits.
const (
	testerCardW  = 124
	testerSmallW = 100
	testerWide   = 6
	sentThumbs   = 30
	donPipW      = 9
)

// handTester deals from the scope's latest list and plays it out turn by turn, with each card's
// record from the games it was dealt in.
func (v *view) handTester(c *ui.Context, d leaderDecks, dealt []stats.DealtCard) {
	t := c.Theme()
	s := &v.decks
	gs := d.scopedGames(s.scope)
	if len(gs) == 0 {
		return
	}
	list := gs[len(gs)-1].Facts.List
	if s.rng == nil {
		s.rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}
	h := &s.tester
	if h.list != listKey(list) {
		h.deal(list, s.rng)
	}
	if c.Shortcut(0, ui.KeyN) {
		h.deal(list, s.rng)
	}
	if c.Shortcut(0, ui.KeyM) {
		h.mulligan(s.rng)
	}
	if c.Shortcut(0, ui.KeyT) {
		h.next()
	}
	byCard := make(map[string]stats.DealtCard, len(dealt))
	for _, dc := range dealt {
		byCard[dc.Card] = dc
	}
	n := 0
	for _, k := range list {
		n += k
	}
	note := "From the latest list. Mulligan once, then draw turn by turn."
	if n < stats.DeckSize {
		note = fmt.Sprintf("From the latest list, of which the logs name %d cards. Mulligan once, then draw turn by turn.", n)
	}
	v.foldTile(c, "hands.try", foldHead{label: "Try a hand"}, func() {
		ui.Text(c, note).FontSize(12).TextColor(t.TextMuted)
		v.testerControls(c, list)
		if len(h.sent) > 0 {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Sent back").FontSize(12).TextColor(faintColor(t))
				v.handCards(c, h.sent, "", sentThumbs, true)
			})
		}
		w := float32(testerCardW)
		if len(h.hand) > testerWide {
			w = testerSmallW
		}
		hide := s.hideNumbers && !h.mulliganed && h.turn == 0
		ui.Row(c).Gap(10).Wrap().Children(func() {
			for i, code := range h.hand {
				v.testerCard(c.Key(i), code, byCard[code], w, hide, h.turn > 0 && max(v.snap.card(code).Cost, 0) <= h.don())
			}
		})
		hint := "Keep, or mulligan once."
		switch {
		case h.turn > 0:
			hint = "Outlined cards cost no more than this turn's DON!!. Printed costs only: leader and card effects aren't applied."
		case h.mulliganed:
			hint = "This hand has to be kept."
		}
		ui.Text(c, hint).FontSize(12).TextColor(faintColor(t)).Wrap()
	})
}

func (v *view) testerControls(c *ui.Context, list map[string]int) {
	t := c.Theme()
	s := &v.decks
	h := &s.tester
	ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		order := 0
		if h.second {
			order = 1
		}
		if ui.Segmented(c, &order, "Going first", "Going second").Changed() {
			h.second = order == 1
			h.deal(list, s.rng)
		}
		if ui.Button(c, "New hand").Tooltip("N").Clicked() {
			h.deal(list, s.rng)
		}
		if ui.Button(c, "Mulligan").Tooltip("M").Disabled(!h.canMulligan()).Clicked() {
			h.mulligan(s.rng)
		}
		next := "Next turn"
		if h.turn == 0 {
			next = "Keep"
		}
		if ui.Button(c, next).Tooltip("T").Disabled(h.turn >= lastTurn).Clicked() {
			h.next()
		}
		ui.Checkbox(c, &s.hideNumbers, "Hide numbers until I choose")
		ui.Spacer(c)
		turn := "Before your first turn"
		if h.turn > 0 {
			turn = fmt.Sprintf("Your turn %d · %d DON!!", h.turn, h.don())
		}
		ui.Text(c, turn).FontSize(13).TextColor(t.TextMuted)
		ui.Row(c).Gap(3).Children(func() {
			for i := range h.don() {
				ui.Box(c.Key(i)).Size(donPipW, 14).Radius(2).Background(warnColor)
			}
		})
	})
}

// testerCard is a card of the hand with its cost and its record from the games it was dealt in,
// outlined when it can be paid for this turn.
func (v *view) testerCard(c *ui.Context, code string, dc stats.DealtCard, w float32, hide, payable bool) {
	t := c.Theme()
	card := v.snap.card(code)
	edge := ui.Transparent
	if payable {
		edge = winColor
	}
	ui.Column(c).Width(w + 6).Gap(3).Children(func() {
		ui.Box(c).Padding(2).Radius(8).Border(2, edge).Children(func() { v.cardArt(c, code, w, w*7/5) })
		ui.Text(c, card.Name).FontSize(13).FontWeight(600).SingleLine()
		ui.Text(c, fmt.Sprintf("Cost %d", max(card.Cost, 0))).FontSize(12).TextColor(faintColor(t))
		switch {
		case hide:
			ui.Text(c, "Choose first").FontSize(12).TextColor(faintColor(t))
		case dc.Dealt() == 0:
			ui.Text(c, "Never dealt in your games").FontSize(12).TextColor(faintColor(t)).Wrap()
		default:
			sent := "Sent back"
			if w < testerCardW {
				sent = "Sent"
			}
			recordLine(c.Key("kept"), "Kept", dc.Kept)
			recordLine(c.Key("sent"), sent, dc.SentBack)
		}
	})
}

// recordLine is a label with a record and its rate, or "few" below MinGames.
func recordLine(c *ui.Context, label string, r stats.Record) {
	t := c.Theme()
	ui.Row(c).Gap(4).AlignItems(ui.End).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).SingleLine()
		if r.Games() == 0 {
			ui.Text(c, "–").FontSize(12).TextColor(faintColor(t))
			return
		}
		ui.Text(c, record(r)).FontSize(12).FontWeight(600).SingleLine()
		rate := "few"
		if x, ok := r.Rate(); ok {
			rate = pct(x)
		}
		ui.Text(c, "· "+rate).FontSize(12).TextColor(t.TextMuted).SingleLine()
	})
}
