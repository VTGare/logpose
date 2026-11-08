package store

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"
)

// Build is one of the user's decks for a leader, defined by the cards every game of it runs.
type Build struct {
	ID     int64
	Leader string
	Name   string
	// Core is the build's cards, sorted.
	Core    []string
	Created time.Time
}

var (
	ErrNoName = errors.New("store: a build needs a name")
	ErrNoCore = errors.New("store: a build needs at least one core card")
)

// Builds returns every build, oldest first.
func (s *Store) Builds(ctx context.Context) ([]Build, error) {
	var out []Build
	byID := make(map[int64]int)
	err := s.each(ctx, `SELECT id, leader, name, created_at FROM builds ORDER BY created_at, id`, nil,
		func(rows *sql.Rows) error {
			var b Build
			var at int64
			if err := rows.Scan(&b.ID, &b.Leader, &b.Name, &at); err != nil {
				return err
			}
			b.Created = time.UnixMilli(at)
			byID[b.ID] = len(out)
			out = append(out, b)
			return nil
		})
	if err != nil {
		return nil, err
	}
	err = s.each(ctx, `SELECT build_id, card FROM build_cards ORDER BY build_id, card`, nil,
		func(rows *sql.Rows) error {
			var id int64
			var card string
			if err := rows.Scan(&id, &card); err != nil {
				return err
			}
			if i, ok := byID[id]; ok {
				out[i].Core = append(out[i].Core, card)
			}
			return nil
		})
	return out, err
}

// SaveBuild adds a build when b.ID is 0 and replaces the stored one otherwise. It returns the
// build's ID. The leader of a stored build doesn't change.
func (s *Store) SaveBuild(ctx context.Context, b Build) (int64, error) {
	name := strings.TrimSpace(b.Name)
	if name == "" {
		return 0, ErrNoName
	}
	core := cleanCore(b.Core)
	if len(core) == 0 {
		return 0, ErrNoCore
	}
	id := b.ID
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if id == 0 {
			res, err := tx.ExecContext(ctx, `INSERT INTO builds (leader, name, created_at) VALUES (?, ?, ?)`,
				b.Leader, name, time.Now().UnixMilli())
			if err != nil {
				return err
			}
			if id, err = res.LastInsertId(); err != nil {
				return err
			}
		} else {
			res, err := tx.ExecContext(ctx, `UPDATE builds SET name = ? WHERE id = ?`, name, id)
			if err != nil {
				return err
			}
			if n, err := res.RowsAffected(); err != nil || n == 0 {
				return errors.Join(err, ErrNotFound)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM build_cards WHERE build_id = ?`, id); err != nil {
				return err
			}
		}
		for _, card := range core {
			if _, err := tx.ExecContext(ctx, `INSERT INTO build_cards (build_id, card) VALUES (?, ?)`, id, card); err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

func (s *Store) DeleteBuild(ctx context.Context, id int64) error {
	res, err := s.write.ExecContext(ctx, `DELETE FROM builds WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return errors.Join(err, ErrNotFound)
	}
	return nil
}

// Dismissal is a suggested build the user said isn't one.
type Dismissal struct {
	Leader string
	// Core is the suggestion's cards, sorted.
	Core []string
}

// DismissSuggestion remembers that a suggested build isn't one, so it isn't suggested again.
func (s *Store) DismissSuggestion(ctx context.Context, d Dismissal) error {
	core := cleanCore(d.Core)
	if len(core) == 0 {
		return ErrNoCore
	}
	_, err := s.write.ExecContext(ctx, `INSERT OR IGNORE INTO build_dismissals (leader, core) VALUES (?, ?)`,
		d.Leader, strings.Join(core, ","))
	return err
}

func (s *Store) Dismissals(ctx context.Context) ([]Dismissal, error) {
	var out []Dismissal
	err := s.each(ctx, `SELECT leader, core FROM build_dismissals ORDER BY leader, core`, nil,
		func(rows *sql.Rows) error {
			var d Dismissal
			var core string
			if err := rows.Scan(&d.Leader, &core); err != nil {
				return err
			}
			d.Core = splitCodes(core)
			out = append(out, d)
			return nil
		})
	return out, err
}

// cleanCore sorts the cards and drops blanks and repeats.
func cleanCore(cards []string) []string {
	var out []string
	for _, c := range cards {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
