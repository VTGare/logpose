package gui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

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
}

var matchColumns = []ui.TableColumn{
	{Title: "Played", Width: 120, Sortable: true},
	{Title: "Result", Width: 70, Sortable: true},
	{Title: "Your leader", Sortable: true},
	{Title: "Opponent", Sortable: true},
	{Title: "Their leader", Sortable: true},
	{Title: "Turn order", Width: 90, Sortable: true},
	{Title: "Turns", Width: 60, Align: ui.End, Sortable: true},
	{Title: "Ended by", Width: 90},
	{Title: "Type", Width: 150, Sortable: true},
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
	rows := s.sorted
	marked := 0
	for _, m := range rows {
		if m.Bookmarked {
			marked++
		}
	}
	if s.bookmarked == 1 {
		rows = slices.DeleteFunc(slices.Clone(rows), func(m stats.Match) bool { return !m.Bookmarked })
	}
	s.list.Sort = &s.sort
	s.list.Selected = &s.row
	s.list.Key = func(i int) any { return rows[i].Fingerprint }
	s.list.CellMenu = func(m *ui.Menu, row, _ int) {
		if row >= 0 && row < len(rows) {
			v.matchMenu(m, rows[row])
		}
	}

	ui.Column(c).Fill().Padding(20).Gap(12).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Matches").FontSize(20).Bold()
			if marked > 0 || s.bookmarked == 1 {
				ui.Segmented(c, &s.bookmarked, "All", fmt.Sprintf("Bookmarked · %d", marked))
			}
			ui.Spacer(c)
			ui.Text(c, summary(rows)).TextColor(t.TextMuted)
		})
		table := ui.Table(c, &s.list, matchColumns, len(rows), func(row, col int) {
			v.matchCell(c, rows[row], col)
		}).Grow(1)
		if table.Changed() && s.row >= 0 && s.row < len(rows) {
			m := rows[s.row]
			// The list keeps no choice, so choosing the same row again opens it again.
			s.row = -1
			v.openMatch(m)
		}
	})
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
	menu.Item("Watch replay").Disabled(true)
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

func (v *view) matchCell(c *ui.Context, m stats.Match, col int) {
	t := c.Theme()
	switch col {
	case 0:
		ui.Row(c).Gap(6).Children(func() {
			ui.Text(c, m.PlayedAt.Format("Jan 2 15:04")).SingleLine()
			if m.Bookmarked {
				ui.Text(c, "★").TextColor(bookmarkColor)
			}
		})
	case 1:
		ui.Text(c, resultWord(m.Result)).Bold().TextColor(resultColor(m.Result))
	case 2:
		v.leaderName(c, m.Me.Leader)
	case 3:
		ui.Text(c, m.Opp.Name).SingleLine()
	case 4:
		v.leaderName(c, m.Opp.Leader)
	case 5:
		ui.Text(c, turnOrderWord(m)).TextColor(t.TextMuted)
	case 6:
		ui.Textf(c, "%d", m.Turns)
	case 7:
		ui.Text(c, endWord(m.End.Kind)).TextColor(t.TextMuted)
	case 8:
		ui.Text(c, gameType(m)).TextColor(t.TextMuted)
	}
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
		case "Your leader":
			return strings.Compare(s.card(a.Me.Leader).Name, s.card(b.Me.Leader).Name)
		case "Opponent":
			return strings.Compare(strings.ToLower(a.Opp.Name), strings.ToLower(b.Opp.Name))
		case "Their leader":
			return strings.Compare(s.card(a.Opp.Leader).Name, s.card(b.Opp.Leader).Name)
		case "Turn order":
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
