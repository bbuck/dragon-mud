package scripting

import (
	"runtime"

	"github.com/bbuck/dragon-mud/scripting/engine"
)

// EngineSpawner is a function that builds a Lua scripting engine and returns
// the built engine. Used by the EnginePool to produce engines specific to
// the current instance of the EnginePool.
type EngineSpawner func() *engine.Lua

// PooledEngine wraps a Lua engine. It's purpose is provide a means with which
// to return the engine to the EnginePool when it's not longer being used.
type PooledEngine struct {
	*engine.Lua
	pool *EnginePool
}

// Release will push the engine back into the queue for available engines for
// the current PooledEngine as well as nil out the reference to the engine
// to prevent continued usage of the engine.
func (pe *PooledEngine) Release() {
	if pe.Lua != nil {
		pe.pool.engines <- pe.Lua
		pe.Lua = nil
	}
}

// EnginePool represents a grouping of predefined/preloaded engines that can be
// grabbed for use when Lua scripts need to run.
type EnginePool struct {
	MaxPoolSize uint8
	spawnFn     EngineSpawner
	numEngines  uint8
	engines     chan *engine.Lua
}

// NewEnginePool constructs a new pool with the specific maximum size and the
// engine spawner. It will seed the pool with one engine.
func NewEnginePool(poolSize uint8, spawner EngineSpawner) *EnginePool {
	if poolSize == 0 {
		poolSize = 1
	}
	ep := &EnginePool{
		MaxPoolSize: poolSize,
		spawnFn:     spawner,
		numEngines:  1,
		engines:     make(chan *engine.Lua),
	}
	ep.engines <- spawner()

	return ep
}

// Get will fetch the next available engine from the EnginePool. If no engines
// are available and the maximum number of active engines in the pool have been
// created yet then the spawner will be invoked to spawn a new engine and return
// that.
func (ep *EnginePool) Get() *PooledEngine {
	var engine *engine.Lua
	if len(ep.engines) > 0 {
		engine = <-ep.engines
	} else if ep.numEngines < ep.MaxPoolSize {
		engine = ep.spawnFn()
		ep.numEngines++
	} else {
		engine = <-ep.engines
	}

	pe := &PooledEngine{
		Lua:  engine,
		pool: ep,
	}
	// NOTE: precaution to prevent leaks for long running servers, not a perfect
	//       solution. BE DILIGENT AND RELEASE YOUR ENGINES!!
	runtime.SetFinalizer(pe, (*PooledEngine).Release)

	return pe
}
