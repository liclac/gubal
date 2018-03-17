package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLevelStore(t *testing.T) {
	tx := TestDB.Begin()
	defer tx.Rollback()

	// Create a test user.
	chStore := NewCharacterStore(tx)
	ch := &Character{ID: 12345, FirstName: "First", LastName: "Last"}
	require.NoError(t, chStore.Save(ch))

	// Add a level.
	store := NewLevelStore(tx)
	lvl := &Level{CharacterID: ch.ID, Job: PLD, Level: 30}
	require.NoError(t, store.Set(lvl))

	// Update a level.
	lvl.Level = 31
	require.NoError(t, store.Set(lvl))
}
