package kit

import (
	"cmp"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

// TableLayout is how the user arranged a hand-built table: its sort, the order of its columns
// after the first, and the widths they were dragged to, by column ID.
type TableLayout struct {
	Sort   string             `json:"sort,omitempty"`
	Desc   bool               `json:"desc,omitempty"`
	Order  []string           `json:"order,omitempty"`
	Widths map[string]float32 `json:"widths,omitempty"`
}

// TableCol is a column of a table of T rows. In a hand-built table the first column holds the
// row's name, stays first and takes the room the others leave.
type TableCol[T any] struct {
	ID, Title, Tip string
	Width          float32
	End            bool
	// Asc sorts ascending on the first click, for text. Numbers start with the most.
	Asc bool
	// By compares two rows ascending. A column without it doesn't sort.
	By func(a, b T) int
	// Few reports a row with too Few games for the column's number. Those rows sort last
	// either way.
	Few  func(T) bool
	Cell func(c *ui.Context, r T)
}

func (col TableCol[T]) Sortable() bool { return col.By != nil }

// Column limits, in DIPs.
const (
	colMinW    = 40
	colMaxW    = 400
	nameMinW   = 160
	colGripW   = 8
	sortIconSz = 10
)

// TableView is a hand-built table's columns as the user arranged them.
type TableView[T any] struct {
	lays tableLayouts
	ID   string
	Gap  float32
	cols []TableCol[T]
	lay  *TableLayout
	// sort and desc are the sort in force: the user's, or the table's default.
	sort string
	desc bool
	// Tie orders rows the sort finds equal. Without it they keep their order.
	Tie func(a, b T) int
	// Trail is room after the columns for a row's own control, such as a chevron.
	Trail float32
}

// tableLayouts keeps each table's sort, column order and widths by table ID.
type tableLayouts interface {
	// Layout is nil for a table the user never changed.
	Layout(id string) *TableLayout
	SaveLayout(id string, lay *TableLayout)
}

// ArrangeTable puts cols in the user's order with their widths, and picks the sort. Columns the
// layout doesn't know keep their place and default width, and IDs it has that cols don't are
// ignored.
func ArrangeTable[T any](lays tableLayouts, id string, gap float32, cols []TableCol[T], sort string, desc bool) TableView[T] {
	lay := lays.Layout(id)
	if lay == nil {
		lay = &TableLayout{}
	}
	tv := TableView[T]{lays: lays, ID: id, Gap: gap, lay: lay, sort: sort, desc: desc}
	if lay.Sort != "" && slices.ContainsFunc(cols, func(c TableCol[T]) bool { return c.ID == lay.Sort && c.Sortable() }) {
		tv.sort, tv.desc = lay.Sort, lay.Desc
	}
	rest := slices.Clone(cols[1:])
	pos := func(c TableCol[T]) int {
		if i := slices.Index(lay.Order, c.ID); i >= 0 {
			return i
		}
		return len(lay.Order)
	}
	slices.SortStableFunc(rest, func(a, b TableCol[T]) int { return cmp.Compare(pos(a), pos(b)) })
	tv.cols = append([]TableCol[T]{cols[0]}, rest...)
	for i := range tv.cols {
		if w, ok := lay.Widths[tv.cols[i].ID]; ok && i > 0 {
			tv.cols[i].Width = min(max(w, colMinW), colMaxW)
		}
	}
	return tv
}

func (tv TableView[T]) Cells(c *ui.Context, r T) {
	for _, col := range tv.cols {
		el := colBox(c, col.ID, col.Width).Clip()
		if col.End {
			el.Justify(ui.End)
		}
		el.Children(func() { col.Cell(c, r) })
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

// Header is the Header row. Clicking a title sorts by it and a second click turns the sort
// around, dragging a title moves the column, and dragging the edge after it resizes it. A double
// click on the edge undoes that width, and Reset columns in the menu undoes it all.
func (tv TableView[T]) Header(c *ui.Context, pad float32) {
	t := c.Theme()
	ui.Row(c).Gap(tv.Gap).Padding(8, pad).AlignItems(ui.Center).PopoverMenu(func(m *ui.Menu) {
		if m.Item("Reset columns").Disabled(len(tv.lay.Order) == 0 && len(tv.lay.Widths) == 0 && tv.lay.Sort == "").Chosen() {
			*tv.lay = TableLayout{}
			tv.save()
		}
	}).Children(func() {
		for i, col := range tv.cols {
			colBox(c, col.ID, col.Width).Children(func() {
				tv.headerTitle(c, i, col)
				if i > 0 {
					tv.colGrip(c, col)
				}
			})
		}
		if tv.Trail > 0 {
			ui.Box(c).Width(tv.Trail)
		}
	})
	ui.Box(c).Height(1).Background(t.Border.Alpha(0.6))
}

func (tv TableView[T]) headerTitle(c *ui.Context, i int, col TableCol[T]) {
	t := c.Theme()
	label := "Sort by " + col.Title
	if !col.Sortable() {
		label = col.Title
	}
	b := ui.ButtonBase(c).Label(label).Row().Gap(4).AlignItems(ui.Center).
		Grow(1).Basis(0).MinWidth(0).Padding(2, 0).Background(ui.Transparent)
	if col.End {
		b.Justify(ui.End)
	} else {
		b.Justify(ui.Start)
	}
	if i > 0 {
		b.Drag(colDrag{tv.ID, col.ID})
		if d, ok := ui.Drop[colDrag](b); ok && d.table == tv.ID {
			tv.moveColumn(d.col, col.ID)
		}
		if d, ok := ui.DragOver[colDrag](b); ok && d.table == tv.ID && d.col != col.ID {
			b.BorderStartWidth(2).BorderColor(t.Accent)
		}
	}
	b.Tooltip(headerTip(col.Tip, col.Sortable(), i > 0))
	if b.Clicked() && col.Sortable() {
		desc := !col.Asc
		if tv.sort == col.ID {
			desc = !tv.desc
		}
		tv.lay.Sort, tv.lay.Desc = col.ID, desc
		tv.save()
	}
	SortTitle(c, b, col.Title, col.End, col.ID == tv.sort, tv.desc, col.Sortable())
}

// SortTitle draws a header title into b, with the accent chevron when its column is sorted and
// a faint one on hover when it can be.
func SortTitle(c *ui.Context, b ui.Element, title string, end, sorted, desc, sortable bool) {
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
			icon := IconChevron
			if !desc {
				icon = IconChevronUp
			}
			ui.Icon(c, icon).FontSize(sortIconSz).TextColor(t.Accent)
		case b.Hovered() && sortable:
			ui.Icon(c, IconChevron).FontSize(sortIconSz).TextColor(FaintColor(t))
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
func (tv TableView[T]) colGrip(c *ui.Context, col TableCol[T]) {
	t := c.Theme()
	g := ui.Box(c).Width(colGripW).Height(18).Margin(0, -colGripW/2, 0, 0).Cursor(ui.CursorResizeEW).Label("Resize " + col.Title)
	line := t.Border.Alpha(0.6)
	if g.Hovered() || g.Pressed() {
		line = t.Accent
	}
	g.Children(func() { ui.Box(c).Width(2).Height(18).Margin(0, 0, 0, colGripW/2-1).Background(line) })
	if dx, _, ok := g.Dragged(); ok && dx != 0 {
		if tv.lay.Widths == nil {
			tv.lay.Widths = make(map[string]float32)
		}
		tv.lay.Widths[col.ID] = min(max(col.Width+dx, colMinW), colMaxW)
		tv.save()
	}
	if g.DoubleClicked() {
		delete(tv.lay.Widths, col.ID)
		tv.save()
	}
}

// moveColumn puts the column from where the column to is, shifting the ones between.
func (tv TableView[T]) moveColumn(from, to string) {
	ids := make([]string, 0, len(tv.cols)-1)
	for _, c := range tv.cols[1:] {
		ids = append(ids, c.ID)
	}
	i, j := slices.Index(ids, from), slices.Index(ids, to)
	if i < 0 || j < 0 || i == j {
		return
	}
	ids = slices.Insert(slices.Delete(ids, i, i+1), j, from)
	tv.lay.Order = ids
	tv.save()
}

func (tv TableView[T]) save() {
	tv.lays.SaveLayout(tv.ID, tv.lay)
}

// Sorted is a Sorted copy of rows, by the sort in force.
func (tv TableView[T]) Sorted(rows []T) []T {
	rows = slices.Clone(rows)
	if i := slices.IndexFunc(tv.cols, func(col TableCol[T]) bool { return col.ID == tv.sort }); i >= 0 {
		SortRows(rows, tv.cols[i], tv.desc, tv.Tie)
	}
	return rows
}

// SortRows sorts rows by col, ascending unless desc. Rows with too few games for col's number go
// last either way, and ties keep tie's order.
func SortRows[T any](rows []T, col TableCol[T], desc bool, tie func(a, b T) int) {
	if col.By == nil {
		return
	}
	slices.SortStableFunc(rows, func(a, b T) int {
		if col.Few != nil {
			if fa, fb := col.Few(a), col.Few(b); fa != fb {
				if fa {
					return 1
				}
				return -1
			}
		}
		r := col.By(a, b)
		if desc {
			r = -r
		}
		if r == 0 && tie != nil {
			return tie(a, b)
		}
		return r
	})
}
