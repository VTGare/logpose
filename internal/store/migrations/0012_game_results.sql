-- The player whose connection dropped in a game, from Player.log. The sim then logs no result.
-- Player.log can tell it before the game is stored, so there is no foreign key.
CREATE TABLE game_drops (
    fingerprint TEXT PRIMARY KEY,
    player      TEXT NOT NULL CHECK (player <> '')
) STRICT, WITHOUT ROWID;

-- The loser the user set for a game whose log doesn't say who won.
CREATE TABLE game_results (
    fingerprint TEXT PRIMARY KEY REFERENCES games (fingerprint) ON DELETE CASCADE,
    loser       TEXT NOT NULL CHECK (loser <> '')
) STRICT, WITHOUT ROWID;
