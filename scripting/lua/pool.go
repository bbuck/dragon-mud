// Copyright (c) 2016-2017 Brandon Buck

package lua

import (
	"runtime"
	"sync"
)

// EnginePoolMetaKey is a string value for associating the pool with an engine
const EnginePoolMetaKey = "engine pool"

// EngineMutator will modify an Engine before it goes into the pool. This can
// run any number of scripts as necessary such as registring libraries,
// executing code, etc...
type EngineMutator func(*Engine)

// PooledEngine wraps a Lua engine. It's purpose is provide a means with which
// to return the engine to the EnginePool when it's not longer being used.
type PooledEngine struct {
	*Engine
	pool *EnginePool
}

// Release will push the engine back into the queue for available engines for
// the current PooledEngine as well as nil out the reference to the engine
// to prevent continued usage of the engine.
func (pe *PooledEngine) Release() {
	if pe.Engine != nil {
		if !pe.pool.drained {
			pe.pool.engines <- pe.Engine
		}
		pe.Engine = nil
	}
}

// EnginePool represents a grouping of predefined/preloaded engines that can be
// grabbed for use when Lua scripts need to run.
type EnginePool struct {
	MaxPoolSize uint8
	Mutator     EngineMutator
	numEngines  uint8
	engines     chan *Engine
	engineCache []*Engine
	mutex       *sync.Mutex
	drained     bool
}

// NewEnginePool constructs a new pool with the specific maximum size and the
// engine mutator. It will seed the pool with one engine.
func NewEnginePool(poolSize uint8, mutator EngineMutator) *EnginePool {
	if poolSize == 0 {
		poolSize = 1
	}
	ep := &EnginePool{
		MaxPoolSize: poolSize,
		Mutator:     mutator,
		numEngines:  1,
		engines:     make(chan *Engine, poolSize),
		engineCache: make([]*Engine, 0),
		mutex:       new(sync.Mutex),
		drained:     false,
	}
	ep.engines <- ep.generateEngine()

	return ep
}

// Drain will fetch and kill all engines in the pool and shutdown.
func (ep *EnginePool) Drain() {
	if !ep.drained {
		ep.drained = true

		for _, eng := range ep.engineCache {
			eng.Meta = nil
			eng.Close()
		}

		close(ep.engines)
		for _ = range ep.engines {
			// do nothing with the engine
		}
	}
}

// Len will return the number of engines that have been spawned during the
// execution fo the pool.
func (ep *EnginePool) Len() int {
	return int(ep.numEngines)
}

// Get will fetch the next available engine from the EnginePool. If no engines
// are available and the maximum number of active engines in the pool have been
// created yet then the spawner will be invoked to spawn a new engine and return
// that.
func (ep *EnginePool) Get() *PooledEngine {
	if ep.MaxPoolSize == 0 {
		ep.MaxPoolSize = 1
	}

	var engine *Engine
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

// generate a new engine, running through the provided mutator (if any)
func (ep *EnginePool) generateEngine() *Engine {
	eng := NewEngine()
	eng.Meta[EnginePoolMetaKey] = ep

	if ep.Mutator != nil {
		ep.Mutator(eng)
	}

	return eng
}
