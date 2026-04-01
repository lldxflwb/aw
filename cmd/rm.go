package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/aw/internal/git"
	"github.com/anthropics/aw/internal/output"
	"github.com/anthropics/aw/internal/session"
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
	var deleteBranch, force, dryRun, jsonOut, saveSession bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dir":
			if i+1 < len(args) {
				i++
				dir = args[i]
			}
		case arg == "--branch" || arg == "-b":
			deleteBranch = true
		case arg == "--force" || arg == "-f":
			force = true
		case arg == "--dry-run":
			dryRun = true
		case arg == "--json":
			jsonOut = true
		case arg == "--save-session":
			saveSession = true
		default:
			// support combined short flags: -fb, -bf, etc.
			if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
				for _, c := range arg[1:] {
					switch c {
					case 'f':
						force = true
					case 'b':
						deleteBranch = true
					}
				}
			}
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

	// Source mismatch check
	if _, err := os.Stat(ws.Source); err != nil {
		msg := fmt.Sprintf("source directory no longer exists at %s", ws.Source)
		if jsonOut {
			output.JSONError("SOURCE_MISMATCH", msg, 1)
		}
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
		fmt.Fprintln(os.Stderr, "hint: cannot safely proceed when source is missing")
		os.Exit(1)
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
	var failedRepos []state.RepoEntry

	// 1. Remove worktrees first (before context links)
	for _, repo := range ws.Repos {
		if !jsonOut {
			fmt.Printf("[%s] removing worktree...\n", repo.Name)
		}
		if err := git.WorktreeRemove(repo.SourcePath, repo.WorktreePath, force); err != nil {
			errMsg := fmt.Sprintf("[%s] worktree remove: %v", repo.Name, err)
			errors = append(errors, errMsg)
			failedRepos = append(failedRepos, repo)
			if !jsonOut {
				fmt.Fprintf(os.Stderr, "  %s\n", errMsg)
			}
			continue
		}
		worktreesRemoved++

		// Delete branch if requested
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

	var linksRemoved int

	if len(failedRepos) == 0 {
		// All worktrees removed — safe to clean up everything

		// 2. Save back sessions if requested
		if saveSession {
			moved, err := session.SaveBackSessions(ws.Source, wsDir)
			if err != nil && !jsonOut {
				fmt.Fprintf(os.Stderr, "warning: save-session: %v\n", err)
			}
			if moved > 0 && !jsonOut {
				fmt.Printf("Saved %d session(s) back to source\n", moved)
			}
		}

		// 3. Cleanup memory symlink
		if ws.SharedMemory != nil {
			memLink := &session.MemoryLink{Src: ws.SharedMemory.Src, Dst: ws.SharedMemory.Dst}
			if session.CleanupMemoryLink(memLink) && !jsonOut {
				fmt.Println("Removed memory symlink")
			}
		}

		// 4. Remove context links
		linksRemoved = workspace.RemoveContextLinks(ws.ContextLinks)
		if !jsonOut {
			fmt.Printf("Removed %d context links\n", linksRemoved)
		}

		// 5. Clean up .aw/ directory
		lock.Release() // release before deleting the lock file's parent dir
		os.RemoveAll(state.AWDir(wsDir))

		// 6. Remove from registry
		if err := state.RemoveWorkspace(ws.Source, wsDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update registry: %v\n", err)
		}

		// 7. Remove workspace dir
		if force {
			// --force: remove entire workspace dir regardless of remaining files
			os.RemoveAll(wsDir)
		} else {
			// Try to remove if empty, otherwise list remaining files
			removeEmptyDir(wsDir)
			if entries, err := os.ReadDir(wsDir); err == nil && len(entries) > 0 {
				if !jsonOut {
					fmt.Fprintf(os.Stderr, "note: workspace dir has remaining files:\n")
					for _, e := range entries {
						fmt.Fprintf(os.Stderr, "  %s\n", e.Name())
					}
					fmt.Fprintf(os.Stderr, "  use --force to remove, or: rm -rf %s\n", wsDir)
				}
			}
		}
	} else {
		// Partial failure — preserve workspace-level links, rewrite state with remaining repos
		if !jsonOut {
			fmt.Printf("Partial failure: keeping workspace-level context links and state\n")
		}
		ws.Repos = failedRepos
		if err := state.Save(wsDir, ws); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to rewrite workspace.json: %v\n", err)
		}
	}

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
