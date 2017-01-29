// Copyright (c) 2016 Brandon Buck

package talon

import (
	"reflect"

	"github.com/bbuck/talon/types"
)

// convert an arbitrary struct into properties
func structToMap(i interface{}) types.Properties {
	value := reflect.ValueOf(i)
	typ := value.Type()
	props := make(types.Properties)
	if value.Kind() != reflect.Struct {
		return props
	}

	fc := value.NumField()
	for i := 0; i < fc; i++ {
		field := typ.Field(i)
		var key string
		if tag, ok := field.Tag.Lookup("talon"); ok {
			key = tag
		} else {
			key = field.Name
		}
		if key != "-" {
			props[key] = value.Field(i).Interface()
		}
	}

	return props
}
