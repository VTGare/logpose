package importer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/VTGare/logpose/internal/simfile"
	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/store"
)

type Result struct {
	Files int
	Games int
	New   int
	// Updated counts stored games replaced by a copy that reaches the game's end, such as the .log
	// the sim saves when a game it wrote to a .partial.log ends. Copies of a game still in
	// progress replace the stored one without counting, so the app doesn't rebuild every pass.
	Updated int
	// Reparsed counts stored games parsed again because the parser changed.
	Reparsed int
}

func (r *Result) add(o Result) {
	r.Files += o.Files
	r.Games += o.Games
	r.New += o.New
	r.Updated += o.Updated
	r.Reparsed += o.Reparsed
}

// Dir imports every combat log in the sim's AutoSaved folder, then parses again the games an
// older parser stored. It stops at the first error.
func Dir(ctx context.Context, st *store.Store, dir string) (Result, error) {
	w := Watcher{Dir: dir, Store: st}
	return w.pass(ctx, func(err error) error { return err })
}

func File(ctx context.Context, st *store.Store, path string) (Result, error) {
	f, err := simfile.Open(path)
	if err != nil {
		return Result{}, err
	}
	games, err := simlog.Parse(f)
	if err = errors.Join(err, f.Close()); err != nil {
		return Result{}, err
	}
	played, err := playedAt(path)
	if err != nil {
		return Result{}, err
	}
	r := Result{Files: 1, Games: len(games)}
	for i := range games {
		saved, err := st.SaveGame(ctx, &games[i], store.Source{File: filepath.Base(path), Index: i, PlayedAt: played})
		if err != nil {
			return r, err
		}
		switch saved {
		case store.Added:
			r.New++
		case store.Completed:
			r.Updated++
		}
	}
	return r, nil
}

// logTimeLayout is the start of the sim's file name, the time it began the file. Since 1.45a
// the time is UTC and has fractional seconds after it. Older names end at the seconds and are
// local time.
const logTimeLayout = "2006-01-02T15.04.05"

func playedAt(path string) (time.Time, error) {
	name := filepath.Base(path)
	if len(name) >= len(logTimeLayout) {
		zone := time.Local
		if rest := name[len(logTimeLayout):]; len(rest) > 1 && rest[0] == '.' && rest[1] >= '0' && rest[1] <= '9' {
			zone = time.UTC
		}
		if t, err := time.ParseInLocation(logTimeLayout, name[:len(logTimeLayout)], zone); err == nil {
			return t, nil
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
