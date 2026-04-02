package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	awDir        = ".aw"
	stateFile    = "workspace.json"
	StateVersion = 1
)

// WorkspaceState represents the .aw/workspace.json file.
type WorkspaceState struct {
	Version          int           `json:"version"`
	Source           string        `json:"source"`
	Branch           string        `json:"branch"`
	CreatedAt        string        `json:"created_at"`
	Repos            []RepoEntry   `json:"repos"`
	ContextLinks     []ContextLink `json:"context_links"`
	ExcludedLinks    []string      `json:"excluded_links,omitempty"`
	ClonedSessionIDs []string      `json:"cloned_session_ids,omitempty"`
	SharedMemory     *MemoryInfo   `json:"shared_memory,omitempty"`
}

// MemoryInfo records a shared memory symlink.
type MemoryInfo struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

// RepoEntry represents a single repo in the workspace.
type RepoEntry struct {
	Name         string `json:"name"`
	SourcePath   string `json:"source_path"`
	WorktreePath string `json:"worktree_path"`
}

// ContextLink represents a linked (symlink or copy) context file.
type ContextLink struct {
	Src    string `json:"src"`
	Dst    string `json:"dst"`
	Type   string `json:"type"`   // "workspace" or "repo"
	Method string `json:"method"` // "symlink" or "copy"
}

// StatePath returns the path to workspace.json for a given workspace root.
func StatePath(workspaceDir string) string {
	return filepath.Join(workspaceDir, awDir, stateFile)
}

// AWDir returns the path to the .aw directory.
func AWDir(workspaceDir string) string {
	return filepath.Join(workspaceDir, awDir)
}

// Load reads workspace.json from the given workspace root.
func Load(workspaceDir string) (*WorkspaceState, error) {
	data, err := os.ReadFile(StatePath(workspaceDir))
	if err != nil {
		return nil, err
	}
	var ws WorkspaceState
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, err
	}
	return &ws, nil
}

// Save writes workspace.json to the given workspace root using atomic write.
func Save(workspaceDir string, ws *WorkspaceState) error {
	dir := AWDir(workspaceDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(StatePath(workspaceDir), data)
}

// AtomicWrite writes data to path via temp file + fsync + rename.
// Atomic on Unix; best-effort on Windows.
func AtomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	return os.Rename(tmp, path)
}
