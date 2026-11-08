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
	// SettingSimDir is the sim's folder, saved whenever logpose finds it, so it's known the next
	// time logpose starts without the sim running.
	SettingSimDir = "sim_dir"
	// SettingSimDirChosen is a sim folder the user picked. It wins over the folder logpose
	// finds.
	SettingSimDirChosen = "sim_dir_chosen"
	// SettingYou is the user's names, one per line. Empty means the name in the most games.
	SettingYou = "you"
	// SettingTheme is "dark" or "light". Empty follows Windows.
	SettingTheme = "theme"
	// SettingColorblind is "1" for blue and orange wins and losses.
	SettingColorblind = "colorblind"
	// SettingCloseQuits is "1" when closing the window quits logpose.
	SettingCloseQuits = "close_quits"
	// SettingStartHidden is "1" when a start with Windows stays in the tray.
	SettingStartHidden = "start_hidden"
	// SettingSidebarWidth is the sidebar's width in DIPs. Empty means the default.
	SettingSidebarWidth = "sidebar_width"
	// SettingCardPanelWidth is the width in DIPs of the Cards page's card panel. Empty means
	// the default.
	SettingCardPanelWidth = "card_panel_width"
)

// Setting returns a setting's value, or "" when it isn't set.
func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.read.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// Settings returns every setting that is set.
func (s *Store) Settings(ctx context.Context) (map[string]string, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		out[k] = v
	}
	return out, errors.Join(rows.Err(), rows.Close())
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
	rows, err := s.read.QueryContext(ctx, `
		SELECT player, count(*) FROM game_players GROUP BY player ORDER BY count(*) DESC, player`)
	if err != nil {
		return nil, err
	}
	var out []PlayerCount
	for rows.Next() {
		var p PlayerCount
		if err := rows.Scan(&p.ID, &p.Games); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		out = append(out, p)
	}
	return out, errors.Join(rows.Err(), rows.Close())
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
