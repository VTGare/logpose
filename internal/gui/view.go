package gui

import (
	"slices"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/gui/cardspage"
	"github.com/VTGare/logpose/internal/gui/decks"

	"github.com/VTGare/logpose/internal/gui/matches"
	"github.com/VTGare/logpose/internal/gui/settings"

	"github.com/VTGare/logpose/internal/gui/formats"
	"github.com/VTGare/logpose/internal/gui/matchups"
	"github.com/VTGare/logpose/internal/gui/overview"

	"github.com/VTGare/logpose/internal/gui/page"

	"github.com/VTGare/logpose/internal/gui/kit"

	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/live"
	"github.com/VTGare/logpose/internal/shortcut"
	"github.com/VTGare/logpose/internal/stats"
)

const (
	// groupTop pages sit at the top, groupRef under a line and groupSetup at the bottom.
	groupTop = iota
	groupRef
	groupSetup
)

type pageDef struct {
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
var pages = []pageDef{
	{id: page.Overview, icon: kit.IconOverview, label: "Overview", games: true, draw: func(v *view, c *ui.Context) {
		overview.Draw(c, &v.Env, func() { v.settings.ShortcutCard(c, &v.Env) })
	}, head: func(v *view, c *ui.Context) { v.formats.DebutBanner(c, &v.Env) }},
	{id: page.Matches, icon: kit.IconMatches, label: "Matches", games: true, draw: func(v *view, c *ui.Context) { v.matches.Draw(c, &v.Env) }},
	{
		id: page.Decks, icon: kit.IconLeaders, label: "Decks", games: true, draw: func(v *view, c *ui.Context) { v.decks.Draw(c, &v.Env) },
		subs: deckPageSubs(), open: func(v *view, sub string) { v.decks.Open(&v.Env, sub) },
	},
	{id: page.Matchups, icon: kit.IconMatchups, label: "Matchups", games: true, draw: func(v *view, c *ui.Context) { v.matchups.Draw(c, &v.Env) }},
	{
		id: page.Cards, icon: kit.IconCards, label: "Cards", group: groupRef, draw: func(v *view, c *ui.Context) {
			v.cards.Note = v.settings.NoteOn(settings.NoteCards)
			v.cards.Draw(c, &v.Env)
		},
		any: "<code>", open: func(v *view, code string) { v.cards.State.Selected, v.cards.State.Viewer = code, true },
	},
	{id: page.Formats, icon: kit.IconFormats, label: "Formats", group: groupSetup, draw: func(v *view, c *ui.Context) { v.formats.Draw(c, &v.Env) }},
	{id: page.Settings, icon: kit.IconSettings, label: "Settings", group: groupSetup, draw: func(v *view, c *ui.Context) { v.settings.Draw(c, &v.Env) }},
}

func deckPageSubs() []string {
	var out []string
	for _, tab := range decks.DeckPageTabs {
		if tab.Sub != "" {
			out = append(out, tab.Sub)
		}
	}
	return append(out, "edit", "build", "nobuild")
}

func pageOf(id string) (pageDef, bool) {
	for _, p := range pages {
		if p.id == id {
			return p, true
		}
	}
	return pageDef{}, false
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
	v.Page, _, _ = strings.Cut(id, "/")
	v.opening = id
}

func (v *view) openSub() {
	if v.opening == "" || !v.Snap.Loaded {
		return
	}
	name, sub, _ := strings.Cut(v.opening, "/")
	v.opening = ""
	if p, ok := pageOf(name); ok && sub != "" && p.open != nil && v.Page == name {
		p.open(v, sub)
	}
}

// actions are everything the frame and the pages ask the app for. The app makes each change
// off the main thread and sends a new snapshot.
type actions interface {
	page.EnvActions
	formats.Actions
	settings.Actions
	matches.Actions
	decks.Actions
	cardspage.Actions
	// NewDay runs at midnight, for what the app shows outside the window.
	NewDay()
}

type view struct {
	page.Env
	act actions
	// wake is the next midnight a frame is asked for, so the day names change without a new
	// snapshot.
	wake    time.Time
	formats formats.Page
	// live is the game in the sim right now, which the log follower updates on its own.
	live live.State
	// opening is the id open was given until the games are in.
	opening string

	matches matches.Page

	decks decks.Page
	// shownPage is the page the last frame showed.
	shownPage string

	matchups matchups.Page
	settings settings.Page
	cards    cardspage.Page
	sideDrag page.PaneDrag
}

// setActions gives the view, the env and each page the app's actions.
func (v *view) setActions(a actions) {
	v.act, v.Act, v.formats.Act, v.settings.Act, v.matches.Act, v.decks.Act, v.cards.Act = a, a, a, a, a, a, a
}

// The setters below are how the app changes what the view shows. It calls them on the main
// thread.

func (v *view) setSnapshot(s *page.Snapshot)    { v.Snap = s }
func (v *view) setLive(s live.State)            { v.live = s }
func (v *view) setArtCache(a *page.ArtCache)    { v.Art = a }
func (v *view) setArtSync(s cardspage.ArtSync)  { v.cards.ArtSync = s }
func (v *view) addCardSync(p cards.SetProgress) { v.cards.CardSync.Add(p) }
func (v *view) endCardSync()                    { v.cards.CardSync = cardspage.CardSync{} }
func (v *view) setSystem(s settings.System)     { v.settings.System = s }
func (v *view) setOpenAtLogin(on bool)          { v.settings.System.OpenAtLogin = on }
func (v *view) setArtUsage(u arts.Usage)        { v.settings.Art = &u }
func (v *view) queueToast(t ui.Toast)           { v.Toast = &t }

func (v *view) setShortcutMade(place shortcut.Place, on bool) {
	v.settings.System.Shortcuts[place] = on
}

func (v *view) setSettingsInfo(info settings.Info) { v.settings.Info = &info }

func newView(art *page.ArtCache) *view {
	v := &view{Env: page.Env{Snap: &page.Snapshot{}, Clock: time.Now, Art: art, Page: page.Overview}, formats: formats.New()}
	v.Nav = v
	v.setPrefs(page.PrefsFrom(nil))
	return v
}

// setPrefs takes the stored page.Prefs, and starts the filters on the format they name.
func (v *view) setPrefs(p page.Prefs) {
	v.Prefs = p
	v.Filter.Format = p.StartFormat
}

// applyLook sets the theme the user picked, or the one following Windows, with the result
// colors of the colorblind setting.
func (v *view) applyLook(c *ui.Context) {
	l := kit.Look{Dark: c.Theme().Dark, Colorblind: v.Prefs.Colorblind}
	switch v.Prefs.Theme {
	case "dark":
		l.Dark = true
	case "light":
		l.Dark = false
	}
	c.SetTheme(kit.Looks[l])
}

func (v *view) build(c *ui.Context) {
	if v.matches.Detail.Fingerprint != "" && v.Page != v.matches.Detail.From {
		v.matches.CloseMatch()
	}
	v.wakeAtMidnight()
	v.openSub()
	v.matches.ReplayKeys(c, &v.Env)
	v.applyLook(c)
	v.showToast(c)
	kit.Toasts(c)
	open := v.matches.Detail.Fingerprint != ""
	shown := v.Page
	if open {
		shown = "match " + v.matches.Detail.Fingerprint
	}
	v.Frame++
	v.Entered = shown != v.shownPage
	v.shownPage = shown
	body := func() {
		ui.Column(c).Grow(1).Fill().Children(func() {
			if v.Snap.Loaded && len(v.Snap.Matches) > 0 && v.current().games && !open {
				v.FilterBar(c)
			}
			if v.Snap.Loaded && v.live.Phase != live.Idle {
				v.liveStrip(c)
			}
			if head := v.current().head; head != nil {
				head(v, c)
			}
			v.pageBody(c, open, shown)
		})
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).FontFeatures("tnum").Children(func() {
		if v.matches.Detail.Replay != nil {
			body()
			return
		}
		shown := v.Prefs.SideWidth
		size := shown
		split := ui.Split(c, &size, func() { v.sidebar(c) }, body).Fill()
		if split.Changed() {
			v.DragPane(page.PrefSideWidth, &v.sideDrag, size-shown, page.SnapSide)
		}
	})
	// A card's menu opens the art viewer on any page. The Cards page draws its own.
	if v.cards.State.Viewer && v.Page != page.Cards && v.Snap.Cards != nil {
		v.cards.ArtViewer(c, &v.Env, v.Snap.Cards.Card(v.cards.State.Selected))
	}
}

func (v *view) wakeAtMidnight() {
	now := v.Now()
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
	for _, m := range v.Snap.Matches {
		if !stats.DayStart(m.PlayedAt).Equal(v.Today()) {
			break
		}
		r.Add(m.Result)
		n++
	}
	if n == 0 {
		return "logpose"
	}
	return "logpose · today " + kit.Record(r)
}

func (v *view) current() pageDef {
	p, ok := pageOf(v.Page)
	if !ok {
		p, _ = pageOf(page.Overview)
	}
	return p
}

// pageBody is the page, or a message when there's nothing to show on it.
func (v *view) pageBody(c *ui.Context, open bool, shown string) {
	p := v.current()
	switch {
	case !v.Snap.Loaded:
		v.message(c, "Loading your games", "")
	case !p.games:
		p.draw(v, c)
	case len(v.Snap.Matches) == 0 && !v.Snap.SimFound:
		v.settings.SetupPage(c)
	case len(v.Snap.Matches) == 0:
		v.message(c, "Play a game in the sim", "Every game you finish shows up here on its own.")
	case open:
		ui.Column(c.Key(shown)).Grow(1).Basis(0).MinHeight(0).Transition(kit.PageEnter).Children(func() { v.matches.MatchPage(c, &v.Env) })
	case len(v.Scoped().Matches) == 0:
		v.message(c, "No games match these filters", "Change or clear the filters above.")
	default:
		ui.Column(c.Key(shown)).Grow(1).Basis(0).MinHeight(0).Transition(kit.PageEnter).Children(func() { p.draw(v, c) })
	}
}

func (*view) PageLabel(id string) string { return pageLabel(id) }

func pageLabel(id string) string {
	p, _ := pageOf(id)
	return p.label
}

// sidebar is the pages as icons, and with their names when it's wider than the rail. The icons
// sit at the same place either way, so folding it only hides the names.
func (v *view) sidebar(c *ui.Context) {
	t := c.Theme()
	wide := v.Prefs.SideWidth > page.SideRail
	item := func(p pageDef) {
		id, label := p.id, p.label
		b := ui.ButtonBase(c.Key(id)).Label(label).Height(40).Radius(8).Background(ui.Transparent)
		col := t.TextMuted
		switch {
		case v.Page == id:
			b.Background(t.Surface)
			col = t.Text
		case b.Hovered():
			b.Background(t.SurfaceHover)
		}
		if b.Clicked() {
			if v.Page == id {
				v.matches.CloseMatch()
			}
			v.Page = id
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
	col := ui.Column(c).Fill().Background(kit.SidebarColor(t)).Padding(12, 0).Gap(4)
	if wide {
		col.Padding(12, 8)
	} else {
		col.AlignItems(ui.Center)
	}
	col.Children(func() {
		ui.Row(c).Height(36).Gap(10).Padding(0, 12).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, kit.IconCompass).Size(20, 20).TextColor(t.Accent)
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
		v.settings.UpdateBox(c, wide)
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

func (v *view) OpenMatch(m stats.Match) { v.matches.OpenMatch(&v.Env, m) }

func (v *view) GameMenu(m stats.Match) func(*ui.Menu) { return v.matches.GameMenu(&v.Env, m) }

func (v *view) CardMenu(card cards.Card) func(*ui.Menu) { return v.cards.CardMenu(&v.Env, card) }

func (v *view) showToast(c *ui.Context) {
	if t := v.Toast; t != nil {
		v.Toast = nil
		c.AddToast(*t)
	}
}

func (v *view) OpenInCards(code string) {
	s := &v.cards.State
	if s.Selected != code {
		s.Selected, s.Viewed, s.Versions = code, "", ""
	}
	s.Viewer = false
	v.Page = page.Cards
}

// ViewArt opens the art viewer over the page, which build draws off the Cards page.
func (v *view) ViewArt(code string) {
	s := &v.cards.State
	if s.Selected != code {
		s.Selected, s.Viewed, s.Versions = code, "", ""
	}
	s.Viewer = true
}

// ShowsHands reports whether the page is a Decks tab where "Show hands with" fits a card.
func (v *view) ShowsHands() bool {
	return v.Page == page.Decks && v.matches.Detail.Fingerprint == "" && v.decks.Tab != decks.TabResults
}

// ShowHands opens the Hands tab on the hands the card was dealt in.
func (v *view) ShowHands(code string) {
	s := &v.decks
	s.Tab, s.HandsCard, s.AllHands, s.Edit = decks.TabHands, code, false, nil
}
