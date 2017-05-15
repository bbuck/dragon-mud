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

// UnmarshalType is a function that will take in a byte value and return an
// interface type (or error) when trying to Unmarshal.
type UnmarshalType func([]byte) (interface{}, error)

// Unmarshalers is a map of type keys to GenUnmarshaler functions for fetching
// types that convert strings into useful values for use.
var Unmarshalers = make(map[string]UnmarshalType)

func init() {
	Unmarshalers["P"] = func(bs []byte) (interface{}, error) {
		p := make(Properties)
		um := &p
		err := um.UnmarshalTalon(bs)

		return p, err
	}
}

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

// MarshalTalon will marshal the Properties object using JSON.
func (p Properties) MarshalTalon() ([]byte, error) {
	after, err := p.MarshaledProperties()
	if err != nil {
		return make([]byte, 0), err
	}

	j := JSON{Data: after}

	bs, err := j.MarshalTalon()
	if err != nil {
		return make([]byte, 0), err
	}

	bs[0] = 'P'

	return bs, nil
}

// UnmarshalTalon will convert a JSON value back into a properties value.
func (p *Properties) UnmarshalTalon(bs []byte) error {
	if bs[0] == 'P' {
		bs[0] = 'J'
	}

	j := JSON{}
	err := j.UnmarshalTalon(bs)
	if err != nil {
		return err
	}

	if m, ok := j.Data.(map[string]interface{}); ok {
		*p = Properties(m)

		return nil
	}

	return errors.New("invalid data format for properties")
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

			continue
		case complex128, complex64:
			bs, err := NewComplex(t).MarshalTalon()
			if err != nil {
				return nil, err
			}
			mp[k] = string(bs)

			continue
		case time.Time:
			mp[k] = t.Unix()

			continue
		}

		if kind := reflect.TypeOf(v).Kind(); kind == reflect.Map || kind == reflect.Slice {
			j := NewJSON(v)
			bs, err := j.MarshalTalon()
			if err != nil {
				return mp, err
			}
			mp[k] = string(bs)

			continue
		}

		mp[k] = v
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
		if uFn, ok := Unmarshalers[typeKey]; ok {
			v, err := uFn([]byte(val))
			if err != nil {
				return nil, err
			}

			return v, nil
		}
	}

	return val, nil
}
