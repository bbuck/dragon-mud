//go:build windows

package main

import "os"

// processAlive reports whether the process pid is running. On Windows,
// finding a process opens it, which fails once it has exited.
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	p.Release()

	return true
}
