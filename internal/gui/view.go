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
	Matches  []stats.Match
	Overview stats.Overview
	Cards    *cards.DB
	Now      time.Time
	// Formats are the user's formats, oldest first, and Debuts the sets that look like they
	// started one.
	Formats  []stats.FormatRange
	Debuts   []stats.SetDebut
	Settings settingsInfo
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
	pageMatchups = "matchups"
	pageFormats  = "formats"
	pageSettings = "settings"
)

// actions are the changes the view asks the app for. The app makes them off the main thread
// and sends a new snapshot.
type actions interface {
	AddFormat(name string, start time.Time)
	UpdateFormat(f store.Format)
	DeleteFormat(id int64)
	DismissSet(set string)
	// LoadTimeline reads a match's life by turn and key moments from its log and shows them with
	// view.setTimeline.
	LoadTimeline(fingerprint, me, opp string)
	SetGameType(fingerprint string, t store.GameType)
	SetGameNote(fingerprint, note string)
	SetBookmark(fingerprint string, on bool)
	// OpenReplay builds a match's replay from its log and shows it with view.setReplay.
	OpenReplay(fingerprint string)
	// After runs fn on the main thread once d has passed.
	After(d time.Duration, fn func())

	SetSetting(key, value string)
	SetOpenAtLogin(on bool)
	// ChooseSimFolder asks for the sim's folder and switches to it.
	ChooseSimFolder()
	// FindSim forgets the chosen folder and finds the sim on its own.
	FindSim()
	OpenPath(path string)
	ShowFile(path string)
	// ImportAgain reads every log in the sim's folder and adds what's missing.
	ImportAgain()
	RefreshCards()
	Backup()
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
	// detail is the open match. Its fingerprint is empty on the match list.
	detail detailState

	filter filters
	scope  scoped

	leaderPoint int
	leaderStats leaderCache
	// shownPage is the page the last frame showed, which tells a page's first frame.
	shownPage string

	matchups matchupsState
	settings settingsState
	prefs    prefs
}

// openLeader shows a leader on the Leaders page.
func (v *view) openLeader(code string) {
	v.page = pageLeaders
	v.setDeck(code)
}

// The themes are built once, since build sets one every frame. The light one is mygo's with
// the bundled font.
var (
	darkTheme  = graphite()
	lightTheme = func() *ui.Theme {
		t := ui.LightTheme()
		t.Font = fontFamily
		return t
	}()
)

func newView(art *artCache) *view {
	return &view{snap: &snapshot{}, art: art, page: pageOverview, leaderPoint: -1, formats: formatsState{day: -1}}
}

// applyLook sets the theme the user picked, or the one following Windows, and the result
// colors.
func (v *view) applyLook(c *ui.Context) {
	dark := c.Theme().Dark
	switch v.prefs.theme {
	case "dark":
		dark = true
	case "light":
		dark = false
	}
	if dark {
		c.SetTheme(darkTheme)
	} else {
		c.SetTheme(lightTheme)
	}
	winColor, lossColor = resultColors(v.prefs.colorblind)
}

func (v *view) build(c *ui.Context) {
	if v.page != pageMatches {
		v.closeMatch()
	}
	v.replayKeys(c)
	v.applyLook(c)
	open := v.detail.fingerprint != ""
	shown := v.page
	if open {
		shown = "match " + v.detail.fingerprint
	}
	pageEntered = shown != v.shownPage
	v.shownPage = shown
	ui.Row(c).Fill().AlignItems(ui.Stretch).FontFeatures("tnum").Children(func() {
		v.sidebar(c)
		ui.Column(c).Grow(1).Fill().Children(func() {
			if v.snap.Loaded && len(v.snap.Matches) > 0 && v.page != pageFormats && v.page != pageSettings && !open {
				v.filterBar(c)
			}
			if v.snap.Loaded && v.live.Phase != live.Idle {
				v.liveStrip(c)
			}
			if v.page == pageOverview && len(v.snap.Debuts) > 0 {
				v.debutBanner(c)
			}
			v.pageBody(c, open, shown)
		})
	})
}

// pageBody is the page, or a message when there's nothing to show on it.
func (v *view) pageBody(c *ui.Context, open bool, shown string) {
	switch {
	case !v.snap.Loaded:
		v.message(c, "Loading your games", "")
	case v.page == pageFormats:
		v.formatsPage(c)
	case v.page == pageSettings:
		v.settingsPage(c)
	case len(v.snap.Matches) == 0 && !v.snap.SimFound:
		v.setupPage(c)
	case len(v.snap.Matches) == 0:
		v.message(c, "Play a game in the sim", "Every game you finish shows up here on its own.")
	case open:
		ui.Column(c.Key(shown)).Grow(1).Basis(0).MinHeight(0).Transition(pageEnter).Children(func() { v.matchPage(c) })
	case len(v.scoped().matches) == 0:
		v.message(c, "No games match these filters", "Change or clear the filters above.")
	default:
		ui.Column(c.Key(shown)).Grow(1).Basis(0).MinHeight(0).Transition(pageEnter).Children(func() { v.statsPage(c) })
	}
}

// statsPage is the page chosen in the sidebar, for games in the filters' scope.
func (v *view) statsPage(c *ui.Context) {
	switch v.page {
	case pageMatches:
		v.matchesPage(c)
	case pageLeaders:
		v.leadersPage(c)
	case pageMatchups:
		v.matchupsPage(c)
	default:
		v.overviewPage(c)
	}
}

func (v *view) sidebar(c *ui.Context) {
	if v.detail.replay != nil {
		return
	}
	t := c.Theme()
	ui.Column(c).Width(200).Background(sidebarColor(t)).BorderEndWidth(1).BorderColor(t.Border.Alpha(0.6)).Padding(12, 8).Gap(8).Children(func() {
		ui.Row(c).Gap(8).Padding(4, 8, 8, 8).Children(func() {
			ui.Icon(c, iconCompass).Size(20, 20).TextColor(t.Accent)
			ui.Text(c, "logpose").Bold().FontSize(16)
		})
		// mygo draws sidebar icons in the accent color, so the sidebar gets a theme whose accent
		// is the muted text.
		ui.Themed(c, sidebarTheme(t)).Background(ui.Transparent).Children(func() {
			ui.Sidebar(c, &v.page, func() {
				ui.SidebarItem(c, pageOverview, iconOverview, "Overview")
				ui.SidebarItem(c, pageLeaders, iconLeaders, "Leaders")
				ui.SidebarItem(c, pageMatchups, iconMatchups, "Matchups")
				ui.SidebarItem(c, pageMatches, iconMatches, "Matches")
				ui.SidebarItem(c, pageFormats, iconFormats, "Formats")
			}).Background(ui.Transparent).Padding(0)
		})
		ui.Spacer(c)
		ui.Themed(c, sidebarTheme(t)).Background(ui.Transparent).Children(func() {
			ui.Sidebar(c, &v.page, func() {
				ui.SidebarItem(c, pageSettings, iconSettings, "Settings")
			}).Background(ui.Transparent).Padding(0)
		})
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
