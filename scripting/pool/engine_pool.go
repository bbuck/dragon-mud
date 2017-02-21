// Copyright (c) 2016-2017 Brandon Buck

package pool

import (
	"runtime"
	"sync"

	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
)

// EngineMutator will modify an Engine before it goes into the pool. This can
// run any number of scripts as necessary such as registring libraries,
// executing code, etc...
type EngineMutator func(*lua.Engine)

// PooledEngine wraps a Lua engine. It's purpose is provide a means with which
// to return the engine to the EnginePool when it's not longer being used.
type PooledEngine struct {
	*lua.Engine
	pool *EnginePool
}

// Release will push the engine back into the queue for available engines for
// the current PooledEngine as well as nil out the reference to the engine
// to prevent continued usage of the engine.
func (pe *PooledEngine) Release() {
	if pe.Engine != nil {
		pe.pool.engines <- pe.Engine
		pe.Engine = nil
	}
}

// EnginePool represents a grouping of predefined/preloaded engines that can be
// grabbed for use when Lua scripts need to run.
type EnginePool struct {
	MaxPoolSize uint8
	mutatorFn   EngineMutator
	numEngines  uint8
	engines     chan *lua.Engine
	mutex       *sync.Mutex
}

// NewEnginePool constructs a new pool with the specific maximum size and the
// engine mutator. It will seed the pool with one engine.
func NewEnginePool(poolSize uint8, mutator EngineMutator) *EnginePool {
	if poolSize == 0 {
		poolSize = 1
	}
	ep := &EnginePool{
		MaxPoolSize: poolSize,
		mutatorFn:   mutator,
		numEngines:  1,
		engines:     make(chan *lua.Engine, poolSize),
		mutex:       new(sync.Mutex),
	}
	ep.engines <- ep.generateEngine()

	return ep
}

// Get will fetch the next available engine from the EnginePool. If no engines
// are available and the maximum number of active engines in the pool have been
// created yet then the spawner will be invoked to spawn a new engine and return
// that.
func (ep *EnginePool) Get() *PooledEngine {
	var engine *lua.Engine
	if len(ep.engines) > 0 {
		engine = <-ep.engines
	} else if ep.numEngines < ep.MaxPoolSize {
		ep.mutex.Lock()
		engine = ep.generateEngine()
		ep.numEngines++
		ep.mutex.Unlock()
	} else {
		engine = <-ep.engines
	}

	pe := &PooledEngine{
		Engine: engine,
		pool:   ep,
	}
	// NOTE: precaution to prevent leaks for long running servers, not a perfect
	//       solution. BE DILIGENT AND RELEASE YOUR ENGINES!!
	runtime.SetFinalizer(pe, (*PooledEngine).Release)

	return pe
}

func (ep *EnginePool) generateEngine() *lua.Engine {
	eng := lua.NewEngine()
	eng.Meta[keys.Pool] = ep

	ep.mutatorFn(eng)

	return eng
}
