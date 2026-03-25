package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// WorktreeAdd creates a new worktree with a new branch.
// If startPoint is non-empty, the new branch is based on that ref instead of HEAD.
func WorktreeAdd(repoDir, worktreePath, branch, startPoint string) error {
	args := []string{"worktree", "add", "-b", branch, worktreePath}
	if startPoint != "" {
		args = append(args, startPoint)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RefExists checks if a git ref (branch, tag, commit) exists in the repo.
func RefExists(repoDir, ref string) bool {
	_, err := GitRun(repoDir, "rev-parse", "--verify", "--quiet", "--", ref)
	return err == nil
}

// WorktreeRemove removes a worktree.
func WorktreeRemove(repoDir, worktreePath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
