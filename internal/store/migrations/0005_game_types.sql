-- The game type the user set for a game. Games without a row are guessed from their lobby. It
-- isn't parsed from the log, so reparsing leaves it alone.
CREATE TABLE game_types (
    fingerprint TEXT    PRIMARY KEY REFERENCES games (fingerprint) ON DELETE CASCADE,
    ranked      INTEGER NOT NULL CHECK (ranked IN (0, 1))
) STRICT, WITHOUT ROWID;
