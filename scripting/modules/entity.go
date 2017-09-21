package modules

import (
	"bytes"
	"strings"
	"time"

	"github.com/bbuck/dragon-mud/data"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/talon"
)

// Entity can represent anything within the game. The Entity itself is a base
// for accessing the Neo4j storage system and creating/accessing database
// values as well as managing relationships, etc... Entities have complex
// lua tables to be dynamic, as the Entity itself holds nothing more than it's
// base database calls and field accessors. Entities are extended via
// "components" which are just Lua tables with specific formats.
type entity struct {
	master *entityMaster
	node   *talon.Node
}

// Is determines if the entity instance is a specific kind of instance, like
// `my_ent:is("player")` for determine if it's a specific type of entity.
func (e *entity) Is(lbl string) bool {
	lbl = strings.ToLower(lbl)

	return e.master.label == lbl
}

// Save will persist the entity in the database.
func (e *entity) Save() bool {
	now := time.Now()
	if e.node.IsNewRecord() {
		e.node.Properties["created_at"] = now
	}
	e.node.Properties["updated_at"] = now
	err := e.node.Save(data.DB())
	if err != nil {
		log("entity").WithFields(logger.Fields{
			"type":       e.master.label,
			"properties": e.node.Properties,
		}).WithError(err).Error("Failed to persist the entity in the database.")
	}

	return err == nil
}

// Touch will set the updated_at value to the current time.
func (e *entity) Touch() {
	e.node.Properties["updated_at"] = time.Now()
}

// #################################################################################
// Lua module
// #################################################################################

// inspect displays the entity details (like if it's a new entity) and it's
// property values
func entityInspect(engine *lua.Engine) int {
	indent := engine.PopString()
	etbl := engine.PopValue()

	e := etbl.Interface().(*entity)
	cm := getComponentMap(engine, e.master.label)

	buf := new(bytes.Buffer)
	if e.node.IsNewRecord() {
		buf.WriteString("new entity \"")
		buf.WriteString(e.master.label)
		buf.WriteString("\"\n")
	} else {
		buf.WriteString("entity \"")
		buf.WriteString(e.master.label)
		buf.WriteString("\"\n")
	}

	for name, propType := range cm.props {
		buf.WriteString(indent)
		buf.WriteString("  ")
		buf.WriteString(name)
		buf.WriteString(" => ")
		val := engine.ValueFor(propType.toLua(e.node.Properties[name]))
		buf.WriteString(val.Inspect(indent + "  "))
		buf.WriteRune('\n')
	}

	engine.PushValue(buf.String())

	return 1
}

// newindex overrides __newindex and allows new assignments only to
// configured properties as specified by all associated components.
func entityNewIndex(engine *lua.Engine) int {
	val := engine.PopValue()
	key := engine.PopString()
	etbl := engine.PopValue()

	if val.IsFunction() {
		return 0
	}

	e := etbl.Interface().(*entity)
	cm := getComponentMap(engine, e.master.label)
	var (
		typ    *entityType
		isProp bool
	)
	if typ, isProp = cm.props[key]; !isProp {
		engine.PushValue(nil)

		return 0
	}
	e.node.Properties[key] = typ.toGo(val)

	engine.PushValue(val)

	return 1
}

// index overrides __index and looks up properties and methods from all the
// associated components and extensions.
func entityIndex(idxFn *lua.Value) lua.ScriptFunction {
	return func(engine *lua.Engine) int {
		key := engine.PopString()
		etbl := engine.PopValue()

		e := etbl.Interface().(*entity)

		// begin property search
		if propFn, isSpecial := specialProperties[key]; isSpecial {
			engine.PushValue(propFn(engine, e))

			return 1
		}

		cm := getComponentMap(engine, e.master.label)
		if val, isSet := e.node.Properties[key]; isSet {
			typ := cm.props[key]
			engine.PushValue(typ.toLua(val))

			return 1
		}
		// end property search

		// begin fn search
		if lfn, isSet := cm.methods[key]; isSet {
			engine.PushValue(lfn)

			return 1
		}
		// end fn search

		vals, err := idxFn.Call(1, etbl, key)
		if err != nil || len(vals) == 0 {
			engine.PushValue(nil)

			return 1
		}

		engine.PushValue(vals[0])

		return 1
	}
}

// eq overrides __eq for entities. An entity is equal to another entity if
// neither is a new record and they have same Node ID and they have the same
// label.
func entityEq(engine *lua.Engine) int {
	o2 := engine.PopValue()
	o1 := engine.PopValue()

	if !o1.IsUserData() || !o2.IsUserData() {
		engine.PushValue(false)

		return 1
	}

	var (
		e1, e2 *entity
		ok     bool
	)
	if e1, ok = o1.Interface().(*entity); !ok {
		engine.PushValue(false)

		return 1
	}

	if e2, ok = o2.Interface().(*entity); !ok {
		engine.PushValue(false)

		return 1
	}

	engine.PushValue(!e1.node.IsNewRecord() &&
		!e2.node.IsNewRecord() &&
		e1.master.label == e2.master.label &&
		e1.node.ID == e2.node.ID)

	return 1
}
