package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// SetGameNote saves a game's note without its outer spaces. An empty note deletes it.
func (s *Store) SetGameNote(ctx context.Context, fingerprint, note string) error {
	note = strings.TrimSpace(note)
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if note == "" {
			_, err := tx.ExecContext(ctx, `DELETE FROM game_notes WHERE fingerprint = ?`, fingerprint)
			return err
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO game_notes (fingerprint, note) SELECT fingerprint, ? FROM games WHERE fingerprint = ?
			ON CONFLICT (fingerprint) DO UPDATE SET note = excluded.note`,
			note, fingerprint)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *Store) SetBookmark(ctx context.Context, fingerprint string, on bool) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if !on {
			_, err := tx.ExecContext(ctx, `DELETE FROM game_bookmarks WHERE fingerprint = ?`, fingerprint)
			return err
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO game_bookmarks (fingerprint, created_at) SELECT fingerprint, ? FROM games WHERE fingerprint = ?
			ON CONFLICT (fingerprint) DO NOTHING`,
			time.Now().UnixMilli(), fingerprint)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil || n > 0 {
			return err
		}
		// Nothing was inserted when the game is missing or already bookmarked.
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM games WHERE fingerprint = ?)`, fingerprint).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		return nil
	})
}
