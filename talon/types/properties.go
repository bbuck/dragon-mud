// Copyright (c) 2016 Brandon Buck

package types

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"time"
)

// Properties is a map[string]interface{} wrapper with a special string function
// designed to produce properties for Neo4j.
type Properties map[string]interface{}

// String brings Properties inline with fmt.Stringer
func (p Properties) String() string {
	return fmt.Sprintf("%+v", map[string]interface{}(p))
}

// QueryString produces a string of key: {key} mappings based on the structure of
// this object for use in queries.
func (p Properties) QueryString() string {
	if len(p) == 0 {
		return ""
	}

	buf := new(bytes.Buffer)

	buf.WriteRune('{')
	keys := p.Keys()
	for i, key := range keys {
		buf.WriteString(key)
		buf.WriteString(": {")
		buf.WriteString(key)
		buf.WriteRune('}')
		if i != len(keys)-1 {
			buf.WriteString(", ")
		}
	}
	buf.WriteRune('}')

	return buf.String()
}

// Keys returns an array of string values representing the keys in the map.
func (p Properties) Keys() []string {
	keys := make([]string, len(p))
	i := 0
	for key := range p {
		keys[i] = key
		i++
	}
	sort.Strings(keys)

	return keys
}

// Merge merges the current Properties key/value pairs with those of the given
// Properties object. This does not modify the current or other input objects
// it instead returns a new Property map representing the merged properties.
func (p Properties) Merge(other Properties) Properties {
	props := make(Properties)
	for key, val := range p {
		props[key] = val
	}

	for key, val := range other {
		props[key] = val
	}

	return props
}

func marshalTalonValue(i interface{}) (interface{}, error) {
	if tm, ok := i.(Marshaler); ok {
		bs, err := tm.MarshalTalon()
		if err != nil {
			return nil, err
		}

		return string(bs), nil
	}

	val := reflect.ValueOf(i)
	switch val.Kind() {
	case reflect.Complex64, reflect.Complex128:
		c128 := val.Complex()
		c := Complex(c128)
		bs, err := c.MarshalTalon()
		if err != nil {
			return nil, err
		}

		return string(bs), nil
	}

	if t, ok := i.(time.Time); ok {
		tt := NewTime(t)
		bs, err := tt.MarshalTalon()
		if err != nil {
			return nil, err
		}

		return string(bs), nil
	}

	return i, nil
}
