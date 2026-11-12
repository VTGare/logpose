package store

import (
	"context"
	"database/sql"
)

type GameType int

const (
	// TypeGuessed leaves the type to the guess from the game's lobby.
	TypeGuessed GameType = iota
	TypeRanked
	TypeFriendly
)

// SetGameType sets a game's type, or with TypeGuessed takes the user's choice away.
func (s *Store) SetGameType(ctx context.Context, fingerprint string, t GameType) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if t == TypeGuessed {
			_, err := tx.ExecContext(ctx, `DELETE FROM game_types WHERE fingerprint = ?`, fingerprint)
			return err
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO game_types (fingerprint, ranked) SELECT fingerprint, ? FROM games WHERE fingerprint = ?
			ON CONFLICT (fingerprint) DO UPDATE SET ranked = excluded.ranked`,
			t == TypeRanked, fingerprint)
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

func gameType(ranked sql.NullBool) GameType {
	switch {
	case !ranked.Valid:
		return TypeGuessed
	case ranked.Bool:
		return TypeRanked
	default:
		return TypeFriendly
	}
}
