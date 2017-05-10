// Copyright (c) 2016-2017 Brandon Buck

package talon

import (
	bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

// Relationship represents a link between two nodes, it can come in two
// different kinds -- bounded or unbounded. If it's bounded then StartNodeID
// and EndNodeID will be set (and Bounded will be true). If it's unbounded
// these values will be 0.
type Relationship struct {
	ID          int64
	StartNodeID int64
	EndNodeID   int64
	Name        string
	Properties  Properties
	Bounded     bool
}

// for bounded relationships
func wrapBoltRelationship(r bolt.Relationship) (*Relationship, error) {
	var err error
	p := Properties(r.Properties)
	p, err = p.UnmarshaledProperties()
	if err != nil {
		return nil, err
	}

	return &Relationship{
		ID:          r.RelIdentity,
		StartNodeID: r.StartNodeIdentity,
		EndNodeID:   r.EndNodeIdentity,
		Name:        r.Type,
		Properties:  Properties(r.Properties),
		Bounded:     true,
	}, nil
}

// for unbounded relationships
func wrapBoltUnboundRelationship(r bolt.UnboundRelationship) (*Relationship, error) {
	var err error
	p := Properties(r.Properties)
	p, err = p.UnmarshaledProperties()
	if err != nil {
		return nil, err
	}

	return &Relationship{
		ID:         r.RelIdentity,
		Name:       r.Type,
		Properties: Properties(r.Properties),
		Bounded:    false,
	}, nil
}

// Type implements Entity for Relationship returning EntityRelationship
func (*Relationship) Type() EntityType {
	return EntityRelationship
}

// Get will fetch the property assocaited with the relationship, returning a
// bool to signify if the property did exist.
func (r *Relationship) Get(key string) (val interface{}, ok bool) {
	val, ok = r.Properties[key]

	return
}

// GetString performs the same function as get, except it handles fetching
// a string or string pointer value (if it is a string). Unlike Get, the type
// Get methods will return 'false' for it's second return if the value is not
// of the requested type.
func (r *Relationship) GetString(key string) (string, bool) {
	val, ok := r.Get(key)
	if ok {
		switch str := val.(type) {
		case string:
			return str, ok
		case *string:
			return *str, ok
		}
	}

	return "", ok
}

// GetInt is similar to get except it handles fetching any integer type and
// converts it to an int64 (if it is any sized integer). Unlike Get, the type
// Get methods will return 'false' for it's second return if the value is not
// of the requested type.
func (r *Relationship) GetInt(key string) (int64, bool) {
	val, ok := r.Get(key)
	if ok {
		switch i := val.(type) {
		case int:
			return int64(i), ok
		case int8:
			return int64(i), ok
		case int16:
			return int64(i), ok
		case int32:
			return int64(i), ok
		case int64:
			return i, ok
		}
	}

	return 0, ok
}

// GetFloat is similar to get except it attempts to convert the found value
// to a float64 (if it is a float32 or float64). Unlike Get, the type Get
// methods will return 'false' for it's second return if the value is not of
// the requested type.
func (r *Relationship) GetFloat(key string) (float64, bool) {
	val, ok := r.Get(key)
	if ok {
		switch f := val.(type) {
		case float32:
			return float64(f), ok
		case float64:
			return f, ok
		}
	}

	return 0, ok
}

// GetBool is similar to Get except it attempts to convert the found value
// to a bool (if it is a bool). Unlike Get, the type Get methods will return
// 'false' for it's second return if the value is not of the requested type.
func (r *Relationship) GetBool(key string) (bool, bool) {
	val, ok := r.Get(key)
	if ok {
		switch b := val.(type) {
		case bool:
			return b, ok
		}
	}

	return false, ok
}

// Set will assign the given value to the associated Key.
func (r *Relationship) Set(key string, val interface{}) {
	r.Properties[key] = val
}
