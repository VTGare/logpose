package gui

import (
	"fmt"
	"math"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/replay"
)

var (
	zoneLine = ui.RGBA(255, 255, 255, 0.09)
	backFace = ui.Hex("#1E2A3D")
	backEdge = ui.RGBA(191, 227, 240, 0.22)
	donGold  = ui.Hex("#C9A44C")
	donInk   = ui.Hex("#1A1408")
	pillText = ui.Hex("#F2F2F0")
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
	m := mat{w: w, h: cardHeight(w), dw: w * 0.6, g: 6, p: 14}
	m.dh = cardHeight(m.dw)
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

// placed is a card in a slot on the mat. x and y are the top left of the slot, h square.
type placed struct {
	card replay.Card
	x, y float32
	// gone is a card the step's attack removed, drawn faded where it stood so the arrow has
	// a card to point at.
	gone bool
	mine bool
}

// goneCard keys the faded copy of a removed defender apart from the card itself, now in the
// trash.
type goneCard int

func (m mat) center(pl placed) (float32, float32) { return pl.x + m.h/2, pl.y + m.h/2 }

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

func (m mat) layout(sv *stepView) []placed {
	var out []placed
	side := func(s *replay.Side, mine bool) {
		chars, gone := matCharacters(sv, s)
		z := m.charsZone(mine)
		for i, c := range chars {
			out = append(out, placed{card: c, x: z.X + slotPad + float32(i)*(m.h+m.g), y: z.Y + slotPad, gone: i == gone, mine: mine})
		}
		l := m.leaderSlot(mine)
		out = append(out, placed{card: s.Leader, x: l.X, y: l.Y, mine: mine})
		st := m.stageSlot(mine)
		for _, c := range s.Stage {
			out = append(out, placed{card: c, x: st.X, y: st.Y, mine: mine})
		}
	}
	side(sv.them, false)
	side(sv.me, true)
	return out
}

func (v *view) playmat(c *ui.Context, m mat, sv *stepView) ui.Element {
	cards := m.layout(sv)
	key := 0
	k := func() *ui.Context { key++; return c.Key(key) }
	bt := sv.step.Battle
	a, t := -1, -1
	var attacker side
	if bt != nil {
		attacker = sv.attacker()
		a, t = battlePair(cards, bt, attacker.mine)
	}
	themCols, meCols := colorsOf(v.snap.card(sv.them.Leader.Code)), colorsOf(v.snap.card(sv.me.Leader.Code))
	return ui.Box(c).Size(m.W, m.H).Radius(12).Background(panelColor(c.Theme())).Border(1, zoneLine).Clip().
		Draw(func(p *ui.Painter, r ui.Rect) {
			// Each half is lit faintly from its own edge in its leader's colors.
			p.FillRadialGradient(r, light(themCols[0].Glow, ui.Point{X: 0.5, Y: -0.15}, r.W*0.55, 0.2), 0)
			p.FillRadialGradient(r, light(meCols[0].Glow, ui.Point{X: 0.5, Y: 1.15}, r.W*0.55, 0.2), 0)
		}).Children(func() {
		zones(c, k, m, sv)
		v.piles(c, k, m, sv)
		for i, pl := range cards {
			notes := v.statusNotes(pl.card)
			switch i {
			case a:
				notes = append([]string{"Power " + v.powerSum(bt.Attack)}, notes...)
			case t:
				notes = append([]string{"Power " + v.powerSum(bt.Defense)}, notes...)
			}
			var key any = cardMatch(pl.card.ID)
			if pl.gone {
				key = goneCard(pl.card.ID)
			}
			box := ui.Box(c.Key(key)).Absolute().Left(pl.x).Top(pl.y).Size(m.h, m.h).Center()
			selected := false
			if !pl.gone {
				rs, id := sv.rs, pl.card.ID
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
				sv.glide(box, pl.card.ID)
			}
			box.Children(func() { v.boardCard(c, pl.card, m.w, selected, sv.seat(pl.mine).color(), notes...) })
		}
		if a >= 0 && t >= 0 {
			v.attackArrow(c, k, m, cards[a], cards[t], bt, attacker)
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
			ui.Text(c, label).FontSize(10).TextColor(faintColor(t))
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

func (v *view) piles(c *ui.Context, k func() *ui.Context, m mat, sv *stepView) {
	for _, s := range []*replay.Side{sv.them, sv.me} {
		mine := s == sv.me
		tr := m.trashSlot(mine)
		v.trash(c, m, sv, s, tr.X, tr.Y)
		if r := lifeStack(c, k, m, sv, s, mine); len(s.Life) > 0 {
			v.pileButton(ui.Box(k()).Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H), sv, pileRef{kind: pileLife, mine: mine})
		}
		d := m.deckSlot(mine)
		v.deckPile(c, k, m, sv, s, d.X, d.Y)
		dd := m.donDeckSlot(mine)
		v.donDeck(c, k, m, s, dd.X, dd.Y)
		cz := m.costZone(mine)
		v.donRow(c, k, m, s, cz, mine)
	}
}

func pill(c *ui.Context, text string, size float32) ui.Element {
	return ui.Text(c, text).FontSize(size).Bold().TextColor(pillText).Padding(1, 6).Radius(8).Background(pillBack)
}

func (v *view) trash(c *ui.Context, m mat, sv *stepView, s *replay.Side, x, y float32) {
	if len(s.Trash) == 0 {
		return
	}
	top := s.Trash[len(s.Trash)-1]
	box := ui.Box(c.Key(cardMatch(top.ID))).Absolute().Left(x).Top(y).Size(m.w, m.h)
	v.pileButton(box, sv, pileRef{kind: pileTrash, mine: s == sv.me})
	sv.glide(box, top.ID).Children(func() {
		v.card(c, top.Code, m.w)
		pill(c, fmt.Sprintf("Trash %d", len(s.Trash)), 10).Absolute().Bottom(4).Left(4)
	})
}

func cardBack(c *ui.Context, w, h float32) ui.Element {
	return ui.Box(c).Size(w, h).Radius(4).Background(backFace).Border(1, backEdge).Center().Children(func() {
		ui.Icon(c, iconCompass).Size(min(w, h)*0.45, min(w, h)*0.45).TextColor(backEdge)
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
		sv.glide(back, card.ID)
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

func (v *view) deckPile(c *ui.Context, k func() *ui.Context, m mat, sv *stepView, s *replay.Side, x, y float32) {
	box := ui.Box(k()).Absolute().Left(x).Top(y).Size(m.w, m.h)
	v.pileButton(box, sv, pileRef{kind: pileDeck, mine: s == sv.me})
	box.Children(func() {
		cardBack(c, m.w, m.h)
		pill(c, fmt.Sprint(len(s.Deck())), 10).Absolute().Left(4).Bottom(4)
	})
}

func (*view) pileButton(box ui.Element, sv *stepView, p pileRef) {
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
		selectRing(box.Radius(6), sv.seat(p.mine).color())
	}
}

// The dark hairline keeps the ring apart from bright art.
func selectRing(el ui.Element, col ui.Color) {
	el.Border(2, col).Shadow(0, 0, 0, 2, ui.RGBA(0, 0, 0, 0.6))
}

func (v *view) donRow(c *ui.Context, k func() *ui.Context, m mat, s *replay.Side, z ui.Rect, mine bool) {
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
	for _, rested := range s.Don {
		box := ui.Box(k()).Absolute().Left(x).Top(y)
		if rested {
			box.Size(m.dh, m.dh).Center().Children(func() {
				v.donCard(c, m.dw, m.dh).Rotate(90).Opacity(0.45)
			})
			x += m.dh + gap
			continue
		}
		box.Children(func() { v.donCard(c, m.dw, m.dh) })
		x += m.dw + gap
	}
	ly := z.Y + z.H - 16
	if !mine {
		ly = z.Y + 3
	}
	ui.Text(k(), fmt.Sprintf("%d of %d active", s.DonActive(), n)).FontSize(10).TextColor(faintColor(c.Theme())).
		Absolute().Left(z.X).Top(ly).Width(z.W - 10).TextAlign(ui.End)
}

func (v *view) donDeck(c *ui.Context, k func() *ui.Context, m mat, s *replay.Side, x, y float32) {
	if s.DonDeck == 0 {
		return
	}
	ui.Box(k()).Absolute().Left(x).Top(y).Size(m.dw, m.dh).Label("DON!! deck").Children(func() {
		if art := v.art.get(donBackCode, artCard); art != nil {
			ui.Image(c, art).Size(m.dw, m.dh).Fit(ui.Cover).Radius(3)
		} else {
			ui.Box(c).Size(m.dw, m.dh).Radius(3).Background(donGold)
		}
		pill(c, fmt.Sprint(s.DonDeck), 10).Absolute().Left(2).Bottom(2)
	})
}

func (v *view) donCard(c *ui.Context, w, h float32) ui.Element {
	if art := v.art.get(donCode, artCard); art != nil {
		return ui.Image(c, art).Size(w, h).Fit(ui.Cover).Radius(3)
	}
	return ui.Box(c).Size(w, h).Radius(3).Background(donGold).Center().Children(func() {
		ui.Text(c, "DON!!").FontSize(max(w*0.2, 8)).Bold().TextColor(donInk)
	})
}

var donPink = ui.Hex("#E8457E")

func (v *view) boardCard(c *ui.Context, card replay.Card, w float32, selected bool, col ui.Color, notes ...string) {
	h := cardHeight(w)
	bw, bh := w, h
	if card.Rested {
		bw, bh = h, w
	}
	ui.Box(c).Size(bw, bh).Children(func() {
		inner := ui.Box(c).Size(w, h)
		if card.Rested {
			inner.Absolute().Top((w - h) / 2).Left((h - w) / 2).Rotate(90)
		}
		inner.Children(func() {
			shown := min(card.Don, 3)
			for i := shown; i >= 1; i-- {
				off := float32(i)
				v.donCard(c.Key(i), w, h).Absolute().Left(-w * 0.07 * off).Top(h * 0.1 * off)
			}
			// Absolute elements draw over the rest, so the card is one too, to sit over its DON.
			art := v.card(c, card.Code, w, notes...).Absolute().Top(0).Left(0)
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

// battlePair finds the placed attacker and the card the attack lands on, by the copy the step
// names.
func battlePair(cards []placed, bt *replay.Battle, mineAttacks bool) (a, t int) {
	find := func(id int, mine bool) int {
		for i, pl := range cards {
			if id != 0 && pl.card.ID == id && pl.mine == mine {
				return i
			}
		}
		return -1
	}
	return find(bt.AttackerID, mineAttacks), find(bt.DefenderID, !mineAttacks)
}

func (v *view) attackArrow(c *ui.Context, k func() *ui.Context, m mat, a, t placed, bt *replay.Battle, attacker side) {
	col := attacker.color()
	ax, ay := m.center(a)
	tx, ty := m.center(t)
	// The arrow covers the whole mat, so clicks have to pass through it to the cards.
	ui.Box(k()).Absolute().Left(0).Top(0).Size(m.W, m.H).PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
		dx, dy := tx-ax, ty-ay
		l := float32(math.Hypot(float64(dx), float64(dy)))
		if l == 0 {
			return
		}
		ux, uy := dx/l, dy/l
		sx, sy := r.X+ax+ux*m.h*0.3, r.Y+ay+uy*m.h*0.3
		ex, ey := r.X+tx-ux*m.h*0.38, r.Y+ty-uy*m.h*0.38
		head := m.w * 0.32
		bx, by := ex-ux*head, ey-uy*head
		p.StrokePolyline([][2]float32{{sx, sy}, {bx, by}}, m.w*0.12, col)
		var path ui.Path
		path.MoveTo(ex, ey).LineTo(bx-uy*head*0.7, by+ux*head*0.7).LineTo(bx+uy*head*0.7, by-ux*head*0.7).Close()
		p.FillPath(&path, col)
	})
	power := func(pl placed, pw replay.Power, col ui.Color) {
		if len(pw.Parts) == 0 {
			return
		}
		x, _ := m.center(pl)
		ui.Row(k()).Absolute().Left(x - m.h/2).Top(pl.y - 10).Width(m.h).Justify(ui.Center).Children(func() {
			ui.Text(c, powerText(pw)).FontSize(max(m.w*0.17, 12)).Bold().TextColor(badgeInk).Padding(1, 9).Radius(10).
				Background(col).Shadow(0, 2, 8, 0, panelShade).Tooltip(v.powerSum(pw))
		})
	}
	power(a, bt.Attack, col)
	power(t, bt.Defense, attacker.other().color())
}
