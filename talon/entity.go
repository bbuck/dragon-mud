// Copyright (c) 2016-2017 Brandon Buck

package talon

type EntityType uint8

const (
	EntityNode EntityType = iota
	EntityRelationship
)

type Entity interface {
	Type() EntityType
}
