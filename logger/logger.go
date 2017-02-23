// Copyright (c) 2016-2017 Brandon Buck

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
	log         Log
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
func TestLog() Log {
	log = newLogrus(new(logrus.JSONFormatter))
	TestBuffer = new(bytes.Buffer)
	log.SetOut(TestBuffer)
	log.SetLevel(DebugLevel)

	return log
}

// New will return an instance of the log utility that should be used for
// send messages to the user. PREFER LogWithSource.
func New() Log {
	if !initialized {
		initialized = true
		if Testing {
			return TestLog()
		}

		log = newLogrus(&prefixed.TextFormatter{DisableTimestamp: false})
		log.SetOut(ConfigureTargets(viper.Get("log.targets")))
		log.SetLevel(GetLogLevel(viper.GetString("log.level")))
	}

	return log
}

// NewWithSource returns a log with a predefined "source" field attached to it.
// This should be the primary method used to fetch a logger for use in other
// parts fo the code.
func NewWithSource(source string) Log {
	log := New()

	return log.WithField("source", source)
}

type logTarget struct {
	Type, Target string
}

// GetLogLevel converts a string value to a logrus.Level value for use in
// providing configuration for the logger from the Gamefile.
func GetLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "info":
		return InfoLevel
	case "warn", "warning":
		return WarnLevel
	case "error":
		return ErrorLevel
	case "panic":
		return PanicLevel
	case "fatal":
		return FatalLevel
	case "debug":
		fallthrough
	default:
		return DebugLevel
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
							fmt.Fprintf(os.Stderr, "ERROR: Failed creating a file log target: %s", err)
							os.Exit(1)
						}
					} else {
						fmt.Fprintf(os.Stderr, "ERROR: %s", err.Error())
						os.Exit(1)
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
