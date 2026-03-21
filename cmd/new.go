package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/aw/internal/git"
	"github.com/anthropics/aw/internal/output"
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

// CmdNew implements "aw new --dir <target> -b <branch> [--json] [--jump]".
func CmdNew(args []string) {
	var dir, branch string
	var jsonOut, jump bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			if i+1 < len(args) {
				i++
				dir = args[i]
			}
		case "-b":
			if i+1 < len(args) {
				i++
				branch = args[i]
			}
		case "--json":
			jsonOut = true
		case "--jump":
			jump = true
		}
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

	// Create worktrees
	var failed []string
	var repoEntries []state.RepoEntry

	for _, repo := range repos {
		repoPath := filepath.Join(cwd, repo)
		worktreePath := filepath.Join(targetDir, repo)

		if !jsonOut {
			fmt.Printf("[%s] creating worktree → %s (branch: %s)\n", repo, worktreePath, branch)
		}

		if err := git.WorktreeAdd(repoPath, worktreePath, branch); err != nil {
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

	// Workspace-level symlinks
	if !jsonOut {
		fmt.Println("== workspace context ==")
	}
	wsLinks := workspace.LinkWorkspaceContext(cwd, targetDir)
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
		repoLinks := workspace.LinkRepoContext(entry.SourcePath, entry.WorktreePath, entry.Name)
		allLinks = append(allLinks, repoLinks...)
		if !jsonOut {
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

	// --jump: print path to stdout for shell eval
	if jump && !jsonOut {
		fmt.Println(targetDir)
	}

	if len(failed) > 0 {
		os.Exit(1)
	}
}
