package matches

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

type Actions interface {
	// LoadTimeline parses a match's log for its life by turn and key moments and shows them,
	// with the parsed game, through view.setTimeline.
	LoadTimeline(fingerprint, me, opp string)
	SetGameType(fingerprint string, t store.GameType)
	SetGameNote(fingerprint, note string)
	SetBookmark(fingerprint string, on bool)
	// OpenReplay builds a match's replay from its parsed game and shows it with view.setReplay.
	OpenReplay(fingerprint string, g *simlog.Game)
	// ExportToSim writes the match as a save state for the sim's LoadState From File, named
	// name, into the export folder. g is the parsed game, or nil to read it from the store.
	ExportToSim(fingerprint, name string, g *simlog.Game)
	After(d time.Duration, fn func())
}

type Page struct {
	Act  Actions
	list matchesState
	// Detail is the open match. Its fingerprint is empty on the match list.
	Detail detailState
}

func (pg *Page) do(fn func(Actions)) {
	if pg.Act != nil {
		fn(pg.Act)
	}
}

type matchesState struct {
	list    ui.ListState
	sort    ui.SortOrder
	sorted  []stats.Match
	sortKey ui.SortOrder
	From    []stats.Match
	row     int
	// bookmarked is 1 while the list shows only bookmarked matches.
	bookmarked int
}

var (
	resultLabels  = []string{"Any result", "Wins", "Losses"}
	resultFilters = []stats.Result{stats.Unknown, stats.Win, stats.Loss}
)

// matchCols are the Matches table's columns. mygo's table names a column by its title. Played
// shows only the time under day headers.
func matchCols(e *page.Env, byDay bool) []kit.TableCol[stats.Match] {
	played := float32(120)
	if byDay {
		played = 92
	}
	name := func(code string) string { return e.Snap.Card(code).Name }
	return []kit.TableCol[stats.Match]{
		{
			Title: colPlayed, Width: played,
			By: func(a, b stats.Match) int { return a.PlayedAt.Compare(b.PlayedAt) },
			Cell: func(c *ui.Context, m stats.Match) {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					at := e.DateTime(m.PlayedAt)
					if byDay {
						at = m.PlayedAt.Format("15:04")
					}
					ui.Text(c, at).TextColor(c.Theme().TextMuted).SingleLine()
					if m.Bookmarked {
						ui.Icon(c, kit.IconStarSet).TextColor(kit.BookmarkColor).Label("Bookmarked")
					}
				})
			},
		},
		{
			Title: "Result", Width: 64,
			By: func(a, b stats.Match) int { return cmp.Compare(a.Result, b.Result) },
			Cell: func(c *ui.Context, m stats.Match) {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(3, 30).Radius(1.5).Background(kit.ResultColor(c.Theme(), m.Result))
					page.ResultBadge(c, m.Result)
				}).Label(page.ResultWord(m.Result))
			},
		},
		{
			Title: colDeck, Width: 60,
			By:   func(a, b stats.Match) int { return strings.Compare(name(a.Me.Leader), name(b.Me.Leader)) },
			Cell: func(c *ui.Context, m stats.Match) { e.ArtStrip(c, m.Me.Leader, 40, 28) },
		},
		{
			Title: "Opponent",
			By:    func(a, b stats.Match) int { return strings.Compare(name(a.Opp.Leader), name(b.Opp.Leader)) },
			Cell: func(c *ui.Context, m stats.Match) {
				t := c.Theme()
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					e.ArtStrip(c, m.Opp.Leader, 44, 30)
					ui.Column(c).Gap(1).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
							ui.Text(c, name(m.Opp.Leader)).FontWeight(600).SingleLine()
							ui.Text(c, cards.SetOf(m.Opp.Leader)).FontSize(11).TextColor(kit.FaintColor(t))
						})
						ui.Text(c, m.Opp.Name).FontSize(12).TextColor(t.TextMuted).SingleLine()
					})
				})
			},
		},
		{
			Title: "Turn", Width: 70,
			By:   func(a, b stats.Match) int { return strings.Compare(turnOrderWord(a), turnOrderWord(b)) },
			Cell: page.TurnMark,
		},
		{
			Title: "Turns", Width: 64, End: true,
			By:   func(a, b stats.Match) int { return cmp.Compare(a.Turns, b.Turns) },
			Cell: func(c *ui.Context, m stats.Match) { ui.Textf(c, "%d", m.Turns).TextColor(c.Theme().TextMuted) },
		},
		{
			Title: "Ended by", Width: 100,
			Cell: func(c *ui.Context, m stats.Match) { ui.Text(c, endWord(m.End.Kind)).TextColor(c.Theme().TextMuted) },
		},
		{
			Title: "Type", Width: 130,
			By:   func(a, b stats.Match) int { return strings.Compare(GameType(a), GameType(b)) },
			Cell: typeChip,
		},
	}
}

const (
	colPlayed = "Played"
	colDeck   = "Deck"
)

func tableColumns(cols []kit.TableCol[stats.Match]) []ui.TableColumn {
	out := make([]ui.TableColumn, len(cols))
	for i, col := range cols {
		out[i] = ui.TableColumn{Title: col.Title, Width: col.Width, Sortable: col.Sortable()}
		if col.End {
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

func (pg *Page) Draw(c *ui.Context, e *page.Env) {
	t := c.Theme()
	s := &pg.list
	if s.sort.Column == "" {
		s.sort = ui.SortOrder{Column: colPlayed, Descending: true}
		s.row = -1
	}
	byDay := s.sort.Column == colPlayed
	cols := matchCols(e, byDay)
	ms := e.Scoped().Matches
	if s.sorted == nil || s.sort != s.sortKey || !sameSlice(s.From, ms) {
		s.sorted = sortMatches(cols, ms, s.sort)
		s.sortKey, s.From = s.sort, ms
	}
	rows, marked := s.shown(e.ListResult)
	items := matchItems(rows, byDay)
	// Deck, your leader, shows only while the games cover more than one deck.
	if e.Filter.Deck != "" || !slices.ContainsFunc(rows, func(m stats.Match) bool { return m.Me.Leader != rows[0].Me.Leader }) {
		cols = slices.DeleteFunc(cols, func(col kit.TableCol[stats.Match]) bool { return col.Title == colDeck })
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
			ui.Segmented(c, &e.ListResult, resultLabels...).Label("Result")
			ui.Spacer(c)
			ui.Text(c, summary(rows)).TextColor(t.TextMuted)
		})
		var table ui.Element
		kit.Panel(c).Grow(1).Clip().Children(func() {
			table = ui.Table(c, &s.list, tableColumns(cols), len(items), func(row, col int) {
				it := items[row]
				if it.day != nil {
					dayHeaderRow(c, e, it.day)
					return
				}
				// The table's own cell menus are the system's, so each cell's content carries
				// the drawn one, stretched over the cell.
				box := ui.Row(c).Grow(1).AlignSelf(ui.Stretch).AlignItems(ui.Center).PopoverMenu(pg.GameMenu(e, it.m))
				if cols[col].End {
					box.Justify(ui.End)
				}
				box.Children(func() { cols[col].Cell(c, it.m) })
			}).Grow(1).Background(ui.Transparent)
		})
		if table.Changed() && s.row >= 0 && s.row < len(items) && items[s.row].day == nil {
			m := items[s.row].m
			// The list keeps no choice, so choosing the same row again opens it again.
			s.row = -1
			pg.OpenMatch(e, m)
		}
	})
}

// shown is the sorted matches the result and bookmark switches let through, and how many of
// them before the bookmark switch are bookmarked.
func (s *matchesState) shown(result int) (rows []stats.Match, marked int) {
	rows = s.sorted
	if result != page.AnyResult {
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
	return fmt.Sprintf("%s · %s", kit.Games(len(ms)), kit.Record(stats.RecordOf(ms)))
}

func dayHeaderRow(c *ui.Context, e *page.Env, h *dayHeader) {
	t := c.Theme()
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, e.DayName(h.date)).FontSize(13).FontWeight(600)
		ui.Text(c, kit.Record(h.rec)).FontSize(13).Bold()
		ui.Text(c, fmt.Sprintf("%s · %s", kit.Pct(h.rec.Share()), kit.Games(h.rec.Games()))).FontSize(12).TextColor(kit.FaintColor(t))
	})
}

func typeChip(c *ui.Context, m stats.Match) {
	t := c.Theme()
	if !m.Ranked {
		ui.Text(c, "Casual").TextColor(kit.FaintColor(t)).Tooltip(GameType(m))
		return
	}
	ui.Text(c, "Ranked").FontSize(11).FontWeight(600).TextColor(t.TextMuted).Padding(1, 8).Radius(9).
		Border(1, t.Border).Tooltip(GameType(m))
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

func GameType(m stats.Match) string {
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
func sortMatches(cols []kit.TableCol[stats.Match], ms []stats.Match, o ui.SortOrder) []stats.Match {
	col := cols[0]
	if i := slices.IndexFunc(cols, func(col kit.TableCol[stats.Match]) bool { return col.Title == o.Column && col.Sortable() }); i >= 0 {
		col = cols[i]
	}
	out := slices.Clone(ms)
	kit.SortRows(out, col, o.Descending, nil)
	return out
}

// sameSlice reports whether two slices are the same backing array, which means the snapshot
// didn't change since the last sort.
func sameSlice(a, b []stats.Match) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

// GameMenu is a game's menu, the same wherever the game shows.
func (pg *Page) GameMenu(e *page.Env, m stats.Match) func(*ui.Menu) {
	return func(menu *ui.Menu) {
		if menu.Item("Open match").Chosen() {
			pg.OpenMatch(e, m)
		}
		if menu.Item("Watch replay").Chosen() {
			pg.openReplay(e, m, -1)
		}
		if menu.Item("Export to sim").Chosen() {
			pg.exportToSim(e, m)
		}
		menu.Separator()
		if stats.CanBeRanked(m.Game) {
			menu.Submenu("Game type", func(sub *ui.Menu) {
				for i, label := range typeOptions(m) {
					if sub.Item(label).Radio(int(m.Type) == i).Chosen() {
						pg.do(func(a Actions) { a.SetGameType(m.Fingerprint, store.GameType(i)) })
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
			pg.OpenMatch(e, m)
			pg.Detail.focusNote = true
		}
		bookmark := "Bookmark"
		if m.Bookmarked {
			bookmark = "Remove bookmark"
		}
		if menu.Item(bookmark).Chosen() {
			pg.do(func(a Actions) { a.SetBookmark(m.Fingerprint, !m.Bookmarked) })
		}
		menu.Separator()
		if menu.Item("Filter to games vs " + m.Opp.Name).Chosen() {
			e.Filter.Opp = m.Opp.ID
		}
		matchup := e.Snap.Card(m.Me.Leader).Name + " vs " + e.Snap.Card(m.Opp.Leader).Name
		if menu.Item("Filter to " + matchup).Chosen() {
			e.Filter.Deck, e.Filter.OppLeader = m.Me.Leader, m.Opp.Leader
		}
	}
}
