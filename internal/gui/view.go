package gui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/live"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
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
	// Formats are the user's formats, oldest first, and Debuts the sets that look like they
	// started one.
	Formats []stats.FormatRange
	Debuts  []stats.SetDebut
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
	pageFormats  = "formats"
)

// actions are the changes the view asks the app for. The app makes them off the main thread
// and sends a new snapshot.
type actions interface {
	AddFormat(name string, start time.Time)
	UpdateFormat(f store.Format)
	DeleteFormat(id int64)
	DismissSet(set string)
}

type view struct {
	snap *snapshot
	act  actions
	// formats is the Formats page's state.
	formats formatsState
	// live is the game in the sim right now, which the log follower updates on its own.
	live live.State
	art  *artCache
	page string

	matches matchesState
	// rollingPoint is the rolling chart's point under the pointer, or -1.
	rollingPoint int

	filter filters
	scope  scoped

	leaderPoint  int
	leaderStats  leaderCache
	matchupOrder int
	hideFew      bool
}

// openLeader shows a leader on the Leaders page.
func (v *view) openLeader(code string) {
	v.page = pageLeaders
	v.setDeck(code)
}

func newView(art *artCache) *view {
	return &view{snap: &snapshot{}, art: art, page: pageOverview, rollingPoint: -1, leaderPoint: -1}
}

func (v *view) build(c *ui.Context) {
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		v.sidebar(c)
		ui.Column(c).Grow(1).Fill().Children(func() {
			if v.snap.Loaded && len(v.snap.Matches) > 0 && v.page != pageFormats {
				v.filterBar(c)
			}
			if v.snap.Loaded && v.live.Phase != live.Idle {
				v.liveStrip(c)
			}
			if v.page == pageOverview && len(v.snap.Debuts) > 0 {
				v.debutBanner(c)
			}
			switch {
			case !v.snap.Loaded:
				v.message(c, "Loading your games", "")
			case v.page == pageFormats:
				v.formatsPage(c)
			case len(v.snap.Matches) == 0 && !v.snap.SimFound:
				v.message(c, "OPTCGSim isn't running",
					"Start the sim and logpose finds it and imports your games. If it's installed somewhere unusual, start logpose with -sim and the sim's folder.")
			case len(v.snap.Matches) == 0:
				v.message(c, "Play a game in the sim", "Every game you finish shows up here on its own.")
			case len(v.scoped().matches) == 0:
				v.message(c, "No games match these filters", "Change or clear the filters above.")
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
			ui.SidebarItem(c, pageFormats, iconFormats, "Formats")
		})
		ui.Spacer(c)
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
