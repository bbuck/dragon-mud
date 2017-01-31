package logger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

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

// Test specific variables, these should never be set unless from within a test
// file.
var (
	Testing    = false
	TestBuffer *bytes.Buffer
)

// TestLog should never be called in normal code, it's purpose is to bypass the
// logger generated from configuration settings
func TestLog() *logrus.Entry {
	log = logrus.New()
	TestBuffer = new(bytes.Buffer)
	log.Out = TestBuffer
	log.Formatter = new(logrus.JSONFormatter)
	log.Level = logrus.DebugLevel

	return logrus.NewEntry(log)
}

// Log will return an instance of the log utility that should be used for
// send messages to the user. PREFER LogWithSource.
func Log() *logrus.Entry {
	if !initialized {
		initialized = true
		if Testing {
			return TestLog()
		}

		log = logrus.New()

		log.Formatter = &prefixed.TextFormatter{DisableTimestamp: false}
		log.Out = ConfigureTargets(viper.Get("log.targets"))
		// TODO: Set logging level
		log.Level = GetLogLevel(viper.GetString("log.level"))
	}

	return logrus.NewEntry(log)
}

// LogWithSource returns a log with a predefined "source" field attached to it.
// This should be the primary method used to fetch a logger for use in other
// parts fo the code.
func LogWithSource(source string) *logrus.Entry {
	log := Log()
	return log.WithField("source", source)
}

type logTarget struct {
	Type, Target string
}

// GetLogLevel converts a string value to a logrus.Level value for use in
// providing configuration for the logger from the Gamefile.
func GetLogLevel(level string) logrus.Level {
	switch strings.ToLower(level) {
	case "info":
		return logrus.InfoLevel
	case "warn", "warning":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	case "panic":
		return logrus.PanicLevel
	case "fatal":
		return logrus.FatalLevel
	case "debug":
		fallthrough
	default:
		return logrus.DebugLevel
	}
}

// ConfigureTargets takes a 'JSON' map that defines what log targets there
// should be and converts them into an io.Writer suitable for being the target
// of a logrus.Output, this can be a io.MultiWriter or just a writer
func ConfigureTargets(targets interface{}) io.Writer {
	if targets != nil {
		var (
			writers    []io.Writer
			logTargets []logTarget
		)
		if err := mapstructure.Decode(targets, &logTargets); err != nil {
			panic(fmt.Errorf("Failed to process log targets: %s", err))
		}
		for _, target := range logTargets {
			switch target.Type {
			case "terminal":
				if target.Target == "terminal" {
					writers = append(writers, output.Stdout())
				} else if target.Target == "error" {
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

		if len(writers) > 1 {
			return io.MultiWriter(writers...)
		}

		return writers[0]
	}

	return output.Stdout()
}
