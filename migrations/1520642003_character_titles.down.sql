BEGIN;

ALTER TABLE characters DROP COLUMN title_id;

DROP TABLE character_titles;

COMMIT;
