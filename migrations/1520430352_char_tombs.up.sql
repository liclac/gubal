BEGIN;

CREATE TABLE character_tombstones (
    id          BIGINT       PRIMARY KEY,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMIT;
