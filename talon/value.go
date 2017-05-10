package talon

// String represents a raw string in Neo4j.
type String string

// Type returns EntityString, bringing String in line with Entity interface.
func (*String) Type() EntityType {
	return EntityString
}

// Int represents a raw integer value in Neo4j
type Int int64

// Type returns EntityInt, bringing Int in line with the Entity interface.
func (*Int) Type() EntityType {
	return EntityInt
}

// Float represents a raw float value in Neo4j
type Float float64

// Type returns EntityFloat, bringing Float in line with the Entity interface.
func (*Float) Type() EntityType {
	return EntityFloat
}

// Bool represents a raw bool value in Neo4j
type Bool bool

// Type returns EntityBool, bringing Bool in line with the Entity interface.
func (*Bool) Type() EntityType {
	return EntityBool
}

// Nil represents null in Neo4j
type Nil struct{}

// Type returns EntityNil, bringin Nil in line with the Entity interface.
func (*Nil) Type() EntityType {
	return EntityNil
}
