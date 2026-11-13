package gui

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/live"
	"github.com/VTGare/logpose/internal/shortcut"
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
	// Formats are the user's formats, oldest first, and Debuts the sets that look like they
	// started one.
	Formats []stats.FormatRange
	Debuts  []stats.SetDebut
	// Stored is how many games the store holds, the local player's or not.
	Stored int
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

const (
	// groupTop pages sit at the top, groupRef under a line and groupSetup at the bottom.
	groupTop = iota
	groupRef
	groupSetup
)

type page struct {
	id    string
	icon  *ui.SVG
	label string
	group int
	// games pages show the games in the filters' scope under the filter bar, and a message
	// instead while there are none. The others show whatever is stored.
	games bool
	draw  func(v *view, c *ui.Context)
	// head draws over the page, under the filter bar and the live strip.
	head func(v *view, c *ui.Context)
	// subs are what -page takes after the id and a slash. any, when set, takes every name
	// there and is how the help shows one. open shows one.
	subs []string
	any  string
	open func(v *view, sub string)
}

// pages are the app's pages in the sidebar's order. A Decks tab is "decks/" and its name,
// "decks/build" chooses the first build and "decks/edit" opens the core editor on it. "cards/"
// and a card's code opens the art viewer on that card.
var pages = []page{
	{id: pageOverview, icon: iconOverview, label: "Overview", games: true, draw: func(v *view, c *ui.Context) {
		overviewPage(c, &v.env, func() { v.settings.shortcutCard(c, &v.env) })
	}, head: func(v *view, c *ui.Context) { v.formats.debutBanner(c, &v.env) }},
	{id: pageMatches, icon: iconMatches, label: "Matches", games: true, draw: func(v *view, c *ui.Context) { v.matches.draw(c, &v.env) }},
	{
		id: pageDecks, icon: iconLeaders, label: "Decks", games: true, draw: func(v *view, c *ui.Context) { v.decks.draw(c, &v.env) },
		subs: deckPageSubs(), open: func(v *view, sub string) { v.decks.open(&v.env, sub) },
	},
	{id: pageMatchups, icon: iconMatchups, label: "Matchups", games: true, draw: func(v *view, c *ui.Context) { v.matchups.draw(c, &v.env) }},
	{
		id: pageCards, icon: iconCards, label: "Cards", group: groupRef, draw: func(v *view, c *ui.Context) {
			v.cards.note = v.settings.noteOn(noteCards)
			v.cards.draw(c, &v.env)
		},
		any: "<code>", open: func(v *view, code string) { v.cards.state.selected, v.cards.state.viewer = code, true },
	},
	{id: pageFormats, icon: iconFormats, label: "Formats", group: groupSetup, draw: func(v *view, c *ui.Context) { v.formats.draw(c, &v.env) }},
	{id: pageSettings, icon: iconSettings, label: "Settings", group: groupSetup, draw: func(v *view, c *ui.Context) { v.settings.draw(c, &v.env) }},
}

func deckPageSubs() []string {
	var out []string
	for _, tab := range deckPageTabs {
		if tab.sub != "" {
			out = append(out, tab.sub)
		}
	}
	return append(out, "edit", "build", "nobuild")
}

func pageOf(id string) (page, bool) {
	for _, p := range pages {
		if p.id == id {
			return p, true
		}
	}
	return page{}, false
}

// Pages are the ids of every page and every name after a slash that -page takes.
func Pages() []string {
	var ids, subs []string
	for _, p := range pages {
		ids = append(ids, p.id)
		for _, s := range p.subs {
			subs = append(subs, p.id+"/"+s)
		}
	}
	for _, p := range pages {
		if p.any != "" {
			subs = append(subs, p.id+"/"+p.any)
		}
	}
	return append(ids, subs...)
}

// IsPage reports whether -page takes the id.
func IsPage(id string) bool {
	name, sub, _ := strings.Cut(id, "/")
	p, ok := pageOf(name)
	switch {
	case !ok:
		return false
	case !strings.Contains(id, "/"):
		return true
	case p.any != "":
		return sub != ""
	}
	return slices.Contains(p.subs, sub)
}

// open shows a page IsPage takes. What follows the slash waits for the games, since it can
// name what only they have, such as a leader's first build.
func (v *view) open(id string) {
	v.page, _, _ = strings.Cut(id, "/")
	v.opening = id
}

func (v *view) openSub() {
	if v.opening == "" || !v.snap.Loaded {
		return
	}
	name, sub, _ := strings.Cut(v.opening, "/")
	v.opening = ""
	if p, ok := pageOf(name); ok && sub != "" && p.open != nil && v.page == name {
		p.open(v, sub)
	}
}

// actions are everything the frame and the pages ask the app for. The app makes each change
// off the main thread and sends a new snapshot.
type actions interface {
	envActions
	formatActions
	settingsActions
	matchActions
	deckActions
	cardActions
	// NewDay runs at midnight, for what the app shows outside the window.
	NewDay()
}

// envActions are the changes the env asks the app for.
type envActions interface {
	SetSetting(key, value string)
	ApplyPrefs(p prefs)
	// After runs fn on the main thread once d has passed.
	After(d time.Duration, fn func())
}

// env is what the frame hands every page: the games, the prefs and the clock.
type env struct {
	snap  *snapshot
	prefs prefs
	act   envActions
	clock func() time.Time
	// pending counts the calls to later by key, so only the last one runs.
	pending map[string]int
	// entered is true in a page's first frame, when bars grow from zero and charts draw in.
	entered bool
	frame   int

	art      *artCache
	filter   filters
	scope    scoped
	periodUI periodState
	// page is the id of the page shown.
	page string
	// listResult indexes resultLabels. It narrows the Matches list only, since on the other pages
	// it would make every win rate 100% or 0%.
	listResult int
	nav        nav
	// toast is a message for the next frame to show.
	toast *ui.Toast
}

// nav is how the shared widgets reach what one page owns.
type nav interface {
	openMatch(m stats.Match)
	openInCards(code string)
	viewArt(code string)
	gameMenu(m stats.Match) func(*ui.Menu)
	cardMenu(card cards.Card) func(*ui.Menu)
	showsHands() bool
	showHands(code string)
}

func (e *env) now() time.Time { return e.clock() }

type view struct {
	env
	act actions
	// wake is the next midnight a frame is asked for, so the day names change without a new
	// snapshot.
	wake    time.Time
	formats formatsPage
	// live is the game in the sim right now, which the log follower updates on its own.
	live live.State
	// opening is the id open was given until the games are in.
	opening string

	matches matchesPage

	decks decksPage
	// shownPage is the page the last frame showed.
	shownPage string

	matchups matchupsPage
	settings settingsPage
	cards    cardsPage
	sideDrag paneDrag
}

// setActions gives the view, the env and each page the app's actions.
func (v *view) setActions(a actions) {
	v.act, v.env.act, v.formats.act, v.settings.act, v.matches.act, v.decks.act, v.cards.act = a, a, a, a, a, a, a
}

// The setters below are how the app changes what the view shows. It calls them on the main
// thread.

func (v *view) setSnapshot(s *snapshot)         { v.snap = s }
func (v *view) setLive(s live.State)            { v.live = s }
func (v *view) setArtCache(a *artCache)         { v.art = a }
func (v *view) setArtSync(s artSync)            { v.cards.artSync = s }
func (v *view) addCardSync(p cards.SetProgress) { v.cards.cardSync.add(p) }
func (v *view) endCardSync()                    { v.cards.cardSync = cardSync{} }
func (v *view) setSystem(s systemState)         { v.settings.system = s }
func (v *view) setOpenAtLogin(on bool)          { v.settings.system.openAtLogin = on }
func (v *view) setArtUsage(u arts.Usage)        { v.settings.art = &u }
func (v *view) queueToast(t ui.Toast)           { v.toast = &t }

func (v *view) setShortcutMade(place shortcut.Place, on bool) {
	v.settings.system.shortcuts[place] = on
}

func (v *view) setSettingsInfo(info settingsInfo) { v.settings.info = &info }

func (e *env) openLeader(code string) {
	e.page = pageDecks
	e.setDeck(code)
}

func newView(art *artCache) *view {
	v := &view{env: env{snap: &snapshot{}, clock: time.Now, art: art, page: pageOverview}, formats: formatsPage{day: -1}}
	v.nav = v
	v.setPrefs(prefsFrom(nil))
	return v
}

// setPrefs takes the stored prefs, and starts the filters on the format they name.
func (v *view) setPrefs(p prefs) {
	v.prefs = p
	v.filter.format = p.startFormat
}

// applyLook sets the theme the user picked, or the one following Windows, with the result
// colors of the colorblind setting.
func (v *view) applyLook(c *ui.Context) {
	l := look{dark: c.Theme().Dark, colorblind: v.prefs.colorblind}
	switch v.prefs.theme {
	case "dark":
		l.dark = true
	case "light":
		l.dark = false
	}
	c.SetTheme(looks[l])
}

func (v *view) build(c *ui.Context) {
	if v.matches.detail.fingerprint != "" && v.page != v.matches.detail.from {
		v.matches.closeMatch()
	}
	v.wakeAtMidnight()
	v.openSub()
	v.matches.replayKeys(c, &v.env)
	v.applyLook(c)
	v.showToast(c)
	toasts(c)
	open := v.matches.detail.fingerprint != ""
	shown := v.page
	if open {
		shown = "match " + v.matches.detail.fingerprint
	}
	v.frame++
	v.entered = shown != v.shownPage
	v.shownPage = shown
	body := func() {
		ui.Column(c).Grow(1).Fill().Children(func() {
			if v.snap.Loaded && len(v.snap.Matches) > 0 && v.current().games && !open {
				v.filterBar(c)
			}
			if v.snap.Loaded && v.live.Phase != live.Idle {
				v.liveStrip(c)
			}
			if head := v.current().head; head != nil {
				head(v, c)
			}
			v.pageBody(c, open, shown)
		})
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).FontFeatures("tnum").Children(func() {
		if v.matches.detail.replay != nil {
			body()
			return
		}
		shown := v.prefs.sideWidth
		size := shown
		split := ui.Split(c, &size, func() { v.sidebar(c) }, body).Fill()
		if split.Changed() {
			v.dragPane(prefSideWidth, &v.sideDrag, size-shown, snapSide)
		}
	})
	// A card's menu opens the art viewer on any page. The Cards page draws its own.
	if v.cards.state.viewer && v.page != pageCards && v.snap.Cards != nil {
		v.cards.artViewer(c, &v.env, v.snap.Cards.Card(v.cards.state.selected))
	}
}

func (v *view) wakeAtMidnight() {
	now := v.now()
	next := stats.DayStart(now).AddDate(0, 0, 1)
	if next.Equal(v.wake) || v.act == nil {
		return
	}
	v.wake = next
	v.act.After(next.Sub(now)+time.Second, func() { v.act.NewDay() })
}

// trayTip is the tray icon's tooltip, with today's record once a game was played today.
func (v *view) trayTip() string {
	var r stats.Record
	n := 0
	for _, m := range v.snap.Matches {
		if !stats.DayStart(m.PlayedAt).Equal(v.today()) {
			break
		}
		r.Add(m.Result)
		n++
	}
	if n == 0 {
		return "logpose"
	}
	return "logpose · today " + record(r)
}

func (v *view) current() page {
	p, ok := pageOf(v.page)
	if !ok {
		p, _ = pageOf(pageOverview)
	}
	return p
}

// pageBody is the page, or a message when there's nothing to show on it.
func (v *view) pageBody(c *ui.Context, open bool, shown string) {
	p := v.current()
	switch {
	case !v.snap.Loaded:
		v.message(c, "Loading your games", "")
	case !p.games:
		p.draw(v, c)
	case len(v.snap.Matches) == 0 && !v.snap.SimFound:
		v.settings.setupPage(c)
	case len(v.snap.Matches) == 0:
		v.message(c, "Play a game in the sim", "Every game you finish shows up here on its own.")
	case open:
		ui.Column(c.Key(shown)).Grow(1).Basis(0).MinHeight(0).Transition(pageEnter).Children(func() { v.matches.matchPage(c, &v.env) })
	case len(v.scoped().matches) == 0:
		v.message(c, "No games match these filters", "Change or clear the filters above.")
	default:
		ui.Column(c.Key(shown)).Grow(1).Basis(0).MinHeight(0).Transition(pageEnter).Children(func() { p.draw(v, c) })
	}
}

// The sidebar is resized by dragging its edge. Narrower than sideSnap it folds to an icon rail.
const (
	sideDefault = 200
	sideMin     = 160
	sideMax     = 320
	sideSnap    = 120
	sideRail    = 60
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
	at float32
}

// dragPane moves the width e holds by moved through snap, and saves it once the drag stops.
func (e *env) dragPane(p prefEntry[float32], d *paneDrag, moved float32, snap func(float32) float32) {
	width := p.field(&e.prefs)
	if d.at == 0 {
		d.at = *width
	}
	d.at += moved
	*width = snap(d.at)
	e.later(p.key, func() {
		d.at = 0
		e.savePref(p)
	})
}

func pageLabel(id string) string {
	p, _ := pageOf(id)
	return p.label
}

// sidebar is the pages as icons, and with their names when it's wider than the rail. The icons
// sit at the same place either way, so folding it only hides the names.
func (v *view) sidebar(c *ui.Context) {
	t := c.Theme()
	wide := v.prefs.sideWidth > sideRail
	item := func(p page) {
		id, label := p.id, p.label
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
			if v.page == id {
				v.matches.closeMatch()
			}
			v.page = id
		}
		if !wide {
			b.Width(44).Center().Tooltip(label)
			b.Children(func() { ui.Icon(c, p.icon).Size(18, 18).TextColor(col) })
			return
		}
		b.Padding(0, 13).Children(func() {
			ui.Row(c).Fill().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, p.icon).Size(18, 18).TextColor(col)
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
		group := func(g int) {
			for _, p := range pages {
				if p.group == g {
					item(p)
				}
			}
		}
		group(groupTop)
		// The line takes the same height in both widths, so the icons under it keep their place.
		line := ui.Box(c).Height(1).Background(t.Border)
		if wide {
			line.Margin(8, 13)
		} else {
			line.Width(24).Margin(8, 0)
		}
		group(groupRef)
		ui.Spacer(c)
		v.settings.updateBox(c, wide)
		group(groupSetup)
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

func (v *view) openMatch(m stats.Match) { v.matches.openMatch(&v.env, m) }

func (v *view) gameMenu(m stats.Match) func(*ui.Menu) { return v.matches.gameMenu(&v.env, m) }

func (v *view) cardMenu(card cards.Card) func(*ui.Menu) { return v.cards.cardMenu(&v.env, card) }
