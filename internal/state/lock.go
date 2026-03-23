package state

import (
	"encoding/json"
	"os"
	"time"
)

const (
	lockFile         = "lock"
	registryLockFile = "registry.lock"
	lockTimeout      = 30 * time.Second
)

// LockInfo contains metadata about who holds the lock.
type LockInfo struct {
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"created_at"`
}

// Lock represents a workspace-level file lock.
type Lock struct {
	file *os.File
	path string
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
