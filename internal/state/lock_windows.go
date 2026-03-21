//go:build windows

package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Acquire attempts to acquire the workspace lock using exclusive file creation.
func Acquire(workspaceDir string) (*Lock, error) {
	dir := AWDir(workspaceDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	lockPath := filepath.Join(dir, lockFile)

	// Check for stale lock
	if _, err := os.Stat(lockPath); err == nil {
		if isStale(lockPath) {
			os.Remove(lockPath)
		} else {
			return nil, fmt.Errorf("LOCK_HELD: workspace locked by another process")
		}
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("LOCK_HELD: workspace locked by another process")
	}

	info := LockInfo{PID: os.Getpid(), CreatedAt: time.Now()}
	data, _ := json.Marshal(info)
	f.Write(data)

	return &Lock{file: f, path: lockPath}, nil
}

// Release releases the workspace lock.
func (l *Lock) Release() {
	if l == nil || l.file == nil {
		return
	}
	l.file.Close()
	os.Remove(l.path)
}
