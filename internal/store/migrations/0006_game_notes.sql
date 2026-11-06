-- The note the user wrote on a game. It isn't parsed from the log, so reparsing leaves it alone.
CREATE TABLE game_notes (
    fingerprint TEXT PRIMARY KEY REFERENCES games (fingerprint) ON DELETE CASCADE,
    note        TEXT NOT NULL CHECK (note <> '')
) STRICT, WITHOUT ROWID;
