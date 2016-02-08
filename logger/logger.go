package logger

import (
	"bytes"
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

// Test specific variables, these should never be set unless from within a test
// file.
var (
	Testing    = false
	TestBuffer *bytes.Buffer
)

// TestLog should never be called in normal code, it's purpose is to bypass the
// logger generated from configuration settings
func TestLog() *logrus.Logger {
	log = logrus.New()
	TestBuffer = new(bytes.Buffer)
	log.Out = TestBuffer
	log.Formatter = new(logrus.JSONFormatter)
	log.Level = logrus.DebugLevel

	return log
}

// Log will return an instance of the log utility that should be used for
// send messages to the user.
func Log() *logrus.Logger {
	if !initialized {
		initialized = true
		if Testing {
			return TestLog()
		}

		log = logrus.New()

		log.Formatter = &prefixed.TextFormatter{DisableTimestamp: false}
		log.Out = ConfigureTargets(viper.Get("log.targets"))
		// TODO: Set logging level
		log.Level = logrus.DebugLevel
	}

	return log
}

type logTarget struct {
	Type, Target string
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

		if len(writers) > 1 {
			return io.MultiWriter(writers...)
		}

		return writers[0]
	}

	return output.Stdout()
}

// Complete contract with Logrus features

// WithField passes into Log()
func WithField(name string, value interface{}) *logrus.Entry {
	return Log().WithField(name, value)
}

// WithFields passes into Log()
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Log().WithFields(fields)
}

// Info passes into Log()
func Info(args ...interface{}) {
	Log().Info(args...)
}

// Infof passes into Log()
func Infof(format string, args ...interface{}) {
	Log().Infof(format, args...)
}

// Infoln passes into Log()
func Infoln(args ...interface{}) {
	Log().Infoln(args...)
}

// Debug passes into Log()
func Debug(args ...interface{}) {
	Log().Debug(args...)
}

// Debugf passes into Log()
func Debugf(format string, args ...interface{}) {
	Log().Debugf(format, args...)
}

// Debugln passes into Log()
func Debugln(args ...interface{}) {
	Log().Debugln(args...)
}

// Warn passes into Log()
func Warn(args ...interface{}) {
	Log().Warn(args...)
}

// Warnf passes into Log()
func Warnf(format string, args ...interface{}) {
	Log().Warnf(format, args...)
}

// Warnln passes into Log()
func Warnln(args ...interface{}) {
	Log().Warnln(args...)
}

// Error passes into Log()
func Error(args ...interface{}) {
	Log().Error(args...)
}

// Errorf passes into Log()
func Errorf(format string, args ...interface{}) {
	Log().Errorf(format, args...)
}

// Errorln passes into Log()
func Errorln(args ...interface{}) {
	Log().Errorln(args...)
}

// Panic passes into Log()
func Panic(args ...interface{}) {
	Log().Panic(args...)
}

// Panicf passes into Log()
func Panicf(format string, args ...interface{}) {
	Log().Panicf(format, args...)
}

// Panicln passes into Log()
func Panicln(args ...interface{}) {
	Log().Panicln(args...)
}

// Fatal passes into Log()
func Fatal(args ...interface{}) {
	Log().Fatal(args...)
}

// Fatalf passes into Log()
func Fatalf(format string, args ...interface{}) {
	Log().Fatalf(format, args...)
}

// Fatalln passes into Log()
func Fatalln(args ...interface{}) {
	Log().Fatalln(args...)
}
