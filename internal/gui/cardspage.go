package gui

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"
	"github.com/VTGare/logpose/internal/store"
)

// The card panel is resized by dragging its edge. Narrower than panelSnap it folds to a strip.
const (
	panelDefault = 470
	panelMin     = 360
	panelMax     = 760
	panelSnap    = 280
	panelStrip   = 44
)

func snapPanel(w float32) float32 {
	switch {
	case w < panelSnap:
		return panelStrip
	case w < panelMin:
		return panelMin
	}
	return min(w, panelMax)
}

const (
	tileMin = 128
	tileGap = 12
	// largeArtMin is the width from which a card is drawn from the full image. artCard's 240
	// pixels look soft at 210 DIPs on a scaled display.
	largeArtMin = 200
)

var cardColors = []cards.Color{cards.Red, cards.Green, cards.Blue, cards.Purple, cards.Black, cards.Yellow}

func (v *view) cardsPage(c *ui.Context) {
	s := &v.cardsUI
	if v.snap.Cards == nil || len(v.snap.Cards.All()) == 0 {
		v.cardsMissing(c)
		return
	}
	if !v.simArt.scanned {
		v.simArt.scanned = true
		v.do(func(a actions) { a.ScanSimArt() })
	}
	shown := s.results(v.snap.Cards)
	if s.selected == "" && len(shown) > 0 {
		s.selected = shown[0].Code
	}
	ui.Column(c).Fill().Padding(18, 20, 0, 20).Gap(12).Children(func() {
		v.cardSearch(c)
		v.quickFilters(c)
		v.chipRow(c, len(shown))
		ui.Row(c).Grow(1).Basis(0).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			if len(shown) == 0 {
				ui.Column(c).Grow(1).Center().Gap(10).Children(func() {
					ui.Text(c, "No cards match").Bold().FontSize(16)
					if ui.Button(c, "Clear filters").Clicked() {
						s.clear()
					}
				})
				return
			}
			list := func() {
				ui.Row(c).Fill().MinHeight(0).AlignItems(ui.Stretch).Padding(0, 14, 0, 0).Children(func() {
					if s.list {
						v.cardRows(c, shown)
					} else {
						v.cardWall(c, shown)
					}
				})
			}
			card := v.snap.Cards.Card(s.selected)
			if !card.Detailed {
				list()
				return
			}
			v.cardSplit(c, list, func() { v.cardDetail(c, card) })
		})
	})
	if s.viewer {
		v.artViewer(c, v.snap.Cards.Card(s.selected))
	}
}

func (v *view) cardsMissing(c *ui.Context) {
	if v.cardSync.total > 0 {
		v.cardsReading(c)
		return
	}
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(10).Padding(32).Children(func() {
		ui.Text(c, "Cards aren't downloaded yet").Bold().FontSize(18)
		ui.Text(c, "logpose reads every card from Limitless once, which takes about a minute, then checks "+
			"for new sets once a day.").TextColor(t.TextMuted).MaxWidth(440).Wrap().TextAlign(ui.Center)
		if ui.Button(c, "Download cards").Clicked() {
			v.do(func(a actions) { a.RefreshCards() })
		}
		if n := v.settings.notes[noteCards]; n != "" {
			ui.Text(c, n).FontSize(13).TextColor(t.TextMuted)
		}
	})
}

// cardsReading fills the page while the first download runs, ticking off each set as it's
// read, the latest on top.
func (v *view) cardsReading(c *ui.Context) {
	sync := v.cardSync
	t := c.Theme()
	ui.Column(c).Fill().Center().Padding(32).Children(func() {
		ui.Column(c).Width(440).Gap(14).Children(func() {
			ui.Text(c, "Reading every card from Limitless").Bold().FontSize(20)
			ui.Text(c, "The first time takes about a minute. After that logpose only checks for new sets, once a day.").
				TextColor(t.TextMuted).Wrap()
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				progressBar(c, float32(sync.done)/float32(max(sync.total, 1))).Grow(1)
				ui.Text(c, fmt.Sprintf("%d of %d sets", sync.done, sync.total)).FontSize(13).TextColor(t.TextMuted)
			})
			ui.Column(c).Gap(6).Children(func() {
				for i, set := range sync.named(5) {
					ui.Row(c.Key(i)).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconCheck).Size(14, 14).TextColor(winColor)
						ui.Text(c, set.Set+" · "+cardCount(set.Cards)).FontSize(13)
					})
				}
			})
		})
	})
}

func progressBar(c *ui.Context, part float32) ui.Element {
	track := ui.Box(c).Height(3).Radius(2).Background(ui.RGBA(255, 255, 255, 0.08))
	return track.Children(func() {
		ui.Box(c).Height(3).Radius(2).Background(c.Theme().Accent).WidthPercent(100 * min(max(part, 0), 1))
	})
}

// cardSplit puts the cards and the card panel side by side, with an edge between them that
// resizes the panel. ui.Split sizes the pane on the left, so the panel's width is worked out
// from the room the split had in the frame before.
func (v *view) cardSplit(c *ui.Context, list, panel func()) {
	s := &v.cardsUI
	total := s.splitBox.W
	if total <= 0 {
		total = 900
	}
	if v.prefs.panelWidth == 0 {
		v.prefs.panelWidth = panelDefault
	}
	shown := max(total-v.prefs.panelWidth-1, 40)
	size := shown
	split := ui.Split(c, &size, list, panel).Grow(1).Basis(0).MinWidth(0)
	if split.Changed() {
		v.dragPane(&v.prefs.panelWidth, &v.panelDrag, shown-size, snapPanel, store.SettingCardPanelWidth)
	}
	// Bounds is empty on the frame the page comes back, so the old width is kept until then.
	if b := split.Bounds(); b.W > 0 && b.W != s.splitBox.W {
		s.splitBox = b
		c.Invalidate()
	}
}

// cardSearch searches names, codes, traits and effects as you type, and offers the filters
// whose names hold what's typed. Picking one turns it into a chip.
func (v *view) cardSearch(c *ui.Context) {
	s := &v.cardsUI
	field := ui.Autocomplete(c, &s.query, s.pool.suggestions).Label("Search cards").
		Placeholder("Search names, effects and traits")
	if pageEntered {
		field.Focus()
	}
	if field.Changed() && s.takeSuggestion() {
		c.Invalidate()
	}
}

// quickFilters are the filters used most: colors, type and the cost curve, each a labelled
// group of toggles.
func (v *view) quickFilters(c *ui.Context) {
	s := &v.cardsUI
	ui.Row(c).Gap(18).Wrap().AlignItems(ui.End).Children(func() {
		filterGroup(c, "Color", func() {
			joined(c, len(cardColors), func(i int) (string, bool) {
				col := cardColors[i]
				return string(col), slices.Contains(s.colors, col)
			}, func(i int) { s.toggleColor(cardColors[i]) }, func(i int) {
				ui.Box(c).Size(11, 11).Radius(6).Background(leaderColors[cardColors[i]].Card).
					Border(1, ui.RGBA(255, 255, 255, 0.25))
			})
		})
		filterGroup(c, "Type", func() {
			joined(c, len(cardKinds), func(i int) (string, bool) { return cardKinds[i], s.kind == i },
				func(i int) { s.kind = i }, nil)
		})
		filterGroup(c, "Cost", func() { v.costCurve(c) })
	})
}

func filterGroup(c *ui.Context, label string, body func()) {
	ui.Column(c.Key(label)).Gap(5).Children(func() {
		ui.Text(c, label).FontSize(11.5).TextColor(c.Theme().TextMuted)
		body()
	})
}

// joined draws n toggles as one control split by hairlines. item names a toggle and says
// whether it's on, and draw, when set, draws it instead of its name.
func joined(c *ui.Context, n int, item func(int) (string, bool), toggle, draw func(int)) {
	t := c.Theme()
	ui.Row(c).Height(30).Radius(8).Border(1, t.Border).Clip().Children(func() {
		for i := range n {
			label, on := item(i)
			b := ui.ButtonBase(c.Key(label)).Label(label).Height(30).MinWidth(32).Padding(0, 10).Center().
				Background(ui.Transparent)
			if i < n-1 {
				b.BorderWidth(0, 1, 0, 0).BorderColor(t.Border)
			}
			if on {
				b.Background(t.Accent.Alpha(0.16))
			} else if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			if b.Clicked() {
				toggle(i)
			}
			b.Children(func() {
				if draw != nil {
					draw(i)
					return
				}
				col := t.TextMuted
				if on {
					col = ui.Hex("#CFE6FB")
				}
				ui.Text(c, label).FontSize(12.5).TextColor(col)
			})
		}
	})
}

// costCurve is a bar per cost, as tall as the number of cards at that cost that match the
// other filters. Each bar picks its cost.
func (v *view) costCurve(c *ui.Context) {
	s := &v.cardsUI
	t := c.Theme()
	top := 1
	for _, n := range s.curve {
		top = max(top, n)
	}
	ui.Row(c).Gap(3).AlignItems(ui.End).Height(50).Children(func() {
		for cost, n := range s.curve {
			on := slices.Contains(s.costs, cost)
			label := fmt.Sprint(cost)
			if cost == maxCost {
				label += "+"
			}
			b := ui.ButtonBase(c.Key(cost)).Label(fmt.Sprintf("Cost %s, %s", label, cardCount(n))).Width(26).Height(50).
				Background(ui.Transparent).Padding(0)
			if b.Clicked() {
				s.toggleCost(cost)
			}
			bar := ui.RGBA(255, 255, 255, 0.16)
			text := t.TextMuted
			switch {
			case on:
				bar, text = t.Accent, ui.Hex("#CFE6FB")
			case b.Hovered():
				bar = ui.RGBA(255, 255, 255, 0.28)
			case n == 0:
				bar = ui.RGBA(255, 255, 255, 0.06)
			}
			h := float32(2)
			if n > 0 {
				h = max(4, 18*float32(n)/float32(top))
			}
			b.Children(func() {
				ui.Column(c).Fill().Gap(3).AlignItems(ui.Center).Justify(ui.End).Children(func() {
					if n > 0 {
						ui.Text(c, fmt.Sprint(n)).FontSize(10).TextColor(faintColor(t))
					}
					ui.Box(c).Width(22).Height(h).Radius(2).Background(bar)
					ui.Text(c, label).FontSize(11).TextColor(text)
				})
			})
		}
	})
}

// chipRow holds More filters, the chips picked from it and the suggestions, the count and the
// switch between images and a list.
func (v *view) chipRow(c *ui.Context, count int) {
	s := &v.cardsUI
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		// More filters comes first, so its menu stays put as chips come and go.
		more := filterPill(c.Key("more"), "More filters", false)
		if more.Clicked() {
			s.more = !s.more
		}
		ui.PopoverBase(c, more, &s.more, func(panel ui.Element) {
			panel.Margin(6, 0, 0, 0).Padding(14).Radius(10).Background(t.Background).Border(1, t.Border).
				Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.45))
			v.moreFilters(c)
		})
		for i, ch := range s.chips {
			b := ui.ButtonBase(c.Key(fmt.Sprint("chip", i, ch.field, ch.value))).Label("Remove "+ch.value).
				Padding(5, 10, 5, 12).Radius(16).Background(t.Accent.Alpha(0.18)).Border(1, t.Accent).Gap(6).AlignItems(ui.Center)
			if ch.out {
				b.Background(lossColor.Alpha(0.14)).Border(1, lossColor.Alpha(0.5))
			}
			if b.Clicked() {
				s.toggleChip(ch)
			}
			b.Children(func() {
				col := t.Text
				if ch.out {
					col = hideColor
					ui.Icon(c, iconMinus).Size(11, 11).TextColor(col)
				}
				ui.Text(c, ch.value).FontSize(13).FontWeight(500).TextColor(col)
				ui.Icon(c, iconClose).FontSize(11).TextColor(t.TextMuted)
			})
		}
		if s.filtered() {
			clear := ui.ButtonBase(c.Key("clear")).Label("Clear the filters").Background(ui.Transparent).Padding(4, 6)
			if clear.Clicked() {
				s.clear()
			}
			clear.Children(func() {
				col := t.Accent
				if clear.Hovered() {
					col = t.AccentHover
				}
				ui.Text(c, "Clear all").FontSize(13).TextColor(col)
			})
		}
		ui.Spacer(c)
		if sync := v.cardSync; sync.running() {
			ui.Text(c, fmt.Sprintf("Updating · %d of %d sets", sync.done, sync.total)).FontSize(13).TextColor(t.Accent)
		} else if sync := v.artSync; sync.running() {
			ui.Text(c, fmt.Sprintf("Reading Bandai's card list · %d of %d", sync.done, sync.total)).FontSize(13).TextColor(t.Accent)
		}
		ui.Text(c, cardCount(count)).FontSize(13).TextColor(t.TextMuted)
		view := 0
		if s.list {
			view = 1
		}
		if ui.Segmented(c, &view, "Images", "List").Label("Show as").Changed() {
			s.list = view == 1
		}
	})
}

func (v *view) moreFilters(c *ui.Context) {
	s := &v.cardsUI
	t := c.Theme()
	group := func(field string, values []string) {
		ui.Column(c.Key(field)).Gap(6).Children(func() {
			ui.Text(c, field).FontSize(12).TextColor(t.TextMuted)
			ui.Row(c).Gap(6).Wrap().MaxWidth(520).Children(func() {
				for _, val := range values {
					on, out := s.chipState(field, val)
					if filterToggle(c.Key(val), val, on, out).Clicked() {
						s.cycleChip(field, val)
					}
				}
			})
		})
	}
	ui.Column(c).Gap(14).Children(func() {
		group(chipAttribute, attributeNames)
		group(chipCounter, []string{"No counter", "+1000", "+2000"})
		group(chipKeyword, s.pool.keywords)
		group(chipBlock, []string{"1", "2", "3", "4", "5", "X"})
		group(chipLegality, []string{"Legal", "Rotated", "Banned", "Restricted", "Upcoming"})
		group(chipRarity, s.pool.rarities)
		ui.Text(c, "Click once to show only these, twice to hide them, three times to clear. For traits and sets, "+
			"type a few letters in the search box.").FontSize(12).TextColor(faintColor(t)).MaxWidth(520).Wrap()
	})
}

// hideColor marks filters that hide cards, apart from the accent of ones that show only.
var hideColor = ui.Hex("#F29A93")

func filterToggle(c *ui.Context, label string, on, out bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(toggleLabel(label, on, out)).Padding(4, 10).Radius(13).Border(1, t.Border).
		Background(ui.Transparent).Gap(5).AlignItems(ui.Center)
	switch {
	case out:
		b.Background(lossColor.Alpha(0.14)).Border(1, lossColor.Alpha(0.5))
	case on:
		b.Background(t.Accent.Alpha(0.18)).Border(1, t.Accent)
	}
	return b.Children(func() {
		col := t.TextMuted
		switch {
		case out:
			col = hideColor
			ui.Icon(c, iconMinus).Size(11, 11).TextColor(col)
		case on:
			col = t.Text
		}
		el := ui.Text(c, label).FontSize(12.5).TextColor(col)
		if out {
			el.Strikethrough()
		}
	})
}

// toggleLabel names a toggle for assistive technology by what a click does next.
func toggleLabel(label string, on, out bool) string {
	switch {
	case out:
		return label + ", hidden. Click to clear"
	case on:
		return label + ", shown only. Click to hide"
	}
	return label
}

// cardWall shows the cards as art, as many to a row as fit. Only the rows in view are built.
func (v *view) cardWall(c *ui.Context, shown []cards.Card) {
	s := &v.cardsUI
	width := s.wallBox.W
	if width <= 0 {
		width = 600
	}
	cols := max(2, int((width+tileGap)/(tileWidthMin(c)+tileGap)))
	tile := (width - tileGap*float32(cols-1)) / float32(cols)
	rows := (len(shown) + cols - 1) / cols
	wall := ui.List(c, &s.wall, rows, func(r int) {
		ui.Row(c.Key(r)).Gap(tileGap).Padding(6, 0, tileGap, 0).Children(func() {
			for i := r * cols; i < min((r+1)*cols, len(shown)); i++ {
				v.cardTile(c.Key(shown[i].Code), shown[i], tile)
			}
		})
	}).Grow(1).Basis(0).MinWidth(0)
	if b := wall.Bounds(); b.W > 0 && b.W != s.wallBox.W {
		s.wallBox = b
		c.Invalidate()
	}
}

const captionH = 20

// tileWidthMin is the narrowest a tile gets: tileMin, or the widest caption when the font
// makes that wider. Its parts don't shrink, so a narrower tile pushes them into the next one.
func tileWidthMin(c *ui.Context) float32 {
	code, _ := c.MeasureText(0, ui.Span{Text: "PRB02-000", Size: 12, Weight: 500})
	var tag float32
	for _, l := range []cards.Legality{cards.Banned, cards.Restricted, cards.Rotated, cards.Unreleased} {
		name, _ := legalityTag(l)
		w, _ := c.MeasureText(0, ui.Span{Text: name, Size: 11.5})
		tag = max(tag, w)
	}
	// The sim icon is 12 wide, and the row has a 6 gap between each of its four parts.
	return max(tileMin, float32(math.Ceil(float64(code+12+3*6+tag))))
}

func (v *view) cardTile(c *ui.Context, card cards.Card, w float32) {
	s := &v.cardsUI
	t := c.Theme()
	h := w * 7 / 5
	selected := card.Code == s.selected
	ui.Column(c).Width(w).Gap(5).Children(func() {
		box := ui.ButtonBase(c).Label(card.Name+" "+card.Code).Size(w, h).Radius(6).Background(ui.Transparent).
			Padding(0).FocusRing(false)
		if box.Clicked() {
			s.selected, s.viewed, s.versions = card.Code, "", ""
		}
		box.ContextMenu(v.cardMenu(card))
		// Hover and keyboard focus lift the card. It has a printed border of its own, so a ring
		// around it reads as a second frame.
		up := float32(0)
		if box.Hovered() || box.FocusVisible() {
			up = 1
		}
		at := box.AnimateTo("lift", up, ui.Animation{Duration: hoverTime})
		box.Translate(0, -3*at).Shadow(0, 2+8*at, 6+14*at, 0, ui.RGBA(0, 0, 0, 0.3+0.25*at))
		box.Children(func() { v.printArt(c, card.Code, w, h) })
		ui.Row(c).Height(captionH).Gap(6).AlignItems(ui.Center).Children(func() {
			col := t.TextMuted
			if selected {
				col = t.Text
			}
			ui.Text(c, card.Code).FontSize(12).TextColor(col).FontWeight(500)
			if v.simArt.changed[card.Code] {
				ui.Icon(c, iconSim).Size(12, 12).TextColor(simGold).Tooltip("The sim shows other art for this card")
			}
			ui.Spacer(c)
			if tag, tc := legalityTag(card.Legality); tag != "" {
				ui.Text(c, tag).FontSize(11.5).TextColor(tc).MaxLines(1)
			}
		})
		// The selected card is marked by a bar under it, as a selected tab is.
		bar := ui.Box(c).Height(3).Width(w).Radius(2)
		if selected {
			bar.Background(t.Accent)
		}
	})
}

// legalityTag names what keeps a card out of Standard, under its tile. Bans are red. Rotation
// and sets not out yet aren't bans, so they're muted.
func legalityTag(l cards.Legality) (string, ui.Color) {
	switch l {
	case cards.Banned:
		return "Banned", lossColor
	case cards.Restricted:
		return "Restricted", warnColor
	case cards.Rotated:
		return "Rotated", faintDark
	case cards.Unreleased:
		return "Upcoming", oppColor
	}
	return "", ui.Color{}
}

// printArt is a printing's image, or a block in the card's colors while it loads.
func (v *view) printArt(c *ui.Context, key string, w, h float32) ui.Element {
	kind := artFull
	switch {
	case w >= largeArtMin:
		kind = artLarge
	case w > webSmallWidth:
		kind = artCard
	}
	art := v.art.get(key, kind)
	if art == nil && kind != artFull {
		art = v.art.get(key, artFull)
	}
	if art == nil {
		base, _, _ := strings.Cut(key, "_")
		return ui.Box(c).Size(w, h).Radius(6).Background(colorsOf(v.snap.card(base))[0].Card.Alpha(0.35))
	}
	return ui.Image(c, art).Size(w, h).Fit(ui.Cover).Radius(6)
}

// cardList shows the cards as rows with their numbers and the start of their text.
func (v *view) cardRows(c *ui.Context, shown []cards.Card) {
	s := &v.cardsUI
	t := c.Theme()
	ui.List(c, &s.wall, len(shown), func(i int) {
		card := shown[i]
		row := ui.ButtonBase(c.Key(card.Code)).Label(card.Name+" "+card.Code).Padding(5, 8).Radius(6).Gap(12).
			AlignItems(ui.Center).Background(ui.Transparent)
		if card.Code == s.selected {
			row.Background(t.Accent.Alpha(0.12))
		}
		if row.Clicked() {
			s.selected, s.viewed, s.versions = card.Code, "", ""
		}
		row.ContextMenu(v.cardMenu(card))
		row.Children(func() {
			v.printArt(c, card.Code, 30, 42)
			ui.Column(c).Width(190).Children(func() {
				ui.Text(c, card.Name).FontWeight(600).MaxLines(1).Ellipsis("…")
				ui.Text(c, card.Code+" · "+card.Type).FontSize(12).TextColor(t.TextMuted)
			})
			num := func(s string) { ui.Text(c, s).Width(48).TextAlign(ui.End) }
			num(costText(card))
			num(printedPower(card.Power))
			num(counterShort(card.Counter))
			first, _, _ := strings.Cut(card.Effect, "\n")
			ui.Text(c, first).Grow(1).Basis(0).MinWidth(0).TextColor(t.TextMuted).MaxLines(1).Ellipsis("…")
		})
	}).Grow(1).Basis(0).MinWidth(0)
}

func costText(c cards.Card) string {
	if c.Cost < 0 {
		return fmt.Sprintf("%d life", c.Life)
	}
	return fmt.Sprint(c.Cost)
}

func printedPower(p int) string {
	if p == 0 {
		return "–"
	}
	return fmt.Sprint(p)
}

func counterShort(n int) string {
	if n == 0 {
		return "–"
	}
	return fmt.Sprintf("+%dk", n/1000)
}

// cardDetail is the selected card: its art beside its name, numbers and legality, then its
// text, traits and printings.
func (v *view) cardDetail(c *ui.Context, card cards.Card) {
	t := c.Theme()
	info, loaded := v.simArtFor(card)
	ca, _ := v.artsFor(card)
	key := v.viewedKey(card, info, ca)
	if v.prefs.panelWidth <= panelStrip {
		v.panelStrip(c)
		return
	}
	// The card grows with the panel, and the text beside it keeps at least half.
	artW := min(max((v.prefs.panelWidth-54)*0.45, 160), 330)
	ui.Scroll(c.Key("detail "+card.Code)).Fill().Radius(12).Background(panelColor(t)).Border(1, t.Border).
		Margin(0, 0, 18, 14).Children(func() {
		ui.Column(c).Padding(18).Gap(16).Children(func() {
			ui.Row(c).Gap(18).AlignItems(ui.Start).Children(func() {
				art := ui.ButtonBase(c).Label("View the art full size").Padding(0).Radius(6).Background(ui.Transparent).
					FocusRing(false).Shadow(0, 4, 14, 0, ui.RGBA(0, 0, 0, 0.45)).ContextMenu(v.cardMenu(card))
				if art.Clicked() {
					v.cardsUI.viewer = true
				}
				art.Children(func() { v.printArt(c, key, artW, artW*7/5) })
				// A preview no bigger than the card adds nothing.
				if artW < previewWidth(c)*0.85 {
					v.cardPreview(c, art, key)
				}
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
					ui.Text(c, card.Name).FontSize(20).Bold().Wrap()
					ui.Text(c, card.Code+" · "+colorNames(card.Colors)+" "+card.Type).FontSize(13).TextColor(t.TextMuted).Wrap()
					if card.Rarity != card.Type {
						ui.Text(c, card.Rarity).FontSize(13).TextColor(t.TextMuted)
					}
					legalityBadge(c, card)
					v.simArtLine(c, ca, info, loaded)
					ui.Box(c).Height(4)
					cardStats(c, card)
				})
			})
			v.detailTabs(c, key, ca, info, loaded)
			if v.cardsUI.tab == tabArt {
				v.artChoices(c, card, info, loaded)
				return
			}
			v.cardFacts(c, card)
		})
	})
}

// The card panel's tabs.
const (
	tabCard = iota
	tabArt
)

// detailTabs switches the panel between the card's text and its art. key is the picture shown
// large, which the buttons save and copy.
func (v *view) detailTabs(c *ui.Context, key string, ca arts.CardArts, info simArtInfo, loaded bool) {
	t := c.Theme()
	labels := []string{"Card", fmt.Sprintf("Art · %d", artCount(ca, info, loaded))}
	ui.Column(c).Children(func() {
		ui.Row(c).AlignItems(ui.End).Gap(6).Children(func() {
			v.tabList(c, labels)
			ui.Spacer(c)
			ui.Box(c).Padding(0, 0, 6, 0).Children(func() {
				ui.Row(c).Gap(6).Children(func() {
					if smallButton(c, "View full size", iconExpand, false).Clicked() {
						v.cardsUI.viewer = true
					}
					v.artButtons(c, key)
				})
			})
		})
		ui.Box(c).Height(1).Background(t.Border)
	})
}

// tabList is the panel's tabs, with the chosen one underlined.
func (v *view) tabList(c *ui.Context, labels []string) {
	t := c.Theme()
	s := &v.cardsUI
	tabs := ui.TabsBase(c, &s.tab, len(labels))
	tabs.List.Gap(4).Children(func() {
		for i, label := range labels {
			tab := tabs.Tab(i).Column().Gap(6).Padding(7, 4, 0, 4)
			col, bar := t.TextMuted, ui.Transparent
			if i == s.tab {
				col, bar = t.Text, t.Accent
			} else if tab.Hovered() {
				col = t.Text
			}
			tab.Children(func() {
				ui.Text(c, label).FontSize(13).TextColor(col).Padding(0, 8).
					FontWeight(map[bool]int{true: 500, false: 400}[i == s.tab])
				ui.Box(c).Height(2).Radius(1).Background(bar).AlignSelf(ui.Stretch)
			})
		}
	})
}

// artCount is how many pictures the Art tab offers.
func artCount(ca arts.CardArts, info simArtInfo, loaded bool) int {
	n := len(ca.Arts)
	if info.ownFile != "" {
		n++
	}
	if loaded && !info.noSim {
		n++
	}
	return n
}

// simArtLine names the art the sim shows when it isn't the sim's own. A click opens the Art tab.
func (v *view) simArtLine(c *ui.Context, ca arts.CardArts, info simArtInfo, loaded bool) {
	if !loaded || info.noSim || info.cur.Kind == sim.ArtOwn {
		return
	}
	name := "your own file"
	if info.cur.Kind == sim.ArtPrint {
		name = "other art"
		if art, _, ok := ca.Find(info.cur.Key); ok {
			name = art.Name()
			if art.Name() == "Base" {
				name = "the base card"
			}
		}
	}
	b := ui.ButtonBase(c).Label("In the sim: " + name).Background(ui.Transparent).Padding(0).Gap(6).AlignItems(ui.Center).
		AlignSelf(ui.Start)
	if b.Clicked() {
		v.cardsUI.tab = tabArt
	}
	b.Tooltip("Show the art")
	b.Children(func() {
		ui.Icon(c, iconSim).Size(13, 13).TextColor(simGold)
		ui.Text(c, "In the sim: "+name).FontSize(12.5).TextColor(simGold)
	})
}

// cardFacts is the Card tab: the effect, traits, block, set and what the card can't share a
// deck with.
func (v *view) cardFacts(c *ui.Context, card cards.Card) {
	t := c.Theme()
	if card.Effect != "" {
		ui.RichText(c, effectSpans(card.Effect, t)...).FontSize(14).LineHeight(1.5).Wrap()
	}
	tags(c, card.Traits)
	var more []string
	if card.Block != "" {
		more = append(more, "Block "+card.Block)
	}
	if card.Set != "" {
		more = append(more, card.Set)
	}
	tags(c, more)
	if len(card.BannedWith) > 0 {
		names := make([]string, len(card.BannedWith))
		for i, code := range card.BannedWith {
			names[i] = v.snap.card(code).Name + " (" + code + ")"
		}
		ui.Text(c, "Can't share a deck with "+strings.Join(names, " or ")+".").FontSize(13).TextColor(warnColor).Wrap()
	}
}

// panelStrip is the folded card panel. Clicking it opens the panel again.
func (v *view) panelStrip(c *ui.Context) {
	t := c.Theme()
	b := ui.ButtonBase(c).Label("Show card details").Fill().Margin(0, 0, 18, 8).Radius(10).
		Background(panelColor(t)).Border(1, t.Border).Justify(ui.Start).Padding(14, 0, 0, 0)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	if b.Clicked() {
		v.prefs.panelWidth = panelDefault
		v.savePane(v.prefs.panelWidth, store.SettingCardPanelWidth)
	}
	b.Tooltip("Show card details")
	b.Children(func() {
		ui.Column(c).AlignItems(ui.Center).Children(func() { ui.Icon(c, iconPrevStep).Size(16, 16).TextColor(t.TextMuted) })
	})
}

func colorNames(cols []cards.Color) string {
	names := make([]string, len(cols))
	for i, col := range cols {
		names[i] = string(col)
	}
	return strings.Join(names, "/")
}

func legalityBadge(c *ui.Context, card cards.Card) {
	text, col, back := "Legal in Standard", ui.Hex("#8FD8A3"), winColor.Alpha(0.14)
	switch card.Legality {
	case cards.Banned:
		text, col, back = "Banned", ui.Hex("#F29A93"), lossColor.Alpha(0.16)
	case cards.Restricted:
		text, col, back = "Restricted to 1 copy", warnColor, warnColor.Alpha(0.16)
	case cards.Rotated:
		text, col, back = "Rotated out of Standard", c.Theme().TextMuted, c.Theme().Surface
	case cards.Unreleased:
		text, col, back = "Not out yet", oppColor, oppColor.Alpha(0.14)
	case "":
		return
	}
	ui.Text(c, text).FontSize(12).TextColor(col).Padding(3, 9).Radius(11).Background(back).AlignSelf(ui.Start)
}

func cardStats(c *ui.Context, card cards.Card) {
	t := c.Theme()
	stat := func(value, label string) {
		ui.Column(c.Key(label)).Children(func() {
			ui.Text(c, value).FontSize(17).Bold()
			ui.Text(c, label).FontSize(11.5).TextColor(t.TextMuted)
		})
	}
	ui.Row(c).Gap(18).Wrap().Children(func() {
		if card.Cost >= 0 {
			stat(fmt.Sprint(card.Cost), "Cost")
		} else {
			stat(fmt.Sprint(card.Life), "Life")
		}
		if card.Power > 0 {
			stat(fmt.Sprint(card.Power), "Power")
		}
		if card.Type == "Character" {
			stat(counterLabelShort(card.Counter), "Counter")
		}
		if len(card.Attributes) > 0 {
			stat(strings.Join(card.Attributes, "/"), "Attribute")
		}
	})
}

func counterLabelShort(n int) string {
	if n == 0 {
		return "–"
	}
	return fmt.Sprintf("+%d", n)
}

func tags(c *ui.Context, values []string) {
	if len(values) == 0 {
		return
	}
	t := c.Theme()
	ui.Row(c).Gap(6).Wrap().Children(func() {
		for i, v := range values {
			ui.Text(c.Key(i), v).FontSize(12).Padding(3, 9).Radius(11).Background(t.Surface).TextColor(t.Text)
		}
	})
}

// Keyword colors follow the cards: blue for timings, pink for Once Per Turn, red for Counter,
// yellow for Trigger, and dark for DON!! and keywords that are always on.
var (
	kwBlue   = ui.Hex("#1E6FB8")
	kwPink   = ui.Hex("#C2185B")
	kwRed    = ui.Hex("#B3261E")
	kwYellow = ui.Hex("#D9B300")
	kwDark   = ui.Hex("#2B2B30")
)

func keywordColor(k string) (back, text ui.Color) {
	white := ui.Hex("#FFFFFF")
	switch {
	case k == "Counter":
		return kwRed, white
	case k == "Trigger":
		return kwYellow, ui.Hex("#1A1A1A")
	case k == "Once Per Turn":
		return kwPink, white
	case strings.HasPrefix(k, "DON!!"), k == "Rush", k == "Blocker", k == "Double Attack", k == "Banish", k == "Unblockable":
		return kwDark, white
	}
	return kwBlue, white
}

// effectSpans draws an effect's [keywords] as on the card and its {traits} in gold.
func effectSpans(effect string, t *ui.Theme) []ui.Span {
	var out []ui.Span
	for i, line := range strings.Split(effect, "\n") {
		if i > 0 {
			out = append(out, ui.Span{Text: "\n"})
		}
		rest := line
		for rest != "" {
			j := strings.IndexAny(rest, "[{")
			if j < 0 {
				out = append(out, ui.Span{Text: rest, Color: t.Text})
				break
			}
			if j > 0 {
				out = append(out, ui.Span{Text: rest[:j], Color: t.Text})
			}
			closer := "]"
			if rest[j] == '{' {
				closer = "}"
			}
			k := strings.Index(rest[j:], closer)
			if k < 0 {
				out = append(out, ui.Span{Text: rest[j:], Color: t.Text})
				break
			}
			word := rest[j+1 : j+k]
			if closer == "]" {
				back, fore := keywordColor(word)
				out = append(out, ui.Span{Text: word, Color: fore, Background: back, Radius: 4, Weight: 600, Size: 12})
			} else {
				out = append(out, ui.Span{Text: "{" + word + "}", Color: cardGold})
			}
			rest = rest[j+k+1:]
		}
	}
	return out
}
