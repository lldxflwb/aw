package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/aw/internal/git"
	"github.com/anthropics/aw/internal/output"
	"github.com/anthropics/aw/internal/state"
	"github.com/anthropics/aw/internal/workspace"
)

type rmResult struct {
	WorktreesRemoved int      `json:"worktrees_removed"`
	BranchesDeleted  int      `json:"branches_deleted"`
	LinksRemoved     int      `json:"links_removed"`
	Errors           []string `json:"errors,omitempty"`
}

// CmdRm implements "aw rm [--dir <path>] [--branch] [--force] [--dry-run] [--json]".
func CmdRm(args []string) {
	var dir string
	var deleteBranch, force, dryRun, jsonOut bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			if i+1 < len(args) {
				i++
				dir = args[i]
			}
		case "--branch":
			deleteBranch = true
		case "--force":
			force = true
		case "--dry-run":
			dryRun = true
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

	// Pre-check dirty state
	var dirtyRepos []string
	for _, repo := range ws.Repos {
		dirty, _ := git.GitRun(repo.WorktreePath, "status", "--porcelain")
		if dirty != "" {
			dirtyRepos = append(dirtyRepos, repo.Name)
		}
	}

	if len(dirtyRepos) > 0 && !force {
		msg := fmt.Sprintf("dirty repos: %s (use --force to override)", strings.Join(dirtyRepos, ", "))
		if jsonOut {
			output.JSONError("DIRTY_REPO", msg, 1)
		}
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
		os.Exit(1)
	}

	// Dry-run mode
	if dryRun {
		if jsonOut {
			result := rmResult{
				WorktreesRemoved: len(ws.Repos),
				LinksRemoved:     len(ws.ContextLinks),
			}
			if deleteBranch {
				result.BranchesDeleted = len(ws.Repos)
			}
			output.JSONSuccess(result, []string{"dry-run mode"})
		} else {
			fmt.Println("[dry-run] Would perform:")
			fmt.Printf("  Remove %d symlinks\n", len(ws.ContextLinks))
			for _, repo := range ws.Repos {
				fmt.Printf("  Remove worktree: %s\n", repo.WorktreePath)
				if deleteBranch {
					fmt.Printf("  Delete branch: %s (from %s)\n", ws.Branch, repo.SourcePath)
				}
			}
			fmt.Println("  Remove .aw/ directory")
		}
		return
	}

	// Acquire lock
	lock, err := state.Acquire(wsDir)
	if err != nil {
		exitErr(jsonOut, "LOCK_HELD", err, 1)
	}
	defer lock.Release()

	var errors []string
	var worktreesRemoved, branchesDeleted int

	// 1. Remove symlinks
	linksRemoved := workspace.RemoveSymlinks(ws.ContextLinks)
	if !jsonOut {
		fmt.Printf("Removed %d symlinks\n", linksRemoved)
	}

	// 2. Remove worktrees
	for _, repo := range ws.Repos {
		if !jsonOut {
			fmt.Printf("[%s] removing worktree...\n", repo.Name)
		}
		if err := git.WorktreeRemove(repo.SourcePath, repo.WorktreePath, force); err != nil {
			errMsg := fmt.Sprintf("[%s] worktree remove: %v", repo.Name, err)
			errors = append(errors, errMsg)
			if !jsonOut {
				fmt.Fprintf(os.Stderr, "  %s\n", errMsg)
			}
			continue
		}
		worktreesRemoved++

		// 3. Delete branch if requested
		if deleteBranch {
			if err := git.BranchDelete(repo.SourcePath, ws.Branch, force); err != nil {
				errMsg := fmt.Sprintf("[%s] branch delete: %v", repo.Name, err)
				errors = append(errors, errMsg)
				if !jsonOut {
					fmt.Fprintf(os.Stderr, "  %s\n", errMsg)
				}
			} else {
				branchesDeleted++
				if !jsonOut {
					fmt.Printf("[%s] branch %s deleted\n", repo.Name, ws.Branch)
				}
			}
		}
	}

	// 4. Clean up .aw/ directory
	os.RemoveAll(state.AWDir(wsDir))

	// 5. Try to remove workspace dir if empty
	removeEmptyDir(wsDir)

	// Output
	result := rmResult{
		WorktreesRemoved: worktreesRemoved,
		BranchesDeleted:  branchesDeleted,
		LinksRemoved:     linksRemoved,
		Errors:           errors,
	}

	if jsonOut {
		if len(errors) > 0 {
			output.JSONPartialFailure(result, "PARTIAL_FAILURE",
				fmt.Sprintf("%d errors", len(errors)),
				[]string{fmt.Sprintf("%d errors occurred", len(errors))})
			os.Exit(1)
		}
		output.JSONSuccess(result, []string{})
	} else {
		fmt.Println("---")
		fmt.Printf("Done. %d worktrees removed, %d branches deleted, %d links removed.\n",
			worktreesRemoved, branchesDeleted, linksRemoved)
		if len(errors) > 0 {
			fmt.Printf("Errors (%d):\n", len(errors))
			for _, e := range errors {
				fmt.Printf("  %s\n", e)
			}
			os.Exit(1)
		}
	}
}
