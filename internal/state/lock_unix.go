//go:build !windows

package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Acquire attempts to acquire the workspace lock using flock.
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

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		if isStale(lockPath) {
			os.Remove(lockPath)
			return Acquire(workspaceDir)
		}
		return nil, fmt.Errorf("LOCK_HELD: workspace locked by another process")
	}

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
