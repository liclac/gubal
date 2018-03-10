package models

import (
	"time"

	"github.com/jinzhu/gorm"
)

//go:generate mockgen -package=models -source=character_tombstone.go -destination=character_tombstone.mock.go

// A CharacterTombstone represents a character that doesn't exist. These are created in lieau of a
// Character to prevent re-examining deleted characters, or the big ID gap between 1.x and 2.0.
type CharacterTombstone struct {
	ID        int64     `json:"id" gorm:"primary_key"`
	CreatedAt time.Time `json:"created_at"`
}

// CharacterTombstoneStore is a data access layer for CharacterTombstones.
type CharacterTombstoneStore interface {
	// Creates or updates a CharacterTombstone.
	Create(cID int64) error

	// Checks if there's a CharacterTombstone for a character.
	Check(cID int64) (bool, error)
}

type characterTombstoneStore struct {
	DB *gorm.DB
}

// NewCharacterTombstoneStore creates a new CharacterTombstoneStore.
func NewCharacterTombstoneStore(db *gorm.DB) CharacterTombstoneStore {
	return &characterTombstoneStore{db}
}

func (s *characterTombstoneStore) Create(cID int64) error {
	return s.DB.Exec(`INSERT INTO "character_tombstones" ("id", "created_at") VALUES (?, NOW()) ON CONFLICT DO NOTHING`, cID).Error
}

func (s *characterTombstoneStore) Check(cID int64) (bool, error) {
	var count int
	if err := s.DB.Model(CharacterTombstone{}).Where(CharacterTombstone{ID: cID}).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
