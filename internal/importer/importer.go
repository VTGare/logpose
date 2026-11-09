package importer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/VTGare/logpose/internal/sim"
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

// Dir imports every combat log in the sim's AutoSaved folder, oldest first, so each game keeps
// the earliest file it was saved in.
func Dir(ctx context.Context, st *store.Store, dir string) (Result, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.log"))
	if err != nil {
		return Result{}, err
	}
	slices.Sort(paths)
	var total Result
	for _, path := range paths {
		r, err := File(ctx, st, path)
		if err != nil {
			return total, fmt.Errorf("importer: %s: %w", filepath.Base(path), err)
		}
		total.add(r)
	}
	return total, nil
}

func File(ctx context.Context, st *store.Store, path string) (Result, error) {
	f, err := sim.OpenShared(path)
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
		case store.Replaced:
			if games[i].Closed {
				r.Updated++
			}
		}
	}
	return r, nil
}

// logTimeLayout is the sim's file name, the local time it saved the log, which is when the
// file's last game ended.
const logTimeLayout = "2006-01-02T15.04.05"

func playedAt(path string) (time.Time, error) {
	name := filepath.Base(path)
	if len(name) >= len(logTimeLayout) {
		if t, err := time.ParseInLocation(logTimeLayout, name[:len(logTimeLayout)], time.Local); err == nil {
			return t, nil
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
