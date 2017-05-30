package modules

import (
	"errors"
	"fmt"

	"github.com/bbuck/dragon-mud/events"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/bbuck/dragon-mud/scripting/pool"
)

// Events is a module for emitting and receiving events in Lua.
//   Halt: (go error)
//     used to halt event exuction, bypassing failure logs
//   emit(event[, data])
//     @param event: string = the event string value to be emitted to
//     @param data: table = a table of initial event properties to seed the
//       event emission.
//     emits the given event with the data, which can be nil or omitted
//   emit_once(event[, data])
//     @param event: string = the event string value to be emitted to
//     @param data: table = a table of initial event properties to seed the
//       event emission.
//     emits the event, similar to #emit, but any future binding to the given
//     event will automatically be fired as this event has already been emitted,
//     this is perfect for initializiation or one time load notices
//   on(event, handler)
//     @param event: string = the event to associate the given handler to.
//     @param handler: function = a function to execute if the event specified
//       is emitted.
//     registers the given function to handle the given event
//   once(event, handler: function)
//     @param event: string = the event to associate the given handler to.
//     @param handler: function = a function to execute if the event specified
//       is emitted.
//     registers the given function to handle the given event only one time
var Events = lua.TableMap{
	"Halt": events.ErrHalt,
	"emit": func(engine *lua.Engine) int {
		dataVal := engine.Nil()
		if engine.StackSize() >= 2 {
			dataVal = engine.PopValue()
		}
		evt := engine.PopValue().AsString()

		var data events.Data
		if dataVal.IsTable() {
			data = events.Data(dataVal.AsMapStringInterface())
		}

		go emitEvent(engine, evt, data)

		return 0
	},
	"emit_once": func(engine *lua.Engine) int {
		dataVal := engine.Nil()
		if engine.StackSize() >= 2 {
			dataVal = engine.PopValue()
		}
		evt := engine.PopValue().AsString()

		var data events.Data
		if dataVal.IsTable() {
			data = events.Data(dataVal.AsMapStringInterface())
		}

		go emitOnceEvent(engine, evt, data)

		return 0
	},
	"on": func(engine *lua.Engine) int {
		fn := engine.PopValue()
		evt := engine.PopValue().AsString()

		if evt != "" {
			bindEvent(engine, fn, evt)
		}

		return 0
	},
	"once": func(engine *lua.Engine) int {
		fn := engine.PopValue()
		evt := engine.PopValue().AsString()

		if evt != "" {
			bindOnceEvent(engine, fn, evt)
		}

		return 0
	},
}

// emit an event to the external event handler
func emitEvent(eng *lua.Engine, evt string, data events.Data) {
	ee := externalEmitterForEngine(eng)

	ee.Emit(evt, data)
}

// emit an event to the external event handler, this uses EmitOnce to emit the
// event once and all future binders will be executed if the event has already
// been emitted.
func emitOnceEvent(eng *lua.Engine, evt string, data events.Data) {
	ee := externalEmitterForEngine(eng)

	ee.EmitOnce(evt, data)
}

// bind the event to the internal and external event emitters
func bindEvent(eng *lua.Engine, fn *lua.Value, evt string) {
	ie := internalEmitterForEngine(eng)
	go func() {
		ie.On(evt, &internalLuaHandler{
			engine: eng,
			fn:     fn,
		})
	}()

	ee := externalEmitterForEngine(eng)
	go func() {
		ee.On(evt, &externalLuaHandler{
			pool:  poolForEngine(eng),
			event: evt,
		})
	}()
}

// bind the event to the internal and external event emitters, this event should
// only be triggered one time.
func bindOnceEvent(eng *lua.Engine, fn *lua.Value, evt string) {
	ie := internalEmitterForEngine(eng)
	ie.Once(evt, &internalLuaHandler{
		engine: eng,
		fn:     fn,
	})

	ee := externalEmitterForEngine(eng)
	ee.Once(evt, &externalLuaHandler{
		pool:  poolForEngine(eng),
		event: evt,
	})
}

// ############################################################################
// internal event handling
// handle events within an engine
// ############################################################################

// wraps an engine and function value associated with that engine for carrying
// out execution of an event.
type internalLuaHandler struct {
	engine *lua.Engine
	fn     *lua.Value
}

// Call matches the events.Handler interface, allowing a Lua method to be called
// from the event system.
func (lh *internalLuaHandler) Call(d events.Data) error {
	tblData := lh.engine.TableFromMap(map[string]interface{}(d))
	vals, err := lh.fn.Call(1, tblData)
	if err != nil {
		return err
	}

	val := vals[0]
	if !val.IsNil() {
		if val.IsString() {
			return errors.New(val.AsString())
		} else if e, ok := val.Interface().(error); ok {
			return e
		}
	}

	return nil
}

// Source returns the pointer to the value, allowing internal lua handlers to
// be identified.
func (lh *internalLuaHandler) Source() interface{} {
	return lh.fn
}

// fetch the internal engine for the engine (or create one)
func internalEmitterForEngine(eng *lua.Engine) *events.Emitter {
	if e, ok := eng.Meta[keys.InternalEmitter].(*events.Emitter); ok {
		return e
	}

	log := logger.NewWithSource(fmt.Sprintf("internal_emitter(%s)", nameForEngine(eng)))

	e := events.NewEmitter(log)
	eng.Meta[keys.InternalEmitter] = e

	return e
}

// ############################################################################
// external event handling
// handle events from outside of an engine.
// ############################################################################

// registering the pool with the global pool events emiter happens here.
type externalLuaHandler struct {
	pool  *pool.EnginePool
	event string
}

// Call will seek to emit the event to an engine within this pool's internal
// emitter.
func (elh *externalLuaHandler) Call(d events.Data) error {
	emitToPool(elh.pool, elh.event, d)

	return nil
}

// Source returns the pool assicaited with this external handler allowing only
// one pool to be associated to any given event.
func (elh *externalLuaHandler) Source() interface{} {
	return elh.pool
}

// fetch the external (pool-based) event emitter for the engine, external
// emitters have to be pre-assigned and cannot be lazily created on the fly
// like internal event emitters.
func externalEmitterForEngine(eng *lua.Engine) *events.Emitter {
	if e, ok := eng.Meta[keys.ExternalEmitter].(*events.Emitter); ok {
		return e
	}

	log("events").WithField("engine", nameForEngine(eng)).Fatal("No external emitter defined for engine, cannot continue execution.")

	return nil
}

// send the event to an engine within the pool using that engines internal
// event emitter
func emitToPool(p *pool.EnginePool, evt string, data events.Data) {
	eng := p.Get()
	defer eng.Release()
	emitter := internalEmitterForEngine(eng.Engine)
	done := emitter.Emit(evt, data)
	<-done
}
