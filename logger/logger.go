package logger

import (
	"fmt"
	"io"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/bbuck/dragon-mud/output"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var (
	log         *logrus.Logger
	initialized = false
)

// Log will return an instance of the log utility that should be used for
// send messages to the user.
func Log() *logrus.Logger {
	if !initialized {
		initialized = true
		log = logrus.New()

		log.Formatter = &prefixed.TextFormatter{DisableTimestamp: false}
		// TODO: Make this customizable
		log.Out = configureTargets()
		// TODO: Set logging level
		log.Level = logrus.DebugLevel
	}

	return log
}

type logTarget struct {
	Type, Target string
}

func configureTargets() io.Writer {
	if viper.IsSet("log.targets") {
		var (
			writers    []io.Writer
			logTargets []logTarget
		)
		if err := mapstructure.Decode(viper.Get("log.targets"), &logTargets); err != nil {
			panic(fmt.Errorf("Failed to process log targets: %s", err))
		}
		for _, target := range logTargets {
			switch target.Type {
			case "os":
				if target.Target == "stdout" {
					writers = append(writers, output.Stdout())
				} else if target.Target == "stderr" {
					writers = append(writers, output.Stderr())
				}
			case "file":
				file, err := os.OpenFile(target.Target, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
				if err != nil {
					if os.IsNotExist(err) {
						file, err = os.Create(target.Target)
						if err != nil {
							panic(fmt.Errorf("Failed creating a file log target: %s", err))
						}
					} else {
						panic(err)
					}
				}
				writers = append(writers, file)
			}
		}

		return io.MultiWriter(writers...)
	}

	return output.Stdout()
}
