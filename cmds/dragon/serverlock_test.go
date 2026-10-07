package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"bbuck.dev/dragon-mud/scaffold"
)

func writeLock(t *testing.T, dir string, pid int) {
	t.Helper()

	path := filepath.Join(dir, serverLockFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A server's lock keeps a second server and offline tasks out while its
// process lives.
func TestServerLock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mygame")
	if err := scaffold.New(dir, scaffold.Data{Name: "My Game"}); err != nil {
		t.Fatal(err)
	}

	server := exec.Command("sleep", "30")
	if err := server.Start(); err != nil {
		t.Skip("can't start a stand-in server process:", err)
	}
	t.Cleanup(func() { server.Process.Kill(); server.Wait() })
	writeLock(t, dir, server.Process.Pid)

	if _, err := lockServer(dir); err == nil || !strings.Contains(err.Error(), "dragon serve is running this game already, as process "+strconv.Itoa(server.Process.Pid)) {
		t.Errorf("second server: %v", err)
	}
	var out strings.Builder
	if err := runTask("seed", []string{"-dir", dir}, &out); err == nil || !strings.Contains(err.Error(), "a task run from the command line changes the database underneath it") {
		t.Errorf("task: %v", err)
	}

	// Once the server is gone, its lock is stale and taken over.
	server.Process.Kill()
	server.Wait()
	unlock, err := lockServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	if pid, ok := readServerLock(filepath.Join(dir, serverLockFile)); !ok || pid != os.Getpid() {
		t.Errorf("lock holds %d", pid)
	}
	unlock()
	if _, err := os.Stat(filepath.Join(dir, serverLockFile)); !os.IsNotExist(err) {
		t.Error("unlocking left the lock file")
	}
}
