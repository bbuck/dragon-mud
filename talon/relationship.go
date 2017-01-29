package talon

import (
	bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"

	"github.com/bbuck/talon/types"
)

type Relationship struct {
	ID          int64
	StartNodeID int64
	EndNodeID   int64
	Name        string
	Properties  types.Properties
}

func wrapBoltRelationship(r bolt.Relationship) *Relationship {
	return &Relationship{
		ID:          r.RelIdentity,
		StartNodeID: r.StartNodeIdentity,
		EndNodeID:   r.EndNodeIdentity,
		Name:        r.Type,
		Properties:  types.Properties(r.Properties),
	}
}

func (*Relationship) Type() EntityType {
	return EntityRelationship
}

func (r *Relationship) Get(key string) (val interface{}, ok bool) {
	val, ok = r.Properties[key]

	return
}

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

func (r *Relationship) Set(key string, val interface{}) {
	r.Properties[key] = val
}
