// Copyright (c) 2016-2017 Brandon Buck

package server

import (
	"fmt"
	"math"
	"sync/atomic"

	"github.com/bbuck/dragon-mud/events"
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/bbuck/dragon-mud/scripting/keys"
	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/spf13/viper"
)

// server utility values
var (
	EnginePool *lua.EnginePool
)

var serverID uint64 = 1

// called during Run() execution preparing server tools for use.
func initialize() {
	size := viper.GetInt("scripting.server.engine_pool_size")
	if size < 0 {
		size = 0
	}
	if size > int(math.MaxUint8) {
		size = int(math.MaxUint8)
	}
	usize := uint8(size)
	EnginePool = lua.NewEnginePool(usize, newServerEngine)
}

// Emit will emit a server event to the server scripts.
func Emit(evt string, d events.Data) events.Done {
	done := make(events.Done)
	go func() {
		eng := EnginePool.Get()
		defer eng.Release()
		if em, ok := eng.Meta[keys.Emitter].(*events.Emitter); ok {
			d := em.Emit(evt, d)
			<-d
		}
		close(done)
	}()

	return done
}

// generate a new lua engine ready for use by server code.
func newServerEngine(eng *lua.Engine) {
	id := atomic.LoadUint64(&serverID)
	atomic.AddUint64(&serverID, 1)

	engID := fmt.Sprintf("server_engine(%d)", id)
	eng.Meta[keys.EngineID] = engID

	eng.OpenMath()
	eng.OpenString()
	eng.OpenTable()
	scripting.OpenLibs(eng, "events", "random", "die", "log", "password",
		"sutil", "tmpl")
}
