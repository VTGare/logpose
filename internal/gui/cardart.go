package gui

import (
	"github.com/egoist/mygo/ui"
)

// cardAspect is a card's height over its width.
const cardAspect = 1.4

func cardHeight(w float32) float32 { return w * cardAspect }

// artScale is the display scaling the card sizes are chosen for. Views don't know the real one.
const artScale = 1.25

// cardSizes are the whole-card images the art cache makes, smallest first, with how wide each
// is in pixels. A card wider than all of them uses artLarge.
var cardSizes = []struct {
	kind  artKind
	width float32
}{
	{artFull, webSmallWidth},
	{artCard, cardArtWidth},
}

// cardKind is the smallest image that stays sharp for a card w DIPs wide.
func cardKind(w float32) artKind {
	for _, s := range cardSizes {
		if w*artScale <= s.width {
			return s.kind
		}
	}
	return artLarge
}

// cardImage is the picture ref names, w wide, or the card's placeholder while it loads or when
// there is none. While a large image loads, the small one stands in.
func (e *env) cardImage(c *ui.Context, ref string, w float32) ui.Element {
	kind := cardKind(w)
	art := e.art.get(ref, kind)
	if art == nil && kind != artFull {
		art = e.art.get(ref, artFull)
	}
	if art == nil {
		return e.cardPlaceholder(c, parseArtRef(ref).card(), w, cardHeight(w))
	}
	return ui.Image(c, art).Size(w, cardHeight(w)).Fit(ui.Cover).Radius(4)
}

// cardPlaceholder is a block in the card's colors, in the game's order.
func (e *env) cardPlaceholder(c *ui.Context, code string, w, h float32) ui.Element {
	return ui.Row(c).Size(w, h).Radius(4).Clip().AlignItems(ui.Stretch).Children(func() {
		for i, col := range colorsOf(e.snap.card(code)) {
			ui.Box(c.Key(i)).Grow(1).Background(col.Card)
		}
	})
}

// card is a card's picture named after it, with the card's menu. A card smaller than the
// preview shows the whole card large while the pointer rests on it, with notes under it.
func (e *env) card(c *ui.Context, code string, w float32, notes ...string) ui.Element {
	card := e.snap.card(code)
	el := e.cardImage(c, code, w).Label(card.Name).PopoverMenu(e.nav.cardMenu(card))
	if needsPreview(c, w) {
		e.cardPreview(c, el, code, notes...)
	}
	return el
}

// cardPreview shows the card large while the pointer rests on anchor, with notes under it.
// Without art it names the card.
func (e *env) cardPreview(c *ui.Context, anchor ui.Element, ref string, notes ...string) {
	pw := previewWidth(c)
	ui.TooltipBase(c, anchor, func(tip ui.Element) {
		art := e.art.get(ref, artLarge)
		if art == nil {
			art = e.art.get(ref, artFull)
		}
		tip.Radius(10).Shadow(0, 6, 24, 0, ui.RGBA(0, 0, 0, 0.5)).Background(panelDark).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				if art == nil {
					ui.Text(c, e.snap.card(parseArtRef(ref).card()).Name).FontSize(13).TextColor(pillText).Padding(6, 10)
				} else {
					ui.Image(c, art).Size(pw, cardHeight(pw)).Fit(ui.Cover).Radius(10)
				}
				for i, n := range notes {
					ui.Text(c.Key(i), n).FontSize(12).TextColor(pillText).Padding(0, 10, 4, 10).Wrap()
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
	return min(max(h*0.4/cardAspect, smallest), largest)
}

// needsPreview reports whether a card w wide is small enough for its preview to add something.
func needsPreview(c *ui.Context, w float32) bool { return w < previewWidth(c)*0.85 }

func (e *env) artStrip(c *ui.Context, code string, w, h float32) {
	if art := e.art.get(code, artCrop); art != nil {
		ui.Image(c, art).Size(w, h).Fit(ui.Cover).Radius(4)
		return
	}
	e.cardPlaceholder(c, code, w, h)
}
