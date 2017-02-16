package modules

import (
	"errors"

	"github.com/Sirupsen/logrus"
	"github.com/bbuck/dragon-mud/events"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/scripting/engine"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/pool"
)

var eventsLog = logger.LogWithSource("lua(events)")

// Events is a module for emitting and receiving events in Lua.
var Events = map[string]interface{}{
	"Halt": events.ErrHalt,
	"emit": func(engine *engine.Lua) int {
		dataVal := engine.PopArg()
		evt := engine.PopArg().AsString()

		var data events.Data
		if dataVal.IsTable() {
			data = events.Data(dataVal.ToMap())
		}

		lpool := engine.GetGlobal(keys.Pool)
		if p, ok := lpool.Interface().(*pool.EnginePool); ok {
			go emitToPool(p, evt, data)
		} else {
			eventsLog.WithFields(logrus.Fields{
				"event":  evt,
				"data":   data,
				"engine": engine,
			}).Error("Tried to emit when no pool is associated with the engine.")
		}

		return 0
	},
	"on": func(engine *engine.Lua) int {
		fn := engine.PopArg()
		evt := engine.PopArg().AsString()

		if evt != "" {
			emitter := emitterForEngine(engine)
			emitter.On(evt, &luaHandler{
				engine: engine,
				fn:     fn,
			})
		}

		return 0
	},
	"once": func(engine *engine.Lua) int {
		fn := engine.PopArg()
		evt := engine.PopArg().AsString()

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
	emitter := emitterForEngine(eng.Lua)
	done := emitter.Emit(evt, data)
	<-done
}

type luaHandler struct {
	engine *engine.Lua
	fn     *engine.LuaValue
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

func emitterForEngine(engine *engine.Lua) *events.Emitter {
	lem := engine.GetGlobal(keys.Emitter)
	if em, ok := lem.Interface().(*events.Emitter); ok {
		return em
	}

	return newEmitterForEngine(engine)
}

func newEmitterForEngine(engine *engine.Lua) *events.Emitter {
	name := engine.GetGlobal(keys.EngineID).AsString()
	if name == "" {
		name = "Unknown Engine"
	}

	em := events.NewEmitter(name)
	engine.SetGlobal(keys.Emitter, em)
	engine.WhitelistFor(em)

	return em
}
