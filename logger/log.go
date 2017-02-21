// Copyright (c) 2016-2017 Brandon Buck

package logger

import "io"

type LogLevel uint8

// Log level definitions for the Log interface
const (
	PanicLevel LogLevel = 1 + iota
	FatalLevel
	ErrorLevel
	WarnLevel
	InfoLevel
	DebugLevel
)

// Fields is a map containing any fields to log with.
type Fields map[string]interface{}

// Log wraps a series of logging fields that should be implemented on any
// log type.
type Log interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Debugln(args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Errorln(args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Fatalln(args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Infoln(args ...interface{})
	Panic(args ...interface{})
	Panicf(format string, args ...interface{})
	Panicln(args ...interface{})
	Print(args ...interface{})
	Printf(format string, args ...interface{})
	Println(args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Warning(args ...interface{})
	Warningf(format string, args ...interface{})
	Warningln(args ...interface{})
	Warnln(args ...interface{})
	WithError(err error) Log
	WithField(key string, value interface{}) Log
	WithFields(fields Fields) Log
	SetLevel(LogLevel)
	SetOut(io.Writer)
}
