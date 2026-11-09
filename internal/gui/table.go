package gui

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/store"
)

// tableLayout is how the user arranged a hand-built table: its sort, the order of its columns
// after the first, and the widths they were dragged to, by column ID.
type tableLayout struct {
	Sort   string             `json:"sort,omitempty"`
	Desc   bool               `json:"desc,omitempty"`
	Order  []string           `json:"order,omitempty"`
	Widths map[string]float32 `json:"widths,omitempty"`
}

func tablesFrom(s string) map[string]*tableLayout {
	out := make(map[string]*tableLayout)
	if s != "" {
		_ = json.Unmarshal([]byte(s), &out)
	}
	return out
}

// tableCol is a column of a hand-built table. The first column holds the row's name, stays
// first and takes the room the others leave.
type tableCol struct {
	id, title, tip string
	width          float32
	end            bool
	// asc sorts ascending on the first click, for text. Numbers start with the most.
	asc    bool
	noSort bool
}

// Column limits, in DIPs.
const (
	colMinW    = 40
	colMaxW    = 400
	nameMinW   = 160
	colGripW   = 8
	sortIconSz = 10
)

// tableView is a table's columns as the user arranged them.
type tableView struct {
	id   string
	gap  float32
	cols []tableCol
	lay  *tableLayout
	// sort and desc are the sort in force: the user's, or the table's default.
	sort string
	desc bool
	// trail is room after the columns for a row's own control, such as a chevron.
	trail float32
}

// arrangeTable puts cols in the user's order with their widths, and picks the sort. Columns the
// layout doesn't know keep their place and default width, and IDs it has that cols don't are
// ignored.
func (v *view) arrangeTable(id string, gap float32, cols []tableCol, sort string, desc bool) tableView {
	lay := v.prefs.tables[id]
	if lay == nil {
		lay = &tableLayout{}
	}
	tv := tableView{id: id, gap: gap, lay: lay, sort: sort, desc: desc}
	if lay.Sort != "" && slices.ContainsFunc(cols, func(c tableCol) bool { return c.id == lay.Sort && !c.noSort }) {
		tv.sort, tv.desc = lay.Sort, lay.Desc
	}
	rest := slices.Clone(cols[1:])
	pos := func(c tableCol) int {
		if i := slices.Index(lay.Order, c.id); i >= 0 {
			return i
		}
		return len(lay.Order)
	}
	slices.SortStableFunc(rest, func(a, b tableCol) int { return cmp.Compare(pos(a), pos(b)) })
	tv.cols = append([]tableCol{cols[0]}, rest...)
	for i := range tv.cols {
		if w, ok := lay.Widths[tv.cols[i].id]; ok && i > 0 {
			tv.cols[i].width = min(max(w, colMinW), colMaxW)
		}
	}
	return tv
}

// cell holds a column's content at the column's width.
func (tableView) cell(c *ui.Context, col tableCol, body func()) {
	el := ui.Row(c.Key(col.id)).AlignItems(ui.Center).MinWidth(0).Clip()
	if col.end {
		el.Justify(ui.End)
	}
	if col.width == 0 {
		el.Grow(1).Basis(0).MinWidth(nameMinW)
	} else {
		el.Width(col.width)
	}
	el.Children(body)
}

// colDrag is a column being dragged to a new place.
type colDrag struct{ table, col string }

// tableHeader is the header row. Clicking a title sorts by it and a second click turns the
// sort around, dragging a title moves the column, and dragging the edge after it resizes it.
// A double click on the edge undoes that width, and Reset columns in the menu undoes it all.
func (v *view) tableHeader(c *ui.Context, tv tableView, pad float32) {
	t := c.Theme()
	ui.Row(c).Gap(tv.gap).Padding(8, pad).AlignItems(ui.Center).ContextMenu(func(m *ui.Menu) {
		if m.Item("Reset columns").Disabled(len(tv.lay.Order) == 0 && len(tv.lay.Widths) == 0 && tv.lay.Sort == "").Chosen() {
			*tv.lay = tableLayout{}
			v.saveTable(tv)
		}
	}).Children(func() {
		for i, col := range tv.cols {
			sorted := col.id == tv.sort
			box := ui.Row(c.Key(col.id)).AlignItems(ui.Center).MinWidth(0)
			if col.width == 0 {
				box.Grow(1).Basis(0).MinWidth(nameMinW)
			} else {
				box.Width(col.width)
			}
			box.Children(func() {
				v.headerTitle(c, tv, i, col, sorted)
				if i > 0 {
					v.colGrip(c, tv, col)
				}
			})
		}
		if tv.trail > 0 {
			ui.Box(c).Width(tv.trail)
		}
	})
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
}

func (v *view) headerTitle(c *ui.Context, tv tableView, i int, col tableCol, sorted bool) {
	t := c.Theme()
	label := "Sort by " + col.title
	if col.noSort {
		label = col.title
	}
	b := ui.ButtonBase(c).Label(label).Row().Gap(4).AlignItems(ui.Center).
		Grow(1).Basis(0).MinWidth(0).Padding(2, 0).Background(ui.Transparent)
	if col.end {
		b.Justify(ui.End)
	} else {
		b.Justify(ui.Start)
	}
	if i > 0 {
		b.Drag(colDrag{tv.id, col.id})
		if d, ok := ui.Drop[colDrag](b); ok && d.table == tv.id {
			v.moveColumn(tv, d.col, col.id)
		}
		if d, ok := ui.DragOver[colDrag](b); ok && d.table == tv.id && d.col != col.id {
			b.BorderStartWidth(2).BorderColor(t.Accent)
		}
	}
	b.Tooltip(headerTip(col, i > 0))
	if b.Clicked() && !col.noSort {
		desc := !col.asc
		if tv.sort == col.id {
			desc = !tv.desc
		}
		tv.lay.Sort, tv.lay.Desc = col.id, desc
		v.saveTable(tv)
	}
	col2 := t.TextMuted
	if sorted || b.Hovered() {
		col2 = t.Text
	}
	b.Children(func() {
		align := ui.Start
		if col.end {
			align = ui.End
		}
		ui.Text(c, col.title).FontSize(12).TextColor(col2).SingleLine().TextAlign(align)
		switch {
		case sorted:
			icon := iconChevron
			if !tv.desc {
				icon = iconChevronUp
			}
			ui.Icon(c, icon).FontSize(sortIconSz).TextColor(t.Accent)
		case b.Hovered() && !col.noSort:
			ui.Icon(c, iconChevron).FontSize(sortIconSz).TextColor(faintColor(t))
		}
	})
}

func headerTip(col tableCol, movable bool) string {
	var how []string
	if !col.noSort {
		how = append(how, "click to sort")
	}
	if movable {
		how = append(how, "drag to move")
	}
	tip := strings.Join(how, ", ")
	if tip != "" {
		tip = strings.ToUpper(tip[:1]) + tip[1:] + "."
	}
	if col.tip != "" {
		tip = strings.TrimSpace(col.tip + " " + tip)
	}
	return tip
}

// colGrip is the edge after a column, which resizes it.
func (v *view) colGrip(c *ui.Context, tv tableView, col tableCol) {
	t := c.Theme()
	g := ui.Box(c).Width(colGripW).Height(18).Margin(0, -colGripW/2, 0, 0).Cursor(ui.CursorResizeEW).Label("Resize " + col.title)
	line := t.Border.Alpha(0.6)
	if g.Hovered() || g.Pressed() {
		line = t.Accent
	}
	g.Children(func() { ui.Box(c).Width(2).Height(18).Margin(0, 0, 0, colGripW/2-1).Background(line) })
	if dx, _, ok := g.Dragged(); ok && dx != 0 {
		if tv.lay.Widths == nil {
			tv.lay.Widths = make(map[string]float32)
		}
		tv.lay.Widths[col.id] = min(max(col.width+dx, colMinW), colMaxW)
		v.saveTable(tv)
	}
	if g.DoubleClicked() {
		delete(tv.lay.Widths, col.id)
		v.saveTable(tv)
	}
}

// moveColumn puts the column from where the column to is, shifting the ones between.
func (v *view) moveColumn(tv tableView, from, to string) {
	ids := make([]string, 0, len(tv.cols)-1)
	for _, c := range tv.cols[1:] {
		ids = append(ids, c.id)
	}
	i, j := slices.Index(ids, from), slices.Index(ids, to)
	if i < 0 || j < 0 || i == j {
		return
	}
	ids = slices.Insert(slices.Delete(ids, i, i+1), j, from)
	tv.lay.Order = ids
	v.saveTable(tv)
}

// saveTable keeps the table's layout, and stores every table's once the changes stop for a
// moment, since a resize changes it on each frame of the drag.
func (v *view) saveTable(tv tableView) {
	if v.prefs.tables == nil {
		v.prefs.tables = make(map[string]*tableLayout)
	}
	v.prefs.tables[tv.id] = tv.lay
	v.tableSaves++
	n := v.tableSaves
	v.do(func(a actions) {
		a.After(sideSaveDelay, func() {
			if n != v.tableSaves {
				return
			}
			b, err := json.Marshal(v.prefs.tables)
			if err == nil {
				v.do(func(a actions) { a.SetSetting(store.SettingTables, string(b)) })
			}
		})
	})
}

// sortRows sorts rows by the column key names, using by's comparison, ascending. Rows few says
// have too few games for their number go last either way, and ties keep tie's order.
func sortRows[T any](rows []T, key string, desc bool, by map[string]func(a, b T) int, few map[string]func(T) bool, tie func(a, b T) int) {
	f, ok := by[key]
	if !ok {
		return
	}
	isFew := few[key]
	slices.SortStableFunc(rows, func(a, b T) int {
		if isFew != nil {
			if fa, fb := isFew(a), isFew(b); fa != fb {
				if fa {
					return 1
				}
				return -1
			}
		}
		r := f(a, b)
		if desc {
			r = -r
		}
		if r == 0 && tie != nil {
			return tie(a, b)
		}
		return r
	})
}
