BEGIN;

CREATE TYPE character_guardian AS ENUM (
    'Halone',
    'Menphina',
    'Thaliak',
    'Nymeia',
    'Llymlaen',
    'Oschon',
    'Byregot',
    'Rhalgr',
    'Azeyma',
    'Naldthal',
    'Nophica',
    'Althyk'
);

ALTER TABLE characters ADD COLUMN guardian character_guardian NOT NULL DEFAULT 'Halone';
ALTER TABLE characters ALTER COLUMN guardian DROP DEFAULT;

COMMIT;
