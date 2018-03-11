package models

// CharacterRace is a constant type for a character's race.
type CharacterRace string

// CharacterClan is a constant type for a character's clan.
type CharacterClan string

// CharacterRace constants.
const (
	Hyur     CharacterRace = "Hyur"
	Elezen   CharacterRace = "Elezen"
	Lalafell CharacterRace = "Lalafell"
	Miqote   CharacterRace = "Miqote"
	Roegadyn CharacterRace = "Roegadyn"
	AuRa     CharacterRace = "AuRa"
)

// CharacterClan constants.
const (
	HyurMidlander      CharacterClan = "Midlander"
	HyurHighlander     CharacterClan = "Highlander"
	ElezenWildwood     CharacterClan = "Wildwood"
	ElezenDuskwight    CharacterClan = "Duskwight"
	LalafellPlainsfolk CharacterClan = "Plainsfolk"
	LalafellDunesfolk  CharacterClan = "Dunesfolk"
	MiqoteSunSeeker    CharacterClan = "SunSeeker"
	MiqoteMoonKeeper   CharacterClan = "MoonKeeper"
	RoegadynSeaWolf    CharacterClan = "SeaWolf"
	RoegadynHellsguard CharacterClan = "Hellsguard"
	AuRaRaen           CharacterClan = "Raen"
	AuRaXaela          CharacterClan = "Xaela"
)
