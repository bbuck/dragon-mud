package modules

import (
	"github.com/bbuck/dragon-mud/scripting/lua"
	uuid "github.com/satori/go.uuid"
)

// UUID enables the generation of UUID v1 values, as necessary.
//   new(): string
//     a new v1 UUID value in string format.
var UUID = lua.TableMap{
	"new": func() string {
		u := uuid.NewV1()

		return u.String()
	},
}
