package modules

import (
	"fmt"
	"io"

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
//   exec(cypher, properties): talon.Result
//     @param cypher: string - the cypher query to execute on the database
//       server
//     @param properties: table - properties to fill the cypher query with
//       before execution
//     @errors raises an error if there is an issue with the database connection
//       or the query construction
//     executes a query on the database and returns a result value, this should
//     be used with queries that don't return a result set (like creating,
//     editing and deleting).
//   query(cypher, properties): talon.Rows
//     @param cypher: string - the cypher query to execute on the database
//       server
//     @param properties: table - properties to fill the cypher query with
//       before execution
//     @errors raises an error if there is an issue with the database connection
//       or the query construction
//     executes a query on the database server and returns a set of rows with
//     the queries results.
//   talon.Rows
//     next(): talon.Row
//       the rowset is a lazy loaded series of rows, next will return the next
//       row in the set of rows returned by the query. If there are no more
//       rows this will return nil.
//     close()
//       close the set of rows, this is a _very_ good thing to do to clean up
//       after your queries. It is undefined behavior to fail to close your
//       row sets.
//     inspect(): string
//       return a debug view into the talon.Rows value.
//   talon.Row
//     get(key): any
//       @param key: string | number - the field name in the row or the field
//         index in the result.
//       this will return any value associated with the index or field name or
//       nil if no value is found.
//   talon.Node
//     @property id: number - numeric auto id number assigned to the node by
//       the database
//     @property labels: table - a list of labels associated with the node; not
//       quite a table, it's a Go slice but functions similarly
//     @property properties: table - a key/vale paring of properties associated
//       with the node; not quite a table, a Go map but functions like a table.
//     get(key): any
//       @param key: string - the name of the property to fetch from the node,
//         mostly a shorthand for accessing properties
//       fetch a property by key from the node.
//   talon.Relationship
//     @property id: number - numeric auto id number assigned to the node by
//       the database
//     @property start_node_id: number - the auto identifier of the node that
//       this relationship originates from
//     @property end_node_id: number - the auto identifier of the node that
//       this relationship executes at
//     @property name: string - the label of the label (or name)
//     @property properties: table - a key/vale paring of properties associated
//       with the node; not quite a table, a Go map but functions like a table.
//     @property bounded: boolean - denotes whether start_node_id and
//       end_node_id will be set. If a relationship is bounded then it's start
//       and end points are recorded, if it's not bounded they will not be set
//       (most likely returning 0).
//     get(key): any
//       @param key: string - the name of the property to fetch from the
//         relationship, mostly a shorthand for accessing properties
//       fetch a property by key from the relationship.
//   talon.Path
//     functionally a table (list) of alternating node/relationship values from
//     one node to another.
var Talon = lua.TableMap{
	"exec": func(engine *lua.Engine) int {
		query, err := getTalonQuery(engine)
		if err != nil {
			engine.RaiseError(err.Error())

			return 0
		}

		result, err := query.Exec()
		if err != nil {
			engine.RaiseError(err.Error())

			return 0
		}

		engine.PushValue(result)

		return 1
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

		// this is the _second_ argument passed to the function
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

	mt.Set("inspect", func(engine *lua.Engine) int {
		if engine.StackSize() < 1 {
			engine.RaiseError("not enough arguments passed")

			return 0
		}

		row, ok := engine.PopValue().Interface().(*talon.Row)
		if !ok {
			engine.RaiseError("row value corrupted")

			return 0
		}

		engine.PushValue(fmt.Sprintf("talon.Row(%+v)", row.Metadata.Fields))

		return 1
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
			if err == io.EOF {
				engine.PushValue(engine.Nil())

				return 1
			}

			engine.RaiseError(err.Error())

			return 0
		}

		engine.PushValue(talonToLua(engine, row))

		return 1
	})

	mt.Set("inspect", func(engine *lua.Engine) int {
		rows, ok := engine.PopValue().Interface().(*talon.Rows)
		if !ok {
			engine.RaiseError("rows value corrupted")

			return 0
		}

		engine.PushValue(fmt.Sprintf("talon.Rows(open = %t)", rows.IsOpen()))

		return 1
	})

	mt.Set("close", func(engine *lua.Engine) int {
		rows, ok := engine.PopValue().Interface().(*talon.Rows)
		if !ok {
			engine.RaiseError("rows value corrupted")

			return 0
		}

		rows.Close()

		return 0
	})

	mt.Set("__index", mt)

	eng.Meta[keys.TalonRowsMetatable] = mt
}
