// Copyright (c) 2016-2017 Brandon Buck

package talon

type EntityType uint8

// Entity types for the various different kinds of return types from Neo. Like
// node, relationship, path, etc...
const (
	EntityNode EntityType = iota
	EntityRelationship
	EntityPath
)

type Entity interface {
	Type() EntityType
}
