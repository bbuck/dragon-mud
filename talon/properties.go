// Copyright (c) 2016-2017 Brandon Buck

package talon

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"
)

// ErrNoNestedProperties is returned when you attempt to Marshal a Properties
// map that has a Properties map inside of it.
var ErrNoNestedProperties = errors.New("cannot nest properties within properties")

// ErrNoRawCollections is returned when a map or slice value (raw) is present
// inside of a Properties map. The correct resolution is to use a new type
// that implements Marshaler.
var ErrNoRawCollections = errors.New("raw maps and slices are not supported property values")

// GenUnmarshaler is a function that takes an interface value and returns a type
// that can unmarshal a string.
type GenUnmarshaler func() Unmarshaler

// Unmarshalers is a map of type keys to GenUnmarshaler functions for fetching
// types that convert strings into useful values for use.
var Unmarshalers = make(map[string]GenUnmarshaler)

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

// MarshaledProperties will attempt to TalonMarshal all property values that
// can be marshaled.
func (p Properties) MarshaledProperties() (Properties, error) {
	mp := make(Properties)
	for k, v := range p {
		switch t := v.(type) {
		case Marshaler:
			bs, err := t.MarshalTalon()
			if err != nil {
				return mp, err
			}
			mp[k] = string(bs)
		case complex128, complex64:
			bs, err := NewComplex(t).MarshalTalon()
			if err != nil {
				return nil, err
			}
			mp[k] = string(bs)
		case time.Time:
			mp[k] = t.UTC().Unix()
		case Properties:
			return nil, ErrNoNestedProperties
		}

		if kind := reflect.TypeOf(v).Kind(); kind == reflect.Map || kind == reflect.Slice {
			return nil, ErrNoRawCollections
		}

		if _, ok := mp[k]; !ok {
			mp[k] = v
		}
	}

	return mp, nil
}

// UnmarshaledProperties assumes that properties are raw strings from the
// database and examines them for potential type values.
func (p Properties) UnmarshaledProperties() (Properties, error) {
	up := make(Properties)

	for k, v := range p {
		switch t := v.(type) {
		case string:
			v, err := tryUnmarshalString(t)
			if err != nil {
				return nil, err
			}

			up[k] = v
		default:
			up[k] = v
		}
	}

	return up, nil
}

// try to unmarshal a string value with the given unmarshalers, or just return
// the string itself.
func tryUnmarshalString(val string) (interface{}, error) {
	if len(val) > 2 && val[1] == '!' {
		typeKey := val[0:1]
		if gu, ok := Unmarshalers[typeKey]; ok {
			u := gu()
			err := u.UnmarshalTalon([]byte(val))
			if err != nil {
				return nil, err
			}

			return u, nil
		}
	}

	return val, nil
}
