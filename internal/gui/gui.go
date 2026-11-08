package gui

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/arts"
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
	// LibraryPath caches every card from Limitless and Bandai's ban list. Empty means none.
	LibraryPath string
	// ArtDir caches card images from Limitless and Bandai. Empty means only the sim's images.
	ArtDir string
	// BandaiPath caches the printings on Bandai's card list, and GridDir the pictures compared
	// to group them. Without both, the Art tab offers Limitless's printings alone.
	BandaiPath string
	GridDir    string
	// DBPath is the database file, which Settings shows.
	DBPath string
	// SimFlag is true when -sim named the sim's folder.
	SimFlag bool
	Log     *slog.Logger
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

	quitting   bool
	closeQuits atomic.Bool
	refreshMu  sync.Mutex

	simMu     sync.Mutex
	inst      *sim.Install
	stopWatch context.CancelFunc

	cardsMu sync.Mutex
	pack    *cards.DB
	cards   *cards.DB
	// fetched is when the cached tcgcsv catalog was fetched, zero without one.
	fetched time.Time
	// checked is when the card library was last refreshed, zero without one.
	checked time.Time
	// playing is set while the sim runs a game, when card data isn't fetched.
	playing atomic.Bool

	// web is the only owner of the art folder. Nil without one.
	web *webArt
	// gallery groups each card's printings from Limitless and Bandai into arts. Nil without an
	// art folder.
	gallery *arts.Gallery
	// catalogMu lets one card data refresh run at a time. stopCatalog cancels the one running.
	catalogMu   sync.Mutex
	stopCatalog atomic.Pointer[context.CancelFunc]

	simArtMu  sync.Mutex
	simArtObj *sim.SimArt
	// matched holds the cards whose own file was compared with their printings this run.
	matched map[string]bool
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
	a := &app{opts: opts, inst: opts.Install, web: newWebArt(opts.ArtDir)}
	a.gallery = openGallery(opts, a.web)
	if err := registerFonts(); err != nil {
		opts.Log.Warn("fonts", "err", err)
	}
	mygo.App.WhenReady(func() { a.ready(ctx) })
	return mygo.App.Run()
}

// bandaiPause is the wait between two pages of Bandai's card list. The first read is 61 pages.
const bandaiPause = 2 * time.Second

func openGallery(opts Options, web *webArt) *arts.Gallery {
	if web == nil || opts.BandaiPath == "" || opts.GridDir == "" {
		return nil
	}
	g, err := arts.Open(arts.Config{
		CatalogPath: opts.BandaiPath, GridDir: opts.GridDir, Pictures: web,
		Client: &http.Client{Timeout: 30 * time.Second}, Site: cards.BandaiSite, Pause: bandaiPause,
	})
	if err != nil {
		opts.Log.Warn("bandai card list", "err", err)
		return nil
	}
	return g
}

func (a *app) ready(ctx context.Context) {
	a.view = newView(nil)
	a.view.act = &appActions{a: a, ctx: ctx}
	// The window's look and close behavior are needed before it shows, so they're read here.
	set, err := a.opts.Store.Settings(ctx)
	if err != nil {
		a.opts.Log.Warn("settings", "err", err)
	}
	a.view.prefs = prefsFrom(set)
	a.view.prefs.openAtLogin = mygo.App.OpenAtLogin()
	if a.web != nil {
		a.web.setOff(a.view.prefs.artOff)
		a.web.setLimit(int64(a.view.prefs.artCap) << 20)
		go a.web.trim()
	}
	a.closeQuits.Store(a.view.prefs.closeQuits)
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
		Hidden:    a.view.prefs.startHidden && mygo.App.WasOpenedAtLogin(),
		Content:   ui.View(a.view.build),
	})
	a.view.art = newArtCache(a.install(), a.web, a.win.Update)
	a.win.OnClose(func(e *mygo.CloseEvent) {
		if a.quitting {
			return
		}
		if a.closeQuits.Load() {
			a.quitting = true
			mygo.App.Quit()
			return
		}
		e.PreventDefault()
		a.win.Hide()
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
	go func() { _ = a.refreshCatalog(ctx, false) }()
	go a.follow(ctx)
	go a.seek(ctx)
}

func (a *app) install() *sim.Install {
	a.simMu.Lock()
	defer a.simMu.Unlock()
	return a.inst
}

// setInstall switches to the sim in another folder: its card art, its card data and its logs.
// chosen is true when the user picked the folder.
func (a *app) setInstall(ctx context.Context, in sim.Install, chosen bool) error {
	st := a.opts.Store
	if err := st.SetSetting(ctx, store.SettingSimDir, in.Root); err != nil {
		return err
	}
	if chosen {
		if err := st.SetSetting(ctx, store.SettingSimDirChosen, in.Root); err != nil {
			return err
		}
	}
	a.simMu.Lock()
	a.inst = &in
	a.simMu.Unlock()
	a.cardsMu.Lock()
	a.pack, a.cards = nil, nil
	a.cardsMu.Unlock()
	a.win.Update(func() {
		a.view.art = newArtCache(&in, a.web, a.win.Update)
		a.view.resetSimArt()
	})
	a.watch(ctx)
	a.refresh(ctx)
	return nil
}

// seek looks for a running sim every few seconds while logpose has none.
func (a *app) seek(ctx context.Context) {
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()
	for {
		if a.install() != nil {
			return
		}
		if in, err := sim.Locate(); err == nil {
			if err := a.setInstall(ctx, in, false); err != nil {
				a.opts.Log.Warn("found sim", "err", err)
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
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
			was := a.playing.Swap(s.Phase == live.Playing)
			if !was && s.Phase == live.Playing {
				if stop := a.stopCatalog.Load(); stop != nil {
					(*stop)()
				}
			}
			if was && s.Phase != live.Playing {
				go func() { _ = a.refreshCatalog(ctx, false) }()
			}
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

// errRefreshing is what Settings' "Check now" says while another refresh runs.
var errRefreshing = errors.New("card data is already being checked")

// refreshCatalog fetches card data from tcgcsv.com and Limitless for sets newer than the
// cached copies, Bandai's ban list and Bandai's card list, then shows them. It does nothing
// while they're fresh or a game is running, unless forced, and a game that starts stops it.
// Without a network logpose keeps the cards it has.
func (a *app) refreshCatalog(ctx context.Context, force bool) error {
	if !a.catalogMu.TryLock() {
		return errRefreshing
	}
	defer a.catalogMu.Unlock()
	ctx, stop := context.WithCancel(ctx)
	a.stopCatalog.Store(&stop)
	defer func() {
		a.stopCatalog.Store(nil)
		stop()
	}()
	// Checked after the cancel is stored, so a game that starts in between still stops it.
	if !force && a.playing.Load() {
		return nil
	}
	client := &http.Client{Timeout: 30 * time.Second}
	changed := false
	err := errors.Join(a.refreshTCGCSV(ctx, client, force, &changed), a.refreshLibrary(ctx, client, force, &changed))
	if changed {
		a.cardsMu.Lock()
		a.cards = nil
		a.cardsMu.Unlock()
		a.refresh(ctx)
	}
	printings, berr := a.refreshBandai(ctx, force)
	if changed || printings {
		a.win.Update(a.view.resetArts)
	}
	return errors.Join(err, berr)
}

// refreshBandai reads Bandai's card list when it's stale. Its printings only offer pictures,
// so it isn't read while art downloads are off.
func (a *app) refreshBandai(ctx context.Context, force bool) (bool, error) {
	if a.gallery == nil || a.web.isOff() {
		return false, nil
	}
	now := time.Now()
	if !force && !a.gallery.Stale(now) {
		return false, nil
	}
	changed, err := a.gallery.Refresh(ctx, now, func(done, total int) {
		a.win.Update(func() { a.view.artSync = artSync{done, total} })
	})
	a.win.Update(func() { a.view.artSync = artSync{} })
	if err != nil {
		a.opts.Log.Warn("refresh bandai card list", "err", err)
		return changed, err
	}
	a.opts.Log.Info("bandai card list refreshed", "changed", changed)
	return changed, nil
}

func (a *app) refreshTCGCSV(ctx context.Context, client *http.Client, force bool, changed *bool) error {
	if a.opts.CatalogPath == "" {
		return nil
	}
	cat, err := cards.LoadCatalog(a.opts.CatalogPath)
	if err != nil {
		a.opts.Log.Warn("card catalog", "err", err)
		return err
	}
	if !force && !cat.Stale(time.Now(), catalogMaxAge) {
		return nil
	}
	n, err := cat.Refresh(ctx, client, cards.TCGCSVBase)
	if err != nil {
		a.opts.Log.Warn("refresh card catalog", "err", err, "sets", n)
		return err
	}
	if err := cat.Save(a.opts.CatalogPath); err != nil {
		a.opts.Log.Warn("save card catalog", "err", err)
		return err
	}
	a.opts.Log.Info("card catalog refreshed", "sets", n, "cards", len(cat.Cards))
	*changed = true
	return nil
}

// libraryPause is the wait between two Limitless pages. The first refresh reads about 60.
const libraryPause = 500 * time.Millisecond

func (a *app) refreshLibrary(ctx context.Context, client *http.Client, force bool, changed *bool) error {
	if a.opts.LibraryPath == "" {
		return nil
	}
	lib, err := cards.LoadLibrary(a.opts.LibraryPath)
	if err != nil {
		a.opts.Log.Warn("card library", "err", err)
		return err
	}
	if !force && !lib.Stale(time.Now(), catalogMaxAge) {
		return nil
	}
	src := cards.Source{Client: client, Limitless: cards.LimitlessBase, Rules: cards.BandaiRestrictions, Pause: libraryPause}
	src.Progress = func(p cards.SetProgress) { a.win.Update(func() { a.view.cardSync.add(p) }) }
	n, err := lib.Refresh(ctx, src, time.Now())
	a.win.Update(func() { a.view.cardSync = cardSync{} })
	if n > 0 || err == nil {
		if serr := lib.Save(a.opts.LibraryPath); serr != nil {
			a.opts.Log.Warn("save card library", "err", serr)
			return serr
		}
		*changed = true
	}
	if err != nil {
		a.opts.Log.Warn("refresh card library", "err", err, "sets", n)
		return err
	}
	a.opts.Log.Info("card library refreshed", "sets", n, "cards", len(lib.Cards))
	return nil
}

// start shows what's stored, then imports and watches the sim's logs.
func (a *app) start(ctx context.Context) {
	a.refresh(ctx)
	a.watch(ctx)
}

// watch imports and watches the current sim's logs, stopping the watcher of the sim before.
func (a *app) watch(ctx context.Context) {
	a.simMu.Lock()
	if a.stopWatch != nil {
		a.stopWatch()
	}
	in := a.inst
	ctx, a.stopWatch = context.WithCancel(ctx)
	a.simMu.Unlock()
	if in == nil {
		return
	}
	w := &importer.Watcher{
		Dir:   in.Logs(),
		Store: a.opts.Store,
		OnImport: func(r importer.Result) {
			a.opts.Log.Info("imported logs", "files", r.Files, "games", r.Games, "new", r.New, "reparsed", r.Reparsed)
			a.refresh(ctx)
		},
		OnError: func(err error) { a.opts.Log.Warn("import", "err", err) },
	}
	go func() {
		if err := w.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			a.opts.Log.Error("watcher stopped", "err", err)
		}
	}()
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
	me, err := st.LocalPlayers(ctx)
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
	set, err := st.Settings(ctx)
	if err != nil {
		return nil, err
	}
	info, err := a.settingsInfo(ctx, set, len(games), db.Len())
	if err != nil {
		return nil, err
	}
	now := time.Now()
	ms := stats.Matches(games, me...)
	return &snapshot{
		Loaded:   true,
		SimFound: a.install() != nil,
		Matches:  ms,
		Overview: stats.BuildOverview(ms, now),
		Cards:    db,
		Now:      now,
		Formats:  stats.Ranges(formats),
		Debuts:   stats.Debuts(ms, formats, splitList(set[store.SettingDismissedSets])),
		Settings: info,
	}, nil
}

// cardDB builds the card database: the catalog in the OPBounty pack the sim ships, the cached
// tcgcsv catalog over it, which has newer sets, and the Limitless library over both, which has
// everything. Names from the logs go on top in load. Without any, cards have the logs' names
// and no colors.
func (a *app) cardDB() *cards.DB {
	a.cardsMu.Lock()
	defer a.cardsMu.Unlock()
	if a.cards != nil {
		return a.cards
	}
	if a.pack == nil {
		a.pack = cards.NewDB()
		if in := a.install(); in != nil {
			if db, err := cards.LoadPack(in.OPBountyPack()); err == nil {
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
			a.fetched = cat.Fetched
		}
	}
	if a.opts.LibraryPath != "" {
		if lib, err := cards.LoadLibrary(a.opts.LibraryPath); err == nil {
			lib.Apply(db)
			a.checked = lib.Checked
		} else {
			a.opts.Log.Warn("card library", "err", err)
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
