package modules

import (
	"bytes"
	"fmt"
	"io"

	"github.com/bbuck/dragon-mud/data"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/talon"
	uuid "github.com/satori/go.uuid"
)

// entityMaster  represents the "static" version of the Entity, allowing
// loading/creating new instances of an entity.
type entityMaster struct {
	label string
}

// New creates a new instance of this entity for use.
func (em *entityMaster) New() *entity {
	node := talon.NewNode()
	node.AddLabel(em.label)
	node.Properties["id"] = uuid.NewV4().String()

	return &entity{
		master: em,
		node:   node,
	}
}

func (em *entityMaster) Find(id string) *entity {
	buf := new(bytes.Buffer)
	buf.WriteString("MATCH (n:")
	buf.WriteString(em.label)
	buf.WriteString(") WHERE n.id = $id RETURN n")

	db := data.DB()
	query, err := db.CypherP(buf.String(), talon.Properties{"id": id})
	if err != nil {
		log("entity").WithError(err).WithFields(logger.Fields{
			"id":    id,
			"label": em.label,
		}).Error("Failed to find entity with given ID")

		return nil
	}

	rows, err := query.Query()
	if err != nil {
		log("entity").WithError(err).WithFields(logger.Fields{
			"id":    id,
			"label": em.label,
		}).Error("Failed to find entity with given ID")

		return nil
	}

	row, err := rows.Next()
	if err != nil && err != io.EOF {
		log("entity").WithError(err).WithFields(logger.Fields{
			"id":    id,
			"label": em.label,
		}).Error("Failed to find entity with given ID")

		return nil
	}

	if err != io.EOF && row != nil {
		niface, exists := row.GetColumn("n")
		if exists {
			n := niface.(*talon.Node)
			e := em.New()
			e.node = n

			return e
		}
	}

	buf = new(bytes.Buffer)
	buf.WriteString("MATCH (n:")
	buf.WriteString(em.label)
	buf.WriteString(") WHERE n.id STARTS WITH $id RETURN n")

	query, err = db.CypherP(buf.String(), talon.Properties{"id": id})
	if err != nil {
		log("entity").WithError(err).WithFields(logger.Fields{
			"id":    id,
			"label": em.label,
		}).Error("Failed to find entity with given ID")

		return nil
	}

	rows, err = query.Query()
	if err != nil {
		log("entity").WithError(err).WithFields(logger.Fields{
			"id":    id,
			"label": em.label,
		}).Error("Failed to find entity with given ID")

		return nil
	}

	row, err = rows.Next()
	if err != nil && err != io.EOF {
		log("entity").WithError(err).WithFields(logger.Fields{
			"id":    id,
			"label": em.label,
		}).Error("Failed to find entity with given ID")

		return nil
	}

	if err != io.EOF && row != nil {
		niface, exists := row.GetColumn("n")
		if exists {
			n := niface.(*talon.Node)
			e := em.New()
			e.node = n

			return e
		}
	}

	return nil
}

// #################################################################################
// Lua module
// #################################################################################

// ptrMethods := emmt.RawGet("ptr_methods")
// ptrMethods.RawSet("register_component", entityMasterRegisterComponent)
// ptrMethods.RawSet("extend", entityMasterExtend)
// ptrMethods.RawSet("inspect", entityMasterInspect)
// ptrMethods.RawSet("from", entityMasterFrom)
// emmt.RawSet("__eq", entityMasterEq)
// emmt.RawSet("__call", entityMasterCall)
// emmt.RawSet("__tostring", goLuaToString(entityMasterInspect))
// emmt.RawSet("__index", entityMasterIndex(emmt.RawGet("__index")))

var entityMasterMap = &luaMapper{
	pointerMethods: map[string]interface{}{
		"register_component": nil,
		"extend":             nil,

		// inspect allows a display for REPL behaviors listing the applied extentions
		// and components on the entity.
		"inspect": func(engine *lua.Engine) int {
			indent := engine.PopString()
			em := engine.PopValue().Interface().(*entityMaster)

			buf := new(bytes.Buffer)
			buf.WriteString("<Entity \"")
			buf.WriteString(em.label)
			buf.WriteString("\">\n")

			cm := getComponentMap(engine, em.label)

			for _, comp := range cm.components {
				buf.WriteString(indent)
				buf.WriteString("  -> \"")
				buf.WriteString(comp)
				buf.WriteString("\"\n")
			}

			engine.PushValue(buf.String())

			return 1
		},
		"from": nil,
	},
}

// index overrides __index and will look up static methods, before falling back
// to the magical special __index added by the Lua engine.
func entityMasterIndex(idxFn *lua.Value) lua.ScriptFunction {
	return func(engine *lua.Engine) int {
		key := engine.PopString()
		self := engine.PopValue()

		em := self.Interface().(*entityMaster)
		cm := getComponentMap(engine, em.label)

		if val, ok := cm.statics[key]; ok {
			engine.PushValue(val)
		} else {
			vals, err := idxFn.Call(1, self, key)
			if err != nil || len(vals) == 0 {
				engine.PushValue(nil)
			} else {
				engine.PushValue(vals[0])
			}
		}

		return 1
	}
}

// eq overrides __eq on the entity master to enable comparison. An entity master
// is equal to another entity master if they both represent the same entity --
// if the label matches up.
func entityMasterEq(engine *lua.Engine) int {
	o2 := engine.PopValue()
	o1 := engine.PopValue()

	if !o1.IsUserData() || !o2.IsUserData() {
		engine.PushValue(false)

		return 1
	}

	var (
		e1, e2 *entityMaster
		ok     bool
	)
	if e1, ok = o1.Interface().(*entityMaster); !ok {
		engine.PushValue(false)

		return 1
	}

	if e2, ok = o2.Interface().(*entityMaster); !ok {
		engine.PushValue(false)

		return 1
	}

	engine.PushValue(e1.label == e2.label)

	return 1
}

// call lets you use the Entity master as a function to create a new instance,
// this is a syntactic alternative to new, so Entity:new() and Entity() do the
// same thing.
func entityMasterCall(engine *lua.Engine) int {
	emtbl := engine.PopValue()

	em := emtbl.Interface().(*entityMaster)
	engine.PushValue(em.New())

	return 1
}

// create a new entity instance seeded with the properties from the given
// table.
func entityMasterFrom(engine *lua.Engine) int {
	tbl := engine.PopValue()
	emval := engine.PopValue()

	em := emval.Interface().(*entityMaster)
	e := em.New()
	cm := getComponentMap(engine, em.label)

	tbl.ForEach(func(key, val *lua.Value) {
		pkey := key.AsString()
		if propType, exists := cm.props[pkey]; exists {
			e.node.Properties[pkey] = propType.toGo(val)
		}
	})
	engine.PushValue(e)

	return 1
}

// register_component is the heavy hitter, designed to add a complete set of
// functionality to an entity, properties, relationships and methods.
func entityMasterRegisterComponent(engine *lua.Engine) int {
	comp := engine.PopValue()
	emtbl := engine.PopValue()

	em := emtbl.Interface().(*entityMaster)
	cm := getComponentMap(engine, em.label)

	compName := comp.Get("name").AsString()
	if compName == "" {
		compName = fmt.Sprintf("Unnamed Component %d", cm.unnamedCount)
		cm.unnamedCount++
	}
	cm.components = append(cm.components, compName)

	mapComponentTable(em, cm, comp, true)

	return 0
}

// extend the entity with the table, same format as a component. The core
// difference between extend and register_component is that extend doesn't
// allow anything other than methods ('methods' and 'statics' keys). The
// 'properties' and 'relationships' will be ignored.
func entityMasterExtend(engine *lua.Engine) int {
	ext := engine.PopValue()
	emtbl := engine.PopValue()

	em := emtbl.Interface().(*entityMaster)
	cm := getComponentMap(engine, em.label)

	extName := ext.Get("name").AsString()
	if extName == "" {
		extName = fmt.Sprintf("Unnamed Extension %d", cm.unnamedCount)
		cm.unnamedCount++
	} else {
		extName += " (extension)"
	}
	cm.components = append(cm.components, extName)

	mapComponentTable(em, cm, ext, false)

	return 0
}
