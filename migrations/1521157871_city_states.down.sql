BEGIN;

ALTER TABLE characters DROP COLUMN city_state;

DROP TYPE city_state;

COMMIT;
