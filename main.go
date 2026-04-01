package main

import (
	"fmt"
	"os"

	"github.com/anthropics/aw/cmd"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "new":
		cmd.CmdNew(os.Args[2:])
	case "rm":
		cmd.CmdRm(os.Args[2:])
	case "status":
		cmd.CmdStatus(os.Args[2:])
	case "list":
		cmd.CmdList(os.Args[2:])
	case "prune":
		cmd.CmdPrune(os.Args[2:])
	case "relink":
		cmd.CmdRelink(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: aw <command> [options]

Commands:
  new      Create a new workspace with worktrees
  rm       Remove a workspace and its worktrees
  status   Show status of workspace repos
  list     List all registered workspaces
  prune    Remove stale entries from workspace registry
  relink   Convert copy-based context links back to symlinks

Examples:
  aw new -usb feature/login          # update, clone session, create branch
  aw new -usb fix -f dev             # based on dev branch
  aw rm -fb                          # force remove + delete branches
  aw list                            # show all workspaces`)
}
