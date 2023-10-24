// Copyright (c) 2016-2017 Brandon Buck

package logger

import (
	"io"

	"github.com/sirupsen/logrus"
)

// ****************************************************************************
// logger
// ****************************************************************************

type logrusLogger struct {
	*logrus.Logger
}

func newLogrusLogger(l *logrus.Logger) Log {
	return &logrusLogger{l}
}

func (ll *logrusLogger) WithError(err error) Log {
	l := ll.Logger.WithError(err)

	return newLogrusEntryLogger(l)
}

func (ll *logrusLogger) WithField(k string, v interface{}) Log {
	l := ll.Logger.WithField(k, v)

	return newLogrusEntryLogger(l)
}

func (ll *logrusLogger) WithFields(fs Fields) Log {
	m := map[string]interface{}(fs)
	l := ll.Logger.WithFields(logrus.Fields(m))

	return newLogrusEntryLogger(l)
}

func (ll *logrusLogger) SetLevel(lvl LogLevel) {
	ll.Logger.Level = logLevelToLogrusLevel(lvl)
}

func (ll *logrusLogger) SetOut(w io.Writer) {
	ll.Out = w
}

// ****************************************************************************
// log entry
// ****************************************************************************

type logrusEntryLogger struct {
	*logrus.Entry
}

func newLogrusEntryLogger(e *logrus.Entry) Log {
	return &logrusEntryLogger{e}
}

func (ll *logrusEntryLogger) WithError(err error) Log {
	l := ll.Entry.WithError(err)

	return newLogrusEntryLogger(l)
}

func (ll *logrusEntryLogger) WithField(k string, v interface{}) Log {
	l := ll.Entry.WithField(k, v)

	return newLogrusEntryLogger(l)
}

func (ll *logrusEntryLogger) WithFields(fs Fields) Log {
	m := map[string]interface{}(fs)
	l := ll.Entry.WithFields(logrus.Fields(m))

	return newLogrusEntryLogger(l)
}

func (ll *logrusEntryLogger) SetLevel(lvl LogLevel) {
	ll.Entry.Level = logLevelToLogrusLevel(lvl)
}

func (ll *logrusEntryLogger) SetOut(w io.Writer) {
	ll.Logger.Out = w
}

// helpers

func newLogrus(is ...interface{}) Log {
	log := logrus.New()

	for _, i := range is {
		if f, ok := i.(logrus.Formatter); ok {
			log.Formatter = f
		}
	}

	return newLogrusLogger(log)
}

func logLevelToLogrusLevel(lvl LogLevel) logrus.Level {
	switch lvl {
	case PanicLevel:
		return logrus.PanicLevel
	case FatalLevel:
		return logrus.FatalLevel
	case ErrorLevel:
		return logrus.ErrorLevel
	case WarnLevel:
		return logrus.WarnLevel
	case InfoLevel:
		return logrus.InfoLevel
	case DebugLevel:
		return logrus.DebugLevel
	default:
		return logrus.WarnLevel
	}
}
