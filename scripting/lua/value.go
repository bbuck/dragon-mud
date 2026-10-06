package lua

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	glua "github.com/yuin/gopher-lua"

	"bbuck.dev/dragon-mud/scripting"
)

// maxDepth limits how deeply nested tables are converted, which also stops
// self-referencing tables from recursing forever.
const maxDepth = 64

var errTooDeep = fmt.Errorf("nested more than %d levels deep", maxDepth)

// function is a Lua function held by Go.
type function struct {
	engine *Engine
	fn     *glua.LFunction
}

// Call invokes the Lua function with args and returns its first result.
func (f *function) Call(ctx context.Context, args ...any) (any, error) {
	results, err := f.call(ctx, 1, args)
	if err != nil {
		return nil, err
	}

	return results[0], nil
}

// Source is the file and line the function is defined at.
func (f *function) Source() string {
	if f.fn.Proto == nil {
		return ""
	}

	return fmt.Sprintf("%s:%d", f.fn.Proto.SourceName, f.fn.Proto.LineDefined)
}

// CallAll invokes the Lua function with args and returns every result.
func (f *function) CallAll(ctx context.Context, args ...any) ([]any, error) {
	return f.call(ctx, glua.MultRet, args)
}

// call invokes the function, keeping nret results (glua.MultRet for all).
func (f *function) call(ctx context.Context, nret int, args []any) ([]any, error) {
	e := f.engine

	lvArgs := make([]glua.LValue, len(args))
	for i, arg := range args {
		lv, err := e.toLua(arg, 0)
		if err != nil {
			return nil, fmt.Errorf("lua: argument #%d: %w", i+1, err)
		}
		lvArgs[i] = lv
	}

	restore := e.useContext(ctx)
	defer restore()

	base := e.state.GetTop()
	err := e.state.CallByParam(glua.P{Fn: f.fn, NRet: nret, Protect: true}, lvArgs...)
	if err != nil {
		return nil, e.callError(ctx, err)
	}

	n := e.state.GetTop() - base
	results := make([]any, n)
	for i := range n {
		value, err := e.fromLua(e.state.Get(base+i+1), 0)
		if err != nil {
			e.state.SetTop(base)
			return nil, fmt.Errorf("lua: result #%d: %w", i+1, err)
		}
		results[i] = value
	}
	e.state.SetTop(base)

	return results, nil
}

// fromLua converts a Lua value into a scripting boundary value. Numbers
// become float64 because Lua 5.1 has a single number type; Args.Int accepts
// integral floats.
func (e *Engine) fromLua(lv glua.LValue, depth int) (any, error) {
	if depth > maxDepth {
		return nil, errTooDeep
	}

	switch v := lv.(type) {
	case *glua.LNilType:
		return nil, nil
	case glua.LBool:
		return bool(v), nil
	case glua.LNumber:
		return float64(v), nil
	case glua.LString:
		return string(v), nil
	case *glua.LFunction:
		return &function{engine: e, fn: v}, nil
	case *glua.LTable:
		return e.tableFromLua(v, depth)
	case *glua.LUserData:
		if h, ok := handleFromLua(v); ok {
			return h, nil
		}
		return nil, errors.New("unsupported userdata")
	default:
		return nil, fmt.Errorf("unsupported Lua type %s", lv.Type())
	}
}

// tableFromLua converts a table to a list when its keys are exactly 1..n,
// and to a map otherwise. Number keys in a map are converted to strings.
func (e *Engine) tableFromLua(t *glua.LTable, depth int) (any, error) {
	count := 0
	t.ForEach(func(_, _ glua.LValue) { count++ })

	if n := t.Len(); n > 0 && n == count {
		list := make([]any, n)
		for i := range list {
			value, err := e.fromLua(t.RawGetInt(i+1), depth+1)
			if err != nil {
				return nil, err
			}
			list[i] = value
		}

		return list, nil
	}

	m := make(map[string]any, count)
	var err error
	t.ForEach(func(key, value glua.LValue) {
		if err != nil {
			return
		}

		switch key.(type) {
		case glua.LString, glua.LNumber:
		default:
			err = fmt.Errorf("unsupported table key type %s", key.Type())
			return
		}

		m[key.String()], err = e.fromLua(value, depth+1)
	})
	if err != nil {
		return nil, err
	}

	return m, nil
}

// toLua converts a Go value into a Lua value.
func (e *Engine) toLua(value any, depth int) (glua.LValue, error) {
	if depth > maxDepth {
		return nil, errTooDeep
	}

	switch v := value.(type) {
	case nil:
		return glua.LNil, nil
	case bool:
		return glua.LBool(v), nil
	case string:
		return glua.LString(v), nil
	case int:
		return glua.LNumber(v), nil
	case int64:
		return glua.LNumber(v), nil
	case float64:
		return glua.LNumber(v), nil
	case *function:
		if v.engine != e {
			return nil, errors.New("function belongs to a different engine")
		}

		return v.fn, nil
	case scripting.Function:
		return nil, errors.New("function belongs to a different engine")
	case scripting.Func:
		return e.wrap("function", v), nil
	case scripting.Module:
		return e.moduleTable(v)
	case scripting.Handle:
		return e.handleToLua(v)
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return glua.LNumber(rv.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return glua.LNumber(rv.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return glua.LNumber(rv.Float()), nil
	case reflect.Slice, reflect.Array:
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			return glua.LNil, nil
		}

		table := e.state.CreateTable(rv.Len(), 0)
		for i := range rv.Len() {
			lv, err := e.toLua(rv.Index(i).Interface(), depth+1)
			if err != nil {
				return nil, err
			}
			table.RawSetInt(i+1, lv)
		}

		return table, nil
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("unsupported map key type %s", rv.Type().Key())
		}
		if rv.IsNil() {
			return glua.LNil, nil
		}

		table := e.state.CreateTable(0, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			lv, err := e.toLua(iter.Value().Interface(), depth+1)
			if err != nil {
				return nil, err
			}
			table.RawSetString(iter.Key().String(), lv)
		}

		return table, nil
	default:
		return nil, fmt.Errorf("unsupported Go type %T", value)
	}
}
