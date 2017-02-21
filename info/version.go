// Copyright (c) 2016-2017 Brandon Buck

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
	Minor: 0,
	Patch: 1,
	Flag:  "dev",
}
