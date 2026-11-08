package store

import (
	"bytes"
	"compress/flate"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/VTGare/logpose/internal/simlog"
)

var ErrNotFound = errors.New("store: not found")

// Source is where a game's log came from.
type Source struct {
	File  string
	Index int
	// PlayedAt is when the game ended, from the log file's name.
	PlayedAt time.Time
}

type GamePlayer struct {
	ID         string
	Name       string
	Tag        string
	Owner      int
	Leader     string
	Mulliganed bool
	HandBefore []string
	HandAfter  []string
}

// Kept is the hand the player went into the game with, and sent the hand they mulliganed away.
// The log only gives a hand after the mulligan when there was one.
func (p GamePlayer) Kept() (kept, sent []string) {
	if p.Mulliganed && len(p.HandAfter) > 0 {
		return p.HandAfter, p.HandBefore
	}
	if len(p.HandBefore) > 0 {
		return p.HandBefore, nil
	}
	return p.HandAfter, nil
}

type Game struct {
	Fingerprint string
	PlayedAt    time.Time
	Lobby       simlog.Lobby
	SimVersion  string
	Players     []GamePlayer
	First       string
	Turns       int
	End         simlog.End
	Source      Source
	// Sets is the card sets of the game's cards, sorted.
	Sets []string
	// SimFormat is the sim's name for the game's format, or "" when logpose didn't see it.
	SimFormat string
	Type      GameType
	// Note is what the user wrote about the game, or "".
	Note       string
	Bookmarked bool
}

func (g Game) Player(id string) (GamePlayer, bool) {
	for _, p := range g.Players {
		if p.ID == id {
			return p, true
		}
	}
	return GamePlayer{}, false
}

// SaveGame stores a parsed game with its log text. A game already stored keeps its copy, unless
// the new one has more moves, which means the stored one was saved before the game ended. It
// reports whether the game is new.
func (s *Store) SaveGame(ctx context.Context, g *simlog.Game, src Source) (bool, error) {
	log, err := compress(g.Text)
	if err != nil {
		return false, err
	}
	created := false
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		var moves int
		err := tx.QueryRowContext(ctx, `SELECT moves FROM games WHERE fingerprint = ?`, g.Fingerprint).Scan(&moves)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			created = true
		case err != nil:
			return err
		case moves >= len(g.Moves):
			return nil
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO games (fingerprint, log, moves, source_file, source_index, played_at, imported_at,
				parser_version, lobby_code, lobby_role, sim_version, first_player, turns, end_kind, loser)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, '', '', '', '', 0, 'unknown', '')
			ON CONFLICT (fingerprint) DO UPDATE SET log = excluded.log, moves = excluded.moves,
				source_file = excluded.source_file, source_index = excluded.source_index`,
			g.Fingerprint, log, len(g.Moves), src.File, src.Index, src.PlayedAt.UnixMilli(), time.Now().UnixMilli(),
		); err != nil {
			return err
		}
		return writeParsed(ctx, tx, g)
	})
	return created, err
}

// writeParsed rewrites everything parsed from a game's log.
func writeParsed(ctx context.Context, tx *sql.Tx, g *simlog.Game) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE games SET parser_version = ?, lobby_code = ?, lobby_role = ?, sim_version = ?,
			first_player = ?, turns = ?, end_kind = ?, loser = ?, sets = ?
		WHERE fingerprint = ?`,
		parserVersion, g.Lobby.Code, roleName(g.Lobby.Role), g.SimVersion, g.First, g.Turns,
		endName(g.End.Kind), g.End.Loser, strings.Join(g.Sets, ","), g.Fingerprint,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM game_players WHERE fingerprint = ?`, g.Fingerprint); err != nil {
		return err
	}
	for code, name := range g.CardNames {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO card_names (code, name) VALUES (?, ?)
			ON CONFLICT (code) DO UPDATE SET name = excluded.name WHERE length(excluded.name) < length(name)`,
			code, name,
		); err != nil {
			return err
		}
	}
	for _, p := range g.Players {
		m := g.Mulligans[p.ID()]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO game_players (fingerprint, player, name, tag, owner, leader, mulliganed, hand_before, hand_after)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			g.Fingerprint, p.ID(), p.Name, p.Tag, p.Owner, p.Leader, m.Mulliganed,
			strings.Join(m.Before, ","), strings.Join(m.After, ","),
		); err != nil {
			return err
		}
	}
	return writeFacts(ctx, tx, g)
}

// Reparse parses again every game stored by an older parser and rewrites what was parsed from
// it. It returns how many games it parsed.
func (s *Store) Reparse(ctx context.Context) (int, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT fingerprint FROM games WHERE parser_version < ?`, parserVersion)
	if err != nil {
		return 0, err
	}
	var stale []string
	for rows.Next() {
		var fp string
		if err := rows.Scan(&fp); err != nil {
			return 0, errors.Join(err, rows.Close())
		}
		stale = append(stale, fp)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return 0, err
	}
	for i, fp := range stale {
		if err := s.reparse(ctx, fp); err != nil {
			return i, fmt.Errorf("store: reparse %s: %w", fp, err)
		}
	}
	return len(stale), nil
}

func (s *Store) reparse(ctx context.Context, fingerprint string) error {
	text, err := s.GameLog(ctx, fingerprint)
	if err != nil {
		return err
	}
	games, err := simlog.Parse(strings.NewReader(text))
	if err != nil {
		return err
	}
	if len(games) != 1 {
		return fmt.Errorf("log parses to %d games", len(games))
	}
	g := &games[0]
	// A parser change can change fingerprints. The row keeps the one it was stored under, so
	// imports of the same file still find it.
	g.Fingerprint = fingerprint
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE games SET moves = ? WHERE fingerprint = ?`, len(g.Moves), fingerprint); err != nil {
			return err
		}
		return writeParsed(ctx, tx, g)
	})
}

// GameLog returns a stored game's log text.
func (s *Store) GameLog(ctx context.Context, fingerprint string) (string, error) {
	var log []byte
	err := s.read.QueryRowContext(ctx, `SELECT log FROM games WHERE fingerprint = ?`, fingerprint).Scan(&log)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return decompress(log)
}

// Games returns every stored game, newest first.
func (s *Store) Games(ctx context.Context) ([]Game, error) {
	rows, err := s.read.QueryContext(ctx, `
		SELECT g.fingerprint, g.played_at, g.lobby_code, g.lobby_role, g.sim_version, g.first_player,
			g.turns, g.end_kind, g.loser, g.source_file, g.source_index, g.sets, coalesce(f.sim_format, ''), t.ranked,
			coalesce(n.note, ''), b.fingerprint IS NOT NULL
		FROM games g LEFT JOIN log_formats f ON f.source_file = g.source_file
		LEFT JOIN game_types t ON t.fingerprint = g.fingerprint
		LEFT JOIN game_notes n ON n.fingerprint = g.fingerprint
		LEFT JOIN game_bookmarks b ON b.fingerprint = g.fingerprint
		ORDER BY g.played_at DESC, g.source_index DESC`)
	if err != nil {
		return nil, err
	}
	var games []Game
	byFP := make(map[string]int)
	for rows.Next() {
		g, err := scanGame(rows)
		if err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		byFP[g.Fingerprint] = len(games)
		games = append(games, g)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	return games, s.addPlayers(ctx, games, byFP)
}

func scanGame(rows *sql.Rows) (Game, error) {
	var g Game
	var played int64
	var role, end, sets string
	var ranked sql.NullBool
	err := rows.Scan(&g.Fingerprint, &played, &g.Lobby.Code, &role, &g.SimVersion, &g.First, &g.Turns,
		&end, &g.End.Loser, &g.Source.File, &g.Source.Index, &sets, &g.SimFormat, &ranked, &g.Note, &g.Bookmarked)
	g.Type = gameType(ranked)
	g.Sets = splitCodes(sets)
	g.PlayedAt = time.UnixMilli(played)
	g.Source.PlayedAt = g.PlayedAt
	g.Lobby.Role = roleFromName(role)
	g.End.Kind = endFromName(end)
	return g, err
}

func (s *Store) addPlayers(ctx context.Context, games []Game, byFP map[string]int) error {
	rows, err := s.read.QueryContext(ctx, `
		SELECT fingerprint, player, name, tag, owner, leader, mulliganed, hand_before, hand_after
		FROM game_players ORDER BY fingerprint, owner, player`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var fp, before, after string
		var p GamePlayer
		if err := rows.Scan(&fp, &p.ID, &p.Name, &p.Tag, &p.Owner, &p.Leader, &p.Mulliganed, &before, &after); err != nil {
			return errors.Join(err, rows.Close())
		}
		p.HandBefore, p.HandAfter = splitCodes(before), splitCodes(after)
		if i, ok := byFP[fp]; ok {
			games[i].Players = append(games[i].Players, p)
		}
	}
	return errors.Join(rows.Err(), rows.Close())
}

// CardNames returns every card name the logs gave, by code.
func (s *Store) CardNames(ctx context.Context) (map[string]string, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT code, name FROM card_names`)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string)
	for rows.Next() {
		var code, name string
		if err := rows.Scan(&code, &name); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		names[code] = name
	}
	return names, errors.Join(rows.Err(), rows.Close())
}

// LocalPlayer returns the player in the most games, who is the one whose sim saved the logs.
// It returns "" when nothing is stored.
func (s *Store) LocalPlayer(ctx context.Context) (string, error) {
	var id string
	err := s.read.QueryRowContext(ctx, `
		SELECT player FROM game_players GROUP BY player ORDER BY count(*) DESC, player LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func splitCodes(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func compress(text string) ([]byte, error) {
	var b bytes.Buffer
	w, err := flate.NewWriter(&b, flate.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(w, text); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func decompress(b []byte) (string, error) {
	r := flate.NewReader(bytes.NewReader(b))
	text, err := io.ReadAll(r)
	if err != nil {
		return "", errors.Join(err, r.Close())
	}
	return string(text), r.Close()
}

var roleNames = map[simlog.LobbyRole]string{simlog.RoleHost: "host", simlog.RoleJoin: "join"}

func roleName(r simlog.LobbyRole) string { return roleNames[r] }

func roleFromName(s string) simlog.LobbyRole {
	for r, name := range roleNames {
		if name == s {
			return r
		}
	}
	return simlog.RoleUnknown
}

var endNames = map[simlog.EndKind]string{
	simlog.EndUnknown: "unknown", simlog.EndConcede: "concede",
	simlog.EndDeckOut: "deck_out", simlog.EndLethal: "lethal",
}

func endName(k simlog.EndKind) string { return endNames[k] }

func endFromName(s string) simlog.EndKind {
	for k, name := range endNames {
		if name == s {
			return k
		}
	}
	return simlog.EndUnknown
}
