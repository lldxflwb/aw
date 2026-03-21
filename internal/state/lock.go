package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	lockFile    = "lock"
	lockTimeout = 30 * time.Second
)

// LockInfo contains metadata about who holds the lock.
type LockInfo struct {
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"created_at"`
}

// Lock represents a workspace-level file lock (flock).
type Lock struct {
	file *os.File
	path string
}

// Acquire attempts to acquire the workspace lock.
func Acquire(workspaceDir string) (*Lock, error) {
	dir := AWDir(workspaceDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	lockPath := filepath.Join(dir, lockFile)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("cannot open lock file: %w", err)
	}

	// Try non-blocking exclusive lock
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		if isStale(lockPath) {
			os.Remove(lockPath)
			return Acquire(workspaceDir)
		}
		return nil, fmt.Errorf("LOCK_HELD: workspace locked by another process")
	}

	// Write lock info
	info := LockInfo{PID: os.Getpid(), CreatedAt: time.Now()}
	data, _ := json.Marshal(info)
	f.Truncate(0)
	f.Seek(0, 0)
	f.Write(data)

	return &Lock{file: f, path: lockPath}, nil
}

// Release releases the workspace lock.
func (l *Lock) Release() {
	if l == nil || l.file == nil {
		return
	}
	syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	l.file.Close()
	os.Remove(l.path)
}

func isStale(lockPath string) bool {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return true
	}
	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return true
	}
	return time.Since(info.CreatedAt) > lockTimeout
}
