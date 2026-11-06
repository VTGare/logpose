package gui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/live"
	"github.com/VTGare/logpose/internal/stats"
)

// snapshot is what the window shows. The engine builds a new one off the main thread, and the
// window swaps it in.
type snapshot struct {
	Loaded   bool
	SimFound bool
	Me       string
	Matches  []stats.Match
	Overview stats.Overview
	Cards    *cards.DB
	Now      time.Time
}

func (s *snapshot) card(code string) cards.Card {
	if s.Cards == nil {
		return cards.Card{Code: code, Name: code}
	}
	return s.Cards.Card(code)
}

const (
	pageOverview = "overview"
	pageMatches  = "matches"
	pageLeaders  = "leaders"
)

type view struct {
	snap *snapshot
	// live is the game in the sim right now, which the log follower updates on its own.
	live live.State
	art  *artCache
	page string

	matches matchesState
	// rollingPoint is the rolling chart's point under the pointer, or -1.
	rollingPoint int

	// leader is the leader the Leaders page shows, or "" for the most played one.
	leader      string
	leaderPoint int
	leaderStats leaderCache
	// leaderType is the Leaders page's game type, a stats.GameType.
	leaderType   int
	matchupOrder int
	hideFew      bool
}

// openLeader shows a leader on the Leaders page.
func (v *view) openLeader(code string) {
	v.page = pageLeaders
	v.leader = code
	v.leaderPoint = -1
}

func newView(art *artCache) *view {
	return &view{snap: &snapshot{}, art: art, page: pageOverview, rollingPoint: -1, leaderPoint: -1}
}

func (v *view) build(c *ui.Context) {
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		v.sidebar(c)
		ui.Column(c).Grow(1).Fill().Children(func() {
			if v.snap.Loaded && v.live.Phase != live.Idle {
				v.liveStrip(c)
			}
			switch {
			case !v.snap.Loaded:
				v.message(c, "Loading your games", "")
			case len(v.snap.Matches) == 0 && !v.snap.SimFound:
				v.message(c, "OPTCGSim isn't running",
					"Start the sim and logpose finds it and imports your games. If it's installed somewhere unusual, start logpose with -sim and the sim's folder.")
			case len(v.snap.Matches) == 0:
				v.message(c, "Play a game in the sim", "Every game you finish shows up here on its own.")
			case v.page == pageMatches:
				v.matchesPage(c)
			case v.page == pageLeaders:
				v.leadersPage(c)
			default:
				v.overviewPage(c)
			}
		})
	})
}

func (v *view) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(200).Background(t.Surface).Padding(12, 8).Gap(8).Children(func() {
		ui.Row(c).Gap(8).Padding(4, 8, 8, 8).Children(func() {
			ui.Icon(c, iconCompass).Size(20, 20).TextColor(t.Accent)
			ui.Text(c, "logpose").Bold().FontSize(16)
		})
		ui.Sidebar(c, &v.page, func() {
			ui.SidebarItem(c, pageOverview, iconOverview, "Overview")
			ui.SidebarItem(c, pageLeaders, iconLeaders, "Leaders")
			ui.SidebarItem(c, pageMatches, iconMatches, "Matches")
		}).Grow(1)
	})
}

func (*view) message(c *ui.Context, title, body string) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(8).Padding(32).Children(func() {
		ui.Text(c, title).Bold().FontSize(18)
		if body != "" {
			ui.Text(c, body).TextColor(t.TextMuted).MaxWidth(420).Wrap()
		}
	})
}
