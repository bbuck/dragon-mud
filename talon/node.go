// Copyright (c) 2016-2017 Brandon Buck

package talon

import (
	"bytes"
	"strings"

	bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver/structures/graph"
)

// UnsetID is an ID used when a blank node is created.
const UnsetID = -1

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

// NewNode creates a new instance of a node with an unset ID, no labels and
// an empty set of properties.
func NewNode() *Node {
	return &Node{
		ID:         UnsetID,
		Labels:     make([]string, 0),
		Properties: make(Properties),
	}
}

// IsNewRecord returns true if the nodes identifier is unset (meaning it hasn't
// been saved yet).
func (n *Node) IsNewRecord() bool {
	return n.ID == UnsetID
}

// AddLabel appends a label to the list of labels this node has.
func (n *Node) AddLabel(lbl string) {
	n.Labels = append(n.Labels, lbl)
}

// Get will fetch the property assocaited with the node, returning a bool
// to signify if the property did exist.
func (n *Node) Get(key string) interface{} {
	if val, ok := n.Properties[key]; ok {
		return val
	}

	return nil
}

// GetString performs the same function as get, except it handles fetching
// a string or string pointer value (if it is a string). Unlike Get, the type
// Get methods will return 'false' for it's second return if the value is not
// of the requested type.
func (n *Node) GetString(key string) (string, bool) {
	val, ok := n.Properties[key]
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
	val, ok := n.Properties[key]
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
	val, ok := n.Properties[key]
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
	val, ok := n.Properties[key]
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

// Save will persist this node in the Neo4j database provided. An important
// note to make about Save is that by default it just creates new nodes, in
// order for a node to be properly be persisted as a unique record you
// should provide your own 'id' property which will be automatically used
// in the query for uniqueness matters.
func (n *Node) Save(db *DB) error {
	_, hasId := n.Properties["id"]
	buf := new(bytes.Buffer)
	if hasId {
		buf.WriteString("MERGE (n:")
	} else {
		buf.WriteString("CREATE (n:")
	}
	buf.WriteString(strings.Join(n.Labels, ":"))
	if hasId {
		buf.WriteString(" {id: $id}")
	}
	buf.WriteString(") SET ")

	i := 0
	for key := range n.Properties {
		if strings.ToLower(key) == "id" {
			i++
			continue
		}
		buf.WriteString("n.")
		buf.WriteString(key)
		buf.WriteString(" = $")
		buf.WriteString(key)
		if i < len(n.Properties)-1 {
			buf.WriteString(",")
		}
		i++
	}

	cypher := buf.String()
	query, err := db.CypherP(cypher, n.Properties)
	if err != nil {
		return err
	}
	_, err = query.Exec()

	return err
}
