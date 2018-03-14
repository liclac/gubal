BEGIN;

ALTER TABLE characters DROP COLUMN gc_rank;

ALTER TABLE characters DROP COLUMN gc;

DROP TYPE grand_company;

COMMIT;
