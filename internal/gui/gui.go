package gui

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/importer"
	"github.com/VTGare/logpose/internal/live"
	"github.com/VTGare/logpose/internal/replay"
	"github.com/VTGare/logpose/internal/sim"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

type Options struct {
	Store *store.Store
	// Install is the sim, or nil when it wasn't found. Without it logpose shows the games it
	// already stored, with no card art and no new imports.
	Install *sim.Install
	// CatalogPath caches the card catalog from tcgcsv.com. Empty means no refresh, and only
	// the catalog the sim's OPBounty pack carries.
	CatalogPath string
	Log         *slog.Logger
	// Dev marks a test copy in the window title.
	Dev bool
}

//go:embed assets/tray.png
var trayIcon []byte

// catalogMaxAge is how often the card catalog is refreshed. tcgcsv updates once a day.
const catalogMaxAge = 24 * time.Hour

type app struct {
	opts Options
	win  *mygo.Window
	tray *mygo.Tray
	view *view

	quitting  bool
	refreshMu sync.Mutex

	cardsMu sync.Mutex
	pack    *cards.DB
	cards   *cards.DB
}

// Claim makes this the only running logpose. It returns false when another one is running,
// which then shows its window, and this one should exit. A dev copy has its own name, so it
// runs next to the real one.
func Claim(dev bool) bool {
	name := "logpose"
	if dev {
		name = "logpose-dev"
	}
	mygo.App.SetName(name)
	return mygo.App.RequestSingleInstanceLock()
}

// Run shows the window and the tray icon until the user quits. Call Claim first.
func Run(ctx context.Context, opts Options) error {
	if opts.Log == nil {
		opts.Log = slog.Default()
	}
	a := &app{opts: opts}
	if err := registerFonts(); err != nil {
		opts.Log.Warn("fonts", "err", err)
	}
	mygo.App.WhenReady(func() { a.ready(ctx) })
	return mygo.App.Run()
}

func (a *app) ready(ctx context.Context) {
	a.view = newView(nil)
	a.view.act = &appActions{a: a, ctx: ctx}
	title := "logpose"
	if a.opts.Dev {
		title = "logpose (dev)"
	}
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:     title,
		Width:     1180,
		Height:    780,
		MinWidth:  900,
		MinHeight: 600,
		Content:   ui.View(a.view.build),
	})
	a.view.art = newArtCache(a.opts.Install, a.win.Update)
	a.win.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
			a.win.Hide()
		}
	})
	a.setUpTray()
	mygo.App.OnSecondInstance(func([]string, string) { a.show() })

	ctx, cancel := context.WithCancel(ctx)
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		cancel()
		if a.tray != nil {
			a.tray.Destroy()
		}
	})
	go a.start(ctx)
	go a.refreshCatalog(ctx)
	go a.follow(ctx)
}

// follow shows the game running in the sim, from its Player.log.
func (a *app) follow(ctx context.Context) {
	path, err := sim.PlayerLog()
	if err != nil {
		a.opts.Log.Warn("player log", "err", err)
		return
	}
	f := &live.Follower{
		Path:    path,
		Running: sim.Running,
		OnChange: func(s live.State) {
			a.win.Update(func() { a.view.live = s })
		},
		OnSaved: func(sv live.Saved) {
			if sv.Format == "" {
				return
			}
			if err := a.opts.Store.SetLogFormat(ctx, sv.File, sim.FormatName(sv.SimVersion, sv.Format)); err != nil {
				a.opts.Log.Warn("log format", "err", err)
				return
			}
			a.refresh(ctx)
		},
	}
	_ = f.Run(ctx)
}

// refreshCatalog fetches card data for sets newer than the cached catalog, then shows it.
// Without a network logpose keeps the cards it has.
func (a *app) refreshCatalog(ctx context.Context) {
	if a.opts.CatalogPath == "" {
		return
	}
	cat, err := cards.LoadCatalog(a.opts.CatalogPath)
	if err != nil {
		a.opts.Log.Warn("card catalog", "err", err)
		return
	}
	if !cat.Stale(time.Now(), catalogMaxAge) {
		return
	}
	client := &http.Client{Timeout: 30 * time.Second}
	n, err := cat.Refresh(ctx, client, cards.TCGCSVBase)
	if err != nil {
		a.opts.Log.Warn("refresh card catalog", "err", err, "sets", n)
		return
	}
	if err := cat.Save(a.opts.CatalogPath); err != nil {
		a.opts.Log.Warn("save card catalog", "err", err)
		return
	}
	a.opts.Log.Info("card catalog refreshed", "sets", n, "cards", len(cat.Cards))
	a.cardsMu.Lock()
	a.cards = nil
	a.cardsMu.Unlock()
	a.refresh(ctx)
}

// start shows what's stored, then imports and watches the sim's logs.
func (a *app) start(ctx context.Context) {
	a.refresh(ctx)
	if a.opts.Install == nil {
		return
	}
	w := &importer.Watcher{
		Dir:   a.opts.Install.Logs(),
		Store: a.opts.Store,
		OnImport: func(r importer.Result) {
			a.opts.Log.Info("imported logs", "files", r.Files, "games", r.Games, "new", r.New, "reparsed", r.Reparsed)
			a.refresh(ctx)
		},
		OnError: func(err error) { a.opts.Log.Warn("import", "err", err) },
	}
	if err := w.Run(ctx); err != nil {
		a.opts.Log.Error("watcher stopped", "err", err)
	}
}

// refresh builds a new snapshot and shows it. The watcher, the log follower, the catalog refresh
// and the view's actions all call it, so builds run one at a time and the newest one shows last.
func (a *app) refresh(ctx context.Context) {
	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()
	snap, err := a.load(ctx)
	if err != nil {
		a.opts.Log.Error("load games", "err", err)
		return
	}
	a.win.Update(func() {
		a.view.snap = snap
		a.updateTray(snap)
	})
}

func (a *app) load(ctx context.Context) (*snapshot, error) {
	st := a.opts.Store
	me, err := st.LocalPlayer(ctx)
	if err != nil {
		return nil, err
	}
	games, err := st.Games(ctx)
	if err != nil {
		return nil, err
	}
	names, err := st.CardNames(ctx)
	if err != nil {
		return nil, err
	}
	// Each snapshot gets its own copy, since the window reads the last one while this one fills.
	db := a.cardDB().Clone()
	for code, name := range names {
		db.Name(code, name)
	}
	formats, err := st.Formats(ctx)
	if err != nil {
		return nil, err
	}
	dismissed, err := st.Setting(ctx, store.SettingDismissedSets)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	ms := stats.Matches(games, me)
	return &snapshot{
		Loaded:   true,
		SimFound: a.opts.Install != nil,
		Me:       me,
		Matches:  ms,
		Overview: stats.BuildOverview(ms, now),
		Cards:    db,
		Now:      now,
		Formats:  stats.Ranges(formats),
		Debuts:   stats.Debuts(ms, formats, splitList(dismissed)),
	}, nil
}

// cardDB builds the card database: the catalog in the OPBounty pack the sim ships, then the
// cached tcgcsv catalog over it, which has newer sets. Names from the logs go on top in load.
// Without either, cards have the logs' names and no colors.
func (a *app) cardDB() *cards.DB {
	a.cardsMu.Lock()
	defer a.cardsMu.Unlock()
	if a.cards != nil {
		return a.cards
	}
	if a.pack == nil {
		a.pack = cards.NewDB()
		if a.opts.Install != nil {
			if db, err := cards.LoadPack(a.opts.Install.OPBountyPack()); err == nil {
				a.pack = db
			} else {
				a.opts.Log.Warn("card database", "err", err)
			}
		}
	}
	db := a.pack.Clone()
	if a.opts.CatalogPath != "" {
		if cat, err := cards.LoadCatalog(a.opts.CatalogPath); err == nil {
			cat.Apply(db)
		}
	}
	a.cards = db
	return db
}

func (a *app) setUpTray() {
	tray, err := mygo.NewTray(mygo.TrayOptions{Icon: trayIcon, ToolTip: "logpose"})
	if err != nil {
		a.opts.Log.Warn("tray", "err", err)
		return
	}
	a.tray = tray
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Open logpose", Click: func(*mygo.MenuItem, *mygo.Window) { a.show() }},
		mygo.Separator(),
		{Label: "Quit", Click: func(*mygo.MenuItem, *mygo.Window) { a.quit() }},
	})
	tray.OnClick(func() {
		if a.win.IsVisible() {
			a.win.Hide()
		} else {
			a.show()
		}
	})
	tray.OnRightClick(func() {
		tray.SetMenu(menu)
		defer tray.SetMenu(nil)
		tray.PopUpMenu()
	})
}

func (a *app) updateTray(s *snapshot) {
	if a.tray == nil {
		return
	}
	tip := "logpose"
	if len(s.Overview.Today) > 0 {
		tip = fmt.Sprintf("logpose · today %s", record(s.Overview.TodayRecord))
	}
	a.tray.SetToolTip(tip)
}

func (a *app) show() {
	a.win.Show()
	a.win.Focus()
}

func (a *app) quit() {
	a.quitting = true
	mygo.App.Quit()
}

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// appActions makes the view's changes on a goroutine, then shows the result.
type appActions struct {
	a   *app
	ctx context.Context
}

func (x *appActions) run(what string, fn func(ctx context.Context, st *store.Store) error) {
	go func() {
		if err := fn(x.ctx, x.a.opts.Store); err != nil {
			x.a.opts.Log.Error(what, "err", err)
		}
		x.a.refresh(x.ctx)
	}()
}

func (x *appActions) AddFormat(name string, start time.Time) {
	x.run("add format", func(ctx context.Context, st *store.Store) error {
		_, err := st.AddFormat(ctx, name, start)
		return err
	})
}

func (x *appActions) UpdateFormat(f store.Format) {
	x.run("update format", func(ctx context.Context, st *store.Store) error { return st.UpdateFormat(ctx, f) })
}

func (x *appActions) DeleteFormat(id int64) {
	x.run("delete format", func(ctx context.Context, st *store.Store) error { return st.DeleteFormat(ctx, id) })
}

func (x *appActions) DismissSet(set string) {
	x.run("dismiss set", func(ctx context.Context, st *store.Store) error {
		cur, err := st.Setting(ctx, store.SettingDismissedSets)
		if err != nil {
			return err
		}
		return st.SetSetting(ctx, store.SettingDismissedSets, strings.Join(append(splitList(cur), set), ","))
	})
}

func (x *appActions) SetGameType(fingerprint string, t store.GameType) {
	x.run("set game type", func(ctx context.Context, st *store.Store) error { return st.SetGameType(ctx, fingerprint, t) })
}

func (x *appActions) SetGameNote(fingerprint, note string) {
	x.run("set game note", func(ctx context.Context, st *store.Store) error { return st.SetGameNote(ctx, fingerprint, note) })
}

func (x *appActions) SetBookmark(fingerprint string, on bool) {
	x.run("set bookmark", func(ctx context.Context, st *store.Store) error { return st.SetBookmark(ctx, fingerprint, on) })
}

func (x *appActions) LoadTimeline(fingerprint, me, opp string) {
	go func() {
		l := matchTimeline{Fingerprint: fingerprint}
		if g, err := x.parseGame(fingerprint); err != nil {
			x.a.opts.Log.Warn("read match log", "err", err)
		} else {
			l.Me, l.Opp = g.LifeByTurn(me), g.LifeByTurn(opp)
			l.MeHand, l.OppHand = g.HandByTurn(me), g.HandByTurn(opp)
			l.MeBoard, l.OppBoard = g.CharactersByTurn(me), g.CharactersByTurn(opp)
			db := x.a.cardDB()
			l.Moments = g.Moments(func(code string) int { return db.Card(code).Power })
			l.Marks = g.Marks()
			l.Replayable = g.HasRZ()
		}
		x.a.win.Update(func() { x.a.view.setTimeline(l) })
	}()
}

// OpenReplay maximizes the window, since the board needs the room, and builds the replay.
func (x *appActions) OpenReplay(fingerprint string) {
	x.a.win.Maximize()
	go func() {
		l := loadedReplay{Fingerprint: fingerprint}
		l.Game, l.Err = x.parseGame(fingerprint)
		if l.Err == nil {
			db := x.a.cardDB()
			l.Replay, l.Err = replay.Build(l.Game, func(code string) int { return db.Card(code).Power })
		}
		x.a.win.Update(func() { x.a.view.setReplay(l) })
	}()
}

func (x *appActions) After(d time.Duration, fn func()) {
	time.AfterFunc(d, func() { x.a.win.Update(fn) })
}

func (x *appActions) parseGame(fingerprint string) (*simlog.Game, error) {
	text, err := x.a.opts.Store.GameLog(x.ctx, fingerprint)
	if err != nil {
		return nil, err
	}
	games, err := simlog.Parse(strings.NewReader(text))
	if err != nil {
		return nil, err
	}
	if len(games) != 1 {
		return nil, fmt.Errorf("log parses to %d games", len(games))
	}
	return &games[0], nil
}
