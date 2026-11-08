package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/VTGare/logpose/internal/deck"
	"github.com/VTGare/logpose/internal/simlog"
)

// parserVersion is what a stored game was parsed with. A change to either parser parses every
// game again.
const parserVersion = simlog.Version*1000 + deck.Version

var kindNames = map[deck.EventKind]string{
	deck.Drew: "drew", deck.FromLife: "from_life", deck.PaidPlay: "paid_play", deck.EffectPlay: "effect_play",
	deck.Counter: "counter", deck.Trigger: "trigger", deck.OwnLines: "own_lines", deck.Discard: "discard",
}

func kindFromName(s string) (deck.EventKind, error) {
	for k, name := range kindNames {
		if name == s {
			return k, nil
		}
	}
	return 0, fmt.Errorf("store: unknown card event %q", s)
}

func writeFacts(ctx context.Context, tx *sql.Tx, g *simlog.Game) error {
	for _, table := range []string{"game_deck_cards", "game_card_events", "game_searches"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE fingerprint = ?`, g.Fingerprint); err != nil {
			return err
		}
	}
	facts, err := deck.Read(g)
	if err != nil {
		return err
	}
	for _, f := range facts {
		if err := writeFact(ctx, tx, g.Fingerprint, f); err != nil {
			return err
		}
	}
	return nil
}

func writeFact(ctx context.Context, tx *sql.Tx, fp string, f deck.Facts) error {
	for card, n := range f.List {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO game_deck_cards (fingerprint, player, card, copies) VALUES (?, ?, ?, ?)`,
			fp, f.Player, card, n); err != nil {
			return err
		}
	}
	for i, e := range f.Events {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO game_card_events (fingerprint, player, seq, card, turn, kind) VALUES (?, ?, ?, ?, ?, ?)`,
			fp, f.Player, i, e.Card, e.Turn, kindNames[e.Kind]); err != nil {
			return err
		}
	}
	for i, s := range f.Searches {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO game_searches (fingerprint, player, seq, source, taken, passed, turn) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			fp, f.Player, i, s.Source, s.Taken, strings.Join(s.Passed, ","), s.Turn); err != nil {
			return err
		}
	}
	don := make([]string, len(f.Don))
	for i, d := range f.Don {
		don[i] = strconv.Itoa(d)
	}
	_, err := tx.ExecContext(ctx, `UPDATE game_players SET don = ? WHERE fingerprint = ? AND player = ?`,
		strings.Join(don, ","), fp, f.Player)
	return err
}

// DeckFacts returns what the players' decks did, by game fingerprint. A game with more than one
// of the players gets the first one's. Games without RZ1 moves have none.
func (s *Store) DeckFacts(ctx context.Context, players ...string) (map[string]deck.Facts, error) {
	out := make(map[string]deck.Facts)
	if len(players) == 0 {
		return out, nil
	}
	in := "(" + strings.TrimSuffix(strings.Repeat("?,", len(players)), ",") + ")"
	args := make([]any, len(players))
	for i, p := range players {
		args[i] = p
	}
	pick := func(fp, player string) (deck.Facts, bool) {
		f, ok := out[fp]
		if ok && f.Player != player {
			return f, false
		}
		f.Player = player
		return f, true
	}
	err := s.each(ctx, `SELECT fingerprint, player, card, copies FROM game_deck_cards WHERE player IN `+in, args,
		func(rows *sql.Rows) error {
			var fp, player, card string
			var n int
			if err := rows.Scan(&fp, &player, &card, &n); err != nil {
				return err
			}
			if f, ok := pick(fp, player); ok {
				if f.List == nil {
					f.List = make(map[string]int)
				}
				f.List[card] = n
				out[fp] = f
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	err = s.each(ctx, `SELECT fingerprint, player, card, turn, kind FROM game_card_events WHERE player IN `+in+` ORDER BY fingerprint, seq`, args,
		func(rows *sql.Rows) error {
			var fp, player, name string
			var e deck.Event
			if err := rows.Scan(&fp, &player, &e.Card, &e.Turn, &name); err != nil {
				return err
			}
			k, err := kindFromName(name)
			if err != nil {
				return err
			}
			e.Kind = k
			if f, ok := pick(fp, player); ok {
				f.Events = append(f.Events, e)
				out[fp] = f
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	err = s.each(ctx, `SELECT fingerprint, player, source, taken, passed, turn FROM game_searches WHERE player IN `+in+` ORDER BY fingerprint, seq`, args,
		func(rows *sql.Rows) error {
			var fp, player, passed string
			var sr deck.Search
			if err := rows.Scan(&fp, &player, &sr.Source, &sr.Taken, &passed, &sr.Turn); err != nil {
				return err
			}
			sr.Passed = splitCodes(passed)
			if f, ok := pick(fp, player); ok {
				f.Searches = append(f.Searches, sr)
				out[fp] = f
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	err = s.each(ctx, `SELECT fingerprint, player, don FROM game_players WHERE don != '' AND player IN `+in, args,
		func(rows *sql.Rows) error {
			var fp, player, don string
			if err := rows.Scan(&fp, &player, &don); err != nil {
				return err
			}
			f, ok := pick(fp, player)
			if !ok {
				return nil
			}
			for _, d := range strings.Split(don, ",") {
				n, err := strconv.Atoi(d)
				if err != nil {
					return fmt.Errorf("store: DON of %s: %w", fp, err)
				}
				f.Don = append(f.Don, n)
			}
			out[fp] = f
			return nil
		})
	return out, err
}

// each runs a query on the read connection and calls scan for every row.
func (s *Store) each(ctx context.Context, query string, args []any, scan func(*sql.Rows) error) error {
	rows, err := s.read.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		if err := scan(rows); err != nil {
			return errors.Join(err, rows.Close())
		}
	}
	return errors.Join(rows.Err(), rows.Close())
}
