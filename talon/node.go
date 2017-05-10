// Copyright (c) 2016-2017 Brandon Buck

package talon

import (
	bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

// Node represents a Neo4j Node type.
type Node struct {
	ID         int64
	Labels     []string
	Properties Properties
}

// convert the bolt node object into a GraphNode.
func wrapBoltNode(n bolt.Node) (*Node, error) {
	var err error
	p := Properties(n.Properties)
	p, err = p.UnmarshaledProperties()
	if err != nil {
		return nil, err
	}

	return &Node{
		ID:         n.NodeIdentity,
		Labels:     n.Labels,
		Properties: p,
	}, nil
}

// Type implements Entity for Node returning EntityNode
func (*Node) Type() EntityType {
	return EntityNode
}

// Get will fetch the property assocaited with the node, returning a bool
// to signify if the property did exist.
func (n *Node) Get(key string) (val interface{}, ok bool) {
	val, ok = n.Properties[key]

	return
}

// GetString performs the same function as get, except it handles fetching
// a string or string pointer value (if it is a string). Unlike Get, the type
// Get methods will return 'false' for it's second return if the value is not
// of the requested type.
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

	return "", false
}

// GetInt is similar to get except it handles fetching any integer type and
// converts it to an int64 (if it is any sized integer). Unlike Get, the type
// Get methods will return 'false' for it's second return if the value is not
// of the requested type.
func (n *Node) GetInt(key string) (int64, bool) {
	val, ok := n.Get(key)
	if ok {
		switch i := val.(type) {
		case int:
			return int64(i), ok
		case *int:
			return int64(*i), ok
		case int8:
			return int64(i), ok
		case *int8:
			return int64(*i), ok
		case int16:
			return int64(i), ok
		case *int16:
			return int64(*i), ok
		case int32:
			return int64(i), ok
		case *int32:
			return int64(*i), ok
		case int64:
			return i, ok
		case *int64:
			return *i, ok
		}
	}

	return 0, false
}

// GetFloat is similar to get except it attempts to convert the found value
// to a float64 (if it is a float32 or float64). Unlike Get, the type Get
// methods will return 'false' for it's second return if the value is not of
// the requested type.
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

// GetBool is similar to Get except it attempts to convert the found value
// to a bool (if it is a bool). Unlike Get, the type Get methods will return
// 'false' for it's second return if the value is not of the requested type.
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

// Set will assign the given value to the associated Key.
func (n *Node) Set(key string, val interface{}) {
	n.Properties[key] = val
}
