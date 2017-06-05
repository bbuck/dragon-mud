package scripting

import (
	"fmt"
	"math"
	"sync/atomic"

	"github.com/bbuck/dragon-mud/events"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/plugins"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/spf13/viper"
)

var (
	// ServerPool is the global pool for server Lua engines. unlike Client and
	// Entity pools which are unique to their resource (1 per resource).
	ServerPool *lua.EnginePool

	// ServerEmitter is a server-level event emitter, it will ping the
	// ServerPool.
	ServerEmitter *events.Emitter

	// ClientEmitter is a client-level event emitter, it will ping every client
	// pool.
	ClientEmitter *events.Emitter

	// EntityEmitter is an entity-level event emitter, it will ping every
	// entity pool in the game (potentially a large number of entities).
	EntityEmitter *events.Emitter
)

func init() {
	size := viper.GetInt("scripting.server.engine_pool_size")
	if size < 0 {
		size = 0
	}
	if size > int(math.MaxUint8) {
		size = int(math.MaxUint8)
	}
	usize := uint8(size)
	ServerPool = lua.NewEnginePool(usize, ServerEngineMutator)
}

var serverID uint64 = 1

// GlobalEmit will emit to all tiers of engines, primarily used for tick
// emissions from the server.
func GlobalEmit(evt string, data events.Data) {
	ServerEmitter.Emit(evt, data)
	ClientEmitter.Emit(evt, data)
	EntityEmitter.Emit(evt, data)
}

// ServerEngineMutator is a mutator function for the server EnginePool to use
// to "build" a server engine.
func ServerEngineMutator(eng *lua.Engine) {
	id := atomic.LoadUint64(&serverID)
	atomic.AddUint64(&serverID, 1)
	engineID := fmt.Sprintf("server_engine(%d)", id)
	eng.Meta[keys.EngineID] = engineID

	eng.SecureRequire(plugins.GetScriptLoadPaths())
	OpenLibs(eng, "*")

	eng.SetGlobal("global_emit", GlobalEmit)
	log := logger.NewWithSource(engineID)
	eng.SetGlobal("print", log.Info)

	err := plugins.LoadServer(eng)
	if err != nil {
		eng.RaiseError(err.Error())
	}
}

// func initialize() {
// 	size := viper.GetInt("scripting.server.engine_pool_size")
// 	if size < 0 {
// 		size = 0
// 	}
// 	if size > int(math.MaxUint8) {
// 		size = int(math.MaxUint8)
// 	}
// 	usize := uint8(size)
// 	EnginePool = lua.NewEnginePool(usize, newServerEngine)
// }

// // Emit will emit a server event to the server scripts.
// func Emit(evt string, d events.Data) events.Done {
// 	done := make(events.Done)
// 	go func() {
// 		eng := EnginePool.Get()
// 		defer eng.Release()
// 		if em, ok := eng.Meta[keys.Emitter].(*events.Emitter); ok {
// 			d := em.Emit(evt, d)
// 			<-d
// 		}
// 		close(done)
// 	}()

// 	return done
// }

// // generate a new lua engine ready for use by server code.
// func newServerEngine(eng *lua.Engine) {
// 	id := atomic.LoadUint64(&serverID)
// 	atomic.AddUint64(&serverID, 1)

// 	engID := fmt.Sprintf("server_engine(%d)", id)
// 	eng.Meta[keys.EngineID] = engID

// 	eng.OpenMath()
// 	eng.OpenString()
// 	eng.OpenTable()
// 	scripting.OpenLibs(eng, "events", "random", "die", "log", "password",
// 		"sutil", "tmpl")
// }
