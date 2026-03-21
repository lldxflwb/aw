package git

import (
	"os/exec"
	"strings"
)

// GitRun executes a git command in the given directory and returns trimmed stdout.
func GitRun(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
