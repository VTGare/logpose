package gui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

type matchActions interface {
	// LoadTimeline parses a match's log for its life by turn and key moments and shows them,
	// with the parsed game, through view.setTimeline.
	LoadTimeline(fingerprint, me, opp string)
	SetGameType(fingerprint string, t store.GameType)
	SetGameNote(fingerprint, note string)
	SetBookmark(fingerprint string, on bool)
	// OpenReplay builds a match's replay from its parsed game and shows it with view.setReplay.
	OpenReplay(fingerprint string, g *simlog.Game)
	After(d time.Duration, fn func())
}

type matchesPage struct {
	act  matchActions
	list matchesState
	// detail is the open match. Its fingerprint is empty on the match list.
	detail detailState
}

func (pg *matchesPage) do(fn func(matchActions)) {
	if pg.act != nil {
		fn(pg.act)
	}
}

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

const anyResult = 0

var (
	resultLabels  = []string{"Any result", "Wins", "Losses"}
	resultFilters = []stats.Result{stats.Unknown, stats.Win, stats.Loss}
)

// matchCols are the Matches table's columns. mygo's table names a column by its title. Played
// shows only the time under day headers.
func matchCols(e *env, byDay bool) []tableCol[stats.Match] {
	played := float32(120)
	if byDay {
		played = 92
	}
	name := func(code string) string { return e.snap.card(code).Name }
	return []tableCol[stats.Match]{
		{
			title: colPlayed, width: played,
			by: func(a, b stats.Match) int { return a.PlayedAt.Compare(b.PlayedAt) },
			cell: func(c *ui.Context, m stats.Match) {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					at := e.dateTime(m.PlayedAt)
					if byDay {
						at = m.PlayedAt.Format("15:04")
					}
					ui.Text(c, at).TextColor(c.Theme().TextMuted).SingleLine()
					if m.Bookmarked {
						ui.Icon(c, iconStarSet).TextColor(bookmarkColor).Label("Bookmarked")
					}
				})
			},
		},
		{
			title: "Result", width: 64,
			by: func(a, b stats.Match) int { return cmp.Compare(a.Result, b.Result) },
			cell: func(c *ui.Context, m stats.Match) {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(3, 30).Radius(1.5).Background(resultColor(c.Theme(), m.Result))
					resultBadge(c, m.Result)
				}).Label(resultWord(m.Result))
			},
		},
		{
			title: colDeck, width: 60,
			by:   func(a, b stats.Match) int { return strings.Compare(name(a.Me.Leader), name(b.Me.Leader)) },
			cell: func(c *ui.Context, m stats.Match) { e.artStrip(c, m.Me.Leader, 40, 28) },
		},
		{
			title: "Opponent",
			by:    func(a, b stats.Match) int { return strings.Compare(name(a.Opp.Leader), name(b.Opp.Leader)) },
			cell: func(c *ui.Context, m stats.Match) {
				t := c.Theme()
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					e.artStrip(c, m.Opp.Leader, 44, 30)
					ui.Column(c).Gap(1).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
							ui.Text(c, name(m.Opp.Leader)).FontWeight(600).SingleLine()
							ui.Text(c, cards.SetOf(m.Opp.Leader)).FontSize(11).TextColor(faintColor(t))
						})
						ui.Text(c, m.Opp.Name).FontSize(12).TextColor(t.TextMuted).SingleLine()
					})
				})
			},
		},
		{
			title: "Turn", width: 70,
			by:   func(a, b stats.Match) int { return strings.Compare(turnOrderWord(a), turnOrderWord(b)) },
			cell: turnMark,
		},
		{
			title: "Turns", width: 64, end: true,
			by:   func(a, b stats.Match) int { return cmp.Compare(a.Turns, b.Turns) },
			cell: func(c *ui.Context, m stats.Match) { ui.Textf(c, "%d", m.Turns).TextColor(c.Theme().TextMuted) },
		},
		{
			title: "Ended by", width: 100,
			cell: func(c *ui.Context, m stats.Match) { ui.Text(c, endWord(m.End.Kind)).TextColor(c.Theme().TextMuted) },
		},
		{
			title: "Type", width: 130,
			by:   func(a, b stats.Match) int { return strings.Compare(gameType(a), gameType(b)) },
			cell: typeChip,
		},
	}
}

const (
	colPlayed = "Played"
	colDeck   = "Deck"
)

func tableColumns(cols []tableCol[stats.Match]) []ui.TableColumn {
	out := make([]ui.TableColumn, len(cols))
	for i, col := range cols {
		out[i] = ui.TableColumn{Title: col.title, Width: col.width, Sortable: col.sortable()}
		if col.end {
			out[i].Align = ui.End
		}
	}
	return out
}

type matchItem struct {
	m   stats.Match
	day *dayHeader
}

type dayHeader struct {
	date time.Time
	rec  stats.Record
}

func matchItems(rows []stats.Match, byDay bool) []matchItem {
	items := make([]matchItem, 0, len(rows)+len(rows)/4)
	var head *dayHeader
	for _, m := range rows {
		if byDay {
			day := stats.DayStart(m.PlayedAt)
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

func (pg *matchesPage) draw(c *ui.Context, e *env) {
	t := c.Theme()
	s := &pg.list
	if s.sort.Column == "" {
		s.sort = ui.SortOrder{Column: colPlayed, Descending: true}
		s.row = -1
	}
	byDay := s.sort.Column == colPlayed
	cols := matchCols(e, byDay)
	ms := e.scoped().matches
	if s.sorted == nil || s.sort != s.sortKey || !sameSlice(s.from, ms) {
		s.sorted = sortMatches(cols, ms, s.sort)
		s.sortKey, s.from = s.sort, ms
	}
	rows, marked := s.shown(e.listResult)
	items := matchItems(rows, byDay)
	// Deck, your leader, shows only while the games cover more than one deck.
	if e.filter.deck != "" || !slices.ContainsFunc(rows, func(m stats.Match) bool { return m.Me.Leader != rows[0].Me.Leader }) {
		cols = slices.DeleteFunc(cols, func(col tableCol[stats.Match]) bool { return col.title == colDeck })
	}
	s.list.Sort = &s.sort
	s.list.Selected = &s.row
	s.list.Header = func(i int) bool { return items[i].day != nil }
	s.list.Key = func(i int) any {
		if h := items[i].day; h != nil {
			return h.date
		}
		return items[i].m.Fingerprint
	}

	ui.Column(c).Fill().Padding(20).Gap(12).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Matches").FontSize(20).FontWeight(600)
			if marked > 0 || s.bookmarked == 1 {
				ui.Segmented(c, &s.bookmarked, "All", fmt.Sprintf("Bookmarked · %d", marked))
			}
			ui.Segmented(c, &e.listResult, resultLabels...).Label("Result")
			ui.Spacer(c)
			ui.Text(c, summary(rows)).TextColor(t.TextMuted)
		})
		var table ui.Element
		panel(c).Grow(1).Clip().Children(func() {
			table = ui.Table(c, &s.list, tableColumns(cols), len(items), func(row, col int) {
				it := items[row]
				if it.day != nil {
					dayHeaderRow(c, e, it.day)
					return
				}
				// The table's own cell menus are the system's, so each cell's content carries
				// the drawn one, stretched over the cell.
				box := ui.Row(c).Grow(1).AlignSelf(ui.Stretch).AlignItems(ui.Center).PopoverMenu(pg.gameMenu(e, it.m))
				if cols[col].end {
					box.Justify(ui.End)
				}
				box.Children(func() { cols[col].cell(c, it.m) })
			}).Grow(1).Background(ui.Transparent)
		})
		if table.Changed() && s.row >= 0 && s.row < len(items) && items[s.row].day == nil {
			m := items[s.row].m
			// The list keeps no choice, so choosing the same row again opens it again.
			s.row = -1
			pg.openMatch(e, m)
		}
	})
}

// shown is the sorted matches the result and bookmark switches let through, and how many of
// them before the bookmark switch are bookmarked.
func (s *matchesState) shown(result int) (rows []stats.Match, marked int) {
	rows = s.sorted
	if result != anyResult {
		want := resultFilters[result]
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

func summary(ms []stats.Match) string {
	return fmt.Sprintf("%s · %s", games(len(ms)), record(stats.RecordOf(ms)))
}

func dayHeaderRow(c *ui.Context, e *env, h *dayHeader) {
	t := c.Theme()
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, e.dayName(h.date)).FontSize(13).FontWeight(600)
		ui.Text(c, record(h.rec)).FontSize(13).Bold()
		ui.Text(c, fmt.Sprintf("%s · %s", pct(h.rec.Share()), games(h.rec.Games()))).FontSize(12).TextColor(faintColor(t))
	})
}

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

func gameType(m stats.Match) string {
	if m.Ranked {
		if q, ok := stats.Rooms[m.Lobby.Room()]; ok {
			return "Ranked · " + q
		}
		return "Ranked"
	}
	if m.SimFormat != "" {
		return "Casual · " + m.SimFormat
	}
	return "Casual"
}

// sortMatches is a sorted copy of ms, by Played when the order names no column that sorts.
func sortMatches(cols []tableCol[stats.Match], ms []stats.Match, o ui.SortOrder) []stats.Match {
	col := cols[0]
	if i := slices.IndexFunc(cols, func(col tableCol[stats.Match]) bool { return col.title == o.Column && col.sortable() }); i >= 0 {
		col = cols[i]
	}
	out := slices.Clone(ms)
	sortRows(out, col, o.Descending, nil)
	return out
}

// sameSlice reports whether two slices are the same backing array, which means the snapshot
// didn't change since the last sort.
func sameSlice(a, b []stats.Match) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

// gameMenu is a game's menu, the same wherever the game shows.
func (pg *matchesPage) gameMenu(e *env, m stats.Match) func(*ui.Menu) {
	return func(menu *ui.Menu) {
		if menu.Item("Open match").Chosen() {
			pg.openMatch(e, m)
		}
		if menu.Item("Watch replay").Chosen() {
			pg.openReplay(e, m, -1)
		}
		menu.Separator()
		if stats.CanBeRanked(m.Game) {
			menu.Submenu("Game type", func(sub *ui.Menu) {
				for i, label := range typeOptions(m) {
					if sub.Item(label).Radio(int(m.Type) == i).Chosen() {
						pg.do(func(a matchActions) { a.SetGameType(m.Fingerprint, store.GameType(i)) })
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
			pg.openMatch(e, m)
			pg.detail.focusNote = true
		}
		bookmark := "Bookmark"
		if m.Bookmarked {
			bookmark = "Remove bookmark"
		}
		if menu.Item(bookmark).Chosen() {
			pg.do(func(a matchActions) { a.SetBookmark(m.Fingerprint, !m.Bookmarked) })
		}
		menu.Separator()
		if menu.Item("Filter to games vs " + m.Opp.Name).Chosen() {
			e.filter.opp = m.Opp.ID
		}
		matchup := e.snap.card(m.Me.Leader).Name + " vs " + e.snap.card(m.Opp.Leader).Name
		if menu.Item("Filter to " + matchup).Chosen() {
			e.filter.deck, e.filter.oppLeader = m.Me.Leader, m.Opp.Leader
		}
	}
}
