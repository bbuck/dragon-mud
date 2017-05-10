// Copyright (c) 2016-2017 Brandon Buck

package talon

type EntityType uint8

// Entity types for the various different kinds of return types from Neo. Like
// node, relationship, path, etc...
const (
	EntityNode EntityType = iota
	EntityRelationship
	EntityPath

	// various types associated to non graph types
	EntityString
	EntityInt
	EntityFloat
	EntityBool
	EntityNil

	// for marshaled types
	EntityComplex
	EntityTime
)

// Entity represents an element in a Neo4j query result, such a node,
// relationship, path or value (like a string, etc...)
type Entity interface {
	Type() EntityType
}
