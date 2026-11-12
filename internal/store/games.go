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

type Source struct {
	// File is the log's base name in the sim's AutoSaved folder. It is also the key that joins a
	// game to the sim format SetLogFormat stored for its file.
	File  string
	Index int
	// PlayedAt is when the sim began the game's log file, from the file's name.
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
	// Match is OPBounty's match number when the sim's ranked panel set the game up, or 0.
	Match  int
	Source Source
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

type Saved int

const (
	Kept Saved = iota
	Added
	// Replaced means a copy of a game still in progress took the stored copy's place.
	Replaced
	// Completed means a copy with the sim's END line took the stored copy's place.
	Completed
)

// parsedColumns are the games columns parsed from a game's log, in the order parsedValues and
// scanParsed use.
var parsedColumns = []string{
	"lobby_code", "lobby_role", "sim_version", "first_player", "turns", "end_kind", "loser", "sets", "match_no",
}

func parsedValues(g *simlog.Game) []any {
	return []any{
		g.Lobby.Code, g.Lobby.Role.String(), g.SimVersion, g.First, g.Turns,
		g.End.Kind.String(), g.End.Loser, strings.Join(g.Sets, ","), g.Match,
	}
}

func scanParsed(g *Game) (dest []any, fill func()) {
	var role, end, sets string
	dest = []any{&g.Lobby.Code, &role, &g.SimVersion, &g.First, &g.Turns, &end, &g.End.Loser, &sets, &g.Match}
	return dest, func() {
		g.Lobby.Role = simlog.ParseLobbyRole(role)
		g.End.Kind = simlog.ParseEndKind(end)
		g.Sets = splitCodes(sets)
	}
}

var (
	parsedList   = strings.Join(parsedColumns, ", ")
	parsedParams = strings.TrimSuffix(strings.Repeat("?, ", len(parsedColumns)), ", ")
	parsedSet    = strings.Join(parsedColumns, " = ?, ") + " = ?"
)

// userTables hold what the user set on a game, by fingerprint. rekey moves their rows to the new
// fingerprint, and ON DELETE CASCADE drops the rows of a table missing here. onConflict is what
// happens when the new fingerprint has a row too.
var userTables = []struct{ name, columns, onConflict string }{
	{"game_types", "ranked", "DO UPDATE SET ranked = excluded.ranked"},
	{"game_notes", "note", "DO UPDATE SET note = note || char(10) || excluded.note WHERE note <> excluded.note"},
	{"game_bookmarks", "created_at", "DO NOTHING"},
}

// SaveGame stores a parsed game with its log text. A game already stored keeps its copy unless
// the new one is a later copy. The sim writes a game to a .partial.log while it runs.
func (s *Store) SaveGame(ctx context.Context, g *simlog.Game, src Source) (Saved, error) {
	log, err := compress(g.Text)
	if err != nil {
		return Kept, err
	}
	saved := Added
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		var (
			moves  int
			stored []byte
		)
		err := tx.QueryRowContext(ctx, `SELECT moves, log FROM games WHERE fingerprint = ?`, g.Fingerprint).Scan(&moves, &stored)
		switch {
		case errors.Is(err, sql.ErrNoRows):
		case err != nil:
			return err
		default:
			later, err := laterCopy(newCopy(g), storedCopy(moves, stored))
			switch {
			case err != nil:
				return err
			case !later:
				saved = Kept
				return nil
			case g.Closed:
				saved = Completed
			default:
				saved = Replaced
			}
		}
		args := append([]any{
			g.Fingerprint, log, len(g.Moves), src.File, src.Index, src.PlayedAt.UnixMilli(),
			time.Now().UnixMilli(), parserVersion,
		}, parsedValues(g)...)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO games (fingerprint, log, moves, source_file, source_index, played_at, imported_at,
				parser_version, `+parsedList+`)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, `+parsedParams+`)
			ON CONFLICT (fingerprint) DO UPDATE SET log = excluded.log, moves = excluded.moves,
				source_file = excluded.source_file, source_index = excluded.source_index`, args...,
		); err != nil {
			return err
		}
		return writeParsed(ctx, tx, g)
	})
	if err != nil {
		return Kept, err
	}
	return saved, nil
}

func writeParsed(ctx context.Context, tx *sql.Tx, g *simlog.Game) error {
	args := append(append([]any{parserVersion}, parsedValues(g)...), g.Fingerprint)
	if _, err := tx.ExecContext(ctx, `UPDATE games SET parser_version = ?, `+parsedSet+` WHERE fingerprint = ?`, args...); err != nil {
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
	var stale []string
	err := s.each(ctx, `SELECT fingerprint FROM games WHERE parser_version < ?`, []any{parserVersion},
		func(rows *sql.Rows) error {
			var fp string
			err := rows.Scan(&fp)
			stale = append(stale, fp)
			return err
		})
	if err != nil {
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
	g, err := simlog.ParseOne(text)
	if err != nil {
		return err
	}
	// Imports fingerprint the file with the current parser, so a row left under its old
	// fingerprint gets stored a second time.
	if g.Fingerprint != fingerprint {
		if err := s.inTx(ctx, func(tx *sql.Tx) error { return rekey(ctx, tx, fingerprint, g) }); err != nil {
			return err
		}
		return s.reparse(ctx, g.Fingerprint)
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE games SET moves = ? WHERE fingerprint = ?`, len(g.Moves), fingerprint); err != nil {
			return err
		}
		return writeParsed(ctx, tx, g)
	})
}

// rekey moves the game stored under from to g's fingerprint. When a row is already there, it
// keeps the later copy and takes the old row's user data. The row is left for reparse to parse
// again.
func rekey(ctx context.Context, tx *sql.Tx, from string, g *simlog.Game) error {
	to := g.Fingerprint
	var (
		toMoves int
		toLog   []byte
	)
	err := tx.QueryRowContext(ctx, `SELECT moves, log FROM games WHERE fingerprint = ?`, to).Scan(&toMoves, &toLog)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		args := append(append([]any{to}, parsedValues(g)...), from)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO games (fingerprint, log, moves, source_file, source_index, played_at, imported_at,
				parser_version, `+parsedList+`)
			SELECT ?, log, moves, source_file, source_index, played_at, imported_at, 0, `+parsedParams+`
			FROM games WHERE fingerprint = ?`, args...); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		var (
			fromMoves int
			fromLog   []byte
		)
		if err := tx.QueryRowContext(ctx, `SELECT moves, log FROM games WHERE fingerprint = ?`, from).Scan(&fromMoves, &fromLog); err != nil {
			return err
		}
		later, err := laterCopy(storedCopy(fromMoves, fromLog), storedCopy(toMoves, toLog))
		if err != nil {
			return err
		}
		if later {
			if _, err := tx.ExecContext(ctx, `
				UPDATE games SET (log, moves, source_file, source_index) =
					(SELECT log, moves, source_file, source_index FROM games WHERE fingerprint = ?)
				WHERE fingerprint = ?`, from, to); err != nil {
				return err
			}
		}
	}
	for _, t := range userTables {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO `+t.name+` (fingerprint, `+t.columns+`) SELECT ?, `+t.columns+` FROM `+t.name+`
			WHERE fingerprint = ? ON CONFLICT (fingerprint) `+t.onConflict, to, from); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM games WHERE fingerprint = ?`, from)
	return err
}

// logCopy is one copy of a game's log. text is read only when two copies have as many moves.
type logCopy struct {
	moves int
	text  func() (string, error)
}

func newCopy(g *simlog.Game) logCopy {
	return logCopy{moves: len(g.Moves), text: func() (string, error) { return g.Text, nil }}
}

func storedCopy(moves int, log []byte) logCopy {
	return logCopy{moves: moves, text: func() (string, error) { return decompress(log) }}
}

// laterCopy reports whether copy a of a game was saved later in the game than copy b. That copy
// has more moves, or as many and the sim's END line b lacks, since a concession adds no move.
func laterCopy(a, b logCopy) (bool, error) {
	if a.moves != b.moves {
		return a.moves > b.moves, nil
	}
	at, err := a.text()
	if err != nil || !simlog.Closed(at) {
		return false, err
	}
	bt, err := b.text()
	if err != nil {
		return false, err
	}
	return !simlog.Closed(bt), nil
}

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
	var games []Game
	byFP := make(map[string]int)
	err := s.each(ctx, `
		SELECT g.fingerprint, g.played_at, g.source_file, g.source_index, g.`+strings.Join(parsedColumns, ", g.")+`,
			coalesce(f.sim_format, ''), t.ranked, coalesce(n.note, ''), b.fingerprint IS NOT NULL
		FROM games g LEFT JOIN log_formats f ON f.source_file = g.source_file
		LEFT JOIN game_types t ON t.fingerprint = g.fingerprint
		LEFT JOIN game_notes n ON n.fingerprint = g.fingerprint
		LEFT JOIN game_bookmarks b ON b.fingerprint = g.fingerprint
		ORDER BY g.played_at DESC, g.source_index DESC`, nil,
		func(rows *sql.Rows) error {
			g, err := scanGame(rows)
			byFP[g.Fingerprint] = len(games)
			games = append(games, g)
			return err
		})
	if err != nil {
		return nil, err
	}
	return games, s.addPlayers(ctx, games, byFP)
}

func scanGame(rows *sql.Rows) (Game, error) {
	var g Game
	var played int64
	var ranked sql.NullBool
	parsed, fill := scanParsed(&g)
	dest := append(append([]any{&g.Fingerprint, &played, &g.Source.File, &g.Source.Index}, parsed...),
		&g.SimFormat, &ranked, &g.Note, &g.Bookmarked)
	err := rows.Scan(dest...)
	fill()
	g.Type = gameType(ranked)
	g.PlayedAt = time.UnixMilli(played)
	g.Source.PlayedAt = g.PlayedAt
	return g, err
}

func (s *Store) addPlayers(ctx context.Context, games []Game, byFP map[string]int) error {
	return s.each(ctx, `
		SELECT fingerprint, player, name, tag, owner, leader, mulliganed, hand_before, hand_after
		FROM game_players ORDER BY fingerprint, owner, player`, nil,
		func(rows *sql.Rows) error {
			var fp, before, after string
			var p GamePlayer
			if err := rows.Scan(&fp, &p.ID, &p.Name, &p.Tag, &p.Owner, &p.Leader, &p.Mulliganed, &before, &after); err != nil {
				return err
			}
			p.HandBefore, p.HandAfter = splitCodes(before), splitCodes(after)
			if i, ok := byFP[fp]; ok {
				games[i].Players = append(games[i].Players, p)
			}
			return nil
		})
}

func (s *Store) CardNames(ctx context.Context) (map[string]string, error) {
	names := make(map[string]string)
	err := s.each(ctx, `SELECT code, name FROM card_names`, nil, func(rows *sql.Rows) error {
		var code, name string
		err := rows.Scan(&code, &name)
		names[code] = name
		return err
	})
	if err != nil {
		return nil, err
	}
	return names, nil
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

// LatestSimVersion is the sim version the newest stored game's log names, or "".
func (s *Store) LatestSimVersion(ctx context.Context) (string, error) {
	var v string
	err := s.read.QueryRowContext(ctx, `
		SELECT sim_version FROM games WHERE sim_version != '' ORDER BY played_at DESC LIMIT 1`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
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
