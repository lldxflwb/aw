package cmd

import (
	"fmt"
	"os"

	"github.com/anthropics/aw/internal/output"
	"github.com/anthropics/aw/internal/state"
)

type pruneResult struct {
	Removed int      `json:"removed"`
	Kept    int      `json:"kept"`
	Dirs    []string `json:"dirs"`
}

// CmdPrune implements "aw prune [--force] [--json]".
func CmdPrune(args []string) {
	var jsonOut, force bool
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--force":
			force = true
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		exitErr(jsonOut, "PATH_ERROR", err, 1)
	}

	reg, err := state.LoadRegistry(cwd)
	if err != nil {
		exitErr(jsonOut, "REGISTRY_ERROR", err, 1)
	}

	if len(reg.Workspaces) == 0 {
		if jsonOut {
			output.JSONSuccess(pruneResult{}, nil)
		} else {
			fmt.Println("No workspaces to prune")
		}
		return
	}

	lock, err := state.AcquireRegistry(cwd)
	if err != nil {
		exitErr(jsonOut, "LOCK_HELD", err, 1)
	}
	defer lock.Release()

	// Re-load after acquiring lock
	reg, err = state.LoadRegistry(cwd)
	if err != nil {
		exitErr(jsonOut, "REGISTRY_ERROR", err, 1)
	}

	var kept []state.RegistryEntry
	var removedDirs []string

	for _, ws := range reg.Workspaces {
		entry := classifyEntry(ws.Dir)

		shouldRemove := false
		switch entry.Status {
		case "missing_dir":
			shouldRemove = true
		case "missing_state", "invalid_state", "source_mismatch":
			shouldRemove = force
		}

		if shouldRemove {
			removedDirs = append(removedDirs, ws.Dir)
			if !jsonOut {
				fmt.Printf("  [prune] %s (%s)\n", ws.Dir, entry.Status)
			}
		} else {
			kept = append(kept, ws)
			if entry.Status != "ok" && !jsonOut {
				fmt.Printf("  [keep] %s (%s, use --force to remove)\n", ws.Dir, entry.Status)
			}
		}
	}

	if len(removedDirs) > 0 {
		reg.Workspaces = kept
		if err := state.SaveRegistry(cwd, reg); err != nil {
			exitErr(jsonOut, "REGISTRY_ERROR", err, 1)
		}
	}

	if jsonOut {
		output.JSONSuccess(pruneResult{
			Removed: len(removedDirs),
			Kept:    len(kept),
			Dirs:    removedDirs,
		}, nil)
	} else {
		fmt.Printf("Pruned %d, kept %d\n", len(removedDirs), len(kept))
	}
}
