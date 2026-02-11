package scripting

type Engine interface {
	// DoString executes the given Lua code.
	DoString(code string) error

	// Register will define the function given under the global name. The value
	// of fn must be a function.
	Register(name string, fn any) error
}
