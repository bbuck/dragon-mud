package keys

import "github.com/bbuck/dragon-mud/scripting/lua"

// Keys used to store data with an engine.
const (
	EngineID = "engine id"
	Emitter  = "events emitter"
	Pool     = lua.EnginePoolMetaKey
	Logger   = "logger"
	RootCmd  = "root command"

	TalonRowMetatable  = "talon row metatable"
	TalonRowsMetatable = "talon rows metatable"
)
