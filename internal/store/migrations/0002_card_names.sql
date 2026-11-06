-- Card names as the sim's logs write them. A name keeps the shortest spelling seen, because the
-- text before a card link can start with a verb, as in "Discard Otama".
CREATE TABLE card_names (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL
) STRICT, WITHOUT ROWID;
