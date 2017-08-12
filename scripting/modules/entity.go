package modules

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/bbuck/dragon-mud/data"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/keys"
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

// inspect allows a display for REPL behaviors listing the applied extentions
// and components on the entity.
func entityMasterInspect(engine *lua.Engine) int {
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
