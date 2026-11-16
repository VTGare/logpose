package store

import (
	"context"
	"database/sql"
)

// ResultBy says where a game's End.Loser comes from.
type ResultBy int

const (
	ResultLog ResultBy = iota
	// ResultDropped means the log doesn't say and the loser is the player whose connection dropped.
	ResultDropped
	// ResultUser means the log doesn't say and the user set the loser.
	ResultUser
)

// applyResult fills in the loser of a game whose log doesn't say who lost.
func (g *Game) applyResult() {
	if g.End.Loser != "" {
		return
	}
	switch {
	case g.SetLoser != "":
		g.End.Loser, g.ResultBy = g.SetLoser, ResultUser
	case g.Dropped != "":
		g.End.Loser, g.ResultBy = g.Dropped, ResultDropped
	}
}

// SaveDrop records the player whose connection dropped in a game. It reports whether that
// changed anything, since the follower reads all of Player.log again on every start.
func (s *Store) SaveDrop(ctx context.Context, fingerprint, player string) (bool, error) {
	var changed bool
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO game_drops (fingerprint, player) VALUES (?, ?)
			ON CONFLICT (fingerprint) DO UPDATE SET player = excluded.player WHERE player <> excluded.player`,
			fingerprint, player)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		changed = n > 0
		return err
	})
	return changed, err
}

// SetGameResult sets the loser of a game, one of its players. An empty loser takes the user's
// choice away.
func (s *Store) SetGameResult(ctx context.Context, fingerprint, loser string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if loser == "" {
			_, err := tx.ExecContext(ctx, `DELETE FROM game_results WHERE fingerprint = ?`, fingerprint)
			return err
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO game_results (fingerprint, loser)
			SELECT fingerprint, player FROM game_players WHERE fingerprint = ? AND player = ?
			ON CONFLICT (fingerprint) DO UPDATE SET loser = excluded.loser`,
			fingerprint, loser)
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
