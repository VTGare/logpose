package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type GameType int

const (
	// TypeGuessed leaves the type to the guess from the game's lobby.
	TypeGuessed GameType = iota
	TypeRanked
	TypeFriendly
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

func (s *Store) setAnnotation(ctx context.Context, clear bool, delSQL, fingerprint, upsertSQL string, args ...any) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if clear {
			_, err := tx.ExecContext(ctx, delSQL, fingerprint)
			return err
		}
		res, err := tx.ExecContext(ctx, upsertSQL, args...)
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

// SetGameType sets a game's type, or with TypeGuessed takes the user's choice away.
func (s *Store) SetGameType(ctx context.Context, fingerprint string, t GameType) error {
	return s.setAnnotation(ctx, t == TypeGuessed,
		`DELETE FROM game_types WHERE fingerprint = ?`, fingerprint,
		`INSERT INTO game_types (fingerprint, ranked) SELECT fingerprint, ? FROM games WHERE fingerprint = ?
		ON CONFLICT (fingerprint) DO UPDATE SET ranked = excluded.ranked`,
		t == TypeRanked, fingerprint)
}

// SetGameNote saves a game's note without its outer spaces. An empty note deletes it.
func (s *Store) SetGameNote(ctx context.Context, fingerprint, note string) error {
	note = strings.TrimSpace(note)
	return s.setAnnotation(ctx, note == "",
		`DELETE FROM game_notes WHERE fingerprint = ?`, fingerprint,
		`INSERT INTO game_notes (fingerprint, note) SELECT fingerprint, ? FROM games WHERE fingerprint = ?
		ON CONFLICT (fingerprint) DO UPDATE SET note = excluded.note`,
		note, fingerprint)
}

func (s *Store) SetBookmark(ctx context.Context, fingerprint string, on bool) error {
	return s.setAnnotation(ctx, !on,
		`DELETE FROM game_bookmarks WHERE fingerprint = ?`, fingerprint,
		`INSERT INTO game_bookmarks (fingerprint, created_at) SELECT fingerprint, ? FROM games WHERE fingerprint = ?
		ON CONFLICT (fingerprint) DO UPDATE SET created_at = game_bookmarks.created_at`,
		time.Now().UnixMilli(), fingerprint)
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
	return s.setAnnotation(ctx, loser == "",
		`DELETE FROM game_results WHERE fingerprint = ?`, fingerprint,
		`INSERT INTO game_results (fingerprint, loser)
		SELECT fingerprint, player FROM game_players WHERE fingerprint = ? AND player = ?
		ON CONFLICT (fingerprint) DO UPDATE SET loser = excluded.loser`,
		fingerprint, loser)
}
