package importer

import (
	"context"
	"errors"
	"fmt"
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

type fileState struct {
	size int64
	mod  time.Time
}

// Watcher imports the sim's logs as they're saved. It polls the folder rather than subscribing
// to changes, since it holds about a hundred small files.
type Watcher struct {
	Dir      string
	Store    *store.Store
	Interval time.Duration
	// OnImport is called after each pass that stored a new game or a later copy of one, and
	// after the first pass whatever it found. It runs on the watcher's goroutine.
	OnImport func(Result)
	// OnError is called with errors that don't stop the watcher, such as a log it couldn't read.
	OnError func(error)

	seen map[string]fileState
	// checked is set once a pass has parsed again what an older parser stored.
	checked bool
}

func (w *Watcher) Run(ctx context.Context) error {
	w.seen = make(map[string]fileState)
	w.checked = false
	interval := w.Interval
	if interval == 0 {
		interval = 3 * time.Second
	}
	first := true
	for {
		r, err := w.pass(ctx, w.report)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return err
		}
		if (first || r.New > 0 || r.Updated > 0) && w.OnImport != nil {
			w.OnImport(r)
		}
		first = false
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

func (w *Watcher) report(err error) error {
	if w.OnError != nil {
		w.OnError(err)
	}
	return nil
}

// pass imports the logs changed since the last pass, oldest first, so each game keeps the
// earliest file it was saved in. fail gets each file's error and stops the pass when it returns
// one.
func (w *Watcher) pass(ctx context.Context, fail func(error) error) (Result, error) {
	if w.seen == nil {
		w.seen = make(map[string]fileState)
	}
	paths, err := simfile.CombatLogs(w.Dir)
	if err != nil {
		return Result{}, err
	}
	var total Result
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		// The sim renames a .partial.log when its game ends, which can happen after the listing.
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		state := fileState{size: info.Size(), mod: info.ModTime()}
		if w.seen[path] == state {
			continue
		}
		r, err := File(ctx, w.Store, path)
		if err != nil {
			if err := fail(fmt.Errorf("importer: %s: %w", filepath.Base(path), err)); err != nil {
				return total, err
			}
			continue
		}
		w.seen[path] = state
		total.add(r)
	}
	// Games an older version of logpose stored may read differently now. New games are always
	// stored by this parser, so a pass only needs this until it succeeds once.
	if !w.checked {
		n, err := w.Store.Reparse(ctx)
		total.Reparsed += n
		w.checked = err == nil
		if err != nil {
			return total, fail(err)
		}
	}
	return total, nil
}
