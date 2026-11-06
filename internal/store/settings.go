package store

import (
	"context"
	"database/sql"
	"errors"
)

// SettingSimDir is the sim's folder, saved whenever logpose finds it, so it's known the next
// time logpose starts without the sim running.
const SettingSimDir = "sim_dir"

// Setting returns a setting's value, or "" when it isn't set.
func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.read.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.write.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
