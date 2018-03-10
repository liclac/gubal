package models

import (
	"github.com/jinzhu/gorm"
)

// A DataStore puts all the various kinds of stores in one place.
type DataStore interface {
	Characters() CharacterStore
	CharacterTombstones() CharacterTombstoneStore
}

type dataStore struct {
	characters          CharacterStore
	characterTombstones CharacterTombstoneStore
}

// NewDataStore creates a new DataStore, full of concrete data stores wrapping the given DB.
func NewDataStore(db *gorm.DB) DataStore {
	return &dataStore{
		characters:          NewCharacterStore(db),
		characterTombstones: NewCharacterTombstoneStore(db),
	}
}

func (ds *dataStore) Characters() CharacterStore {
	return ds.characters
}

func (ds *dataStore) CharacterTombstones() CharacterTombstoneStore {
	return ds.characterTombstones
}
