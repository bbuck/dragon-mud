// Copyright (c) 2016-2017 Brandon Buck

package lua

import (
	"runtime"
	"sync"
	"time"

	"github.com/bbuck/dragon-mud/scripting/keys"
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
		if !pe.pool.closed {
			pe.pool.engines <- pe.Engine
		}
		pe.Engine = nil
	}
}

// EnginePool represents a grouping of predefined/preloaded engines that can be
// grabbed for use when Lua scripts need to run.
type EnginePool struct {
	MaxPoolSize   uint8
	Mutator       EngineMutator
	numEngines    uint8
	engines       chan *Engine
	cachedEngines []*Engine
	mutex         *sync.Mutex
	closed        bool
}

// NewEnginePool constructs a new pool with the specific maximum size and the
// engine mutator. It will seed the pool with one engine.
func NewEnginePool(poolSize uint8, mutator EngineMutator) *EnginePool {
	if poolSize == 0 {
		poolSize = 1
	}
	ep := &EnginePool{
		MaxPoolSize:   poolSize,
		Mutator:       mutator,
		numEngines:    1,
		engines:       make(chan *Engine, poolSize),
		mutex:         new(sync.Mutex),
		cachedEngines: make([]*Engine, 0),
		closed:        false,
	}
	ep.engines <- ep.generateEngine()

	return ep
}

// Len will return the number of engines that have been spawned during the
// execution fo the pool.
func (ep *EnginePool) Len() int {
	return len(ep.cachedEngines)
}

// Get will fetch the next available engine from the EnginePool. If no engines
// are available and the maximum number of active engines in the pool have been
// created yet then the spawner will be invoked to spawn a new engine and return
// that.
func (ep *EnginePool) Get() *PooledEngine {
	if ep.closed {
		return nil
	}

	if ep.MaxPoolSize == 0 {
		ep.MaxPoolSize = 1
	}

	var engine *Engine
	select {
	case eng := <-ep.engines:
		engine = eng
	case <-time.After(250 * time.Millisecond):
		if uint8(ep.Len()) < ep.MaxPoolSize {
			ep.mutex.Lock()
			engine = ep.generateEngine()
			ep.mutex.Unlock()
		} else {
			engine = <-ep.engines
		}
	}
	// if len(ep.engines) > 0 {

	// } else if uint8(ep.Len()) < ep.MaxPoolSize {
	// 	ep.mutex.Lock()
	// 	engine = ep.generateEngine()
	// 	ep.mutex.Unlock()
	// } else {
	// 	engine = <-ep.engines
	// }

	pe := &PooledEngine{
		Engine: engine,
		pool:   ep,
	}
	// NOTE: precaution to prevent leaks for long running servers, not a perfect
	//       solution. BE DILIGENT AND RELEASE YOUR ENGINES!!
	runtime.SetFinalizer(pe, (*PooledEngine).Release)

	return pe
}

// EachEngine will call the provided handler with each engine. IN NO WAY SHOULD
// THIS BE USED TO UNDERMINE GET, THIS IS FOR MAINTENANCE.
func (ep *EnginePool) EachEngine(fn func(*Engine)) {
	for _, eng := range ep.cachedEngines {
		fn(eng)
	}
}

// Shutdown will empty the channel, close all generated engines and mark the
// pool closed.
func (ep *EnginePool) Shutdown() {
	if !ep.closed {
		ep.closed = true

		close(ep.engines)

		for range ep.engines {
			// emptyting out the engines channel
		}

		for _, eng := range ep.cachedEngines {
			eng.Close()
		}
	}
}

// create a new engine for use in the pool
func (ep *EnginePool) generateEngine() *Engine {
	eng := NewEngine()
	eng.Meta[keys.Pool] = ep
	ep.cachedEngines = append(ep.cachedEngines, eng)

	if ep.Mutator != nil {
		ep.Mutator(eng)
	}

	return eng
}
