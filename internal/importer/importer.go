package importer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/VTGare/logpose/internal/simlog"
	"github.com/VTGare/logpose/internal/store"
)

type Result struct {
	Files int
	Games int
	New   int
}

func (r *Result) add(o Result) {
	r.Files += o.Files
	r.Games += o.Games
	r.New += o.New
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
	f, err := os.Open(path)
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
		created, err := st.SaveGame(ctx, &games[i], store.Source{File: filepath.Base(path), Index: i, PlayedAt: played})
		if err != nil {
			return r, err
		}
		if created {
			r.New++
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
