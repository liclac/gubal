package models

import (
	"time"
)

// A Character represents an FFXIV player character.
type Character struct {
	ID        int64     `json:"id" gorm:"primary_key"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// A CharacterTombstone represents a character that doesn't exist. These are created in lieau of a
// Character to prevent re-examining deleted characters, or the big ID gap between 1.x and 2.0.
type CharacterTombstone struct {
	ID        int64     `json:"id" gorm:"primary_key"`
	CreatedAt time.Time `json:"created_at"`

	StatusCode int `json:"status_code"`
}
