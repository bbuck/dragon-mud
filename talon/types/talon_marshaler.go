// Copyright (c) 2016 Brandon Buck

package types

// Marshaler represents a type that can turn itself into a string representation
// suitable for use with Talon query building.
type Marshaler interface {
	MarshalTalon() ([]byte, error)
}

// Unmarshaler represents a type that is capable of converting a byte array
// into a Go valid representation of itself. If the byte array is malformed,
// then an error can be returned.
type Unmarshaler interface {
	UnmarshalTalon([]byte) error
}
