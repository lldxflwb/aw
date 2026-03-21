package git

import (
	"strconv"
	"strings"
)

// RepoStatus holds status info for a git repo worktree.
type RepoStatus struct {
	Branch     string `json:"branch"`
	DirtyCount int    `json:"dirty_count"`
	Ahead      int    `json:"ahead"`
	Behind     int    `json:"behind"`
	LastCommit string `json:"last_commit"`
}

// GetRepoStatus collects branch, dirty count, ahead/behind, and last commit.
func GetRepoStatus(dir string) (*RepoStatus, error) {
	s := &RepoStatus{}

	branch, err := GitRun(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}
	s.Branch = branch

	dirty, err := GitRun(dir, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	if dirty != "" {
		s.DirtyCount = len(strings.Split(dirty, "\n"))
	}

	// ahead/behind — ignore error (no upstream is fine)
	ab, err := GitRun(dir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err == nil {
		parts := strings.Fields(ab)
		if len(parts) == 2 {
			s.Ahead, _ = strconv.Atoi(parts[0])
			s.Behind, _ = strconv.Atoi(parts[1])
		}
	}

	last, err := GitRun(dir, "log", "-1", "--format=%h %s")
	if err == nil {
		s.LastCommit = last
	}

	return s, nil
}
