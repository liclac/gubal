BEGIN;

CREATE TYPE world AS ENUM (
    'Aegis',
    'Atomos',
    'Carbuncle',
    'Garuda',
    'Gungnir',
    'Kujata',
    'Ramuh',
    'Tonberry',
    'Typhon',
    'Unicorn',

    'Alexander',
    'Bahamut',
    'Durandal',
    'Fenrir',
    'Ifrit',
    'Ridill',
    'Tiamat',
    'Ultima',
    'Valefor',
    'Yojimbo',
    'Zeromus',

    'Anima',
    'Asura',
    'Belias',
    'Chocobo',
    'Hades',
    'Ixion',
    'Mandragora',
    'Masamune',
    'Pandemonium',
    'Shinryu',
    'Titan',

    'Adamantoise',
    'Balmung',
    'Cactuar',
    'Coeurl',
    'Faerie',
    'Gilgamesh',
    'Goblin',
    'Jenova',
    'Maetus',
    'Midgardsormr',
    'Sargatanas',
    'Siren',
    'Zalera',

    'Behemoth',
    'Brynhildr',
    'Diabolos',
    'Excalibur',
    'Exodus',
    'Famfrit',
    'Hyperion',
    'Lamia',
    'Leviathan',
    'Malboro',
    'Ultros',

    'Cerberus',
    'Lich',
    'Moogle',
    'Odin',
    'Phoenix',
    'Ragnarok',
    'Shiva',
    'Zodiark'
);

ALTER TABLE characters ADD COLUMN world world NOT NULL DEFAULT 'Zodiark';
ALTER TABLE characters ALTER COLUMN world DROP DEFAULT;

COMMIT;
