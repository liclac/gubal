package models

import (
	"testing"

	"github.com/jinzhu/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCharacterStore(t *testing.T) {
	tx := TestDB.Begin()
	defer tx.Rollback()

	store := NewCharacterStore(tx)

	id := int64(1234)

	// Getting a nonexistent character should error.
	t.Run("Nonexistent", func(t *testing.T) {
		_, err := store.Get(id)
		require.EqualError(t, err, "record not found")
		assert.True(t, gorm.IsRecordNotFoundError(err))
	})

	t.Run("Create", func(t *testing.T) {
		require.NoError(t, store.Save(&Character{
			ID:        id,
			FirstName: "First",
			LastName:  "Last",
		}))

		t.Run("Get", func(t *testing.T) {
			ch, err := store.Get(id)
			require.NoError(t, err)
			assert.Equal(t, id, ch.ID)
			assert.Equal(t, "First", ch.FirstName)
			assert.Equal(t, "Last", ch.LastName)

			t.Run("Save", func(t *testing.T) {
				ch.FirstName = "NewFirst"
				ch.LastName = "NewLast"
				require.NoError(t, store.Save(ch))

				t.Run("Get", func(t *testing.T) {
					ch, err := store.Get(id)
					require.NoError(t, err)
					assert.Equal(t, id, ch.ID)
					assert.Equal(t, "NewFirst", ch.FirstName)
					assert.Equal(t, "NewLast", ch.LastName)
				})
			})
		})
	})
}
