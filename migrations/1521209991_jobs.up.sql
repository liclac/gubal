BEGIN;

CREATE TYPE job AS ENUM (
	'PLD',
	'WAR',
	'DRK',
	'WHM',
	'SCH',
	'AST',
	'MNK',
	'DRG',
	'NIN',
	'SAM',
	'BRD',
	'MCH',
	'BLM',
	'SMN',
	'RDM',
	'CRP',
	'BSM',
	'ARM',
	'GSM',
	'LTW',
	'WVR',
	'ALC',
	'CUL',
	'MIN',
	'BOT',
	'FSH'
);

CREATE TABLE levels (
    character_id BIGINT       NOT NULL REFERENCES characters (id) DEFERRABLE INITIALLY DEFERRED,
    job          job          NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

	level        INT          NOT NULL,

    PRIMARY KEY (character_id, job)
);

COMMIT;
