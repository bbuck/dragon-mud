package log

import (
	"os"

	"github.com/Sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var log = logrus.New()

func init() {
	// TODO: Make this customizable
	log.Out = os.Stdout
	// TODO: Set logging level
	log.Level = logrus.DebugLevel
	log.Formatter = new(prefixed.TextFormatter)
}

// Logger will return an instance of the log utility that should be used for
// send messages to the user.
func Logger() *logrus.Logger {
	return log
}
