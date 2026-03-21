package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/aw/internal/output"
	"github.com/anthropics/aw/internal/state"
	"github.com/anthropics/aw/internal/workspace"
)

type relinkResult struct {
	Converted int      `json:"converted"`
	Skipped   int      `json:"skipped"`
	Failed    []string `json:"failed,omitempty"`
}

// CmdRelink converts copy-based context links back to symlinks.
// Intended for Windows users who later enable Developer Mode and want
// to upgrade their context links from copies to symlinks.
func CmdRelink(args []string) {
	var dir string
	var jsonOut bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			if i+1 < len(args) {
				i++
				dir = args[i]
			}
		case "--json":
			jsonOut = true
		}
	}

	wsDir, err := resolveWSDir(dir)
	if err != nil {
		exitErr(jsonOut, "PATH_ERROR", err, 2)
	}

	ws, err := state.Load(wsDir)
	if err != nil {
		if jsonOut {
			output.JSONError("NOT_WORKSPACE", "no .aw/workspace.json found: "+err.Error(), 2)
		}
		fmt.Fprintf(os.Stderr, "error: no .aw/workspace.json found in %s\n", wsDir)
		fmt.Fprintln(os.Stderr, "hint: is this an aw workspace?")
		os.Exit(2)
	}

	// Check if there are any copy links
	var copyLinks int
	for _, link := range ws.ContextLinks {
		if link.Method == "copy" {
			copyLinks++
		}
	}
	if copyLinks == 0 {
		if jsonOut {
			output.JSONSuccess(relinkResult{}, []string{})
		} else {
			fmt.Println("All context links are already symlinks. Nothing to do.")
		}
		return
	}

	lock, err := state.Acquire(wsDir)
	if err != nil {
		exitErr(jsonOut, "LOCK_HELD", err, 1)
	}
	defer lock.Release()

	var converted, skipped int
	var failed []string

	for i, link := range ws.ContextLinks {
		if link.Method != "copy" {
			skipped++
			continue
		}

		name := filepath.Base(link.Dst)

		// Verify source still exists
		if _, err := os.Lstat(link.Src); err != nil {
			msg := fmt.Sprintf("%s: source missing", name)
			failed = append(failed, msg)
			if !jsonOut {
				fmt.Fprintf(os.Stderr, "  [error] %s\n", msg)
			}
			continue
		}

		// Remove the copy
		if err := os.RemoveAll(link.Dst); err != nil {
			msg := fmt.Sprintf("%s: cannot remove copy: %v", name, err)
			failed = append(failed, msg)
			if !jsonOut {
				fmt.Fprintf(os.Stderr, "  [error] %s\n", msg)
			}
			continue
		}

		// Try symlink
		if err := os.Symlink(link.Src, link.Dst); err != nil {
			// Symlink still fails — restore the copy
			if !jsonOut {
				fmt.Fprintf(os.Stderr, "  [error] %s: symlink failed: %v\n", name, err)
			}

			// Restore copy
			fi, statErr := os.Stat(link.Src)
			if statErr == nil {
				if fi.IsDir() {
					workspace.CopyDir(link.Src, link.Dst)
				} else {
					workspace.CopyFile(link.Src, link.Dst)
				}
			}

			msg := fmt.Sprintf("%s: symlink failed, copy restored (enable Developer Mode or run as admin)", name)
			failed = append(failed, msg)
			if !jsonOut {
				fmt.Fprintf(os.Stderr, "  [hint] %s\n", msg)
			}
			continue
		}

		ws.ContextLinks[i].Method = "symlink"
		converted++
		if !jsonOut {
			fmt.Printf("  [ok] %s: copy → symlink\n", name)
		}
	}

	// Save updated state
	if err := state.Save(wsDir, ws); err != nil && !jsonOut {
		fmt.Fprintf(os.Stderr, "warning: failed to update workspace.json: %v\n", err)
	}

	if jsonOut {
		result := relinkResult{
			Converted: converted,
			Skipped:   skipped,
			Failed:    failed,
		}
		if len(failed) > 0 {
			output.JSONPartialFailure(result, "RELINK_FAILED",
				fmt.Sprintf("%d links could not be converted", len(failed)), nil)
			os.Exit(1)
		}
		output.JSONSuccess(result, nil)
	} else {
		fmt.Printf("---\nDone. %d converted, %d already symlinks", converted, skipped)
		if len(failed) > 0 {
			fmt.Printf(", %d failed", len(failed))
		}
		fmt.Println()
		if len(failed) > 0 {
			fmt.Fprintln(os.Stderr, "\nhint: on Windows, enable Developer Mode (Settings → Update & Security → For developers) then re-run 'aw relink'")
			os.Exit(1)
		}
	}
}
