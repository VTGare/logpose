-- The games the user bookmarked. Like notes, they aren't parsed from the log.
CREATE TABLE game_bookmarks (
    fingerprint TEXT PRIMARY KEY REFERENCES games (fingerprint) ON DELETE CASCADE,
    created_at  INTEGER NOT NULL
) STRICT, WITHOUT ROWID;
