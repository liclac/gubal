package models

import (
	"time"

	"github.com/jinzhu/gorm"
)

//go:generate mockgen -package=models -source=level.go -destination=level.mock.go

// levelConflictAssignments is the update string to be passed to an ON CONFLICT DO UPDATE clause.
var levelConflictAssignments = buildConflictAssignments(Level{}, true)

// A Level records a character's level in a certain job. PK is (character_id, job).
type Level struct {
	CharacterID int64     `json:"character_id"`
	Job         Job       `json:"job"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Level int `json:"level"`
}

// A LevelStore is a data access layer for Levels.
type LevelStore interface {
	Get(cID int64, job Job) (*Level, error)
	Set(lvl *Level) error
}

type levelStore struct {
	DB *gorm.DB
}

// NewLevelStore creates a new LevelStore.
func NewLevelStore(db *gorm.DB) LevelStore {
	return &levelStore{db}
}

func (s *levelStore) Get(cID int64, job Job) (*Level, error) {
	lvl := Level{CharacterID: cID, Job: job}
	return &lvl, s.DB.FirstOrInit(&lvl, lvl).Error
}

func (s *levelStore) Set(lvl *Level) error {
	return s.DB.Set("gorm:insert_option", `ON CONFLICT (character_id, job) DO UPDATE SET `+levelConflictAssignments).Create(lvl).Error
}
