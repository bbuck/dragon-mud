package modules

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/talon"
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

	return &entity{
		master: em,
		node:   node,
	}
}

// Inspect presents a friendly value for the REPL representing that this is
// an entity master.
func (em *entityMaster) Inspect(indent string) string {
	return fmt.Sprintf("entity master %q", em.label)
}

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

// Inspect returns a friendly view for the Lua REPL that displays details about
// this type.
func (e *entity) Inspect(indent string) string {
	if e.node.IsNewRecord() {
		return fmt.Sprintf("new entity %q", e.master.label)
	}

	return fmt.Sprintf("entity %q #%d", e.master.label, e.node.ID)
}

// typeFunc converts a lua value into a go type, there are a few valid type
// functions.
type typeFunc func(*lua.Value) interface{}

var typeFuncMap = map[string]typeFunc{
	"string": func(val *lua.Value) interface{} {
		return val.AsString()
	},
	"number": func(val *lua.Value) interface{} {
		return val.AsNumber()
	},
	"time": func(val *lua.Value) interface{} {
		var t time.Time
		if iv, ok := val.Interface().(*instantValue); ok {
			t = time.Time(*iv)
		}

		return t
	},
	"table": func(val *lua.Value) interface{} {
		return val.AsMapStringInterface()
	},
	"boolean": func(val *lua.Value) interface{} {
		return val.AsBool()
	},
}

// fetch special properties from an entity
type propertyFunc func(engine *lua.Engine, e *entity) interface{}

// specialProperties defines a set of special meta properties that can be
// fetched from entities.
var specialProperties = map[string]propertyFunc{
	"__id__": func(engine *lua.Engine, e *entity) interface{} {
		return e.node.ID
	},
	"__properties__": func(engine *lua.Engine, e *entity) interface{} {
		cm := getComponentMap(engine, e.master.label)
		props := make([]string, len(cm.props))
		index := 0
		for k := range cm.props {
			props[index] = k
			index++
		}

		return engine.TableFromSlice(props)
	},
	"__label__": func(engine *lua.Engine, e *entity) interface{} {
		return e.master.label
	},
	"__components__": func(engine *lua.Engine, e *entity) interface{} {
		cm := getComponentMap(engine, e.master.label)

		return engine.TableFromSlice(cm.components)
	},
}

// ComponentMap maps an entity to a set of functions and valid properties.
// This provides quick look up for component values instead of iterating over
// lists. The properties here are used to determine what properties can actually
// be set and a type for that string.
type ComponentMap struct {
	fns          map[string]*lua.Value
	props        map[string]typeFunc
	components   []string
	unnamedCount int
	mutex        *sync.Mutex
}

// ComponentMapping maps an Entity (by name) to a set of component functions.
// Component functions are mapped to provide quick access when looking up
// extend functions.
type ComponentMapping map[string]*ComponentMap

// EntityModule represents the entity library with in the Lua plugin system.
// The EntityModule is a set of methods used to access/return entities around,
// containing methods to create entities, register components on them, etc...
var EntityModule = lua.TableMap{
	"get": func(engine *lua.Engine) int {
		lbl := engine.PopString()
		lbl = strings.ToLower(lbl)

		em := &entityMaster{
			label: lbl,
		}

		engine.PushValue(em)

		// create the mapping if it's not already created
		getComponentMap(engine, lbl)

		return 1
	},
}

// EntityLoader loads the entity module into the given engine, and configures
// the metatable for the Entity objects.
func EntityLoader(engine *lua.Engine) {
	engine.RegisterModule("entity", EntityModule)

	emmt := engine.MetatableFor(&entityMaster{})
	emmt.RawGet("ptr_methods").RawSet("register_component", entityMasterRegisterComponent)
	emmt.RawGet("ptr_methods").RawSet("extend", entityMasterExtend)
	emmt.RawSet("__eq", entityMasterEq)
	emmt.RawSet("__call", entityMasterCall)
	emmt.RawSet("__tostring", goToString)

	emt := engine.MetatableFor(entity{})
	emt.RawSet("__index", entityIndex)
	emt.RawSet("__newindex", entityNewIndex)
	emt.RawSet("__eq", entityEq)
	emt.RawSet("__tostring", goToString)
}

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
		typeFn typeFunc
		isProp bool
	)
	if typeFn, isProp = cm.props[key]; !isProp {
		engine.PushValue(nil)

		return 0
	}

	e.node.Properties[key] = typeFn(val)

	engine.PushValue(val)

	return 1
}

func entityIndex(engine *lua.Engine) int {
	key := engine.PopString()
	etbl := engine.PopValue()

	e := etbl.Interface().(*entity)

	// begin property search
	if propFn, isSpecial := specialProperties[key]; isSpecial {
		engine.PushValue(propFn(engine, e))

		return 1
	}

	if val, isSet := e.node.Properties[key]; isSet {
		engine.PushValue(val)

		return 1
	}
	// end property search

	// begin fn search
	cm := getComponentMap(engine, e.master.label)
	if lfn, isSet := cm.fns[key]; isSet {
		engine.PushValue(lfn)

		return 1
	}
	// end fn search

	engine.PushValue(nil)

	return 1
}

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

func entityMasterCall(engine *lua.Engine) int {
	emtbl := engine.PopValue()

	em := emtbl.Interface().(*entityMaster)
	engine.PushValue(em.New())

	return 1
}

func entityMasterRegisterComponent(engine *lua.Engine) int {
	comp := engine.PopValue()
	emtbl := engine.PopValue()

	em := emtbl.Interface().(*entityMaster)
	cm := getComponentMap(engine, em.label)

	compName := comp.Get("name").AsString()
	if compName == "" {
		compName = fmt.Sprintf("unnamed_component_%d", cm.unnamedCount)
		cm.unnamedCount++
	}
	cm.components = append(cm.components, compName)

	mapComponentTable(em, cm, comp)

	return 0
}

func entityMasterExtend(engine *lua.Engine) int {
	ext := engine.PopValue()
	emtbl := engine.PopValue()

	em := emtbl.Interface().(*entityMaster)
	cm := getComponentMap(engine, em.label)

	extName := ext.Get("name").AsString()
	if extName == "" {
		extName = fmt.Sprintf("unnamed_extension_%d", cm.unnamedCount)
		cm.unnamedCount++
	} else {
		extName += " (extension)"
	}
	cm.components = append(cm.components, extName)

	mapComponentTable(em, cm, ext)

	return 0
}

func mapComponentTable(em *entityMaster, cm *ComponentMap, tbl *lua.Value) {
	tbl.Get("properties").ForEach(func(key, val *lua.Value) {
		typ := val.AsString()
		if tf, ok := typeFuncMap[typ]; ok {
			cm.props[key.AsString()] = tf
		} else {
			log("entity").WithFields(logger.Fields{
				"entity":   em.label,
				"property": key.AsString(),
				"type":     typ,
			}).Warn("Component property has an unknown type")
		}
	})

	tbl.Get("methods").ForEach(func(key, val *lua.Value) {
		if val.IsFunction() {
			cm.fns[key.AsString()] = val
		} else {
			log("entity").WithFields(logger.Fields{
				"entity":   em.label,
				"function": key.AsString(),
			}).Warn("Component function is not a function")
		}
	})
}

// return a component mapping for the given engine, component maps should all
// be the same across engines (i.e. entity "a" has components "b" and "c" in
// all engines) but since lua.Value is not safe outside of the context of an
// engine this map has to exist seperately for every engine instance (YIKES).
func getComponentMapping(engine *lua.Engine) ComponentMapping {
	if iecm, ok := engine.Meta[keys.EntityComponentMapping]; ok {
		if ecm, ok := iecm.(ComponentMapping); ok {
			return ecm
		}
	}

	ecm := make(ComponentMapping)
	engine.Meta[keys.EntityComponentMapping] = ecm

	return ecm
}

// fetch the component map for the given entity label, for caching on an
// entity
func getComponentMap(engine *lua.Engine, entityLabel string) *ComponentMap {

	cm := getComponentMapping(engine)
	if ecm, ok := cm[entityLabel]; ok {
		return ecm
	}

	ecm := &ComponentMap{
		fns:        make(map[string]*lua.Value),
		props:      make(map[string]typeFunc),
		components: make([]string, 0),
		mutex:      new(sync.Mutex),
	}
	cm[entityLabel] = ecm

	return ecm
}
