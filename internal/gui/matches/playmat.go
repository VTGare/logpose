package matches

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
)

var (
	zoneLine = ui.RGBA(255, 255, 255, 0.09)
	backFace = ui.Hex("#1E2A3D")
	backEdge = ui.RGBA(191, 227, 240, 0.22)
	donGold  = ui.Hex("#C9A44C")
	donInk   = ui.Hex("#1A1408")
	pillBack = ui.RGBA(0, 0, 0, 0.72)
)

// mat is the playmat's geometry, laid out as the sim's playmat: the characters toward the
// middle, then life at the outer edge with the leader, stage and deck, then the DON!! deck, the
// cost area and the trash. w is a card's width, and every zone follows from it. Positions are
// for the user's half, and the opponent's half is the same turned around.
type mat struct {
	w, h, dw, dh, g, p, W, H float32
}

const slotPad = 4

func newMat(w float32) mat {
	m := mat{w: w, h: page.CardHeight(w), dw: w * 0.6, g: 6, p: 14}
	m.dh = page.CardHeight(m.dw)
	m.W = 2*m.p + 6*m.h + 5*m.g
	m.H = 2*m.p + 2*m.halfH() + 2*m.g
	return m
}

// fitMat is the largest mat that fits a box, within card widths that stay readable.
func fitMat(r ui.Rect) mat {
	if r.W <= 0 || r.H <= 0 {
		return newMat(80)
	}
	// A mat's size grows in step with its card width.
	m0, m1 := newMat(0), newMat(1)
	w := min((r.W-m0.W)/(m1.W-m0.W), (r.H-m0.H)/(m1.H-m0.H))
	return newMat(min(max(float32(math.Floor(float64(w))), 52), 150))
}

func (m mat) charsH() float32 { return m.h + 2*slotPad }
func (m mat) halfH() float32  { return m.charsH() + 2*m.g + 2*m.h }

// box is a zone at x and y in the user's half, of w by h, placed in a side's half: the
// opponent's turned around, so their characters face the user's across the middle.
func (m mat) box(mine bool, x, y, w, h float32) ui.Rect {
	if mine {
		return ui.Rect{X: x, Y: m.p + m.halfH() + 2*m.g + y, W: w, H: h}
	}
	return ui.Rect{X: m.W - x - w, Y: m.p + m.halfH() - y - h, W: w, H: h}
}

func (m mat) charsX() float32 { return m.p + m.h + m.g }

func (m mat) slotX(i int) float32 {
	return m.charsX() + float32(i)*(m.h+m.g)
}

func (m mat) charsZone(mine bool) ui.Rect {
	return m.box(mine, m.charsX()-slotPad, 0, 5*m.h+4*m.g+2*slotPad, m.charsH())
}
func (m mat) rowB() float32 { return m.charsH() + m.g }
func (m mat) rowC() float32 { return m.rowB() + m.h + m.g }
func (m mat) leaderSlot(mine bool) ui.Rect {
	return m.box(mine, m.slotX(2), m.rowB(), m.h, m.h)
}
func (m mat) stageSlot(mine bool) ui.Rect { return m.box(mine, m.slotX(3), m.rowB(), m.h, m.h) }
func (m mat) deckSlot(mine bool) ui.Rect {
	return m.box(mine, m.slotX(4)+(m.h-m.w)/2, m.rowB(), m.w, m.h)
}

func (m mat) costZone(mine bool) ui.Rect {
	return m.box(mine, m.charsX()-slotPad, m.rowC(), 4*m.h+3*m.g+2*slotPad, m.h)
}

func (m mat) trashSlot(mine bool) ui.Rect {
	return m.box(mine, m.slotX(4)+(m.h-m.w)/2, m.rowC(), m.w, m.h)
}

func (m mat) donDeckSlot(mine bool) ui.Rect {
	return m.box(mine, m.p+(m.h-m.dw)/2, m.rowC()+(m.h-m.dh)/2, m.dw, m.dh)
}

// lifeZone is the outer column beside the characters and the leader's row.
func (m mat) lifeZone(mine bool) ui.Rect { return m.box(mine, m.p, 0, m.h, m.rowB()+m.h) }

// Placed is a card in a slot on the mat. x and y are the top left of the slot, h square.
type Placed struct {
	Card replay.Card
	x, y float32
	// gone is a card the step's attack removed, drawn faded where it stood so the arrow has
	// a card to point at.
	gone bool
	Mine bool
}

// donSlot keys a side's i-th DON!! in the cost area.
type donSlot struct {
	mine bool
	i    int
}

type arrowKey struct{ attacker, target int }

// powerKey keys the power pill over a card in a battle.
type powerKey int

// goneCard keys the faded copy of a removed defender apart from the card itself, now in the
// trash.
type goneCard int

func (m mat) center(pl Placed) (float32, float32) { return pl.x + m.h/2, pl.y + m.h/2 }

// matCharacters is a side's characters, with the attack's defender put back where it stood
// when the step removed it.
func matCharacters(sv *stepView, s *replay.Side) ([]replay.Card, int) {
	chars := s.Characters
	bt := sv.step.Battle
	if bt == nil || bt.Left == nil || s.Owner == bt.AttackerOwner {
		return chars, -1
	}
	for _, c := range chars {
		if c.ID == bt.Left.ID {
			return chars, -1
		}
	}
	at := min(max(bt.LeftSlot, 0), len(chars))
	return slices.Insert(slices.Clone(chars), at, *bt.Left), at
}

func (m mat) layout(sv *stepView) []Placed {
	var out []Placed
	side := func(s *replay.Side, mine bool) {
		chars, gone := matCharacters(sv, s)
		z := m.charsZone(mine)
		for i, c := range chars {
			out = append(out, Placed{Card: c, x: z.X + slotPad + float32(i)*(m.h+m.g), y: z.Y + slotPad, gone: i == gone, Mine: mine})
		}
		l := m.leaderSlot(mine)
		out = append(out, Placed{Card: s.Leader, x: l.X, y: l.Y, Mine: mine})
		st := m.stageSlot(mine)
		for _, c := range s.Stage {
			out = append(out, Placed{Card: c, x: st.X, y: st.Y, Mine: mine})
		}
	}
	side(sv.them, false)
	side(sv.me, true)
	return out
}

func playmat(c *ui.Context, e *page.Env, m mat, sv *stepView) ui.Element {
	cards := m.layout(sv)
	key := 0
	k := func() *ui.Context { key++; return c.Key(key) }
	bt := sv.step.Battle
	a, t := -1, -1
	var attacker page.Side
	if bt != nil {
		attacker = sv.attacker()
		a, t = BattlePair(cards, bt, attacker.Mine)
	}
	themCols, meCols := kit.ColorsOf(e.Snap.Card(sv.them.Leader.Code)), kit.ColorsOf(e.Snap.Card(sv.me.Leader.Code))
	// No Clip, so cards on their way between the mat and a hand show the whole way.
	return ui.Box(c).Size(m.W, m.H).Radius(12).Background(kit.PanelColor(c.Theme())).Border(1, zoneLine).
		Draw(func(p *ui.Painter, r ui.Rect) {
			// Each half is lit faintly from its own edge in its leader's colors.
			p.FillRadialGradient(r, page.Light(themCols[0].Glow, ui.Point{X: 0.5, Y: -0.15}, r.W*0.55, 0.2), 12)
			p.FillRadialGradient(r, page.Light(meCols[0].Glow, ui.Point{X: 0.5, Y: 1.15}, r.W*0.55, 0.2), 12)
		}).Children(func() {
		zones(c, k, m, sv)
		piles(c, e, k, m, sv)
		for i, pl := range cards {
			notes := statusNotes(e, pl.Card)
			switch i {
			case a:
				notes = append([]string{"Power " + powerSum(e, bt.Attack)}, notes...)
			case t:
				notes = append([]string{"Power " + powerSum(e, bt.Defense)}, notes...)
			}
			var key any = cardMatch(pl.Card.ID)
			if pl.gone {
				key = goneCard(pl.Card.ID)
			}
			box := ui.Box(c.Key(key)).Absolute().Left(pl.x).Top(pl.y).Size(m.h, m.h).Center()
			selected := false
			if !pl.gone {
				rs, id := sv.rs, pl.Card.ID
				box.Cursor(ui.CursorPointer)
				if box.Clicked() {
					rs.pile = pileRef{}
					if rs.selected == id {
						rs.selected = 0
					} else {
						rs.selected = id
					}
				}
				selected = rs.selected == id
			}
			if pl.gone {
				box.Opacity(0.35)
			} else {
				sv.Glide(box, pl.Card.ID)
			}
			box.Children(func() { boardCard(c, e, sv, pl.Card, m.w, selected, sv.seat(pl.Mine).Color(), notes...) })
		}
		if a >= 0 && t >= 0 {
			attackArrow(c, e, sv, m, cards[a], cards[t], bt, attacker)
			if sv.hit() {
				impact(c, sv, m, cards[t], attacker.Color())
			}
		}
	})
}

func zones(c *ui.Context, k func() *ui.Context, m mat, sv *stepView) {
	t := c.Theme()
	well := func(r ui.Rect) {
		ui.Box(k()).Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H).Radius(8).Background(ui.RGBA(0, 0, 0, 0.22))
	}
	empty := func(r ui.Rect, label string) {
		ui.Box(k()).Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H).Radius(5).Border(1, zoneLine).
			BorderStyle(ui.BorderDashed).Center().Children(func() {
			ui.Text(c, label).FontSize(10).TextColor(kit.FaintColor(t))
		})
	}
	for _, s := range []*replay.Side{sv.them, sv.me} {
		mine := s == sv.me
		well(m.charsZone(mine))
		well(m.costZone(mine))
		if len(s.Stage) == 0 {
			st := m.stageSlot(mine)
			empty(ui.Rect{X: st.X + (m.h-m.w)/2, Y: st.Y, W: m.w, H: m.h}, "Stage")
		}
		if len(s.Trash) == 0 {
			empty(m.trashSlot(mine), "Trash")
		}
	}
}

func piles(c *ui.Context, e *page.Env, k func() *ui.Context, m mat, sv *stepView) {
	for _, s := range []*replay.Side{sv.them, sv.me} {
		mine := s == sv.me
		tr := m.trashSlot(mine)
		trash(c, e, m, sv, s, tr.X, tr.Y)
		if r := lifeStack(c, k, m, sv, s, mine); len(s.Life) > 0 {
			pileButton(ui.Box(k()).Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H), sv, pileRef{kind: pileLife, Mine: mine})
		}
		d := m.deckSlot(mine)
		deckPile(c, k, m, sv, s, d.X, d.Y)
		dd := m.donDeckSlot(mine)
		donDeck(c, e, k, m, s, dd.X, dd.Y)
		cz := m.costZone(mine)
		donRow(c, e, k, m, sv, s, cz, mine)
	}
}

func Pill(c *ui.Context, text string, size float32) ui.Element {
	return ui.Text(c, text).FontSize(size).Bold().TextColor(page.PillText).Padding(1, 6).Radius(8).Background(pillBack)
}

func trash(c *ui.Context, e *page.Env, m mat, sv *stepView, s *replay.Side, x, y float32) {
	if len(s.Trash) == 0 {
		return
	}
	top := s.Trash[len(s.Trash)-1]
	box := ui.Box(c.Key(cardMatch(top.ID))).Absolute().Left(x).Top(y).Size(m.w, m.h)
	pileButton(box, sv, pileRef{kind: pileTrash, Mine: s == sv.me})
	sv.Glide(box, top.ID).Children(func() {
		e.Card(c, top.Code, m.w)
		Pill(c, fmt.Sprintf("Trash %d", len(s.Trash)), 10).Absolute().Bottom(4).Left(4)
	})
}

func cardBack(c *ui.Context, w, h float32) ui.Element {
	return ui.Box(c).Size(w, h).Radius(4).Background(backFace).Border(1, backEdge).Center().Children(func() {
		ui.Icon(c, kit.IconCompass).Size(min(w, h)*0.45, min(w, h)*0.45).TextColor(backEdge)
	})
}

// lifeStack is the life cards face down and turned sideways, each half over the one before,
// in the outer column. The user's grow up from the leader's row, and the opponent's down
// from theirs, as the sim lays them. It returns the box the stack takes.
func lifeStack(c *ui.Context, k func() *ui.Context, m mat, sv *stepView, s *replay.Side, mine bool) ui.Rect {
	z := m.lifeZone(mine)
	n := len(s.Life)
	cw, ch := m.h*0.9, m.w*0.9
	step := min(ch*0.5, (z.H-ch-20)/float32(max(n-1, 1)))
	x := z.X + (z.W-cw)/2
	for i, card := range s.Life {
		y := z.Y + z.H - ch - float32(i)*step
		if !mine {
			y = z.Y + float32(i)*step
		}
		back := cardBack(c.Key(cardMatch(card.ID)), cw, ch).Absolute().Left(x).Top(y)
		sv.Glide(back, card.ID)
	}
	labelY := z.Y + z.H - ch - float32(max(n-1, 0))*step - 20
	if !mine {
		labelY = z.Y + float32(max(n-1, 0))*step + ch + 4
	}
	ui.Text(k(), fmt.Sprintf("Life %d", n)).FontSize(11).FontWeight(600).TextColor(c.Theme().TextMuted).
		Absolute().Left(z.X).Top(labelY).Width(z.W).TextAlign(ui.Center)
	h := ch + float32(max(n-1, 0))*step
	top := z.Y + z.H - h
	if !mine {
		top = z.Y
	}
	return ui.Rect{X: x, Y: top, W: cw, H: h}
}

// deckPile is keyed as its top card, so a card drawn flies from it.
func deckPile(c *ui.Context, k func() *ui.Context, m mat, sv *stepView, s *replay.Side, x, y float32) {
	key, top := k(), 0
	if d := s.Deck(); len(d) > 0 {
		top = d[len(d)-1].ID
		key = c.Key(cardMatch(top))
	}
	box := ui.Box(key).Absolute().Left(x).Top(y).Size(m.w, m.h)
	sv.Glide(box, top)
	// A draw keys the pile anew, and a new element has no bounds yet.
	if b := box.Bounds(); b.W > 0 {
		sv.rs.deckAt[deckSide(sv, s)] = b
	}
	pileButton(box, sv, pileRef{kind: pileDeck, Mine: s == sv.me})
	box.Children(func() {
		cardBack(c, m.w, m.h)
		Pill(c, fmt.Sprint(len(s.Deck())), 10).Absolute().Left(4).Bottom(4)
	})
}

func pileButton(box ui.Element, sv *stepView, p pileRef) {
	rs := sv.rs
	box.Cursor(ui.CursorPointer).Label(sv.pileName(p))
	if box.Clicked() {
		rs.selected = 0
		if rs.pile == p {
			rs.pile = pileRef{}
		} else {
			rs.pile = p
		}
	}
	if rs.pile == p {
		selectRing(box.Radius(6), sv.seat(p.Mine).Color())
	}
}

// The dark hairline keeps the ring apart from bright art.
func selectRing(el ui.Element, col ui.Color) {
	el.Border(2, col).Shadow(0, 0, 0, 2, ui.RGBA(0, 0, 0, 0.6))
}

func donRow(c *ui.Context, e *page.Env, k func() *ui.Context, m mat, sv *stepView, s *replay.Side, z ui.Rect, mine bool) {
	n := len(s.Don)
	if n == 0 {
		return
	}
	width := float32(0)
	for _, rested := range s.Don {
		if rested {
			width += m.dh
		} else {
			width += m.dw
		}
	}
	room := z.W - 16
	gap := min(float32(4), (room-width)/float32(max(n-1, 1)))
	x := z.X + (z.W-width-gap*float32(n-1))/2
	y := z.Y + (z.H-m.dh)/2
	dd := m.donDeckSlot(mine)
	for i, rested := range s.Don {
		w := m.dw
		if rested {
			w = m.dh
		}
		box := ui.Box(c.Key(donSlot{mine, i})).Absolute().Left(x).Top(y).Height(m.dh).Center()
		// New DON!! come off the DON!! deck.
		box.Transition(sv.glide(nil, &ui.Motion{X: dd.X - x, Y: dd.Y - y, Opacity: 1}))
		rest := float32(0)
		if rested {
			rest = 1
		}
		rest = sv.ease(box, "rest", rest)
		box.Width(lerp(m.dw, m.dh, rest))
		box.Children(func() {
			don := donCard(c, e, m.dw, m.dh).Opacity(1 - 0.55*rest)
			if rest != 0 {
				don.Rotate(90 * rest)
			}
		})
		x += w + gap
	}
	ly := z.Y + z.H - 16
	if !mine {
		ly = z.Y + 3
	}
	ui.Text(k(), fmt.Sprintf("%d of %d active", s.DonActive(), n)).FontSize(10).TextColor(kit.FaintColor(c.Theme())).
		Absolute().Left(z.X).Top(ly).Width(z.W - 10).TextAlign(ui.End)
}

func donDeck(c *ui.Context, e *page.Env, k func() *ui.Context, m mat, s *replay.Side, x, y float32) {
	if s.DonDeck == 0 {
		return
	}
	ui.Box(k()).Absolute().Left(x).Top(y).Size(m.dw, m.dh).Label("DON!! deck").Children(func() {
		if art := e.Art.Get(page.DonBackCode, page.ArtCard); art != nil {
			ui.Image(c, art).Size(m.dw, m.dh).Fit(ui.Cover).Radius(3)
		} else {
			ui.Box(c).Size(m.dw, m.dh).Radius(3).Background(donGold)
		}
		Pill(c, fmt.Sprint(s.DonDeck), 10).Absolute().Left(2).Bottom(2)
	})
}

func donCard(c *ui.Context, e *page.Env, w, h float32) ui.Element {
	if art := e.Art.Get(page.DonCode, page.ArtCard); art != nil {
		return ui.Image(c, art).Size(w, h).Fit(ui.Cover).Radius(3)
	}
	return ui.Box(c).Size(w, h).Radius(3).Background(donGold).Center().Children(func() {
		ui.Text(c, "DON!!").FontSize(max(w*0.2, 8)).Bold().TextColor(donInk)
	})
}

var donPink = ui.Hex("#E8457E")

func boardCard(c *ui.Context, e *page.Env, sv *stepView, card replay.Card, w float32, selected bool, col ui.Color, notes ...string) {
	h := page.CardHeight(w)
	outer := ui.Box(c)
	rest := float32(0)
	if card.Rested {
		rest = 1
	}
	// The card turns on its center while its box swaps width and height with it.
	rest = sv.ease(outer, "rest", rest)
	bw, bh := lerp(w, h, rest), lerp(h, w, rest)
	outer.Size(bw, bh).Children(func() {
		inner := ui.Box(c).Size(w, h).Absolute().Top((bh - h) / 2).Left((bw - w) / 2)
		if rest != 0 {
			inner.Rotate(90 * rest)
		}
		inner.Children(func() {
			shown := min(card.Don, 3)
			for i := shown; i >= 1; i-- {
				off := float32(i)
				don := donCard(c.Key(i), e, w, h).Absolute().Left(-w * 0.07 * off).Top(h * 0.1 * off)
				// A DON!! given to the card slides out from under it.
				don.Transition(sv.glide(nil, &ui.Motion{X: w * 0.07 * off, Y: -h * 0.1 * off}))
			}
			// Absolute elements draw over the rest, so the card is one too, to sit over its DON.
			art := e.Card(c, card.Code, w, notes...).Absolute().Top(0).Left(0)
			if selected {
				selectRing(art, col)
			}
			if card.Don > 0 {
				ui.Text(c, fmt.Sprintf("DON!! ×%d", card.Don)).FontSize(max(w*0.13, 10)).Bold().
					TextColor(ui.Hex("#FFFFFF")).Padding(1, 6).Radius(6).Background(donPink).
					Absolute().Bottom(h * 0.08).Left(w * 0.08)
			}
		})
		badges(c, badgesOf(card), bw, bh)
	})
}

// BattlePair finds the placed attacker and the card the attack lands on, by the copy the step
// names.
func BattlePair(cards []Placed, bt *replay.Battle, mineAttacks bool) (a, t int) {
	find := func(id int, mine bool) int {
		for i, pl := range cards {
			if id != 0 && pl.Card.ID == id && pl.Mine == mine {
				return i
			}
		}
		return -1
	}
	return find(bt.AttackerID, mineAttacks), find(bt.DefenderID, !mineAttacks)
}

func attackArrow(c *ui.Context, e *page.Env, sv *stepView, m mat, a, t Placed, bt *replay.Battle, attacker page.Side) {
	col := attacker.Color()
	ax, ay := m.center(a)
	tx, ty := m.center(t)
	// The arrow covers the whole mat, so clicks have to pass through it to the cards.
	arrow := ui.Box(c.Key(arrowKey{a.Card.ID, t.Card.ID})).Absolute().Left(0).Top(0).Size(m.W, m.H).PassThrough()
	// A new attack's arrow reaches out from the attacker to its target.
	if sv.moved && !sv.sameBattleBefore() {
		arrow.Jump("reach", 0)
	}
	reach := sv.ease(arrow, "reach", 1)
	arrow.Draw(func(p *ui.Painter, r ui.Rect) { cometArrow(p, m, r.X+ax, r.Y+ay, r.X+tx, r.Y+ty, reach, col) })
	power := func(pl Placed, pw replay.Power, col ui.Color) {
		if len(pw.Parts) == 0 {
			return
		}
		x, _ := m.center(pl)
		ui.Row(c.Key(powerKey(pl.Card.ID))).Transition(sv.glide(nil, &ui.Motion{Y: 6, Scale: 0.85})).Absolute().Left(x - m.h/2).Top(pl.y - 10).Width(m.h).Justify(ui.Center).Children(func() {
			ui.Text(c, powerText(pw)).FontSize(max(m.w*0.17, 12)).Bold().TextColor(badgeInk).Padding(1, 9).Radius(10).
				Background(col).Shadow(0, 2, 8, 0, kit.PanelShade).Tooltip(powerSum(e, pw))
		})
	}
	power(a, bt.Attack, col)
	power(t, bt.Defense, attacker.Other().Color())
}

// impact rings the card an attack hit once, in the attacker's color.
func impact(c *ui.Context, sv *stepView, m mat, t Placed, col ui.Color) {
	p := sv.pulse(c, 420*time.Millisecond)
	if p >= 1 {
		return
	}
	k := 1 - (1-p)*(1-p)
	grow := m.h * 0.18 * k
	x, y := m.center(t)
	r := ui.Rect{X: x - m.w/2 - grow, Y: y - m.h/2 - grow, W: m.w + 2*grow, H: m.h + 2*grow}
	if t.Card.Rested {
		r = ui.Rect{X: x - m.h/2 - grow, Y: y - m.w/2 - grow, W: m.h + 2*grow, H: m.w + 2*grow}
	}
	ui.Box(c.Key(impactKey(t.Card.ID))).Absolute().Left(0).Top(0).Size(m.W, m.H).PassThrough().Draw(func(pt *ui.Painter, b ui.Rect) {
		rr := ui.Rect{X: b.X + r.X, Y: b.Y + r.Y, W: r.W, H: r.H}
		a := 1 - p
		pt.Shadow(rr, 8, 0, 0, 18*k, 0, col.Alpha(0.55*a))
		pt.Stroke(rr, col.Alpha(0.95*a), 8, 3*(1-k)+1.5)
	})
}

type impactKey int

// cometArrow runs from the attacker toward its target, of the way reach, fading in from the
// attacker and glowing toward its head.
func cometArrow(p *ui.Painter, m mat, ax, ay, tx, ty, reach float32, col ui.Color) {
	dx, dy := tx-ax, ty-ay
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if l == 0 {
		return
	}
	ux, uy := dx/l, dy/l
	sx, sy := ax+ux*m.h*0.3, ay+uy*m.h*0.3
	ex, ey := tx-ux*m.h*0.38, ty-uy*m.h*0.38
	ex, ey = sx+(ex-sx)*reach, sy+(ey-sy)*reach
	size := m.w * 0.3
	if float32(math.Hypot(float64(ex-sx), float64(ey-sy))) < size {
		return
	}
	bx, by := ex-ux*size*0.8, ey-uy*size*0.8
	// A gradient's angle is in degrees clockwise from up.
	angle := float32(math.Atan2(float64(ux), float64(-uy)) * 180 / math.Pi)
	var path ui.Path
	path.MoveTo(sx, sy).LineTo(bx, by)
	p.StrokePathGradient(&path, m.w*0.32, ui.LinearGradient{From: col.Alpha(0), To: col.Alpha(0.28), Angle: angle})
	p.StrokePathGradient(&path, m.w*0.11, ui.LinearGradient{From: col.Alpha(0.05), To: col, Angle: angle})
	hx, hy := ex-ux*size, ey-uy*size
	var head ui.Path
	head.MoveTo(ex, ey).LineTo(hx-uy*size*0.7, hy+ux*size*0.7).LineTo(hx+uy*size*0.7, hy-ux*size*0.7).Close()
	p.FillPath(&head, col)
}
