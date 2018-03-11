BEGIN;

CREATE TYPE character_race AS ENUM (
    'Hyur',
    'Elezen',
    'Lalafell',
    'Miqote',
    'Roegadyn',
    'AuRa'
);

CREATE TYPE character_clan AS ENUM (
    'Midlander',
    'Highlander',
    'Wildwood',
    'Duskwight',
    'Plainsfolk',
    'Dunesfolk',
    'SunSeeker',
    'MoonKeeper',
    'SeaWolf',
    'Hellsguard',
    'Raen',
    'Xaela'
);

ALTER TABLE characters ADD COLUMN gender CHAR(1) NOT NULL DEFAULT ' ';

ALTER TABLE characters ADD COLUMN race character_race NOT NULL DEFAULT 'Hyur';
ALTER TABLE characters ALTER COLUMN race DROP DEFAULT;

ALTER TABLE characters ADD COLUMN clan character_clan NOT NULL DEFAULT 'Midlander';
ALTER TABLE characters ALTER COLUMN clan DROP DEFAULT;

COMMIT;
