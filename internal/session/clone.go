package session

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.jsonl$`)

const activeThreshold = 30 * time.Second

// CloneResult describes the outcome of cloning a single session.
type CloneResult struct {
	ID     string `json:"id"`
	File   string `json:"file"`
	Status string `json:"status"` // "ok", "skipped", "active"
}

// MemoryLink records a shared memory symlink.
type MemoryLink struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

// CloneSessions copies session files from source to target project directory.
// sessionID: if non-empty, clone only this UUID.
// limit: max number of sessions to clone (by mtime desc). Ignored if sessionID is set.
func CloneSessions(sourceDir, targetDir, sessionID string, limit int) ([]CloneResult, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("unsupported_platform: session clone not supported on Windows")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot get home dir: %w", err)
	}

	projectsDir := filepath.Join(home, ".claude", "projects")
	srcProjectDir := filepath.Join(projectsDir, encodeProjectPath(sourceDir))
	dstProjectDir := filepath.Join(projectsDir, encodeProjectPath(targetDir))

	// Find candidate sessions
	candidates, err := findBaseSessionsSorted(srcProjectDir)
	if err != nil || len(candidates) == 0 {
		return nil, nil
	}

	// Select sessions
	var selected []sessionFile
	if sessionID != "" {
		for _, c := range candidates {
			if c.id == sessionID {
				selected = append(selected, c)
				break
			}
		}
		if len(selected) == 0 {
			return []CloneResult{{ID: sessionID, Status: "skipped"}}, nil
		}
		// For explicit --session-id, active = error (don't fall through to next)
		if selected[0].active {
			return []CloneResult{{ID: sessionID, Status: "active"}}, nil
		}
	} else {
		for _, c := range candidates {
			if c.active {
				continue // skip active sessions in auto mode
			}
			selected = append(selected, c)
			if len(selected) >= limit {
				break
			}
		}
	}

	if len(selected) == 0 {
		return nil, nil
	}

	// Create target project dir
	if err := os.MkdirAll(dstProjectDir, 0755); err != nil {
		return nil, fmt.Errorf("create project dir: %w", err)
	}

	var results []CloneResult
	for _, s := range selected {
		// Generate new UUID to avoid conflict when saving back
		newID := newUUID()
		dstFile := filepath.Join(dstProjectDir, newID+".jsonl")
		data, err := os.ReadFile(s.path)
		if err != nil {
			results = append(results, CloneResult{ID: newID, Status: "skipped"})
			continue
		}
		if err := os.WriteFile(dstFile, data, 0600); err != nil {
			results = append(results, CloneResult{ID: newID, Status: "skipped"})
			continue
		}
		results = append(results, CloneResult{ID: newID, File: dstFile, Status: "ok"})
	}

	return results, nil
}

// LinkMemory creates a symlink from source project's memory/ to target project dir.
// Returns nil MemoryLink if not applicable.
func LinkMemory(sourceDir, targetDir string) (*MemoryLink, string) {
	if runtime.GOOS == "windows" {
		return nil, "unsupported_platform"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "error"
	}

	projectsDir := filepath.Join(home, ".claude", "projects")
	srcMemory := filepath.Join(projectsDir, encodeProjectPath(sourceDir), "memory")
	dstProjectDir := filepath.Join(projectsDir, encodeProjectPath(targetDir))
	dstMemory := filepath.Join(dstProjectDir, "memory")

	// Source doesn't exist
	if _, err := os.Stat(srcMemory); err != nil {
		return nil, "missing_source"
	}

	// Check target
	if fi, err := os.Lstat(dstMemory); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			if target, err := os.Readlink(dstMemory); err == nil && target == srcMemory {
				return nil, "already_linked"
			}
		}
		return nil, "target_conflict"
	}

	// Create target project dir if needed
	os.MkdirAll(dstProjectDir, 0755)

	if err := os.Symlink(srcMemory, dstMemory); err != nil {
		return nil, "symlink_failed"
	}

	return &MemoryLink{Src: srcMemory, Dst: dstMemory}, "ok"
}

// CleanupMemoryLink removes a memory symlink if it's still valid.
func CleanupMemoryLink(link *MemoryLink) bool {
	if link == nil {
		return false
	}
	fi, err := os.Lstat(link.Dst)
	if err != nil {
		return false // already gone
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return false // not a symlink, don't touch
	}
	target, err := os.Readlink(link.Dst)
	if err != nil || target != link.Src {
		return false // points elsewhere, don't touch
	}
	os.Remove(link.Dst)
	return true
}

// SaveBackSessions moves all UUID-matching sessions from workspace project dir back to source.
// Since clone uses new UUIDs, all sessions in the workspace dir can be safely moved back.
func SaveBackSessions(sourceDir, targetDir string) (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}

	projectsDir := filepath.Join(home, ".claude", "projects")
	srcProjectDir := filepath.Join(projectsDir, encodeProjectPath(sourceDir))
	dstProjectDir := filepath.Join(projectsDir, encodeProjectPath(targetDir))

	entries, err := os.ReadDir(dstProjectDir)
	if err != nil {
		return 0, nil // no project dir, nothing to save
	}

	os.MkdirAll(srcProjectDir, 0755)

	moved := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !uuidPattern.MatchString(name) {
			continue
		}

		srcFile := filepath.Join(dstProjectDir, name)
		dstFile := filepath.Join(srcProjectDir, name)

		// Don't overwrite existing (shouldn't happen with unique UUIDs, but be safe)
		if _, err := os.Stat(dstFile); err == nil {
			continue
		}

		data, err := os.ReadFile(srcFile)
		if err != nil {
			continue
		}
		if err := os.WriteFile(dstFile, data, 0600); err != nil {
			continue
		}
		os.Remove(srcFile)
		moved++
	}

	return moved, nil
}

// RollbackClonedFiles removes files that were successfully cloned.
func RollbackClonedFiles(results []CloneResult) {
	for _, r := range results {
		if r.Status == "ok" && r.File != "" {
			os.Remove(r.File)
		}
	}
}

// newUUID generates a random UUID v4.
func newUUID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// encodeProjectPath converts an absolute path to Claude Code's project directory encoding.
// Uses filepath.Abs (NOT CanonicalizePath/EvalSymlinks) to match Claude Code's behavior.
func encodeProjectPath(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return "-" + strings.ReplaceAll(strings.TrimPrefix(abs, "/"), "/", "-")
}

type sessionFile struct {
	id     string
	path   string
	mtime  time.Time
	active bool
}

func findBaseSessionsSorted(projectDir string) ([]sessionFile, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}

	var files []sessionFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !uuidPattern.MatchString(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(name, ".jsonl")
		files = append(files, sessionFile{
			id:     id,
			path:   filepath.Join(projectDir, name),
			mtime:  info.ModTime(),
			active: time.Since(info.ModTime()) < activeThreshold,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].mtime.After(files[j].mtime)
	})

	return files, nil
}

