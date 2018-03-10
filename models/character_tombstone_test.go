package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCharacterTombstoneStore(t *testing.T) {
	tx := TestDB.Begin()
	defer tx.Rollback()

	store := NewCharacterTombstoneStore(tx)
	id := int64(1234)

	// If the character has no tombstone, checking it should return false.
	t.Run("None", func(t *testing.T) {
		dead, err := store.Check(id)
		require.NoError(t, err)
		assert.False(t, dead)
	})

	// Create a tombstone.
	t.Run("Create", func(t *testing.T) {
		require.NoError(t, store.Create(id))

		// If the character has a tombstone, checking it should return true.
		t.Run("Check", func(t *testing.T) {
			dead, err := store.Check(id)
			require.NoError(t, err)
			assert.True(t, dead)
		})

		// If the character has a tombstone, creating it again should do nothing.
		t.Run("Re-create", func(t *testing.T) {
			require.NoError(t, store.Create(id))

			// Re-creating an existing tombstone shouldn't touch it at all.
			t.Run("Check", func(t *testing.T) {
				dead, err := store.Check(id)
				require.NoError(t, err)
				assert.True(t, dead)
			})
		})
	})
}
