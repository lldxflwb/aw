package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/aw/internal/output"
)

// resolveWSDir resolves the workspace directory from --dir flag or cwd.
func resolveWSDir(dir string) (string, error) {
	if dir != "" {
		return filepath.Abs(dir)
	}
	return os.Getwd()
}

// exitErr prints an error (human or JSON) and exits.
func exitErr(jsonOut bool, code string, err error, exitCode int) {
	if jsonOut {
		output.JSONError(code, err.Error(), exitCode)
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(exitCode)
}

// removeEmptyDir removes a directory only if it is empty.
func removeEmptyDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		os.Remove(dir)
	}
}
