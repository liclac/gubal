package models

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jinzhu/gorm"
)

func buildConflictAssignments(mod interface{}, excludeID bool, exclude ...string) string {
	if excludeID {
		exclude = append(exclude, "id")
	}
	exclude = append(exclude, "created_at", "deleted_at")

	cols := walkColumnNames(reflect.TypeOf(mod), exclude...)
	for i, col := range cols {
		cols[i] = fmt.Sprintf(`%s=EXCLUDED.%s`, col, col)
	}
	return strings.Join(cols, ", ")
}

func walkColumnNames(t reflect.Type, exclude ...string) (cols []string) {
fieldLoop:
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		switch {
		case f.PkgPath != "":
			// skip over unexported fields
			continue
		case f.Anonymous:
			// recurse through embedded types
			cols = append(cols, walkColumnNames(f.Type, exclude...)...)
			continue
		}
		name := gorm.ToDBName(f.Name)
		for _, exclusion := range exclude {
			if name == exclusion {
				continue fieldLoop
			}
		}
		cols = append(cols, name)
	}
	return
}
