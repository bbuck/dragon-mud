package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// serverLockFile marks a game whose server is running, holding its process
// id, so tasks run from the command line don't change the database
// underneath it.
const serverLockFile = "data/server.lock"

// lockServer records that this process serves the game in dir, and returns
// a function that removes the record. It's an error if another live
// process is serving it. A lock left behind by a server that crashed is
// taken over.
func lockServer(dir string) (unlock func(), err error) {
	if pid, ok := runningServer(dir); ok {
		return nil, fmt.Errorf("dragon serve is running this game already, as process %d. Stop it first; two servers would save over each other.", pid)
	}

	path := filepath.Join(dir, serverLockFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	pid := os.Getpid()
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		return nil, err
	}

	return func() {
		if other, ok := readServerLock(path); ok && other == pid {
			os.Remove(path)
		}
	}, nil
}

// runningServer returns the process id of the live server for the game in
// dir, if there is one.
func runningServer(dir string) (int, bool) {
	pid, ok := readServerLock(filepath.Join(dir, serverLockFile))
	if !ok || pid == os.Getpid() || !processAlive(pid) {
		return 0, false
	}

	return pid, true
}

func readServerLock(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) || err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))

	return pid, err == nil && pid > 0
}
