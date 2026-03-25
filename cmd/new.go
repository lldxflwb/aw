package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/aw/internal/git"
	"github.com/anthropics/aw/internal/output"
	"github.com/anthropics/aw/internal/session"
	"github.com/anthropics/aw/internal/state"
	"github.com/anthropics/aw/internal/workspace"
)

type newResult struct {
	Source       string            `json:"source"`
	Target       string            `json:"target"`
	Branch       string            `json:"branch"`
	Repos        []state.RepoEntry `json:"repos"`
	Failed       []string          `json:"failed"`
	ContextLinks int               `json:"context_links"`
}

// CmdNew implements "aw new --dir <target> -b <branch> [--json] [-u] [-s] [--session-limit N] [--session-id UUID]".
func CmdNew(args []string) {
	var dir, branch, sessionID, fromBranch string
	var jsonOut, update, cloneSession bool
	sessionLimit := 0

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dir":
			if i+1 < len(args) {
				i++
				dir = args[i]
			}
		case arg == "--json":
			jsonOut = true
		case arg == "--update":
			update = true
		case arg == "--from":
			if i+1 < len(args) {
				i++
				fromBranch = args[i]
			} else {
				fmt.Fprintln(os.Stderr, "error: --from requires a value")
				os.Exit(2)
			}
		case arg == "--clone-session":
			cloneSession = true
		case arg == "--session-limit":
			if i+1 < len(args) {
				i++
				n, err := strconv.Atoi(args[i])
				if err != nil || n < 1 {
					fmt.Fprintf(os.Stderr, "error: --session-limit must be a positive integer\n")
					os.Exit(2)
				}
				sessionLimit = n
			}
		case arg == "--session-id":
			if i+1 < len(args) {
				i++
				sessionID = args[i]
			} else {
				fmt.Fprintln(os.Stderr, "error: --session-id requires a value")
				os.Exit(2)
			}
		default:
			// short flags: -b requires a value, -u/-s are boolean
			if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
				for ci, c := range arg[1:] {
					switch c {
					case 'b':
						if ci == len(arg[1:])-1 && i+1 < len(args) {
							i++
							branch = args[i]
						}
					case 'f':
						if ci == len(arg[1:])-1 && i+1 < len(args) {
							i++
							fromBranch = args[i]
						}
					case 'u':
						update = true
					case 's':
						cloneSession = true
					}
				}
			}
		}
	}

	// Resolve session clone parameters
	doClone := cloneSession || sessionLimit > 0 || sessionID != ""
	if sessionID != "" {
		sessionLimit = 1 // --session-id implies exactly 1
	} else if doClone && sessionLimit == 0 {
		sessionLimit = 1 // --clone-session/-s defaults to 1
	}
	if sessionLimit > 10 {
		sessionLimit = 10
	}

	if dir == "" || branch == "" {
		if jsonOut {
			output.JSONError("USAGE_ERROR", "--dir and -b are required", 2)
		}
		fmt.Fprintln(os.Stderr, "error: --dir and -b are required")
		fmt.Fprintln(os.Stderr, "Usage: aw new --dir <target> -b <branch>")
		os.Exit(2)
	}

	targetDir, err := filepath.Abs(dir)
	if err != nil {
		exitErr(jsonOut, "PATH_ERROR", err, 1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		exitErr(jsonOut, "PATH_ERROR", err, 1)
	}

	repos, err := workspace.ScanRepos(cwd)
	if err != nil {
		exitErr(jsonOut, "SCAN_ERROR", err, 1)
	}
	if len(repos) == 0 {
		exitErr(jsonOut, "NO_REPOS", fmt.Errorf("no git repos found in current directory"), 1)
	}

	if !jsonOut {
		fmt.Printf("Found %d repos: %s\n", len(repos), strings.Join(repos, ", "))
		fmt.Printf("Target: %s\n", targetDir)
		fmt.Printf("Branch: %s\n\n", branch)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		exitErr(jsonOut, "DIR_ERROR", err, 1)
	}

	// Acquire lock
	lock, err := state.Acquire(targetDir)
	if err != nil {
		exitErr(jsonOut, "LOCK_HELD", err, 1)
	}
	defer lock.Release()

	// Update repos before branching
	if update {
		if !jsonOut {
			fmt.Println("== updating repos ==")
		}
		for _, repo := range repos {
			repoPath := filepath.Join(cwd, repo)
			if !jsonOut {
				fmt.Printf("[%s] pulling...", repo)
			}
			_, err := git.GitRun(repoPath, "pull", "--ff-only")
			if err != nil {
				if !jsonOut {
					fmt.Printf(" skipped (not fast-forwardable)\n")
				}
			} else {
				if !jsonOut {
					fmt.Printf(" ok\n")
				}
			}
		}
		if !jsonOut {
			fmt.Println()
		}
	}

	// Create worktrees
	var failed []string
	var repoEntries []state.RepoEntry

	for _, repo := range repos {
		repoPath := filepath.Join(cwd, repo)
		worktreePath := filepath.Join(targetDir, repo)

		// Determine start point for this repo
		startPoint := ""
		displayFrom := "HEAD"
		if b, err := git.GitRun(repoPath, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
			displayFrom = b
		}

		if fromBranch != "" {
			if git.RefExists(repoPath, fromBranch) {
				startPoint = fromBranch
				displayFrom = fromBranch
			} else if !jsonOut {
				fmt.Fprintf(os.Stderr, "[%s] warn: %s not found, using %s\n", repo, fromBranch, displayFrom)
			}
		}

		if !jsonOut {
			fmt.Printf("[%s] %s → %s\n", repo, displayFrom, branch)
		}

		if err := git.WorktreeAdd(repoPath, worktreePath, branch, startPoint); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] FAILED: %v\n", repo, err)
			failed = append(failed, repo)
			continue
		}

		repoEntries = append(repoEntries, state.RepoEntry{
			Name:         repo,
			SourcePath:   repoPath,
			WorktreePath: worktreePath,
		})

		if !jsonOut {
			fmt.Printf("[%s] OK\n\n", repo)
		}
	}

	// Load config
	cfg, created, err := workspace.LoadOrCreateConfig(cwd)
	if err != nil {
		exitErr(jsonOut, "CONFIG_ERROR", err, 1)
	}
	if created && !jsonOut {
		fmt.Println("Created aw.yml with default context files")
	}

	// Workspace-level symlinks
	if !jsonOut {
		fmt.Println("== workspace context ==")
	}
	wsLinks := workspace.LinkWorkspaceContext(cwd, targetDir, cfg.Context)
	if !jsonOut {
		for _, link := range wsLinks {
			fmt.Printf("  [link] %s\n", filepath.Base(link.Dst))
		}
	}

	// Repo-level symlinks
	if !jsonOut {
		fmt.Println("== repo context ==")
	}
	var allLinks []state.ContextLink
	allLinks = append(allLinks, wsLinks...)

	for _, entry := range repoEntries {
		repoLinks, skipped := workspace.LinkRepoContext(entry.SourcePath, entry.WorktreePath, entry.Name, cfg.Context)
		allLinks = append(allLinks, repoLinks...)
		if !jsonOut {
			for _, s := range skipped {
				fmt.Printf("  [skip] %s/%s (%s)\n", entry.Name, s.Name, s.Reason)
			}
			for _, link := range repoLinks {
				fmt.Printf("  [link] %s/%s (untracked)\n", entry.Name, filepath.Base(link.Dst))
			}
		}
	}

	// Write workspace.json
	ws := &state.WorkspaceState{
		Version:      state.StateVersion,
		Source:       cwd,
		Branch:       branch,
		CreatedAt:    time.Now().Format(time.RFC3339),
		Repos:        repoEntries,
		ContextLinks: allLinks,
	}
	if err := state.Save(targetDir, ws); err != nil && !jsonOut {
		fmt.Fprintf(os.Stderr, "warning: failed to write workspace.json: %v\n", err)
	}

	// Clone sessions if requested
	if doClone {
		if !jsonOut {
			fmt.Println("== session ==")
		}
		sessionFound := false
		results, err := session.CloneSessions(cwd, targetDir, sessionID, sessionLimit)
		if err != nil {
			if !jsonOut {
				fmt.Fprintf(os.Stderr, "  [warn] session clone: %v\n", err)
			}
		} else {
			for _, r := range results {
				short := r.ID
				if len(short) > 8 {
					short = short[:8]
				}
				if r.Status == "ok" {
					ws.ClonedSessionIDs = append(ws.ClonedSessionIDs, r.ID)
					sessionFound = true
					if !jsonOut {
						fmt.Printf("  [clone] %s\n", short)
					}
				} else if !jsonOut {
					fmt.Printf("  [skip] %s (%s)\n", short, r.Status)
				}
			}

			// Symlink memory directory
			memLink, memStatus := session.LinkMemory(cwd, targetDir)
			if memLink != nil {
				ws.SharedMemory = &state.MemoryInfo{Src: memLink.Src, Dst: memLink.Dst}
				if !jsonOut {
					fmt.Printf("  [link] memory/\n")
				}
			} else if memStatus != "missing_source" && memStatus != "already_linked" && !jsonOut {
				fmt.Fprintf(os.Stderr, "  [warn] memory: %s\n", memStatus)
			}

			// Re-save workspace.json with session info
			if err := state.Save(targetDir, ws); err != nil {
				// Rollback cloned files since state won't record them
				session.RollbackClonedFiles(results)
				if memLink != nil {
					session.CleanupMemoryLink(&session.MemoryLink{Src: memLink.Src, Dst: memLink.Dst})
				}
				ws.ClonedSessionIDs = nil
				ws.SharedMemory = nil
				if !jsonOut {
					fmt.Fprintf(os.Stderr, "  [warn] state save failed, rolled back session clone\n")
				}
			}
		}

		if !sessionFound && !jsonOut {
			fmt.Println("  [skip] no session found")
		}
	}

	// Register in source directory's registry
	if err := state.UpsertWorkspace(cwd, targetDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: REGISTRY_DESYNC: %v\n", err)
		fmt.Fprintf(os.Stderr, "  to fix: add {\"dir\":\"%s\"} to %s\n",
			state.CanonicalizePath(targetDir), state.RegistryPath(cwd))
	}

	// Output
	if jsonOut {
		var warnings []string
		if len(failed) > 0 {
			warnings = append(warnings, fmt.Sprintf("%d repos failed", len(failed)))
		}
		output.JSONSuccess(newResult{
			Source:       cwd,
			Target:       targetDir,
			Branch:       branch,
			Repos:        repoEntries,
			Failed:       failed,
			ContextLinks: len(allLinks),
		}, warnings)
	} else {
		fmt.Println("---")
		fmt.Printf("Done. %d/%d repos, %d context links.\n",
			len(repoEntries), len(repos), len(allLinks))
		if len(failed) > 0 {
			fmt.Printf("Failed: %s\n", strings.Join(failed, ", "))
		}
	}

	if len(failed) > 0 {
		os.Exit(1)
	}
}
