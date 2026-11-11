-- Since 1.45a the sim names its logs with the UTC time, as in 2026-11-11T00.27.09.2230886-<id>.log,
-- and these games were stored as if that were local time. Older names have no fraction and stay.
UPDATE games
SET played_at = unixepoch(substr(source_file, 1, 10) || ' ' || replace(substr(source_file, 12, 8), '.', ':')) * 1000
WHERE source_file GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9].[0-9][0-9].[0-9][0-9].[0-9]*';
