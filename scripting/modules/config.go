// Copyright 2017 Brandon Buck

package modules

import (
	"reflect"

	"github.com/bbuck/dragon-mud/scripting/lua"
	"github.com/spf13/viper"
)

// Config provides a way for scripts to access data defined inside the
// Dragonfile.toml.
//   get(key): any
//     @param key: string = the dot notation key to look up in the application
//       configuration
//     fetches a configuration value for the application by key
var Config = lua.TableMap{
	"get": func(eng *lua.Engine) int {
		key := eng.PopString()
		iface := viper.Get(key)
		t := reflect.TypeOf(iface)
		switch t.Kind() {
		case reflect.Map:
			eng.PushValue(eng.TableFromMap(iface))
		case reflect.Slice:
			eng.PushValue(eng.TableFromSlice(iface))
		default:
			eng.PushValue(iface)
		}

		return 1
	},
}
