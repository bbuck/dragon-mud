package info

import "fmt"

type version struct {
	Major, Minor, Patch uint
	Flag                string
}

func (v version) String() string {
	return fmt.Sprintf("DragonMUD version %d.%d.%d %s", v.Major, v.Minor, v.Patch, v.Flag)
}

// Version is the struct that represents the version of DragonMUD in use.
var Version = version{
	Major: 0,
	Minor: 2,
	Patch: 0,
	Flag:  "dev",
}
