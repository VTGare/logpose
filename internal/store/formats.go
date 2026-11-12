package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// Format is a stretch of time the user names, such as "OP17 + EB05". It ends where the next one
// starts.
type Format struct {
	ID    int64
	Name  string
	Start time.Time
}

// Formats returns every format, oldest first.
func (s *Store) Formats(ctx context.Context) ([]Format, error) {
	var out []Format
	err := s.each(ctx, `SELECT id, name, starts_at FROM formats ORDER BY starts_at`, nil,
		func(rows *sql.Rows) error {
			var f Format
			var at int64
			err := rows.Scan(&f.ID, &f.Name, &at)
			f.Start = time.UnixMilli(at)
			out = append(out, f)
			return err
		})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AddFormat starts a format. Two formats can't start at the same moment.
func (s *Store) AddFormat(ctx context.Context, name string, start time.Time) (int64, error) {
	res, err := s.write.ExecContext(ctx, `INSERT INTO formats (name, starts_at) VALUES (?, ?)`,
		strings.TrimSpace(name), start.UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateFormat(ctx context.Context, f Format) error {
	_, err := s.write.ExecContext(ctx, `UPDATE formats SET name = ?, starts_at = ? WHERE id = ?`,
		strings.TrimSpace(f.Name), f.Start.UnixMilli(), f.ID)
	return err
}

func (s *Store) DeleteFormat(ctx context.Context, id int64) error {
	_, err := s.write.ExecContext(ctx, `DELETE FROM formats WHERE id = ?`, id)
	return err
}

// SetLogFormat records the sim's format for a combat log. file is its base name, as in
// Source.File.
func (s *Store) SetLogFormat(ctx context.Context, file, simFormat string) error {
	_, err := s.write.ExecContext(ctx, `
		INSERT INTO log_formats (source_file, sim_format) VALUES (?, ?)
		ON CONFLICT (source_file) DO UPDATE SET sim_format = excluded.sim_format`, file, simFormat)
	return err
}
