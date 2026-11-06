-- Timestamps are Unix milliseconds.

-- One row per game, keyed by simlog's fingerprint. log is the game's own text, compressed with
-- DEFLATE, and the source of truth: every other column, and game_players, is parsed from it and
-- rewritten when parser_version is behind simlog.Version.
CREATE TABLE games (
    fingerprint    TEXT    PRIMARY KEY,
    log            BLOB    NOT NULL,
    moves          INTEGER NOT NULL,
    source_file    TEXT    NOT NULL,
    source_index   INTEGER NOT NULL,
    played_at      INTEGER NOT NULL,
    imported_at    INTEGER NOT NULL,
    parser_version INTEGER NOT NULL,
    lobby_code     TEXT    NOT NULL,
    lobby_role     TEXT    NOT NULL CHECK (lobby_role IN ('', 'host', 'join')),
    sim_version    TEXT    NOT NULL,
    first_player   TEXT    NOT NULL,
    turns          INTEGER NOT NULL,
    end_kind       TEXT    NOT NULL CHECK (end_kind IN ('unknown', 'concede', 'deck_out', 'lethal')),
    loser          TEXT    NOT NULL
) STRICT;

CREATE INDEX games_played_at ON games (played_at);
CREATE INDEX games_parser_version ON games (parser_version);

-- hand_before and hand_after are comma-separated card codes, empty when the log doesn't show
-- them. The sim usually logs only the local player's hand before the mulligan.
CREATE TABLE game_players (
    fingerprint TEXT    NOT NULL REFERENCES games (fingerprint) ON DELETE CASCADE,
    player      TEXT    NOT NULL,
    name        TEXT    NOT NULL,
    tag         TEXT    NOT NULL,
    owner       INTEGER NOT NULL,
    leader      TEXT    NOT NULL,
    mulliganed  INTEGER NOT NULL CHECK (mulliganed IN (0, 1)),
    hand_before TEXT    NOT NULL,
    hand_after  TEXT    NOT NULL,
    PRIMARY KEY (fingerprint, player)
) STRICT, WITHOUT ROWID;

CREATE INDEX game_players_player ON game_players (player);
