package models

import (
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/require"
)

type buildConflictAssignmentsTestType struct {
	gorm.Model // embedded, included

	Title         string `gorm:"unique_index"`
	RenamedField  string `gorm:"column:a_column;unique_index"`
	ManualExclude string // manually excluded
	anonymous     string // excluded
}

func Test_buildConflictAssignments(t *testing.T) {
	require.Equal(t, "updated_at=EXCLUDED.updated_at, title=EXCLUDED.title, a_column=EXCLUDED.a_column", buildConflictAssignments(buildConflictAssignmentsTestType{}, true, "manual_exclude"))
}
