package importer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/VTGare/logpose/internal/simfile"
	"github.com/VTGare/logpose/internal/store"
)

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
