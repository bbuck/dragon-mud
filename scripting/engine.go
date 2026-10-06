// Package scripting defines a language-neutral interface for running plugin
// scripts. The engine exposes functionality to scripts as modules of Go
// functions; each scripting language (see scripting/lua) implements Engine
// and converts values between its own types and the Go values listed below.
//
// Values crossing the boundary are always one of:
//
//	nil
//	bool
//	int64 or float64 (Go to script also accepts any integer or float type)
//	string
//	[]any                 (a list)
//	map[string]any        (a map)
//	Function              (a script function, callable from Go)
//	Handle                (a reference to something Go owns; see Type)
//
// Go to script also accepts slices of any supported type, maps with string
// keys, and Func.
package scripting

import (
	"context"
	"io/fs"
)

// Engine runs scripts in one language. An Engine is not safe for concurrent
// use; it belongs to the game loop.
type Engine interface {
	// Load makes a module available to scripts through require(m.Name),
	// such as require("dragon.world"). It is an error to load a module whose
	// name is already in use. Modules aren't globals.
	Load(m Module) error

	// Run executes source. name identifies the script in error messages. If
	// ctx is cancelled or its deadline passes, the script is interrupted and
	// the returned error wraps ctx.Err().
	Run(ctx context.Context, name, source string) error

	// Eval executes source like Run and returns the script's return value.
	// Plugin files are evaluated this way: each returns a table the engine
	// registers.
	Eval(ctx context.Context, name, source string) (any, error)

	// Scope returns a scope for one plugin's scripts: globals of their own,
	// falling back to the engine's, and a require that loads the engine's
	// modules, the scope's own modules (which take precedence), and the
	// plugin's script modules in files. A module name's first part, such as
	// "dragon" in "dragon.world", is reserved for modules: require never
	// looks in files for it. dir describes where files are, such as
	// "game", for script names and error messages.
	Scope(dir string, files fs.FS, modules []Module) (Scope, error)

	// Close releases the engine's resources.
	Close()
}

// Scope evaluates scripts that share globals and modules, such as one
// plugin's files. Functions defined in a scope keep using it when they're
// called later.
type Scope interface {
	// Eval executes source in the scope like Engine.Eval.
	Eval(ctx context.Context, name, source string) (any, error)
}

// Module is a named group of functions and values exposed to scripts, such
// as die.roll or room.get.
type Module struct {
	Name   string
	Funcs  map[string]Func
	Values map[string]any
}

// Func is a Go function callable from scripts. Returning an error raises it
// as an error in the script. Return Results for several values.
type Func func(args Args) (any, error)

// Results are several values returned at once from a Func or Method, such
// as a value and a reason: Results{nil, "You can't go that way."}.
type Results []any

// Function is a script function held by Go, such as a hook handler a plugin
// registered. It can only be called on the engine that created it.
type Function interface {
	// Call invokes the function and returns its first result. If ctx is
	// cancelled or its deadline passes, the call is interrupted and the
	// returned error wraps ctx.Err(). When called from inside a running
	// script, the running script's context applies instead of ctx.
	Call(ctx context.Context, args ...any) (any, error)

	// CallAll is Call returning every result, for functions that return
	// several values, such as a value and an error message.
	CallAll(ctx context.Context, args ...any) ([]any, error)

	// Source is where the function is defined, such as
	// "game/handlers.lua:12", for messages; "" when that isn't known.
	Source() string
}
