BEGIN;

ALTER TABLE characters DROP COLUMN clan;

ALTER TABLE characters DROP COLUMN race;

ALTER TABLE characters DROP COLUMN gender;

DROP TYPE character_clan;

DROP TYPE character_race;

COMMIT;
