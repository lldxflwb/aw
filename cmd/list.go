package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/anthropics/aw/internal/output"
	"github.com/anthropics/aw/internal/state"
)

type listResult struct {
	Workspaces []listEntry `json:"workspaces"`
}

type listEntry struct {
	Dir       string `json:"dir"`
	Branch    string `json:"branch,omitempty"`
	Repos     int    `json:"repos,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	Status    string `json:"status"`
}

// CmdList implements "aw list [--json]".
func CmdList(args []string) {
	var jsonOut bool
	for _, arg := range args {
		if arg == "--json" {
			jsonOut = true
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
			output.JSONSuccess(listResult{Workspaces: []listEntry{}}, nil)
		} else {
			fmt.Println("No workspaces")
		}
		return
	}

	var entries []listEntry
	for _, ws := range reg.Workspaces {
		entry := classifyEntry(ws.Dir)
		entries = append(entries, entry)
	}

	if jsonOut {
		output.JSONSuccess(listResult{Workspaces: entries}, nil)
		return
	}

	headers := []string{"DIR", "BRANCH", "REPOS", "CREATED", "STATUS"}
	var rows [][]string
	for _, e := range entries {
		branch, repos, created := "-", "-", "-"
		if e.Branch != "" {
			branch = e.Branch
		}
		if e.Repos > 0 {
			repos = fmt.Sprintf("%d", e.Repos)
		}
		if e.CreatedAt != "" {
			created = relativeTime(e.CreatedAt)
		}
		rows = append(rows, []string{e.Dir, branch, repos, created, e.Status})
	}
	output.Table(headers, rows)
}

func classifyEntry(dir string) listEntry {
	entry := listEntry{Dir: dir}

	fi, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			entry.Status = "missing_dir"
		} else {
			entry.Status = "inaccessible"
		}
		return entry
	}
	if !fi.IsDir() {
		entry.Status = "invalid_state"
		return entry
	}

	ws, err := state.Load(dir)
	if err != nil {
		if os.IsNotExist(err) {
			entry.Status = "missing_state"
		} else {
			entry.Status = "invalid_state"
		}
		return entry
	}

	if _, err := os.Stat(ws.Source); err != nil {
		entry.Status = "source_mismatch"
		entry.Branch = ws.Branch
		entry.Repos = len(ws.Repos)
		entry.CreatedAt = ws.CreatedAt
		return entry
	}

	entry.Status = "ok"
	entry.Branch = ws.Branch
	entry.Repos = len(ws.Repos)
	entry.CreatedAt = ws.CreatedAt
	return entry
}

func relativeTime(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
