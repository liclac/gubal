BEGIN;

CREATE TABLE characters (
    id         BIGINT       PRIMARY KEY,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    first_name VARCHAR(25) NOT NULL,
    last_name  VARCHAR(25) NOT NULL
);

COMMIT;
