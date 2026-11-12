package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// SettingYou is the user's names, one per line. Empty means the name in the most games.
	SettingYou = "you"
)

// Setting returns a setting's value, or "" when it isn't set. Settings are free-form, and only
// the keys the engine reads are named here.
func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.read.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) Settings(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string)
	err := s.each(ctx, `SELECT key, value FROM settings`, nil, func(rows *sql.Rows) error {
		var k, v string
		err := rows.Scan(&k, &v)
		out[k] = v
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ChangeSetting sets a setting to what change makes of its value, in one transaction.
func (s *Store) ChangeSetting(ctx context.Context, key string, change func(string) string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var cur string
		err := tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&cur)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO settings (key, value) VALUES (?, ?)
			ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, change(cur))
		return err
	})
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.write.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// LocalPlayers returns the names the user picked, or else the name in the most games. It
// returns nil when nothing is stored and nothing is picked.
func (s *Store) LocalPlayers(ctx context.Context) ([]string, error) {
	picked, err := s.Setting(ctx, SettingYou)
	if err != nil {
		return nil, err
	}
	if names := SplitNames(picked); len(names) > 0 {
		return names, nil
	}
	me, err := s.LocalPlayer(ctx)
	if err != nil || me == "" {
		return nil, err
	}
	return []string{me}, nil
}

// JoinNames writes names for SettingYou.
func JoinNames(names []string) string { return strings.Join(names, "\n") }

// SplitNames reads SettingYou.
func SplitNames(s string) []string {
	var out []string
	for n := range strings.SplitSeq(s, "\n") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

type PlayerCount struct {
	ID    string
	Games int
}

// PlayerCounts returns every player with their number of games, most games first.
func (s *Store) PlayerCounts(ctx context.Context) ([]PlayerCount, error) {
	var out []PlayerCount
	err := s.each(ctx, `
		SELECT player, count(*) FROM game_players GROUP BY player ORDER BY count(*) DESC, player`, nil,
		func(rows *sql.Rows) error {
			var p PlayerCount
			err := rows.Scan(&p.ID, &p.Games)
			out = append(out, p)
			return err
		})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Backup writes a copy of the database to path. VACUUM INTO refuses a file that exists, so it
// writes next to path and then replaces it.
func (s *Store) Backup(ctx context.Context, path string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".logpose-backup-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := errors.Join(tmp.Close(), os.Remove(name)); err != nil {
		return err
	}
	if _, err := s.write.ExecContext(ctx, `VACUUM INTO ?`, name); err != nil {
		return fmt.Errorf("store: backup: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		return errors.Join(err, os.Remove(name))
	}
	return nil
}
