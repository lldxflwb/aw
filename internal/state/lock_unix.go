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
	return acquireFlock(AWDir(workspaceDir), lockFile, true)
}

// AcquireRegistry acquires the registry lock in the source directory.
// No stale recovery on Unix — flock is kernel-level and auto-releases on process exit.
func AcquireRegistry(sourceDir string) (*Lock, error) {
	return acquireFlock(AWDir(sourceDir), registryLockFile, false)
}

func acquireFlock(dir, name string, staleRecovery bool) (*Lock, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	lockPath := filepath.Join(dir, name)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("cannot open lock file: %w", err)
	}

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		if staleRecovery && isStale(lockPath) {
			os.Remove(lockPath)
			return acquireFlock(dir, name, staleRecovery)
		}
		return nil, fmt.Errorf("LOCK_HELD: locked by another process")
	}

	info := LockInfo{PID: os.Getpid(), CreatedAt: time.Now()}
	data, _ := json.Marshal(info)
	f.Truncate(0)
	f.Seek(0, 0)
	f.Write(data)

	return &Lock{file: f, path: lockPath}, nil
}

// Release releases the lock.
func (l *Lock) Release() {
	if l == nil || l.file == nil {
		return
	}
	syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	l.file.Close()
	os.Remove(l.path)
}
