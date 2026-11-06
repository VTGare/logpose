package gui

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/VTGare/logpose/internal/cards"
	"github.com/VTGare/logpose/internal/importer"
	"github.com/VTGare/logpose/internal/sim"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

type Options struct {
	Store *store.Store
	// Install is the sim, or nil when it wasn't found. Without it logpose shows the games it
	// already stored, with no card art and no new imports.
	Install *sim.Install
	Log     *slog.Logger
}

type app struct {
	opts Options
	win  *mygo.Window
	tray *mygo.Tray
	view *view

	quitting bool

	cardsOnce sync.Once
	cards     *cards.DB
}

// Run shows the window and the tray icon until the user quits.
func Run(ctx context.Context, opts Options) error {
	if opts.Log == nil {
		opts.Log = slog.Default()
	}
	a := &app{opts: opts}
	mygo.App.WhenReady(func() { a.ready(ctx) })
	return mygo.App.Run()
}

func (a *app) ready(ctx context.Context) {
	a.view = newView(nil)
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:     "logpose",
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

	ctx, cancel := context.WithCancel(ctx)
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		cancel()
		if a.tray != nil {
			a.tray.Destroy()
		}
	})
	go a.start(ctx)
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
			a.opts.Log.Info("imported logs", "files", r.Files, "games", r.Games, "new", r.New)
			a.refresh(ctx)
		},
		OnError: func(err error) { a.opts.Log.Warn("import", "err", err) },
	}
	if err := w.Run(ctx); err != nil {
		a.opts.Log.Error("watcher stopped", "err", err)
	}
}

func (a *app) refresh(ctx context.Context) {
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
	db := a.cardDB()
	for code, name := range names {
		db.Name(code, name)
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
	}, nil
}

// cardDB loads the card database from the OPBounty client the sim ships, once. Without it,
// cards have the names the logs give them and no colors.
func (a *app) cardDB() *cards.DB {
	a.cardsOnce.Do(func() {
		a.cards = cards.NewDB()
		if a.opts.Install == nil {
			return
		}
		db, err := cards.LoadPack(a.opts.Install.OPBountyPack())
		if err != nil {
			a.opts.Log.Warn("card database", "err", err)
			return
		}
		a.cards = db
	})
	return a.cards
}

func (a *app) setUpTray() {
	tray, err := mygo.NewTray(mygo.TrayOptions{Icon: trayIcon(), ToolTip: "logpose"})
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

// trayIcon draws the compass mark.
func trayIcon() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	center := float64(size-1) / 2
	ring := color.RGBA{55, 138, 221, 255}
	needle := color.RGBA{226, 75, 74, 255}
	for y := range size {
		for x := range size {
			dx, dy := float64(x)-center, float64(y)-center
			d := math.Hypot(dx, dy)
			switch {
			case d > 12.5 && d < 15.5:
				img.Set(x, y, ring)
			case math.Abs(dx+dy) < 2.6 && d < 10:
				img.Set(x, y, needle)
			}
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}
