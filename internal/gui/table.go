package gui

import (
	"cmp"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

// tableLayout is how the user arranged a hand-built table: its sort, the order of its columns
// after the first, and the widths they were dragged to, by column ID.
type tableLayout struct {
	Sort   string             `json:"sort,omitempty"`
	Desc   bool               `json:"desc,omitempty"`
	Order  []string           `json:"order,omitempty"`
	Widths map[string]float32 `json:"widths,omitempty"`
}

// tableCol is a column of a table of T rows. In a hand-built table the first column holds the
// row's name, stays first and takes the room the others leave.
type tableCol[T any] struct {
	id, title, tip string
	width          float32
	end            bool
	// asc sorts ascending on the first click, for text. Numbers start with the most.
	asc bool
	// by compares two rows ascending. A column without it doesn't sort.
	by func(a, b T) int
	// few reports a row with too few games for the column's number. Those rows sort last
	// either way.
	few  func(T) bool
	cell func(c *ui.Context, r T)
}

func (col tableCol[T]) sortable() bool { return col.by != nil }

// Column limits, in DIPs.
const (
	colMinW    = 40
	colMaxW    = 400
	nameMinW   = 160
	colGripW   = 8
	sortIconSz = 10
)

// tableView is a hand-built table's columns as the user arranged them.
type tableView[T any] struct {
	v    *view
	id   string
	gap  float32
	cols []tableCol[T]
	lay  *tableLayout
	// sort and desc are the sort in force: the user's, or the table's default.
	sort string
	desc bool
	// tie orders rows the sort finds equal. Without it they keep their order.
	tie func(a, b T) int
	// trail is room after the columns for a row's own control, such as a chevron.
	trail float32
}

// arrangeTable puts cols in the user's order with their widths, and picks the sort. Columns the
// layout doesn't know keep their place and default width, and IDs it has that cols don't are
// ignored.
func arrangeTable[T any](v *view, id string, gap float32, cols []tableCol[T], sort string, desc bool) tableView[T] {
	lay := v.prefs.tables[id]
	if lay == nil {
		lay = &tableLayout{}
	}
	tv := tableView[T]{v: v, id: id, gap: gap, lay: lay, sort: sort, desc: desc}
	if lay.Sort != "" && slices.ContainsFunc(cols, func(c tableCol[T]) bool { return c.id == lay.Sort && c.sortable() }) {
		tv.sort, tv.desc = lay.Sort, lay.Desc
	}
	rest := slices.Clone(cols[1:])
	pos := func(c tableCol[T]) int {
		if i := slices.Index(lay.Order, c.id); i >= 0 {
			return i
		}
		return len(lay.Order)
	}
	slices.SortStableFunc(rest, func(a, b tableCol[T]) int { return cmp.Compare(pos(a), pos(b)) })
	tv.cols = append([]tableCol[T]{cols[0]}, rest...)
	for i := range tv.cols {
		if w, ok := lay.Widths[tv.cols[i].id]; ok && i > 0 {
			tv.cols[i].width = min(max(w, colMinW), colMaxW)
		}
	}
	return tv
}

func (tv tableView[T]) cells(c *ui.Context, r T) {
	for _, col := range tv.cols {
		el := colBox(c, col.id, col.width).Clip()
		if col.end {
			el.Justify(ui.End)
		}
		el.Children(func() { col.cell(c, r) })
	}
}

func colBox(c *ui.Context, id string, width float32) ui.Element {
	el := ui.Row(c.Key(id)).AlignItems(ui.Center).MinWidth(0)
	if width == 0 {
		return el.Grow(1).Basis(0).MinWidth(nameMinW)
	}
	return el.Width(width)
}

type colDrag struct{ table, col string }

// header is the header row. Clicking a title sorts by it and a second click turns the sort
// around, dragging a title moves the column, and dragging the edge after it resizes it. A double
// click on the edge undoes that width, and Reset columns in the menu undoes it all.
func (tv tableView[T]) header(c *ui.Context, pad float32) {
	t := c.Theme()
	ui.Row(c).Gap(tv.gap).Padding(8, pad).AlignItems(ui.Center).PopoverMenu(func(m *ui.Menu) {
		if m.Item("Reset columns").Disabled(len(tv.lay.Order) == 0 && len(tv.lay.Widths) == 0 && tv.lay.Sort == "").Chosen() {
			*tv.lay = tableLayout{}
			tv.save()
		}
	}).Children(func() {
		for i, col := range tv.cols {
			colBox(c, col.id, col.width).Children(func() {
				tv.headerTitle(c, i, col)
				if i > 0 {
					tv.colGrip(c, col)
				}
			})
		}
		if tv.trail > 0 {
			ui.Box(c).Width(tv.trail)
		}
	})
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
}

func (tv tableView[T]) headerTitle(c *ui.Context, i int, col tableCol[T]) {
	t := c.Theme()
	label := "Sort by " + col.title
	if !col.sortable() {
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
			tv.moveColumn(d.col, col.id)
		}
		if d, ok := ui.DragOver[colDrag](b); ok && d.table == tv.id && d.col != col.id {
			b.BorderStartWidth(2).BorderColor(t.Accent)
		}
	}
	b.Tooltip(headerTip(col.tip, col.sortable(), i > 0))
	if b.Clicked() && col.sortable() {
		desc := !col.asc
		if tv.sort == col.id {
			desc = !tv.desc
		}
		tv.lay.Sort, tv.lay.Desc = col.id, desc
		tv.save()
	}
	sortTitle(c, b, col.title, col.end, col.id == tv.sort, tv.desc, col.sortable())
}

// sortTitle draws a header title into b, with the accent chevron when its column is sorted and
// a faint one on hover when it can be.
func sortTitle(c *ui.Context, b ui.Element, title string, end, sorted, desc, sortable bool) {
	t := c.Theme()
	col := t.TextMuted
	if sorted || b.Hovered() {
		col = t.Text
	}
	b.Children(func() {
		align := ui.Start
		if end {
			align = ui.End
		}
		ui.Text(c, title).FontSize(12).TextColor(col).SingleLine().TextAlign(align)
		switch {
		case sorted:
			icon := iconChevron
			if !desc {
				icon = iconChevronUp
			}
			ui.Icon(c, icon).FontSize(sortIconSz).TextColor(t.Accent)
		case b.Hovered() && sortable:
			ui.Icon(c, iconChevron).FontSize(sortIconSz).TextColor(faintColor(t))
		}
	})
}

func headerTip(colTip string, sortable, movable bool) string {
	var how []string
	if sortable {
		how = append(how, "click to sort")
	}
	if movable {
		how = append(how, "drag to move")
	}
	tip := strings.Join(how, ", ")
	if tip != "" {
		tip = strings.ToUpper(tip[:1]) + tip[1:] + "."
	}
	if colTip != "" {
		tip = strings.TrimSpace(colTip + " " + tip)
	}
	return tip
}

// colGrip is the edge after a column, which resizes it.
func (tv tableView[T]) colGrip(c *ui.Context, col tableCol[T]) {
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
		tv.save()
	}
	if g.DoubleClicked() {
		delete(tv.lay.Widths, col.id)
		tv.save()
	}
}

// moveColumn puts the column from where the column to is, shifting the ones between.
func (tv tableView[T]) moveColumn(from, to string) {
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
	tv.save()
}

// save keeps the table's layout, and stores every table's once the changes stop for a moment,
// since a resize changes it on each frame of the drag.
func (tv tableView[T]) save() {
	v := tv.v
	v.prefs.tables[tv.id] = tv.lay
	v.later(keyTables, func() { v.savePref(prefTables) })
}

// sorted is a sorted copy of rows, by the sort in force.
func (tv tableView[T]) sorted(rows []T) []T {
	rows = slices.Clone(rows)
	if i := slices.IndexFunc(tv.cols, func(col tableCol[T]) bool { return col.id == tv.sort }); i >= 0 {
		sortRows(rows, tv.cols[i], tv.desc, tv.tie)
	}
	return rows
}

// sortRows sorts rows by col, ascending unless desc. Rows with too few games for col's number go
// last either way, and ties keep tie's order.
func sortRows[T any](rows []T, col tableCol[T], desc bool, tie func(a, b T) int) {
	if col.by == nil {
		return
	}
	slices.SortStableFunc(rows, func(a, b T) int {
		if col.few != nil {
			if fa, fb := col.few(a), col.few(b); fa != fb {
				if fa {
					return 1
				}
				return -1
			}
		}
		r := col.by(a, b)
		if desc {
			r = -r
		}
		if r == 0 && tie != nil {
			return tie(a, b)
		}
		return r
	})
}
