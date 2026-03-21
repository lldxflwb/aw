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

Run 'aw <command> --help' for command-specific help.`)
}
