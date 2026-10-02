package scripting

import (
	"fmt"
	"math"
)

// Args are the arguments a script passed to a Func. Accessors take a
// zero-based index and return an error naming the argument (counted from 1,
// as script authors see it) when it is missing or has the wrong type.
type Args []any

// Len returns the number of arguments passed.
func (a Args) Len() int {
	return len(a)
}

// String returns argument i as a string.
func (a Args) String(i int) (string, error) {
	s, ok := a.get(i).(string)
	if !ok {
		return "", a.typeError(i, "string")
	}

	return s, nil
}

// Bool returns argument i as a bool. Only booleans are accepted; use Truthy
// for the script language's notion of truth.
func (a Args) Bool(i int) (bool, error) {
	b, ok := a.get(i).(bool)
	if !ok {
		return false, a.typeError(i, "boolean")
	}

	return b, nil
}

// Truthy reports whether argument i is anything other than nil or false. A
// missing argument is false.
func (a Args) Truthy(i int) bool {
	switch v := a.get(i).(type) {
	case nil:
		return false
	case bool:
		return v
	default:
		return true
	}
}

// Float returns argument i as a float64.
func (a Args) Float(i int) (float64, error) {
	switch v := a.get(i).(type) {
	case float64:
		return v, nil
	case int64:
		return float64(v), nil
	default:
		return 0, a.typeError(i, "number")
	}
}

// Int returns argument i as an int. Numbers with a fractional part are
// rejected rather than truncated.
func (a Args) Int(i int) (int, error) {
	switch v := a.get(i).(type) {
	case int64:
		return int(v), nil
	case float64:
		if v != math.Trunc(v) || v > math.MaxInt || v < math.MinInt {
			return 0, fmt.Errorf("argument #%d: expected integer, got %v", i+1, v)
		}

		return int(v), nil
	default:
		return 0, a.typeError(i, "integer")
	}
}

// List returns argument i as a list.
func (a Args) List(i int) ([]any, error) {
	l, ok := a.get(i).([]any)
	if !ok {
		return nil, a.typeError(i, "list")
	}

	return l, nil
}

// Map returns argument i as a map.
func (a Args) Map(i int) (map[string]any, error) {
	m, ok := a.get(i).(map[string]any)
	if !ok {
		return nil, a.typeError(i, "map")
	}

	return m, nil
}

// Function returns argument i as a script function.
func (a Args) Function(i int) (Function, error) {
	f, ok := a.get(i).(Function)
	if !ok {
		return nil, a.typeError(i, "function")
	}

	return f, nil
}

func (a Args) get(i int) any {
	if i < 0 || i >= len(a) {
		return nil
	}

	return a[i]
}

func (a Args) typeError(i int, want string) error {
	if i >= len(a) {
		return fmt.Errorf("argument #%d: expected %s, got nothing", i+1, want)
	}

	return fmt.Errorf("argument #%d: expected %s, got %s", i+1, want, TypeName(a[i]))
}

// TypeName returns the script-facing name of a boundary value's type.
func TypeName(v any) string {
	switch v.(type) {
	case nil:
		return "nil"
	case bool:
		return "boolean"
	case int64, float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "list"
	case map[string]any:
		return "map"
	case Function:
		return "function"
	default:
		return fmt.Sprintf("%T", v)
	}
}
