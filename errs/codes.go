// Copyright 2017 Brandon Buck

package errs

// Predefined exit codes
const (
	ErrGeneral = 1

	// Logger related failures
	ErrLoggerLoad     = 50
	ErrLoggerFileOpen = 51

	// plugins fail to load
	ErrPluginLoad = 100
)
