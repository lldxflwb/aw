package workspace

import (
	"os"
	"path/filepath"
)

// ScanRepos scans a directory for git repositories (subdirs containing .git/).
func ScanRepos(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var repos []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		gitDir := filepath.Join(dir, e.Name(), ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			repos = append(repos, e.Name())
		}
	}
	return repos, nil
}
