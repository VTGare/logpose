package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/live"
	"github.com/VTGare/logpose/internal/shortcut"
	"github.com/VTGare/logpose/internal/sim"
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
	// Decks are the matches with what the local player's deck did, oldest first, keyed by
	// fingerprint. Builds are the user's builds, and Dismissed the cores of suggestions they said
	// aren't builds, by leader.
	Decks     map[string]stats.DeckGame
	Builds    []store.Build
	Dismissed map[string][][]string
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
	pageDecks    = "decks"
	pageMatchups = "matchups"
	pageFormats  = "formats"
	pageSettings = "settings"
	pageCards    = "cards"
)

// Pages are the ids of every page, which -page takes. A Decks tab is "decks/" and its name, and
// "decks/edit" opens the core editor on the first build.
func Pages() []string {
	ids := []string{pageSettings}
	for _, p := range navPages {
		ids = append(ids, p.id)
	}
	for _, tab := range deckTabLabels[1:] {
		ids = append(ids, pageDecks+"/"+strings.ToLower(tab))
	}
	return append(ids, pageDecks+"/edit", pageDecks+"/build", pageDecks+"/nobuild")
}

// open shows a page from Pages.
func (v *view) open(id string) {
	page, tab, _ := strings.Cut(id, "/")
	v.page = page
	v.decks.editOnOpen = tab == "edit"
	v.decks.buildOnOpen = tab == "build"
	if tab == "nobuild" {
		v.decks.scope = deckScope{kind: scopeNoBuild}
	}
	for i, label := range deckTabLabels {
		if strings.ToLower(label) == tab {
			v.decks.tab = i
		}
	}
}

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
	SaveBuild(b store.Build)
	DeleteBuild(id int64)
	DismissSuggestion(d store.Dismissal)
	// OpenReplay builds a match's replay from its log and shows it with view.setReplay.
	OpenReplay(fingerprint string)
	// After runs fn on the main thread once d has passed.
	After(d time.Duration, fn func())

	SetSetting(key, value string)
	SetOpenAtLogin(on bool)
	// SetShortcut adds or removes logpose's shortcut in place.
	SetShortcut(place shortcut.Place, on bool)
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
	LoadArtUsage()
	ClearArt()
	SetArtCap(mb int)
	SetArtDownloads(on bool)

	// LoadSimArt reads what the sim shows for the card and shows it with view.setSimArt.
	LoadSimArt(card cards.Card)
	// LoadArts groups the card's pictures and shows each step with view.setArts.
	LoadArts(card cards.Card, gen int)
	// ScanSimArt finds the cards the sim shows other art for.
	ScanSimArt()
	// UseSimArt makes the sim show choice for the card. prev is what it showed, for Undo.
	UseSimArt(card cards.Card, choice, prev sim.ArtChoice)
	SaveArt(key string)
	CopyArt(key string)
	OpenURL(url string)
}

type view struct {
	snap *snapshot
	act  actions
	// toast is a message for the next frame to show.
	toast *ui.Toast
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
	decks       decksState
	// shownPage is the page the last frame showed, which tells a page's first frame.
	shownPage string

	matchups  matchupsState
	settings  settingsState
	prefs     prefs
	periodUI  periodState
	cardsUI   cardsState
	cardSync  cardSync
	artSync   artSync
	sideDrag  paneDrag
	panelDrag paneDrag
	simArt    simArtState
	arts      artsState
}

// openLeader shows a leader on the Decks page.
func (v *view) openLeader(code string) {
	v.page = pageDecks
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
	v.showToast(c)
	toasts(c)
	open := v.detail.fingerprint != ""
	shown := v.page
	if open {
		shown = "match " + v.detail.fingerprint
	}
	pageEntered = shown != v.shownPage
	v.shownPage = shown
	body := func() {
		ui.Column(c).Grow(1).Fill().Children(func() {
			if v.snap.Loaded && len(v.snap.Matches) > 0 && v.page != pageFormats && v.page != pageSettings && v.page != pageCards && !open {
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
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).FontFeatures("tnum").Children(func() {
		if v.detail.replay != nil {
			body()
			return
		}
		if v.prefs.sideWidth == 0 {
			v.prefs.sideWidth = sideDefault
		}
		shown := v.prefs.sideWidth
		size := shown
		split := ui.Split(c, &size, func() { v.sidebar(c) }, body).Fill()
		if split.Changed() {
			v.dragPane(&v.prefs.sideWidth, &v.sideDrag, size-shown, snapSide, store.SettingSidebarWidth)
		}
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
	case v.page == pageCards:
		v.cardsPage(c)
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
	case pageDecks:
		v.decksPage(c)
	case pageMatchups:
		v.matchupsPage(c)
	default:
		v.overviewPage(c)
	}
}

// The sidebar is resized by dragging its edge. Narrower than sideSnap it folds to an icon rail.
const (
	sideDefault = 200
	sideMin     = 160
	sideMax     = 320
	sideSnap    = 120
	sideRail    = 60
	// sideSaveDelay waits for a drag to end before a width is saved.
	sideSaveDelay = 400 * time.Millisecond
)

func snapSide(w float32) float32 {
	switch {
	case w < sideSnap:
		return sideRail
	case w < sideMin:
		return sideMin
	}
	return min(w, sideMax)
}

// paneDrag follows a drag of a split's edge. The split moves the edge a few DIPs a frame from
// where it shows it, so the drag's own position is kept apart from the snapped width, or each
// small move would snap straight back.
type paneDrag struct {
	at    float32
	saves int
}

// dragPane moves *width by moved through snap, and saves it under key once the drag stops.
func (v *view) dragPane(width *float32, d *paneDrag, moved float32, snap func(float32) float32, key string) {
	if d.at == 0 {
		d.at = *width
	}
	d.at += moved
	*width = snap(d.at)
	d.saves++
	n := d.saves
	v.do(func(a actions) {
		a.After(sideSaveDelay, func() {
			if n != d.saves {
				return
			}
			d.at = 0
			v.savePane(*width, key)
		})
	})
}

func (v *view) savePane(width float32, key string) {
	v.do(func(a actions) { a.SetSetting(key, fmt.Sprint(int(width))) })
}

var navPages = []struct {
	id    string
	icon  *ui.SVG
	label string
}{
	{pageOverview, iconOverview, "Overview"},
	{pageDecks, iconLeaders, "Decks"},
	{pageMatchups, iconMatchups, "Matchups"},
	{pageMatches, iconMatches, "Matches"},
	{pageFormats, iconFormats, "Formats"},
	{pageCards, iconCards, "Cards"},
}

// sidebar is the pages as icons, and with their names when it's wider than the rail. The icons
// sit at the same place either way, so folding it only hides the names.
func (v *view) sidebar(c *ui.Context) {
	t := c.Theme()
	wide := v.prefs.sideWidth > sideRail
	item := func(id string, icon *ui.SVG, label string) {
		b := ui.ButtonBase(c.Key(id)).Label(label).Height(40).Radius(8).Background(ui.Transparent)
		col := t.TextMuted
		switch {
		case v.page == id:
			b.Background(t.Surface)
			col = t.Text
		case b.Hovered():
			b.Background(t.SurfaceHover)
		}
		if b.Clicked() {
			v.page = id
		}
		if !wide {
			b.Width(44).Center().Tooltip(label)
			b.Children(func() { ui.Icon(c, icon).Size(18, 18).TextColor(col) })
			return
		}
		b.Padding(0, 13).Children(func() {
			ui.Row(c).Fill().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icon).Size(18, 18).TextColor(col)
				ui.Text(c, label).TextColor(col)
			})
		})
	}
	col := ui.Column(c).Fill().Background(sidebarColor(t)).Padding(12, 0).Gap(4)
	if wide {
		col.Padding(12, 8)
	} else {
		col.AlignItems(ui.Center)
	}
	col.Children(func() {
		ui.Row(c).Height(36).Gap(10).Padding(0, 12).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconCompass).Size(20, 20).TextColor(t.Accent)
			if wide {
				ui.Text(c, "logpose").Bold()
			}
		})
		for _, p := range navPages {
			item(p.id, p.icon, p.label)
		}
		ui.Spacer(c)
		item(pageSettings, iconSettings, "Settings")
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
