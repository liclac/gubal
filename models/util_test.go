package models

import (
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/require"
)

type buildConflictAssignmentsTestType struct {
	gorm.Model // embedded, included

	Title         string // included
	ManualExclude string // manually excluded
	anonymous     string // excluded
}

func Test_buildConflictAssignments(t *testing.T) {
	require.Equal(t, "created_at=EXCLUDED.created_at, updated_at=EXCLUDED.updated_at, deleted_at=EXCLUDED.deleted_at, title=EXCLUDED.title", buildConflictAssignments(buildConflictAssignmentsTestType{}, true, "manual_exclude"))
}
