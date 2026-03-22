package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/anthropics/aw/internal/git"
	"github.com/anthropics/aw/internal/state"
)

// LinkWorkspaceContext links workspace-level AI context files from srcDir to dstDir.
// Tries symlink first; falls back to copy on failure (e.g. Windows without dev mode).
func LinkWorkspaceContext(srcDir, dstDir string, candidates []string) []state.ContextLink {
	var links []state.ContextLink
	for _, name := range candidates {
		src := filepath.Join(srcDir, name)
		if _, err := os.Lstat(src); err != nil {
			continue
		}
		dst := filepath.Join(dstDir, name)
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		method, err := linkOrCopy(src, dst)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [error] %s: %v\n", name, err)
			continue
		}
		links = append(links, state.ContextLink{Src: src, Dst: dst, Type: "workspace", Method: method})
	}
	return links
}

// LinkRepoContext links untracked, gitignored AI context files from a source repo to its worktree.
// Files that are tracked by git or not gitignored are skipped with reasons.
func LinkRepoContext(srcRepo, dstRepo, repoName string, candidates []string) ([]state.ContextLink, []SkipInfo) {
	var links []state.ContextLink
	var skipped []SkipInfo
	for _, name := range candidates {
		src := filepath.Join(srcRepo, name)
		if _, err := os.Lstat(src); err != nil {
			continue
		}
		if IsTracked(srcRepo, name) {
			skipped = append(skipped, SkipInfo{Name: name, Reason: "tracked by git, already in worktree"})
			continue
		}
		dst := filepath.Join(dstRepo, name)
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		method, err := linkOrCopy(src, dst)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [error] %s/%s: %v\n", repoName, name, err)
			continue
		}
		links = append(links, state.ContextLink{Src: src, Dst: dst, Type: "repo", Method: method})
	}
	return links, skipped
}

// IsTracked checks if a path is tracked by git in the given repo.
func IsTracked(repoDir, path string) bool {
	_, err := git.GitRun(repoDir, "ls-files", "--error-unmatch", path)
	return err == nil
}

// SkipInfo describes why a context file was skipped during linking.
type SkipInfo struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// RemoveContextLinks removes context links (symlinks or copied files/dirs).
func RemoveContextLinks(links []state.ContextLink) int {
	removed := 0
	for _, link := range links {
		if _, err := os.Lstat(link.Dst); err != nil {
			continue // already gone
		}
		if err := os.RemoveAll(link.Dst); err == nil {
			removed++
		}
	}
	return removed
}

var symlinkHintShown bool

// linkOrCopy tries os.Symlink first. If that fails, falls back to copying
// and prints a warning. Returns the method used ("symlink" or "copy").
func linkOrCopy(src, dst string) (string, error) {
	if err := os.Symlink(src, dst); err == nil {
		return "symlink", nil
	} else {
		fmt.Fprintf(os.Stderr, "  [warn] symlink failed for %s: %v, falling back to copy\n", filepath.Base(src), err)
		if !symlinkHintShown {
			fmt.Fprintf(os.Stderr, "  [hint] to fix: enable Developer Mode (Windows Settings → System → For developers), then run 'aw relink'\n")
			symlinkHintShown = true
		}
	}

	fi, err := os.Stat(src)
	if err != nil {
		return "", fmt.Errorf("cannot stat %s: %w", src, err)
	}

	if fi.IsDir() {
		err = CopyDir(src, dst)
	} else {
		err = CopyFile(src, dst)
	}
	if err != nil {
		return "", fmt.Errorf("copy failed for %s: %w", filepath.Base(src), err)
	}
	return "copy", nil
}

// CopyFile copies a single file from src to dst.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// CopyDir recursively copies a directory from src to dst.
func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return CopyFile(path, target)
	})
}
