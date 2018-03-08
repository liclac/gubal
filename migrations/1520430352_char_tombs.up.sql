BEGIN;

CREATE TABLE character_tombstones (
    id          BIGINT       PRIMARY KEY,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    status_code INT NOT NULL CHECK (status_code > 0)
);

COMMIT;
