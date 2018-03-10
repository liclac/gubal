package models

import (
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/require"
)

func Test_buildConflictAssignments(t *testing.T) {
	require.Equal(t, "created_at=EXCLUDED.created_at, updated_at=EXCLUDED.updated_at", buildConflictAssignments(gorm.Model{}, true, "deleted_at"))
}
