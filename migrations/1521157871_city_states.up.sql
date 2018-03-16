BEGIN;

CREATE TYPE city_state AS ENUM (
    'Gridania',
    'Uldah',
    'Limsa'
);

ALTER TABLE characters ADD COLUMN city_state city_state NOT NULL DEFAULT 'Gridania';
ALTER TABLE characters ALTER COLUMN city_state DROP DEFAULT;

COMMIT;
