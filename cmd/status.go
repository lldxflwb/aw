package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/anthropics/aw/internal/git"
	"github.com/anthropics/aw/internal/output"
	"github.com/anthropics/aw/internal/state"
)

type statusResult struct {
	Source string              `json:"source"`
	Branch string             `json:"branch"`
	Repos  []repoStatusEntry  `json:"repos"`
}

type repoStatusEntry struct {
	Name       string `json:"name"`
	Branch     string `json:"branch"`
	DirtyCount int    `json:"dirty_count"`
	Ahead      int    `json:"ahead"`
	Behind     int    `json:"behind"`
	LastCommit string `json:"last_commit"`
}

// CmdStatus implements "aw status [--dir <path>] [--json] [--short]".
func CmdStatus(args []string) {
	var dir string
	var jsonOut, short bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			if i+1 < len(args) {
				i++
				dir = args[i]
			}
		case "--json":
			jsonOut = true
		case "--short":
			short = true
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

	var entries []repoStatusEntry
	for _, repo := range ws.Repos {
		s, err := git.GetRepoStatus(repo.WorktreePath)
		if err != nil {
			if !jsonOut {
				fmt.Fprintf(os.Stderr, "[%s] error: %v\n", repo.Name, err)
			}
			continue
		}
		entries = append(entries, repoStatusEntry{
			Name:       repo.Name,
			Branch:     s.Branch,
			DirtyCount: s.DirtyCount,
			Ahead:      s.Ahead,
			Behind:     s.Behind,
			LastCommit: s.LastCommit,
		})
	}

	if jsonOut {
		output.JSONSuccess(statusResult{
			Source: ws.Source,
			Branch: ws.Branch,
			Repos:  entries,
		}, []string{})
		return
	}

	if short {
		for _, e := range entries {
			status := "clean"
			if e.DirtyCount > 0 {
				status = fmt.Sprintf("%dM", e.DirtyCount)
			}
			fmt.Printf("%s\t%s\t%s\n", e.Name, e.Branch, status)
		}
		return
	}

	// Table output
	headers := []string{"REPO", "BRANCH", "STATUS", "AHEAD", "BEHIND", "LAST COMMIT"}
	var rows [][]string
	for _, e := range entries {
		status := "clean"
		if e.DirtyCount > 0 {
			status = fmt.Sprintf("%dM", e.DirtyCount)
		}
		rows = append(rows, []string{
			e.Name,
			e.Branch,
			status,
			strconv.Itoa(e.Ahead),
			strconv.Itoa(e.Behind),
			e.LastCommit,
		})
	}
	output.Table(headers, rows)
}
