// Package guitest hosts one page for its specs, with what the window's frame gives it.
package guitest

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/deck"
	"github.com/VTGare/logpose/internal/gui/kit"
	"github.com/VTGare/logpose/internal/gui/page"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

// Me is the local player in every snapshot.
const Me = "Me#1"

// Now is the time on the host's clock.
var Now = time.Date(2026, 11, 6, 20, 0, 0, 0, time.Local)

// Snapshot is n games with Luffy against Kaido, alternating wins and losses, newest first.
func Snapshot(n int) *page.Snapshot {
	var games []store.Game
	for i := range n {
		loser := Me
		if i%2 == 0 {
			loser = "Opp#2"
		}
		games = append(games, store.Game{
			Fingerprint: fmt.Sprint(i),
			PlayedAt:    Now.Add(-time.Duration(i+1) * time.Hour),
			Turns:       12,
			First:       Me,
			Lobby:       simlog.Lobby{Code: fmt.Sprintf("XNJ8M%02d", i)},
			End:         simlog.End{Kind: simlog.EndConcede, Loser: loser},
			Players: []store.GamePlayer{
				{ID: Me, Name: "Me", Leader: "OP13-001"},
				{ID: "Opp#2", Name: "Opp", Leader: "OP17-058"},
			},
		})
	}
	db := cards.NewDB()
	db.Name("OP13-001", "Monkey D. Luffy")
	db.Name("OP17-058", "Kaido")
	ms := stats.Matches(games, Me)
	return &page.Snapshot{Loaded: true, SimFound: true, Matches: ms, Overview: stats.BuildOverview(ms, Now), Cards: db}
}

// DeckSnapshot is Snapshot's games with two lists. The even games, all wins, run W-1 and
// search with Garp and Otama. The odd ones, all losses, search with Garp, Kid & Killer and Luffy.
func DeckSnapshot(builds ...store.Build) *page.Snapshot {
	s := Snapshot(10)
	for code, name := range map[string]string{"GARP": "Garp", "OTA": "Otama", "KID": "Kid & Killer", "LUF": "Luffy"} {
		s.Cards.Name(code, name)
	}
	facts := make(map[string]deck.Facts)
	for i := range 10 {
		list := map[string]int{"GARP": 4, "OTA": 4, "W-1": 4}
		searchers := []string{"GARP", "OTA"}
		if i%2 == 1 {
			list = map[string]int{"GARP": 4, "KID": 4, "LUF": 4}
			searchers = []string{"GARP", "KID", "LUF"}
		}
		var searches []deck.Search
		for _, by := range searchers {
			searches = append(searches, deck.Search{Source: by, Taken: "X-1"})
		}
		facts[fmt.Sprint(i)] = deck.Facts{Player: Me, List: list, Searches: searches}
	}
	s.Decks = make(map[string]stats.DeckGame)
	for _, g := range stats.DeckGames(s.Matches, facts) {
		s.Decks[g.Fingerprint] = g
	}
	s.Builds = builds
	return s
}

// HandsSnapshot is DeckSnapshot's games, all going first. The even ones, all wins, keep a hand
// with Garp and Otama. The odd ones, all losses, are dealt Shanks with Garp and mulligan.
func HandsSnapshot() *page.Snapshot {
	s := DeckSnapshot()
	s.Cards.Name("SHK", "Shanks")
	for i := range s.Matches {
		m := &s.Matches[i]
		if i%2 == 0 {
			m.Me.HandBefore = []string{"GARP", "OTA", "X-1", "X-2", "X-3"}
		} else {
			m.Me.Mulliganed = true
			m.Me.HandBefore = []string{"SHK", "GARP", "X-1", "X-2", "X-3"}
			m.Me.HandAfter = []string{"OTA", "X-1", "X-2", "X-3", "X-4"}
			m.Opp.Mulliganed = true
		}
	}
	return s
}

// Host stands in for the window around one page.
type Host struct {
	page.Env
	// Bar reports whether the filter bar shows over the page. Nil shows none.
	Bar func() bool
	// Shown names what the frame shows, for the page's first-frame motion. Nil names the page.
	Shown func() string
	// Before runs at the start of each frame, for what a page asks of the frame there.
	Before func(c *ui.Context)
	// Full reports whether the page takes the whole window, with no sidebar. Nil means never.
	Full func() bool
	// Draw is the page.
	Draw func(c *ui.Context, e *page.Env)

	// The Nav calls go to these when set.
	MatchOpener func(m stats.Match)
	CardsOpener func(code string)
	ArtViewer   func(code string)
	GameMenus   func(m stats.Match) func(*ui.Menu)
	CardMenus   func(card cards.Card) func(*ui.Menu)
	HandsShown  func() bool
	HandsOpener func(code string)
}

// New is a host showing draw in a window as large as the specs' window, with the space the
// sidebar takes left empty.
func New(s *page.Snapshot, draw func(c *ui.Context, e *page.Env)) (*Host, *ui.Tester) {
	return Start(&Host{}, s, draw)
}

// Start is New with a host the spec set up first. The tester draws its first frame at once,
// so what that frame needs has to be on the host already.
func Start(h *Host, s *page.Snapshot, draw func(c *ui.Context, e *page.Env)) (*Host, *ui.Tester) {
	h.Env, h.Draw = page.Env{Snap: s, Clock: func() time.Time { return Now }, Page: page.Overview}, draw
	h.Nav = h
	h.SetPrefs(page.PrefsFrom(nil))
	// A Tester's clock stands still. A transition would never end, and every settle would run
	// all 20 frames mygo allows. DisableAnimations shows each frame's changes at once.
	tt := ui.NewTester(func(c *ui.Context) {
		c.DisableAnimations()
		h.build(c)
	}, 1180, 780)
	// The specs check what pages show, and frames of motion only slow them down.
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	return h, tt
}

// WithBar is New with the filter bar over the page while there are games.
func WithBar(s *page.Snapshot, draw func(c *ui.Context, e *page.Env)) (*Host, *ui.Tester) {
	return Start(&Host{Bar: func() bool { return true }}, s, draw)
}

// SetPrefs takes the prefs as the window does, starting the filters on the format they name.
func (h *Host) SetPrefs(p page.Prefs) {
	h.Prefs = p
	h.Filter.Format = p.StartFormat
}

func (h *Host) build(c *ui.Context) {
	if h.Before != nil {
		h.Before(c)
	}
	h.ApplyLook(c)
	h.ShowToast(c)
	kit.Toasts(c)
	shown := h.Page
	if h.Shown != nil {
		shown = h.Shown()
	}
	h.BeginFrame(shown)
	ui.Row(c).Fill().AlignItems(ui.Stretch).FontFeatures("tnum").Children(func() {
		body := func() {
			ui.Column(c).Grow(1).Fill().Children(func() {
				if h.Bar != nil && h.Bar() && h.Snap.Loaded && len(h.Snap.Matches) > 0 {
					h.FilterBar(c)
				}
				ui.Column(c.Key(shown)).Grow(1).Basis(0).MinHeight(0).Children(func() { h.Draw(c, &h.Env) })
			})
		}
		if h.Full != nil && h.Full() {
			body()
			return
		}
		size := h.Prefs.SideWidth
		ui.Split(c, &size, func() { h.sidebar(c) }, body).Fill()
	})
}

func (h *Host) OpenMatch(m stats.Match) {
	if h.MatchOpener != nil {
		h.MatchOpener(m)
	}
}

func (h *Host) OpenInCards(code string) {
	if h.CardsOpener != nil {
		h.CardsOpener(code)
	}
}

func (h *Host) ViewArt(code string) {
	if h.ArtViewer != nil {
		h.ArtViewer(code)
	}
}

func (h *Host) GameMenu(m stats.Match) func(*ui.Menu) {
	if h.GameMenus != nil {
		return h.GameMenus(m)
	}
	return func(*ui.Menu) {}
}

func (h *Host) CardMenu(card cards.Card) func(*ui.Menu) {
	if h.CardMenus != nil {
		return h.CardMenus(card)
	}
	return func(*ui.Menu) {}
}

func (h *Host) ShowsHands() bool { return h.HandsShown != nil && h.HandsShown() }

func (h *Host) ShowHands(code string) {
	if h.HandsOpener != nil {
		h.HandsOpener(code)
	}
}

var pageLabels = []struct{ id, label string }{
	{page.Overview, "Overview"},
	{page.Matches, "Matches"},
	{page.Decks, "Decks"},
	{page.Matchups, "Matchups"},
	{page.Cards, "Cards"},
	{page.Formats, "Formats"},
	{page.Settings, "Settings"},
}

// sidebar is a button per page, named as the window's sidebar names them. Picking one changes
// the page's id. The host goes on drawing its own page.
func (h *Host) sidebar(c *ui.Context) {
	ui.Column(c).Children(func() {
		for _, p := range pageLabels {
			if ui.Button(c, p.label).Clicked() {
				h.Page = p.id
			}
		}
	})
}

func (*Host) PageLabel(id string) string {
	for _, p := range pageLabels {
		if p.id == id {
			return p.label
		}
	}
	return ""
}

// EnvActions records what the env asks the app for. A page's fake embeds it.
type EnvActions struct {
	Calls   []string
	Applied []page.Prefs
	timers  []func()
}

func (f *EnvActions) SetSetting(key, value string) { f.Calls = append(f.Calls, key+"="+value) }

func (f *EnvActions) ApplyPrefs(p page.Prefs) { f.Applied = append(f.Applied, p) }

// After keeps the timer for a spec to fire with Fire.
func (f *EnvActions) After(_ time.Duration, fn func()) { f.timers = append(f.timers, fn) }

// Timers is how many timers wait to fire.
func (f *EnvActions) Timers() int { return len(f.timers) }

func (f *EnvActions) Fire() {
	timers := f.timers
	f.timers = nil
	for _, fn := range timers {
		fn()
	}
}

func Click(tt *ui.Tester, label string) {
	ginkgo.GinkgoHelper()
	gomega.Expect(tt.Click(label)).To(gomega.Succeed())
	tt.Frame()
}

// Choose clicks each label in turn, as when walking a menu.
func Choose(tt *ui.Tester, path ...string) error {
	for _, label := range path {
		tt.Frame()
		if err := tt.Click(label); err != nil {
			return err
		}
	}
	tt.Frame()
	return nil
}

func Shows(tt *ui.Tester, items ...string) {
	ginkgo.GinkgoHelper()
	tt.Frame()
	for _, item := range items {
		gomega.Expect(tt.HasText(item)).To(gomega.BeTrue(), item)
	}
}

func Labelled(tt *ui.Tester, label string) bool {
	tt.Frame()
	_, ok := tt.Find(label)
	return ok
}

// Hover rests the pointer on the text for a second.
func Hover(tt *ui.Tester, text string) {
	ginkgo.GinkgoHelper()
	x, y := Center(tt, text)
	tt.Move(x, y)
	tt.Advance(time.Second)
	tt.Frame()
}

// Retype replaces what the labelled input holds.
func Retype(tt *ui.Tester, label, text string) {
	ginkgo.GinkgoHelper()
	gomega.Expect(tt.Click(label)).To(gomega.Succeed())
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	tt.Type(text)
	tt.Frame()
}

func Center(tt *ui.Tester, label string) (x, y float32) {
	ginkgo.GinkgoHelper()
	r, ok := tt.Find(label)
	gomega.Expect(ok).To(gomega.BeTrue(), label)
	return r.X + r.W/2, r.Y + r.H/2
}

// Drag presses at the first point and lets go at the second, moving in eight steps.
func Drag(tt *ui.Tester, x0, y0, x1, y1 float32) {
	tt.Press(x0, y0)
	tt.Frame()
	for i := 1; i <= 8; i++ {
		f := float32(i) / 8
		tt.Move(x0+(x1-x0)*f, y0+(y1-y0)*f)
		tt.Frame()
	}
	tt.Release(x1, y1)
	tt.Frame()
}
