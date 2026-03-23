package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const RegistryVersion = 1

// Registry represents the .aw/registry.json file in the source directory.
type Registry struct {
	Version    int             `json:"version"`
	Workspaces []RegistryEntry `json:"workspaces"`
}

// RegistryEntry is a registered workspace directory.
type RegistryEntry struct {
	Dir string `json:"dir"`
}

// RegistryPath returns the path to registry.json for a given source directory.
func RegistryPath(sourceDir string) string {
	return filepath.Join(sourceDir, awDir, "registry.json")
}

// CanonicalizePath returns a canonical absolute path.
// Uses EvalSymlinks to resolve aliases (e.g. /tmp → /private/tmp on macOS).
func CanonicalizePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	abs = filepath.Clean(abs)
	if runtime.GOOS == "windows" && len(abs) >= 2 && abs[1] == ':' {
		abs = strings.ToUpper(abs[:1]) + abs[1:]
	}
	return abs
}

// LoadRegistry reads registry.json from the source directory.
// Returns an empty registry if the file doesn't exist.
func LoadRegistry(sourceDir string) (*Registry, error) {
	data, err := os.ReadFile(RegistryPath(sourceDir))
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{Version: RegistryVersion}, nil
		}
		return nil, err
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}
	return &reg, nil
}

// SaveRegistry writes registry.json atomically.
func SaveRegistry(sourceDir string, reg *Registry) error {
	dir := AWDir(sourceDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(RegistryPath(sourceDir), data)
}

// UpsertWorkspace registers a workspace in the source directory's registry.
// Acquires registry lock, loads, upserts by canonical path, saves.
func UpsertWorkspace(sourceDir, workspaceDir string) error {
	canonical := CanonicalizePath(workspaceDir)

	lock, err := AcquireRegistry(sourceDir)
	if err != nil {
		return fmt.Errorf("registry lock: %w", err)
	}
	defer lock.Release()

	reg, err := LoadRegistry(sourceDir)
	if err != nil {
		return err
	}

	for _, entry := range reg.Workspaces {
		if entry.Dir == canonical {
			return nil
		}
	}

	reg.Workspaces = append(reg.Workspaces, RegistryEntry{Dir: canonical})
	return SaveRegistry(sourceDir, reg)
}

// RemoveWorkspace removes a workspace from the source directory's registry.
func RemoveWorkspace(sourceDir, workspaceDir string) error {
	canonical := CanonicalizePath(workspaceDir)

	lock, err := AcquireRegistry(sourceDir)
	if err != nil {
		return fmt.Errorf("registry lock: %w", err)
	}
	defer lock.Release()

	reg, err := LoadRegistry(sourceDir)
	if err != nil {
		return err
	}

	var filtered []RegistryEntry
	for _, entry := range reg.Workspaces {
		if entry.Dir != canonical {
			filtered = append(filtered, entry)
		}
	}
	reg.Workspaces = filtered
	return SaveRegistry(sourceDir, reg)
}
