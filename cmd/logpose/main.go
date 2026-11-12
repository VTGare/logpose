package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/gui"
	"github.com/VTGare/logpose/internal/importer"
	"github.com/VTGare/logpose/internal/stats"
	"github.com/VTGare/logpose/internal/store"
)

const usage = `usage: logpose [-db <file>] [-sim <folder>]   open the app
       logpose <command> [flags]

commands:
  import -dir <CombatLogs/AutoSaved>   copy the sim's combat logs into logpose
  games                                list stored games, newest first
  version                              print the version`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], "-") {
		err = runApp(ctx, os.Args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "logpose:", err)
			os.Exit(1)
		}
		return
	}
	switch os.Args[1] {
	case "import":
		err = runImport(ctx, os.Args[2:])
	case "games":
		err = runGames(ctx, os.Args[2:])
	case "version":
		fmt.Println("logpose", gui.Version())
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "logpose:", err)
		os.Exit(1)
	}
}

func dbFlag(fs *flag.FlagSet) *string {
	return fs.String("db", "", "database file (default: logpose.db in the user config folder)")
}

// dbPath falls back to the working folder when Windows has no user config folder.
func dbPath(path string) string {
	if path != "" {
		return path
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "logpose.db"
	}
	return filepath.Join(dir, "logpose", "logpose.db")
}

func openStore(ctx context.Context, path string) (*store.Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return store.Open(ctx, path)
}

func runApp(ctx context.Context, args []string) (err error) {
	fs := flag.NewFlagSet("logpose", flag.ExitOnError)
	db := dbFlag(fs)
	simDir := fs.String("sim", "", "OPTCGSim's folder, the one with OPTCGSim.exe (default: found from the running sim)")
	dev := fs.Bool("dev", false, "run as a test copy next to the real logpose, with -db pointing at a scratch database")
	page := fs.String("page", "", "the page to open on, for screenshots of a test copy: "+strings.Join(gui.Pages(), ", "))
	_ = fs.Parse(args)
	if *page != "" && !gui.IsPage(*page) {
		return fmt.Errorf("-page: no page %q", *page)
	}
	if !gui.Claim(*dev) {
		return nil
	}

	path := dbPath(*db)
	st, err := openStore(ctx, path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, st.Close()) }()

	// Everything logpose keeps goes next to the database, so a test copy run with -db leaves the
	// installed app's files alone.
	dir := filepath.Dir(path)
	opts := gui.Options{
		Store: st, Log: logger(dir), Dev: *dev, Page: *page,
		DBPath:      path,
		CatalogPath: filepath.Join(dir, "cards.json"),
		LibraryPath: filepath.Join(dir, "library.json"),
		ArtDir:      filepath.Join(dir, "art"),
		BandaiPath:  filepath.Join(dir, "bandai.json"),
		SimArtDir:   filepath.Join(dir, "sim-art"),
		SimFlag:     *simDir != "",
	}
	if in, err := gui.LocateSim(ctx, st, *simDir); err == nil {
		opts.Install = &in
	} else {
		opts.Log.Warn("sim not found", "err", err)
	}
	return gui.Run(ctx, opts)
}

// logger writes to logpose.log in dir, since the Windows build has no console.
func logger(dir string) *slog.Logger {
	f, err := os.OpenFile(filepath.Join(dir, "logpose.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return slog.Default()
	}
	return slog.New(slog.NewTextHandler(f, nil))
}

func runImport(ctx context.Context, args []string) (err error) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	db := dbFlag(fs)
	dir := fs.String("dir", "", "the sim's CombatLogs/AutoSaved folder")
	_ = fs.Parse(args)
	if *dir == "" {
		return errors.New("import needs -dir")
	}
	st, err := openStore(ctx, dbPath(*db))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, st.Close()) }()

	start := time.Now()
	r, err := importer.Dir(ctx, st, *dir)
	if err != nil {
		return err
	}
	fmt.Printf("%d files, %d games, %d new, %d parsed again, in %s\n",
		r.Files, r.Games, r.New, r.Reparsed, time.Since(start).Round(time.Millisecond))
	return nil
}

func runGames(ctx context.Context, args []string) (err error) {
	fs := flag.NewFlagSet("games", flag.ExitOnError)
	db := dbFlag(fs)
	_ = fs.Parse(args)
	st, err := openStore(ctx, dbPath(*db))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, st.Close()) }()

	me, err := st.LocalPlayers(ctx)
	if err != nil {
		return err
	}
	games, err := st.Games(ctx)
	if err != nil {
		return err
	}
	ms := stats.Matches(games, me...)
	for _, m := range ms {
		result := "?"
		switch m.Result {
		case stats.Win:
			result = "W"
		case stats.Loss:
			result = "L"
		}
		order := "?"
		if m.TurnOrderKnown {
			order = "2nd"
			if m.WentFirst {
				order = "1st"
			}
		}
		kind := "casual"
		if m.Ranked {
			kind = "ranked"
		}
		fmt.Printf("%s  %s  %-8s vs %-8s %-22s %-3s  T%-2d %-8s %s\n", m.PlayedAt.Format("2006-01-02 15:04"), result,
			m.Me.Leader, m.Opp.Leader, m.Opp.ID, order, m.Turns, m.End.Kind, kind)
	}
	if len(ms) > 0 {
		r := stats.RecordOf(ms)
		fmt.Printf("%d games as %s, %d–%d\n", len(ms), strings.Join(me, ", "), r.Wins, r.Losses)
	}
	return nil
}
