BEGIN;

CREATE TABLE character_titles (
    id         SERIAL      PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    title VARCHAR(255) NOT NULL UNIQUE
);

ALTER TABLE characters ADD COLUMN title_id INT REFERENCES character_titles (id);

COMMIT;
