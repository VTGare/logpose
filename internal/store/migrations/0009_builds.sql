-- The builds the user made for a leader. A game belongs to a build when its list runs every
-- card of the build's core. Like notes, builds aren't parsed from the log.
CREATE TABLE builds (
    id         INTEGER PRIMARY KEY,
    leader     TEXT    NOT NULL,
    name       TEXT    NOT NULL,
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE build_cards (
    build_id INTEGER NOT NULL REFERENCES builds (id) ON DELETE CASCADE,
    card     TEXT    NOT NULL,
    PRIMARY KEY (build_id, card)
) STRICT, WITHOUT ROWID;

-- Suggested builds the user said aren't builds, by leader and the suggestion's core: its cards
-- sorted and comma-separated.
CREATE TABLE build_dismissals (
    leader TEXT NOT NULL,
    core   TEXT NOT NULL,
    PRIMARY KEY (leader, core)
) STRICT, WITHOUT ROWID;
