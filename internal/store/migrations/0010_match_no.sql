-- OPBounty's match number from the log's RZ1|META line, or 0.
ALTER TABLE games ADD COLUMN match_no INTEGER NOT NULL DEFAULT 0;
