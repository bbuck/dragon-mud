package modules

import (
	"strings"
	"sync"
	"time"

	"github.com/bbuck/dragon-mud/data"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
)

// entityType converts a lua value into a go type, there are a few valid type
// functions.
type entityType struct {
	toGo  func(*lua.Value) interface{}
	toLua func(interface{}) interface{}
}

var typeFuncMap = map[string]*entityType{
	"string": &entityType{
		toGo: func(val *lua.Value) interface{} {
			return val.AsString()
		},
		toLua: func(iface interface{}) interface{} {
			return iface
		},
	},
	"number": &entityType{
		toGo: func(val *lua.Value) interface{} {
			return val.AsNumber()
		},
		toLua: func(iface interface{}) interface{} {
			return iface
		},
	},
	"time": &entityType{
		toGo: func(val *lua.Value) interface{} {
			var t time.Time
			if iv, ok := val.Interface().(*instantValue); ok {
				t = time.Time(*iv)
			}

			return t
		},
		toLua: func(iface interface{}) interface{} {
			if iface == nil {
				return nil
			}

			switch t := iface.(type) {
			case time.Time:
				iv := instantValue(t)

				return &iv
			case int64:
				tt := time.Unix(t, 0)
				iv := instantValue(tt)

				return &iv
			}

			return nil
		},
	},
	"table": &entityType{
		toGo: func(val *lua.Value) interface{} {
			return val.AsMapStringInterface()
		},
		toLua: func(iface interface{}) interface{} {
			return iface
		},
	},
	"boolean": &entityType{
		toGo: func(val *lua.Value) interface{} {
			return val.AsBool()
		},
		toLua: func(iface interface{}) interface{} {
			return iface
		},
	},
}

// fetch special properties from an entity
type propertyFunc func(engine *lua.Engine, e *entity) interface{}

// specialProperties defines a set of special meta properties that can be
// fetched from entities.
var specialProperties = map[string]propertyFunc{
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
	methods      map[string]*lua.Value
	statics      map[string]*lua.Value
	props        map[string]*entityType
	components   []string
	unnamedCount int
	mutex        *sync.Mutex
}

// EntityToComponentMap maps an Entity (by name) to a set of component functions.
// Component functions are mapped to provide quick access when looking up
// extend functions.
type EntityToComponentMap map[string]*ComponentMap

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
	ptrMethods := emmt.RawGet("ptr_methods")
	ptrMethods.RawSet("register_component", entityMasterRegisterComponent)
	ptrMethods.RawSet("extend", entityMasterExtend)
	ptrMethods.RawSet("inspect", entityMasterInspect)
	ptrMethods.RawSet("from", entityMasterFrom)
	emmt.RawSet("__eq", entityMasterEq)
	emmt.RawSet("__call", entityMasterCall)
	emmt.RawSet("__tostring", goLuaToString(entityMasterInspect))
	emmt.RawSet("__index", entityMasterIndex(emmt.RawGet("__index")))

	emt := engine.MetatableFor(entity{})
	ptrMethods = emt.RawGet("ptr_methods")
	ptrMethods.RawSet("inspect", entityInspect)
	emt.RawSet("__index", entityIndex(emt.RawGet("__index")))
	emt.RawSet("__newindex", entityNewIndex)
	emt.RawSet("__eq", entityEq)
	emt.RawSet("__tostring", goLuaToString(entityInspect))
}

func mapComponentTable(em *entityMaster, cm *ComponentMap, tbl *lua.Value, isComponent bool) {
	if isComponent {
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
	}

	tbl.Get("methods").ForEach(func(key, val *lua.Value) {
		if val.IsFunction() {
			cm.methods[key.AsString()] = val
		} else {
			log("entity").WithFields(logger.Fields{
				"entity":   em.label,
				"function": key.AsString(),
			}).Warn("Component function is not a function")
		}
	})

	tbl.Get("statics").ForEach(func(key, val *lua.Value) {
		if val.IsFunction() {
			cm.statics[key.AsString()] = val
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
func getEntityToComponentMapping(engine *lua.Engine) EntityToComponentMap {
	if ietcm, ok := engine.Meta[keys.EntityToComponentMapping]; ok {
		if etcm, ok := ietcm.(EntityToComponentMap); ok {
			return etcm
		}
	}

	etcm := make(EntityToComponentMap)
	engine.Meta[keys.EntityToComponentMapping] = etcm

	return etcm
}

// fetch the component map for the given entity label, for caching on an
// entity
func getComponentMap(engine *lua.Engine, entityLabel string) *ComponentMap {
	etcm := getEntityToComponentMapping(engine)
	if cm, ok := etcm[entityLabel]; ok {
		return cm
	}

	go func() {
		err := data.DB().CreateIndex(entityLabel, []string{"id"})
		if err != nil {
			log("entity").WithError(err).WithField("label", entityLabel).Error("Failed to creat an index for the new entity label.")
		}
	}()

	cm := &ComponentMap{
		methods:      make(map[string]*lua.Value),
		statics:      make(map[string]*lua.Value),
		props:        make(map[string]*entityType),
		components:   make([]string, 0),
		mutex:        new(sync.Mutex),
		unnamedCount: 1,
	}
	cm.props["id"] = typeFuncMap["string"]
	cm.props["created_at"] = typeFuncMap["time"]
	cm.props["updated_at"] = typeFuncMap["time"]
	etcm[entityLabel] = cm

	return cm
}
