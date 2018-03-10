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

	t := reflect.TypeOf(mod)

	var cols []string
fieldLoop:
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			continue
		}
		name := gorm.ToDBName(f.Name)
		for _, exclusion := range exclude {
			if name == exclusion {
				continue fieldLoop
			}
		}
		cols = append(cols, fmt.Sprintf(`%s=EXCLUDED.%s`, name, name))
	}
	return strings.Join(cols, ", ")
}
