package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/aw/internal/git"
	"github.com/anthropics/aw/internal/state"
)

// ContextCandidates are AI context file/dir names to discover and link.
var ContextCandidates = []string{
	"CLAUDE.md",
	"AGENTS.md",
	"codex.md",
	".claude",
	".codex",
	".cursorrules",
	".cursor",
	"aw.yml",
}

// LinkWorkspaceContext symlinks workspace-level AI context files from srcDir to dstDir.
func LinkWorkspaceContext(srcDir, dstDir string) []state.ContextLink {
	var links []state.ContextLink
	for _, name := range ContextCandidates {
		src := filepath.Join(srcDir, name)
		if _, err := os.Lstat(src); err != nil {
			continue
		}
		dst := filepath.Join(dstDir, name)
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if err := os.Symlink(src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "  [skip] %s: %v\n", name, err)
			continue
		}
		links = append(links, state.ContextLink{Src: src, Dst: dst, Type: "workspace"})
	}
	return links
}

// LinkRepoContext symlinks untracked AI context files from a source repo to its worktree.
func LinkRepoContext(srcRepo, dstRepo, repoName string) []state.ContextLink {
	var links []state.ContextLink
	for _, name := range ContextCandidates {
		src := filepath.Join(srcRepo, name)
		if _, err := os.Lstat(src); err != nil {
			continue
		}
		if IsTracked(srcRepo, name) {
			continue // worktree already has it
		}
		dst := filepath.Join(dstRepo, name)
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if err := os.Symlink(src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "  [skip] %s/%s: %v\n", repoName, name, err)
			continue
		}
		links = append(links, state.ContextLink{Src: src, Dst: dst, Type: "repo"})
	}
	return links
}

// IsTracked checks if a path is tracked by git in the given repo.
func IsTracked(repoDir, path string) bool {
	_, err := git.GitRun(repoDir, "ls-files", "--error-unmatch", path)
	return err == nil
}

// RemoveSymlinks removes symlinks that are still symlinks (not replaced by real files).
func RemoveSymlinks(links []state.ContextLink) int {
	removed := 0
	for _, link := range links {
		fi, err := os.Lstat(link.Dst)
		if err != nil {
			continue // already gone
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			continue // not a symlink anymore
		}
		if err := os.Remove(link.Dst); err == nil {
			removed++
		}
	}
	return removed
}
