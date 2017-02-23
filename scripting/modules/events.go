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
//   emit(event: string[, data: table])
//     emits the given event with the data, which can be nil or omitted
//   on(event: string, handler: function)
//     registers the given function to handle the given event
//   once(event: string, handler: function)
//     registers the given function to handle the given event only one time
var Events = map[string]interface{}{
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

		if p, ok := engine.Meta[keys.Pool].(*pool.EnginePool); ok {
			go emitToPool(p, evt, data)
		} else {
			log("events").WithFields(logger.Fields{
				"event":  evt,
				"data":   data,
				"engine": engine,
			}).Error("Tried to emit when no pool is associated with the engine.")
		}

		return 0
	},
	"on": func(engine *lua.Engine) int {
		fn := engine.PopValue()
		evt := engine.PopValue().AsString()

		if evt != "" {
			emitter := emitterForEngine(engine)
			emitter.On(evt, &luaHandler{
				engine: engine,
				fn:     fn,
			})
		}

		return 0
	},
	"once": func(engine *lua.Engine) int {
		fn := engine.PopValue()
		evt := engine.PopValue().AsString()

		if evt != "" {
			emitter := emitterForEngine(engine)
			emitter.Once(evt, &luaHandler{
				engine: engine,
				fn:     fn,
			})
		}

		return 0
	},
}

func emitToPool(p *pool.EnginePool, evt string, data events.Data) {
	eng := p.Get()
	defer eng.Release()
	emitter := emitterForEngine(eng.Engine)
	done := emitter.Emit(evt, data)
	<-done
}

type luaHandler struct {
	engine *lua.Engine
	fn     *lua.Value
}

func (lh *luaHandler) Call(d events.Data) error {
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

func emitterForEngine(engine *lua.Engine) *events.Emitter {
	if em, ok := engine.Meta[keys.Emitter].(*events.Emitter); ok {
		return em
	}

	return newEmitterForEngine(engine)
}

func newEmitterForEngine(engine *lua.Engine) *events.Emitter {
	name := "emitter(engine(unknown))"
	if n, ok := engine.Meta[keys.EngineID].(string); ok {
		name = fmt.Sprintf("emitter(%s)", n)
	}

	log := logger.NewWithSource(name)
	em := events.NewEmitter(log)
	engine.Meta[keys.Emitter] = em

	return em
}
