package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCharacterTitleStore(t *testing.T) {
	tx := TestDB.Begin()
	defer tx.Rollback()

	store := NewCharacterTitleStore(tx)
	title := "Khloe's Friend"

	tl, err := store.GetOrCreate(title)
	require.NoError(t, err)
	require.NotNil(t, tl)
	assert.NotZero(t, tl.ID)
	assert.Equal(t, title, tl.Title)

	tl2, err := store.GetOrCreate(title)
	require.NoError(t, err)
	assert.Equal(t, tl2.ID, tl.ID)
	assert.Equal(t, tl2.CreatedAt.Unix(), tl.CreatedAt.Unix())
	assert.Equal(t, tl2.Title, tl.Title)
}
