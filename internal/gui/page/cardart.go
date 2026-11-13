package page

import (
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/gui/kit"
)

// CardAspect is a card's height over its width.
const CardAspect = 1.4

func CardHeight(w float32) float32 { return w * CardAspect }

// ArtScale is the display scaling the card sizes are chosen for. Views don't know the real one.
const ArtScale = 1.25

// cardSizes are the whole-card images the art cache makes, smallest first, with how wide each
// is in pixels. A card wider than all of them uses artLarge.
var cardSizes = []struct {
	kind  artKind
	width float32
}{
	{ArtFull, WebSmallWidth},
	{ArtCard, CardArtWidth},
}

// CardKind is the smallest image that stays sharp for a card w DIPs wide.
func CardKind(w float32) artKind {
	for _, s := range cardSizes {
		if w*ArtScale <= s.width {
			return s.kind
		}
	}
	return ArtLarge
}

// CardImage is the picture ref names, w wide, or the card's placeholder while it loads or when
// there is none. While a large image loads, the small one stands in.
func (e *Env) CardImage(c *ui.Context, ref string, w float32) ui.Element {
	kind := CardKind(w)
	art := e.Art.Get(ref, kind)
	if art == nil && kind != ArtFull {
		art = e.Art.Get(ref, ArtFull)
	}
	if art == nil {
		return e.CardPlaceholder(c, ParseArtRef(ref).Card(), w, CardHeight(w))
	}
	return ui.Image(c, art).Size(w, CardHeight(w)).Fit(ui.Cover).Radius(4)
}

// CardPlaceholder is a block in the card's colors, in the game's order.
func (e *Env) CardPlaceholder(c *ui.Context, code string, w, h float32) ui.Element {
	return ui.Row(c).Size(w, h).Radius(4).Clip().AlignItems(ui.Stretch).Children(func() {
		for i, col := range kit.ColorsOf(e.Snap.Card(code)) {
			ui.Box(c.Key(i)).Grow(1).Background(col.Card)
		}
	})
}

// Card is a Card's picture named after it, with the Card's menu. A Card smaller than the
// preview shows the whole Card large while the pointer rests on it, with notes under it.
func (e *Env) Card(c *ui.Context, code string, w float32, notes ...string) ui.Element {
	card := e.Snap.Card(code)
	el := e.CardImage(c, code, w).Label(card.Name).PopoverMenu(e.Nav.CardMenu(card))
	if NeedsPreview(c, w) {
		e.CardPreview(c, el, code, notes...)
	}
	return el
}

// CardPreview shows the card large while the pointer rests on anchor, with notes under it.
// Without art it names the card.
func (e *Env) CardPreview(c *ui.Context, anchor ui.Element, ref string, notes ...string) {
	pw := previewWidth(c)
	ui.TooltipBase(c, anchor, func(tip ui.Element) {
		art := e.Art.Get(ref, ArtLarge)
		if art == nil {
			art = e.Art.Get(ref, ArtFull)
		}
		tip.Radius(10).Shadow(0, 6, 24, 0, ui.RGBA(0, 0, 0, 0.5)).Background(kit.PanelDark).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				if art == nil {
					ui.Text(c, e.Snap.Card(ParseArtRef(ref).Card()).Name).FontSize(13).TextColor(PillText).Padding(6, 10)
				} else {
					ui.Image(c, art).Size(pw, CardHeight(pw)).Fit(ui.Cover).Radius(10)
				}
				for i, n := range notes {
					ui.Text(c.Key(i), n).FontSize(12).TextColor(PillText).Padding(0, 10, 4, 10).Wrap()
				}
			})
		})
	})
}

// previewWidth is the width of the card shown while the pointer rests on a small one. It takes
// about 40% of the window's height, and stays wide enough to read the card's text.
func previewWidth(c *ui.Context) float32 {
	const smallest, largest = 180, 300
	_, h := c.Size()
	return min(max(h*0.4/CardAspect, smallest), largest)
}

// NeedsPreview reports whether a card w wide is small enough for its preview to add something.
func NeedsPreview(c *ui.Context, w float32) bool { return w < previewWidth(c)*0.85 }

func (e *Env) ArtStrip(c *ui.Context, code string, w, h float32) {
	if art := e.Art.Get(code, artCrop); art != nil {
		ui.Image(c, art).Size(w, h).Fit(ui.Cover).Radius(4)
		return
	}
	e.CardPlaceholder(c, code, w, h)
}

var PillText = ui.Hex("#F2F2F0")
