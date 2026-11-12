package gui

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/sim"
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
	shown := s.results(v.snap.Cards, v.prefs.cardSort)
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
				if v.prefs.cardList {
					ui.Column(c).Fill().MinHeight(0).Padding(0, 14, 0, 0).Children(func() {
						v.cardHeader(c)
						v.cardRows(c, shown)
					})
					return
				}
				ui.Row(c).Fill().MinHeight(0).AlignItems(ui.Stretch).Padding(0, 14, 0, 0).Children(func() {
					v.cardWall(c, shown)
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
						ui.Icon(c, iconCheck).Size(14, 14).TextColor(winColor(t))
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
	shown := max(total-v.prefs.panelWidth-1, 40)
	size := shown
	split := ui.Split(c, &size, list, panel).Grow(1).Basis(0).MinWidth(0)
	if split.Changed() {
		v.dragPane(prefPanelWidth, &v.panelDrag, shown-size, snapPanel)
	}
	// Bounds is empty on the frame the page comes back, so the old width is kept until then.
	if b := split.Bounds(); b.W > 0 && b.W != s.splitBox.W {
		s.splitBox = b
		c.Invalidate()
	}
}

func (v *view) cardSearch(c *ui.Context) {
	s := &v.cardsUI
	field := ui.Autocomplete(c, &s.query, s.pool.suggestions).Label("Search cards").
		Placeholder("Search names, effects and traits")
	if v.entered {
		field.Focus()
	}
	if field.Changed() && s.takeSuggestion() {
		c.Invalidate()
	}
}

func (v *view) quickFilters(c *ui.Context) {
	s := &v.cardsUI
	// Sized so every filter fits on one row, with the longest power range, in the default window.
	ui.Row(c).Gap(16).Wrap().AlignItems(ui.End).Children(func() {
		filterGroup(c, "Color", func() {
			joined(c, len(cardColors), 26, 7, func(i int) (string, bool) {
				col := cardColors[i]
				return string(col), slices.Contains(s.colors, col)
			}, func(i int) { s.toggleColor(cardColors[i]) }, func(i int) {
				ui.Box(c).Size(11, 11).Radius(6).Background(leaderColors[cardColors[i]].Card).
					Border(1, ui.RGBA(255, 255, 255, 0.25))
			})
		})
		filterGroup(c, "Type", func() {
			joined(c, len(cardKinds), 32, 8, func(i int) (string, bool) { return cardKinds[i], s.kind == i },
				s.setKind, nil)
		})
		filterGroup(c, "Cost", func() { v.costToggles(c) })
		filterGroup(c, "Power", func() { v.powerPill(c) })
	})
}

func filterGroup(c *ui.Context, label string, body func()) {
	ui.Column(c.Key(label)).Gap(5).Children(func() {
		ui.Text(c, label).FontSize(11.5).TextColor(c.Theme().TextMuted)
		body()
	})
}

// joined draws n toggles as one control split by hairlines, each at least width wide and padded
// by pad on each side. item names a toggle and says whether it's on, and draw, when set, draws it
// instead of its name.
func joined(c *ui.Context, n int, width, pad float32, item func(int) (string, bool), toggle, draw func(int)) {
	t := c.Theme()
	ui.Row(c).Height(30).Radius(8).Border(1, t.Border).Clip().Children(func() {
		for i := range n {
			label, on := item(i)
			b := ui.ButtonBase(c.Key(label)).Label(label).Height(30).MinWidth(width).Padding(0, pad).Center().
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
				ui.Text(c, label).FontSize(12.5).TextColor(toggleText(t, on))
			})
		}
	})
}

func toggleText(t *ui.Theme, on bool) ui.Color {
	if on {
		return onText
	}
	return t.TextMuted
}

var onText = ui.Hex("#CFE6FB")

// powerPillWidth fits the longest power range, such as "10000 or less", and its clear button.
const powerPillWidth = 150

// emptyColor marks a value no card has under the other filters. It stays clickable.
var emptyColor = ui.Hex("#5C5C64")

// costToggles names each cost's count for assistive technology only, since the card count
// already shows once a cost is picked.
func (v *view) costToggles(c *ui.Context) {
	s := &v.cardsUI
	t := c.Theme()
	joined(c, len(s.curve), 26, 4, func(cost int) (string, bool) {
		return fmt.Sprintf("Cost %d, %s", cost, cardCount(s.curve[cost])), slices.Contains(s.costs, cost)
	}, s.toggleCost, func(cost int) {
		col := toggleText(t, slices.Contains(s.costs, cost))
		if s.curve[cost] == 0 && col != onText {
			col = emptyColor
		}
		ui.Text(c, fmt.Sprint(cost)).FontSize(12.5).TextColor(col)
	})
}

func (v *view) powerPill(c *ui.Context) {
	s := &v.cardsUI
	t := c.Theme()
	all := s.pool.power
	set := s.power != all
	label := s.power.label(all)
	// A fixed width keeps the popover, which hangs off the pill, from sliding under the pointer
	// as the label changes during a drag.
	box := ui.Row(c).Width(powerPillWidth).Height(30).Radius(8).Border(1, t.Border).AlignItems(ui.Center)
	if set {
		box.Background(t.Accent.Alpha(0.16)).Border(1, t.Accent.Alpha(0.55))
	}
	if !kindHasPower(s.kind) {
		box.Tooltip(cardKinds[s.kind] + "s have no power").Opacity(0.45)
	}
	box.Children(func() {
		open := ui.ButtonBase(c.Key("power")).Label("Power: "+label).Grow(1).Height(30).Padding(0, 10, 0, 12).
			Gap(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).Background(ui.Transparent).
			Disabled(!kindHasPower(s.kind))
		if open.Clicked() {
			s.powerOpen = !s.powerOpen
		}
		ui.PopoverBase(c, box, &s.powerOpen, func(panel ui.Element) {
			popover(panel.Margin(6, 0, 0, 0).Padding(16), t)
			v.powerPopover(c)
		})
		open.Children(func() {
			ui.Text(c, label).FontSize(12.5).TextColor(toggleText(t, set)).SingleLine()
			if !set {
				ui.Icon(c, iconChevron).FontSize(12).TextColor(t.TextMuted)
			}
		})
		if !set {
			return
		}
		x := ui.ButtonBase(c.Key("power clear")).Label("Clear the power range").Size(22, 22).Margin(0, 6, 0, 0).
			Radius(11).Center().Background(ui.Transparent)
		if x.Hovered() {
			x.Background(t.SurfaceHover)
		}
		if x.Clicked() {
			s.power = all
		}
		x.Children(func() { ui.Icon(c, iconClose).FontSize(11).TextColor(t.TextMuted) })
	})
}

// powerPopover offers the three ways card text puts a threshold, besides the slider.
func (v *view) powerPopover(c *ui.Context) {
	s := &v.cardsUI
	t := c.Theme()
	all := s.pool.power
	ui.Column(c).Width(340).Gap(8).Children(func() {
		ui.RangeSlider(c, &s.power.lo, &s.power.hi, all.lo, all.hi, powerStep).Label("Power")
		ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
			ui.Text(c, fmt.Sprint(all.lo)).FontSize(11.5).TextColor(faintColor(t))
			ui.Text(c, fmt.Sprint(all.hi)).FontSize(11.5).TextColor(faintColor(t))
		})
		if s.power == all {
			ui.Text(c, "Drag either end to narrow the power.").FontSize(12).TextColor(faintColor(t))
			return
		}
		// The shortcuts work from the end that moved off the pool's end, the bottom if both did.
		at := s.power.lo
		if at == all.lo {
			at = s.power.hi
		}
		type shortcut struct {
			name string
			r    powerRange
		}
		slots := []shortcut{
			{fmt.Sprintf("Exactly %g", at), powerRange{at, at}},
			{fmt.Sprintf("%g or less", at), powerRange{all.lo, at}},
			{fmt.Sprintf("%g or more", at), powerRange{at, all.hi}},
		}
		ui.Row(c).Gap(6).Margin(6, 0, 0, 0).Children(func() {
			for i, sc := range slots {
				// At either end of the pool a shortcut repeats an earlier one or means any power. It
				// stays in its slot, disabled, so the row keeps its width under the pointer.
				off := sc.r == all || slices.ContainsFunc(slots[:i], func(o shortcut) bool { return o.r == sc.r })
				b := filterToggle(c.Key(i), sc.name, s.power == sc.r, false).Disabled(off)
				if off {
					b.Opacity(0.4)
				}
				if b.Clicked() {
					s.power = sc.r
				}
			}
		})
	})
}

func (v *view) chipRow(c *ui.Context, count int) {
	ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
		ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Wrap().AlignItems(ui.Center).MinHeight(chipLineH).
			Children(func() { v.chips(c) })
		ui.Row(c).Gap(8).AlignItems(ui.Center).Height(chipLineH).Children(func() { v.viewControls(c, count) })
	})
}

// chipLineH is the height of the chip row's first line, which the Images | List switch sets.
const chipLineH = 32

func (v *view) chips(c *ui.Context) {
	s := &v.cardsUI
	t := c.Theme()
	// More filters comes first, so its menu stays put as chips come and go.
	more := filterPill(c.Key("more"), "More filters", false)
	if more.Clicked() {
		s.more = !s.more
	}
	ui.PopoverBase(c, more, &s.more, func(panel ui.Element) {
		popover(panel.Margin(6, 0, 0, 0).Padding(14), t)
		v.moreFilters(c)
	})
	order := s.chipOrder()
	for i, ch := range order {
		// The joiner sits with the chip after it, so a wrap never leaves it ending a line.
		join := i > 0 && ch.field.several && !ch.out && !order[i-1].out && order[i-1].field == ch.field
		ui.Row(c.Key(fmt.Sprint("chip", ch.field.name, ch.value))).Gap(8).AlignItems(ui.Center).Children(func() {
			if join {
				v.joiner(c, ch.field)
			}
			v.chip(c, ch)
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
}

func (v *view) chip(c *ui.Context, ch cardChip) {
	s := &v.cardsUI
	t := c.Theme()
	b := ui.ButtonBase(c.Key("chip")).Label("Remove "+ch.value).Padding(5, 10, 5, 12).Radius(16).Gap(6).
		AlignItems(ui.Center)
	tinted(b, t)
	if ch.out {
		b.Background(lossColor(t).Alpha(0.14)).Border(1, lossColor(t).Alpha(0.5))
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

// joiner reads "or" or "and" between two shown chips of a field, and a click flips it.
func (v *view) joiner(c *ui.Context, field *chipField) {
	s := &v.cardsUI
	t := c.Theme()
	word, next := "or", "all"
	if s.every[field] {
		word, next = "and", "any"
	}
	does := fmt.Sprintf("Show cards with %s of these %ss", next, strings.ToLower(field.name))
	b := ui.ButtonBase(c.Key("join")).Label(does).Tooltip(does).Height(24).Padding(0, 7, 0, 9).Radius(12).Gap(4).
		AlignItems(ui.Center).Border(1, t.Accent.Alpha(0.45)).Background(ui.Transparent)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	if b.Clicked() {
		s.toggleEvery(field)
	}
	b.Children(func() {
		ui.Text(c, word).FontSize(12).FontWeight(500).TextColor(onText)
		ui.Icon(c, iconSwap).Size(11, 11).TextColor(faintColor(t))
	})
}

func (v *view) viewControls(c *ui.Context, count int) {
	t := c.Theme()
	status := ""
	if sync := v.cardSync; sync.running() {
		status = fmt.Sprintf("Updating · %d of %d sets", sync.done, sync.total)
	} else if sync := v.artSync; sync.running() {
		status = fmt.Sprintf("Reading Bandai's card list · %d of %d", sync.done, sync.total)
	}
	// Faint, not the accent, so the status doesn't read as a link next to Clear all.
	if status != "" {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Margin(0, 10, 0, 0).Children(func() {
			ui.Spinner(c).FontSize(12)
			ui.Text(c, status).FontSize(12.5).TextColor(faintColor(t)).SingleLine()
		})
	}
	ui.Text(c, cardCount(count)).FontSize(13).TextColor(t.TextMuted).SingleLine()
	v.sortPill(c)
	view := boolInt(v.prefs.cardList)
	if ui.Segmented(c, &view, "Images", "List").Label("Show as").Changed() {
		prefCardList.set(v, view == 1)
	}
}

// sortPill isn't tinted, since sorting narrows nothing.
func (v *view) sortPill(c *ui.Context) {
	t := c.Theme()
	by := v.prefs.cardSort
	key := sortKeyOf(by.key)
	label := by.label()
	b := ui.ButtonBase(c.Key("sort")).Label("Sort: "+label).Padding(5, 12).Radius(16).Border(1, t.Border).
		Background(ui.Transparent).Gap(6).AlignItems(ui.Center)
	b.PopoverMenuButton(func(m *ui.Menu) {
		for _, k := range sortKeys {
			if m.Item(k.title).Radio(k.id == key.id).Chosen() {
				v.setCardSort(by.clicked(k.id))
			}
		}
		m.Separator()
		for i, d := range key.dirs {
			if m.Item(d).Radio(boolInt(by.flip) == i).Chosen() {
				v.setCardSort(cardSort{key: key.id, flip: i == 1})
			}
		}
	})
	b.Children(func() {
		ui.Icon(c, iconSort).Size(13, 13).TextColor(t.TextMuted)
		ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(t.TextMuted).SingleLine()
		ui.Icon(c, iconChevron).FontSize(12).TextColor(t.TextMuted)
	})
}

func (v *view) setCardSort(by cardSort) {
	prefCardSort.set(v, by)
}

func (v *view) moreFilters(c *ui.Context) {
	s := &v.cardsUI
	t := c.Theme()
	group := func(field *chipField) {
		ui.Column(c.Key(field.name)).Gap(6).Children(func() {
			ui.Text(c, field.name).FontSize(12).TextColor(t.TextMuted)
			ui.Row(c).Gap(6).Wrap().MaxWidth(520).Children(func() {
				for _, val := range field.values(s.pool) {
					on, out := s.chipState(field, val)
					if filterToggle(c.Key(val), val, on, out).Clicked() {
						s.cycleChip(field, val)
					}
				}
			})
		})
	}
	ui.Column(c).Gap(14).Children(func() {
		var more []*chipField
		for _, f := range chipFields {
			if f.more > 0 {
				more = append(more, f)
			}
		}
		slices.SortFunc(more, func(a, b *chipField) int { return cmp.Compare(a.more, b.more) })
		for _, f := range more {
			group(f)
		}
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
		b.Background(lossColor(t).Alpha(0.14)).Border(1, lossColor(t).Alpha(0.5))
	case on:
		tinted(b, t)
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
	for _, l := range legalities {
		w, _ := c.MeasureText(0, ui.Span{Text: l.tag, Size: 11.5})
		tag = max(tag, w)
	}
	// The sim icon is 12 wide, and the row has a 6 gap between each of its four parts.
	return max(tileMin, float32(math.Ceil(float64(code+12+3*6+tag))))
}

func (v *view) cardTile(c *ui.Context, card cards.Card, w float32) {
	s := &v.cardsUI
	t := c.Theme()
	h := cardHeight(w)
	selected := card.Code == s.selected
	ui.Column(c).Width(w).Gap(5).Children(func() {
		box := ui.ButtonBase(c).Label(card.Name+" "+card.Code).Size(w, h).Radius(6).Background(ui.Transparent).
			Padding(0).FocusRing(false)
		if box.Clicked() {
			s.selected, s.viewed, s.versions = card.Code, "", ""
		}
		box.PopoverMenu(v.cardMenu(card))
		// Hover and keyboard focus lift the card. It has a printed border of its own, so a ring
		// around it reads as a second frame.
		up := float32(0)
		if box.Hovered() || box.FocusVisible() {
			up = 1
		}
		at := box.AnimateTo("lift", up, ui.Animation{Duration: hoverTime})
		box.Translate(0, -3*at).Shadow(0, 2+8*at, 6+14*at, 0, ui.RGBA(0, 0, 0, 0.3+0.25*at))
		box.Children(func() { v.cardImage(c, card.Code, w).Radius(6) })
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
			if l := legalityOf(card.Legality); l.tag != "" {
				ui.Text(c, l.tag).FontSize(11.5).TextColor(l.tagColor(t)).MaxLines(1)
			}
		})
		// The selected card is marked by a bar under it, as a selected tab is.
		bar := ui.Box(c).Height(3).Width(w).Radius(2)
		if selected {
			bar.Background(t.Accent)
		}
	})
}

// legality words a card legality: label is its More filters value, tag names what keeps a
// card out of Standard under its tile, and badge says it on the card's panel. Bans are red.
// Rotation and sets not out yet aren't bans, so their tags are muted.
type legality struct {
	is                cards.Legality
	label, tag, badge string
	tagColor          func(*ui.Theme) ui.Color
	ink, back         func(*ui.Theme) ui.Color
}

func themed(col ui.Color) func(*ui.Theme) ui.Color { return func(*ui.Theme) ui.Color { return col } }

// legalities are in the More filters' order.
var legalities = []legality{
	{
		is: cards.Legal, label: "Legal", badge: "Legal in Standard",
		ink: themed(ui.Hex("#8FD8A3")), back: func(t *ui.Theme) ui.Color { return winColor(t).Alpha(0.14) },
	},
	{
		is: cards.Rotated, label: "Rotated", tag: "Rotated", badge: "Rotated out of Standard", tagColor: themed(faintDark),
		ink: func(t *ui.Theme) ui.Color { return t.TextMuted }, back: func(t *ui.Theme) ui.Color { return t.Surface },
	},
	{
		is: cards.Banned, label: "Banned", tag: "Banned", badge: "Banned", tagColor: lossColor,
		ink: themed(hideColor), back: func(t *ui.Theme) ui.Color { return lossColor(t).Alpha(0.16) },
	},
	{
		is: cards.Restricted, label: "Restricted", tag: "Restricted", badge: "Restricted to 1 copy", tagColor: themed(warnColor),
		ink: themed(warnColor), back: themed(warnColor.Alpha(0.16)),
	},
	{
		is: cards.Unreleased, label: "Upcoming", tag: "Upcoming", badge: "Not out yet", tagColor: themed(oppColor),
		ink: themed(oppColor), back: themed(oppColor.Alpha(0.14)),
	},
}

// legalityOf is how a legality reads. One the card data doesn't give has no words.
func legalityOf(l cards.Legality) legality {
	for _, e := range legalities {
		if e.is == l {
			return e
		}
	}
	return legality{}
}

func legalityLabels() []string {
	out := make([]string, len(legalities))
	for i, l := range legalities {
		out[i] = l.label
	}
	return out
}

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
		row.PopoverMenu(v.cardMenu(card))
		row.Children(func() {
			v.cardImage(c, card.Code, rowArtW).Radius(6)
			ui.Column(c).Width(rowNameW).Children(func() {
				ui.Text(c, card.Name).FontWeight(600).MaxLines(1).Ellipsis("…")
				ui.Text(c, card.Code+" · "+card.Type).FontSize(12).TextColor(t.TextMuted)
			})
			num := func(s string) { ui.Text(c, s).Width(rowNumW).TextAlign(ui.End) }
			num(costText(card))
			num(printedPower(card))
			num(counterText(card.Counter, true, "–"))
			first, _, _ := strings.Cut(card.Effect, "\n")
			ui.Text(c, first).Grow(1).Basis(0).MinWidth(0).TextColor(t.TextMuted).MaxLines(1).Ellipsis("…")
		})
	}).Grow(1).Basis(0).MinWidth(0).MinHeight(0)
}

// The list's columns, in DIPs. A number column fits "Counter" and its chevron.
const (
	rowArtW  = 30
	rowNameW = 190
	rowNumW  = 60
	rowGap   = 12
	rowPadX  = 8
)

func (v *view) cardHeader(c *ui.Context) {
	t := c.Theme()
	by := v.prefs.cardSort
	title := func(key string, end bool) {
		k := sortKeyOf(key)
		sorted := sortKeyOf(by.key).id == key
		b := ui.ButtonBase(c.Key(key)).Label("Sort by "+k.title).Row().Gap(4).AlignItems(ui.Center).
			Padding(2, 0).Background(ui.Transparent)
		if b.Clicked() {
			v.setCardSort(by.clicked(key))
		}
		sortTitle(c, b, k.title, end, sorted, by.desc(), true)
	}
	ui.Row(c).Gap(rowGap).Padding(8, rowPadX).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(rowArtW)
		ui.Row(c).Width(rowNameW).Gap(10).AlignItems(ui.Center).Children(func() {
			title(sortName, false)
			title(sortSet, false)
		})
		for _, key := range []string{sortCost, sortPower, sortCounter} {
			ui.Row(c.Key(key)).Width(rowNumW).Justify(ui.End).AlignItems(ui.Center).Children(func() { title(key, true) })
		}
		ui.Text(c, "Effect").FontSize(12).TextColor(t.TextMuted).Grow(1).Basis(0).MinWidth(0).SingleLine()
	})
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
}

func costText(c cards.Card) string {
	if c.IsLeader() {
		return fmt.Sprintf("%d life", c.Life)
	}
	return fmt.Sprint(c.Cost)
}

func printedPower(c cards.Card) string {
	if !c.HasPower() {
		return "–"
	}
	return fmt.Sprint(c.Power)
}

// counterText writes a counter as "+1000", or "+1k" when short. none stands for no counter.
func counterText(n int, short bool, none string) string {
	switch {
	case n == 0:
		return none
	case short:
		return fmt.Sprintf("+%dk", n/1000)
	}
	return fmt.Sprintf("+%d", n)
}

// counterChip is a counter as the More filters name it.
func counterChip(n int) string { return counterText(n, false, "No counter") }

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
					FocusRing(false).Shadow(0, 4, 14, 0, ui.RGBA(0, 0, 0, 0.45)).PopoverMenu(v.cardMenu(card))
				if art.Clicked() {
					v.cardsUI.viewer = true
				}
				art.Children(func() { v.cardImage(c, key, artW).Radius(6) })
				if needsPreview(c, artW) {
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
			v.detailTabs(c, key, len(v.artEntries(card, ca, info, loaded)))
			if v.cardsUI.tab == tabArt {
				v.artChoices(c, card, info, loaded)
				return
			}
			v.cardFacts(c, card)
		})
	})
}

const (
	tabCard = iota
	tabArt
)

// detailTabs switches the panel between the card's text and its art. key is the picture shown
// large, which the buttons save and copy, and pictures how many the Art tab offers.
func (v *view) detailTabs(c *ui.Context, key string, pictures int) {
	t := c.Theme()
	labels := []string{"Card", fmt.Sprintf("Art · %d", pictures)}
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

func (v *view) panelStrip(c *ui.Context) {
	t := c.Theme()
	b := ui.ButtonBase(c).Label("Show card details").Fill().Margin(0, 0, 18, 8).Radius(10).
		Background(panelColor(t)).Border(1, t.Border).Justify(ui.Start).Padding(14, 0, 0, 0)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	if b.Clicked() {
		prefPanelWidth.set(v, panelDefault)
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
	t := c.Theme()
	l := legalityOf(card.Legality)
	if l.badge == "" {
		return
	}
	ui.Text(c, l.badge).FontSize(12).TextColor(l.ink(t)).Padding(3, 9).Radius(11).Background(l.back(t)).AlignSelf(ui.Start)
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
		if !card.IsLeader() {
			stat(fmt.Sprint(card.Cost), "Cost")
		} else {
			stat(fmt.Sprint(card.Life), "Life")
		}
		if card.HasPower() {
			stat(fmt.Sprint(card.Power), "Power")
		}
		if card.Type == cards.Character {
			stat(counterText(card.Counter, false, "–"), "Counter")
		}
		if len(card.Attributes) > 0 {
			stat(strings.Join(card.Attributes, "/"), "Attribute")
		}
	})
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
