package log

import (
	"github.com/Sirupsen/logrus"
	"github.com/bbuck/dragon-mud/output"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var log = logrus.New()

func init() {
	log.Formatter = &prefixed.TextFormatter{DisableTimestamp: false}
	// TODO: Make this customizable
	log.Out = output.Stdout()
	// TODO: Set logging level
	log.Level = logrus.DebugLevel
}

// Logger will return an instance of the log utility that should be used for
// send messages to the user.
func Logger() *logrus.Logger {
	return log
}
