package modules

import (
	"fmt"

	"github.com/bbuck/dragon-mud/data"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/talon"
)

// TalonLoader will create the meta tables for talon.Row and talon.Rows for
// this Engine.
func TalonLoader(engine *lua.Engine) {
	loadTalonRow(engine)
	loadTalonRows(engine)

	engine.RegisterModule("talon", Talon)
}

// Talon is the core database Lua wrapper, giving the coder access to running
// queries against the database
var Talon = lua.TableMap{
	"exec": func(engine *lua.Engine) int {
		return 0
	},
	"query": func(engine *lua.Engine) int {
		query, err := getTalonQuery(engine)
		if err != nil {
			engine.RaiseError(err.Error())

			return 0
		}

		rows, err := query.Query()
		if err != nil {
			engine.RaiseError(err.Error())

			return 0
		}

		engine.PushValue(talonToLua(engine, rows))

		return 1
	},
}

// pull passed values off the engine and build the database query
func getTalonQuery(engine *lua.Engine) (*talon.Query, error) {
	var props map[string]interface{}
	if engine.StackSize() == 2 {
		props = engine.PopValue().AsMapStringInterface()
	}
	query := engine.PopString()

	if props == nil || len(props) == 0 {
		return data.DB().Cypher(query), nil
	}

	return data.DB().CypherP(query, talon.Properties(props))
}

// convert a talon type to a type valid in Lua
func talonToLua(engine *lua.Engine, v interface{}) interface{} {
	switch t := v.(type) {
	case *talon.Rows:
		return engine.NewUserData(t, engine.Meta[keys.TalonRowsMetatable])
	case *talon.Row:
		return engine.NewUserData(t, engine.Meta[keys.TalonRowMetatable])
	default:
		return v
	}
}

// this builds a lua table for a *talon.Row object containg a single get
// method.
func loadTalonRow(eng *lua.Engine) {
	mt := eng.NewTable()
	mt.Set("get", func(engine *lua.Engine) int {
		if engine.StackSize() < 2 {
			engine.RaiseError("not enough arguments passed")

			return 0
		}

		arg := engine.PopValue()
		row, ok := engine.PopValue().Interface().(*talon.Row)
		if !ok {
			engine.RaiseError("row value corrupted")

			return 0
		}

		if arg.IsNumber() {
			idx := int(arg.AsNumber())
			if dbVal, ok := row.GetIndex(idx); ok {
				engine.PushValue(talonToLua(engine, dbVal))

				return 1
			}
		}

		if str := arg.AsString(); str != "" {
			if dbVal, ok := row.GetColumn(str); ok {
				engine.PushValue(talonToLua(engine, dbVal))

				return 1
			}
		}

		engine.RaiseError("no value in row a %s", fmt.Sprint(arg.AsRaw()))

		return 0
	})
	mt.Set("__index", mt)

	eng.Meta[keys.TalonRowMetatable] = mt
}

// build a lua type for *talon.Rows that contains a next method.
func loadTalonRows(eng *lua.Engine) {
	mt := eng.NewTable()
	mt.Set("next", func(engine *lua.Engine) int {
		rows, ok := engine.PopValue().Interface().(*talon.Rows)
		if !ok {
			engine.RaiseError("rows value corrupted")

			return 0
		}

		row, err := rows.Next()
		if err != nil {
			engine.RaiseError(err.Error())

			return 0
		}

		engine.PushValue(talonToLua(engine, row))

		return 1
	})
	mt.Set("__index", mt)

	eng.Meta[keys.TalonRowsMetatable] = mt
}
