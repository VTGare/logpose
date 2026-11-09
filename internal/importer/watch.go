package importer

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"time"

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
		r, err := w.pass(ctx)
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

func (w *Watcher) pass(ctx context.Context) (Result, error) {
	paths, err := filepath.Glob(filepath.Join(w.Dir, "*.log"))
	if err != nil {
		return Result{}, err
	}
	slices.Sort(paths)
	var total Result
	for _, path := range paths {
		if ctx.Err() != nil {
			return total, nil
		}
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
			if w.OnError != nil {
				w.OnError(err)
			}
			continue
		}
		w.seen[path] = state
		total.add(r)
	}
	// The first pass catches games an older parser stored, which a new version of logpose
	// reads differently. Later passes only need it when a game was added.
	if total.New > 0 || !w.checked {
		n, err := w.Store.Reparse(ctx)
		if err != nil && w.OnError != nil {
			w.OnError(err)
		}
		total.Reparsed += n
		w.checked = err == nil
	}
	return total, nil
}
