BEGIN;

ALTER TABLE characters DROP COLUMN world;

DROP TYPE world;

COMMIT;
