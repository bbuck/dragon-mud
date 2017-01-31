package scripting

import (
	"runtime"

	"github.com/bbuck/dragon-mud/scripting/engine"
)

type EngineSpawner func() *engine.Lua

type PooledEngine struct {
	*engine.Lua
	pool *EnginePool
}

func (pe *PooledEngine) Release() {
	if pe.Lua != nil {
		pe.pool.engines <- pe.Lua
		pe.Lua = nil
	}
}

type EnginePool struct {
	MaxPoolSize uint8
	spawnFn     EngineSpawner
	numEngines  uint8
	engines     chan *engine.Lua
}

func NewEnginePool(poolSize uint8, spawner EngineSpawner) *EnginePool {
	return &EnginePool{
		MaxPoolSize: poolSize,
		spawnFn:     spawner,
		numEngines:  0,
		engines:     make(chan *engine.Lua),
	}
}

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
