package gui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

type matchesState struct {
	list    ui.ListState
	sort    ui.SortOrder
	sorted  []stats.Match
	sortKey ui.SortOrder
	from    []stats.Match
	row     int
	// bookmarked is 1 while the list shows only bookmarked matches.
	bookmarked int
	// result indexes resultLabels. It narrows the list only, since on the other pages it would
	// make every win rate 100% or 0%.
	result int
}

const anyResult = 0

var (
	resultLabels  = []string{"Any result", "Wins", "Losses"}
	resultFilters = []stats.Result{stats.Unknown, stats.Win, stats.Loss}
)

// matchColumns are the Matches table's columns. Played shows only the time under day headers.
// Deck, your leader, shows only while the games cover more than one deck.
func matchColumns(byDay, decks bool) []ui.TableColumn {
	played := float32(120)
	if byDay {
		played = 92
	}
	cols := []ui.TableColumn{
		{Title: "Played", Width: played, Sortable: true},
		{Title: "Result", Width: 64, Sortable: true},
	}
	if decks {
		cols = append(cols, ui.TableColumn{Title: "Deck", Width: 60, Sortable: true})
	}
	return append(cols,
		ui.TableColumn{Title: "Opponent", Sortable: true},
		ui.TableColumn{Title: "Turn", Width: 70, Sortable: true},
		ui.TableColumn{Title: "Turns", Width: 64, Align: ui.End, Sortable: true},
		ui.TableColumn{Title: "Ended by", Width: 100},
		ui.TableColumn{Title: "Type", Width: 130, Sortable: true},
	)
}

// matchItem is a row of the Matches table: a game, or the header of the day under it.
type matchItem struct {
	m   stats.Match
	day *dayHeader
}

type dayHeader struct {
	date time.Time
	rec  stats.Record
}

// matchItems puts a header over each day's games while the table is in date order.
func matchItems(rows []stats.Match, byDay bool) []matchItem {
	items := make([]matchItem, 0, len(rows)+len(rows)/4)
	var head *dayHeader
	for _, m := range rows {
		if byDay {
			y, mo, d := m.PlayedAt.Date()
			day := time.Date(y, mo, d, 0, 0, 0, 0, m.PlayedAt.Location())
			if head == nil || !head.date.Equal(day) {
				head = &dayHeader{date: day}
				items = append(items, matchItem{day: head})
			}
			head.rec.Add(m.Result)
		}
		items = append(items, matchItem{m: m})
	}
	return items
}

func (v *view) matchesPage(c *ui.Context) {
	t := c.Theme()
	s := &v.matches
	if s.sort.Column == "" {
		s.sort = ui.SortOrder{Column: "Played", Descending: true}
		s.row = -1
	}
	ms := v.scoped().matches
	if s.sorted == nil || s.sort != s.sortKey || !sameSlice(s.from, ms) {
		s.sorted = sortMatches(v.snap, ms, s.sort)
		s.sortKey, s.from = s.sort, ms
	}
	rows, marked := s.shown()
	byDay := s.sort.Column == "Played"
	items := matchItems(rows, byDay)
	decks := v.filter.deck == "" && slices.ContainsFunc(rows, func(m stats.Match) bool { return m.Me.Leader != rows[0].Me.Leader })
	cols := matchColumns(byDay, decks)
	s.list.Sort = &s.sort
	s.list.Selected = &s.row
	s.list.Header = func(i int) bool { return items[i].day != nil }
	s.list.Key = func(i int) any {
		if h := items[i].day; h != nil {
			return h.date
		}
		return items[i].m.Fingerprint
	}
	s.list.CellMenu = func(m *ui.Menu, row, _ int) {
		if row >= 0 && row < len(items) && items[row].day == nil {
			v.matchMenu(m, items[row].m)
		}
	}

	ui.Column(c).Fill().Padding(20).Gap(12).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Matches").FontSize(20).FontWeight(600)
			if marked > 0 || s.bookmarked == 1 {
				ui.Segmented(c, &s.bookmarked, "All", fmt.Sprintf("Bookmarked · %d", marked))
			}
			ui.Segmented(c, &s.result, resultLabels...).Label("Result")
			ui.Spacer(c)
			ui.Text(c, summary(rows)).TextColor(t.TextMuted)
		})
		var table ui.Element
		panel(c).Grow(1).Clip().Children(func() {
			table = ui.Table(c, &s.list, cols, len(items), func(row, col int) {
				it := items[row]
				if it.day != nil {
					v.dayHeaderRow(c, it.day)
					return
				}
				v.matchCell(c, it.m, cols[col].Title)
			}).Grow(1).Background(ui.Transparent)
		})
		if table.Changed() && s.row >= 0 && s.row < len(items) && items[s.row].day == nil {
			m := items[s.row].m
			// The list keeps no choice, so choosing the same row again opens it again.
			s.row = -1
			v.openMatch(m)
		}
	})
}

// shown is the sorted matches the result and bookmark switches let through, and how many of
// them before the bookmark switch are bookmarked.
func (s *matchesState) shown() (rows []stats.Match, marked int) {
	rows = s.sorted
	if s.result != anyResult {
		want := resultFilters[s.result]
		rows = slices.DeleteFunc(slices.Clone(rows), func(m stats.Match) bool { return m.Result != want })
	}
	for _, m := range rows {
		if m.Bookmarked {
			marked++
		}
	}
	if s.bookmarked == 1 {
		rows = slices.DeleteFunc(slices.Clone(rows), func(m stats.Match) bool { return !m.Bookmarked })
	}
	return rows, marked
}

// matchMenu is a match row's right-click menu.
func (v *view) matchMenu(menu *ui.Menu, m stats.Match) {
	if m.Lobby.InRoom() {
		menu.Submenu("Game type", func(sub *ui.Menu) {
			for i, label := range v.typeOptions(m) {
				if sub.Item(label).Radio(int(m.Type) == i).Chosen() {
					v.act.SetGameType(m.Fingerprint, store.GameType(i))
				}
			}
		})
	} else {
		menu.Item("Game type: private lobby").Disabled(true)
	}
	noteLabel := "Add note…"
	if m.Note != "" {
		noteLabel = "Edit note…"
	}
	if menu.Item(noteLabel).Chosen() {
		v.openMatch(m)
		v.detail.focusNote = true
	}
	bookmark := "Bookmark"
	if m.Bookmarked {
		bookmark = "Remove bookmark"
	}
	if menu.Item(bookmark).Chosen() {
		v.act.SetBookmark(m.Fingerprint, !m.Bookmarked)
	}
	if menu.Item("Watch replay").Chosen() {
		v.openReplay(m, -1)
	}
	menu.Separator()
	if menu.Item("Filter to games vs " + m.Opp.Name).Chosen() {
		v.filter.opp = m.Opp.ID
		v.resetPoints()
	}
	matchup := v.snap.card(m.Me.Leader).Name + " vs " + v.snap.card(m.Opp.Leader).Name
	if menu.Item("Filter to " + matchup).Chosen() {
		v.filter.deck, v.filter.oppLeader = m.Me.Leader, m.Opp.Leader
		v.resetPoints()
	}
}

func summary(ms []stats.Match) string {
	var r stats.Record
	for _, m := range ms {
		r.Add(m.Result)
	}
	return fmt.Sprintf("%s · %s", games(len(ms)), record(r))
}

// dayHeaderRow names a day with its record across the games under it.
func (v *view) dayHeaderRow(c *ui.Context, h *dayHeader) {
	t := c.Theme()
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, v.dayName(h.date)).FontSize(13).FontWeight(600)
		ui.Text(c, record(h.rec)).FontSize(13).Bold()
		ui.Text(c, fmt.Sprintf("%s · %s", pct(winShare(h.rec)), games(h.rec.Games()))).FontSize(12).TextColor(faintColor(t))
	})
}

func (v *view) matchCell(c *ui.Context, m stats.Match, col string) {
	t := c.Theme()
	switch col {
	case "Played":
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			at := m.PlayedAt.Format("Jan 2 15:04")
			if v.matches.sort.Column == "Played" {
				at = m.PlayedAt.Format("15:04")
			}
			ui.Text(c, at).TextColor(t.TextMuted).SingleLine()
			if m.Bookmarked {
				ui.Icon(c, iconStarSet).TextColor(bookmarkColor).Label("Bookmarked")
			}
		})
	case "Result":
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(3, 30).Radius(1.5).Background(resultColor(m.Result))
			resultBadge(c, m.Result)
		}).Label(resultWord(m.Result))
	case "Deck":
		v.artStrip(c, m.Me.Leader, 40, 28)
	case "Opponent":
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			v.artStrip(c, m.Opp.Leader, 44, 30)
			ui.Column(c).Gap(1).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
					ui.Text(c, v.snap.card(m.Opp.Leader).Name).FontWeight(600).SingleLine()
					ui.Text(c, setOf(m.Opp.Leader)).FontSize(11).TextColor(faintColor(t))
				})
				ui.Text(c, m.Opp.Name).FontSize(12).TextColor(t.TextMuted).SingleLine()
			})
		})
	case "Turn":
		turnMark(c, m)
	case "Turns":
		ui.Textf(c, "%d", m.Turns).TextColor(t.TextMuted)
	case "Ended by":
		ui.Text(c, endWord(m.End.Kind)).TextColor(t.TextMuted)
	case "Type":
		typeChip(c, m)
	}
}

// typeChip is ranked as a small outlined chip, with the room on hover, and casual as plain text.
func typeChip(c *ui.Context, m stats.Match) {
	t := c.Theme()
	if !m.Ranked {
		ui.Text(c, "Casual").TextColor(faintColor(t)).Tooltip(gameType(m))
		return
	}
	ui.Text(c, "Ranked").FontSize(11).FontWeight(600).TextColor(t.TextMuted).Padding(1, 8).Radius(9).
		Border(1, t.Border).Tooltip(gameType(m))
}

func resultWord(r stats.Result) string {
	switch r {
	case stats.Win:
		return "Win"
	case stats.Loss:
		return "Loss"
	default:
		return "Unknown"
	}
}

func turnOrderWord(m stats.Match) string {
	switch {
	case !m.TurnOrderKnown:
		return "Unknown"
	case m.WentFirst:
		return "First"
	default:
		return "Second"
	}
}

func endWord(k simlog.EndKind) string {
	switch k {
	case simlog.EndConcede:
		return "Concession"
	case simlog.EndDeckOut:
		return "Deck out"
	case simlog.EndLethal:
		return "Lethal"
	default:
		return "Unknown"
	}
}

// gameType is ranked with the sim room its lobby was in, or casual with the sim's format when
// logpose saw which it was.
func gameType(m stats.Match) string {
	if m.Ranked {
		if q, ok := stats.Rooms[m.Lobby.Code[0]]; ok {
			return "Ranked · " + q
		}
		return "Ranked"
	}
	if m.SimFormat != "" {
		return "Casual · " + m.SimFormat
	}
	return "Casual"
}

func sortMatches(s *snapshot, ms []stats.Match, o ui.SortOrder) []stats.Match {
	out := slices.Clone(ms)
	by := func(a, b stats.Match) int {
		switch o.Column {
		case "Result":
			return cmp.Compare(a.Result, b.Result)
		case "Deck":
			return strings.Compare(s.card(a.Me.Leader).Name, s.card(b.Me.Leader).Name)
		case "Opponent":
			return strings.Compare(s.card(a.Opp.Leader).Name, s.card(b.Opp.Leader).Name)
		case "Turn":
			return strings.Compare(turnOrderWord(a), turnOrderWord(b))
		case "Turns":
			return cmp.Compare(a.Turns, b.Turns)
		case "Type":
			return strings.Compare(gameType(a), gameType(b))
		default:
			return a.PlayedAt.Compare(b.PlayedAt)
		}
	}
	slices.SortStableFunc(out, func(a, b stats.Match) int {
		if o.Descending {
			return by(b, a)
		}
		return by(a, b)
	})
	return out
}

// sameSlice reports whether two slices are the same backing array, which means the snapshot
// didn't change since the last sort.
func sameSlice(a, b []stats.Match) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}
