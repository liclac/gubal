BEGIN;

ALTER TABLE characters DROP COLUMN guardian;

DROP TYPE character_guardian;

COMMIT;
