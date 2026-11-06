-- sets is the comma-separated card sets of a game's cards, parsed from its log.
ALTER TABLE games ADD COLUMN sets TEXT NOT NULL DEFAULT '';

-- A format runs from starts_at until the next format starts. The user names them, since no log
-- says which format a game was played in.
CREATE TABLE formats (
    id        INTEGER PRIMARY KEY,
    name      TEXT    NOT NULL,
    starts_at INTEGER NOT NULL UNIQUE
) STRICT;

-- sim_format is the name the sim showed for the format a combat log's lobby used, such as
-- "Spoiler", which Player.log tells while the sim runs. Logs saved while logpose wasn't
-- running have none.
CREATE TABLE log_formats (
    source_file TEXT PRIMARY KEY,
    sim_format  TEXT NOT NULL
) STRICT, WITHOUT ROWID;
