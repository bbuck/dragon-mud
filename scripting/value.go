package scripting

// Value represents a value in the scripting implementation.
type Value interface {
	// IsNumber will return true if the wrapped scripting value is a numeric
	// value.
	IsNumber() bool

	// IsString will return true if the wrapped scripting value is a string.
	IsString() bool

	// ToBoolean will coerce the underlying Go type according to the scripting
	// engine's rules (i.e. non nil values may resolve to true).
	ToBool() bool

	// ToNumber will return the wrapped scripting numeric value as a float64.
	// If the value cannot be represented as a number the second return value
	// will be false.
	ToNumber() (float64, bool)

	// ToString will convert the wrapped scripting type into a string.
	ToString() (string, bool)
}
