package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/VTGare/logpose/internal/importer"
	"github.com/VTGare/logpose/internal/store"
)

var version = "dev"

const usage = `usage: logpose <command> [flags]

commands:
  import -dir <CombatLogs/AutoSaved>   copy the sim's combat logs into logpose
  games                                list stored games, newest first
  version                              print the version`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	switch os.Args[1] {
	case "import":
		err = runImport(ctx, os.Args[2:])
	case "games":
		err = runGames(ctx, os.Args[2:])
	case "version":
		fmt.Println("logpose", version)
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

func openStore(ctx context.Context, path string) (*store.Store, error) {
	if path == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(dir, "logpose", "logpose.db")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return store.Open(ctx, path)
}

func runImport(ctx context.Context, args []string) (err error) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	db := dbFlag(fs)
	dir := fs.String("dir", "", "the sim's CombatLogs/AutoSaved folder")
	_ = fs.Parse(args)
	if *dir == "" {
		return errors.New("import needs -dir")
	}
	st, err := openStore(ctx, *db)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, st.Close()) }()

	start := time.Now()
	r, err := importer.Dir(ctx, st, *dir)
	if err != nil {
		return err
	}
	reparsed, err := st.Reparse(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%d files, %d games, %d new, %d parsed again, in %s\n",
		r.Files, r.Games, r.New, reparsed, time.Since(start).Round(time.Millisecond))
	return nil
}

func runGames(ctx context.Context, args []string) (err error) {
	fs := flag.NewFlagSet("games", flag.ExitOnError)
	db := dbFlag(fs)
	_ = fs.Parse(args)
	st, err := openStore(ctx, *db)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, st.Close()) }()

	me, err := st.LocalPlayer(ctx)
	if err != nil {
		return err
	}
	games, err := st.Games(ctx)
	if err != nil {
		return err
	}
	wins := 0
	for _, g := range games {
		mine, _ := g.Player(me)
		var theirs store.GamePlayer
		for _, p := range g.Players {
			if p.ID != me {
				theirs = p
			}
		}
		result := "W"
		if g.End.Loser == me {
			result = "L"
		} else {
			wins++
		}
		order := "2nd"
		if g.First == me {
			order = "1st"
		}
		kind := "casual"
		if g.Lobby.OPBounty() {
			kind = "ranked"
		}
		fmt.Printf("%s  %s  %-8s vs %-8s %-22s %s  T%-2d %-8s %s\n", g.PlayedAt.Format("2006-01-02 15:04"), result,
			mine.Leader, theirs.Leader, theirs.ID, order, g.Turns, g.End.Kind, kind)
	}
	if len(games) > 0 {
		fmt.Printf("%d games as %s, %d–%d\n", len(games), me, wins, len(games)-wins)
	}
	return nil
}
