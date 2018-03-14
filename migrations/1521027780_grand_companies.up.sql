BEGIN;

CREATE TYPE grand_company AS ENUM (
    'Maelstrom',
    'Adders',
    'Flames'
);

ALTER TABLE characters ADD COLUMN gc grand_company;

ALTER TABLE characters ADD COLUMN gc_rank INT NOT NULL DEFAULT 0;

COMMIT;
