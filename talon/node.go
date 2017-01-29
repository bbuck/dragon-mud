// Copyright (c) 2016 Brandon Buck

package talon

import (
	"github.com/bbuck/talon/types"
	bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

type Node struct {
	ID         int64
	Labels     []string
	Properties types.Properties
}

func wrapBoltNode(n bolt.Node) *Node {
	return &Node{
		ID:         n.NodeIdentity,
		Labels:     n.Labels,
		Properties: types.Properties(n.Properties),
	}
}

func (*Node) Type() EntityType {
	return EntityNode
}

func (n *Node) Get(key string) (val interface{}, ok bool) {
	val, ok = n.Properties[key]

	return
}

func (n *Node) GetString(key string) (string, bool) {
	val, ok := n.Get(key)
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

func (n *Node) GetInt(key string) (int64, bool) {
	val, ok := n.Get(key)
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

func (n *Node) GetFloat(key string) (float64, bool) {
	val, ok := n.Get(key)
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

func (n *Node) GetBool(key string) (bool, bool) {
	val, ok := n.Get(key)
	if ok {
		switch b := val.(type) {
		case bool:
			return b, ok
		}
	}

	return false, ok
}

func (n *Node) Set(key string, val interface{}) {
	n.Properties[key] = val
}
