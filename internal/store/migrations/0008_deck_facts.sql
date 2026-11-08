-- What each player's deck did in a game, read from its log by internal/deck. Like game_players,
-- it's rewritten whenever the game is parsed again.

-- The deck's cards as far as the log names them.
CREATE TABLE game_deck_cards (
    fingerprint TEXT    NOT NULL REFERENCES games (fingerprint) ON DELETE CASCADE,
    player      TEXT    NOT NULL,
    card        TEXT    NOT NULL,
    copies      INTEGER NOT NULL CHECK (copies > 0),
    PRIMARY KEY (fingerprint, player, card)
) STRICT, WITHOUT ROWID;

-- Cards entering or leaving the hand, in log order. turn is the game turn, 0 for the setup.
CREATE TABLE game_card_events (
    fingerprint TEXT    NOT NULL REFERENCES games (fingerprint) ON DELETE CASCADE,
    player      TEXT    NOT NULL,
    seq         INTEGER NOT NULL,
    card        TEXT    NOT NULL,
    turn        INTEGER NOT NULL,
    kind        TEXT    NOT NULL CHECK (kind IN ('drew', 'from_life', 'paid_play', 'effect_play',
        'counter', 'trigger', 'own_lines', 'discard')),
    PRIMARY KEY (fingerprint, player, seq)
) STRICT, WITHOUT ROWID;

-- A searcher's look at the top of the deck. taken is '' when nothing was taken, and passed is
-- the comma-separated cards it showed that went to the bottom.
CREATE TABLE game_searches (
    fingerprint TEXT    NOT NULL REFERENCES games (fingerprint) ON DELETE CASCADE,
    player      TEXT    NOT NULL,
    seq         INTEGER NOT NULL,
    source      TEXT    NOT NULL,
    taken       TEXT    NOT NULL,
    passed      TEXT    NOT NULL,
    turn        INTEGER NOT NULL,
    PRIMARY KEY (fingerprint, player, seq)
) STRICT, WITHOUT ROWID;

-- The player's DON!! in play during each game turn, comma-separated from turn 0.
ALTER TABLE game_players ADD COLUMN don TEXT NOT NULL DEFAULT '';

CREATE INDEX game_deck_cards_player ON game_deck_cards (player);
CREATE INDEX game_card_events_player ON game_card_events (player);
CREATE INDEX game_searches_player ON game_searches (player);
