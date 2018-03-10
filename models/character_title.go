package models

import (
	"time"

	"github.com/jinzhu/gorm"
)

//go:generate mockgen -package=models -source=character_title.go -destination=character_title.mock.go

// CharacterTitle is a Character's Title, eg. "Khloe's Friend", "The Final Witness".
type CharacterTitle struct {
	ID        int       `json:"id" gorm:"primary_key"`
	CreatedAt time.Time `json:"created_at"`

	Title string `json:"title"`
}

// CharacterTitleStore is a data access layer for CharacterTitles.
type CharacterTitleStore interface {
	// GetOrCreate returns an existing CharacterTitle if there is one, or creates one.
	GetOrCreate(title string) (*CharacterTitle, error)
}

type characterTitleStore struct {
	DB *gorm.DB
}

// NewCharacterTitleStore creates a NewCharacterTitleStore.
func NewCharacterTitleStore(db *gorm.DB) CharacterTitleStore {
	return &characterTitleStore{db}
}

func (s *characterTitleStore) GetOrCreate(titleStr string) (*CharacterTitle, error) {
	var title CharacterTitle
	err := s.DB.FirstOrCreate(&title, CharacterTitle{Title: titleStr}).Error
	return &title, err
}
