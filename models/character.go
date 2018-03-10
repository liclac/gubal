package models

import (
	"time"

	"github.com/jinzhu/gorm"
)

//go:generate mockgen -package=models -source=character.go -destination=character.mock.go

// characterConflictAssignments is the update string to be passed to an ON CONFLICT DO UPDATE clause.
var characterConflictAssignments = buildConflictAssignments(Character{}, true)

// A Character represents an FFXIV player character.
type Character struct {
	ID        int64     `json:"id" gorm:"primary_key"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// A CharacterStore is a data access layer for Characters.
type CharacterStore interface {
	// Returns the named character, or an error if it doesn't exist.
	Get(cID int64) (*Character, error)

	// Inserts or updates the character's record.
	Save(ch *Character) error
}

type characterStore struct {
	DB *gorm.DB
}

// NewCharacterStore creates a new CharacterStore.
func NewCharacterStore(db *gorm.DB) CharacterStore {
	return &characterStore{db}
}

func (s *characterStore) Get(cID int64) (*Character, error) {
	var ch Character
	if err := s.DB.First(&ch, Character{ID: cID}).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

func (s *characterStore) Save(ch *Character) error {
	return s.DB.Set("gorm:insert_option", `ON CONFLICT (id) DO UPDATE SET `+characterConflictAssignments).Create(ch).Error
}
