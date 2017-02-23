// Copyright 2016-2017 Brandon Buck

package modules

import (
	"fmt"

	"github.com/bbuck/dragon-mud/logger"
)

var logCache = make(map[string]logger.Log)

func log(name string) logger.Log {
	if l, ok := logCache[name]; ok {
		return l
	}

	l := logger.NewWithSource(fmt.Sprintf("lua(%s)", name))
	logCache[name] = l

	return l
}
