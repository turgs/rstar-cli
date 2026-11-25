// Command rstar is the ReceptionStar CLI (agent spine: commands, skill, stdio MCP).
// Product leaf commands and MCP land in turgs/rstar-issues#479; this bootstrap only
// ships version and a doctor stub.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/turgs/rstar-cli/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return 2
	}

	switch args[0] {
	case "version", "--version", "-V":
		return cmdVersion()
	case "doctor":
		return cmdDoctor()
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "rstar: unknown command %q\n\n", args[0])
		printUsage(os.Stderr)
		return 2
	}
}

func cmdVersion() int {
	parts := []string{"rstar", version.Version}
	if c := strings.TrimSpace(version.Commit); c != "" {
		parts = append(parts, "("+c+")")
	}
	fmt.Println(strings.Join(parts, " "))
	return 0
}

func cmdDoctor() int {
	// Stub: real checks (API reachability, PAT, profile) arrive with #479.
	fmt.Println("rstar doctor: ok (stub)")
	fmt.Println("  binary: present")
	fmt.Printf("  version: %s\n", version.Version)
	fmt.Println("  auth: not configured yet")
	fmt.Println("  api: not checked yet")
	return 0
}

func printUsage(w *os.File) {
	fmt.Fprint(w, `rstar — ReceptionStar CLI

Usage:
  rstar <command>

Commands:
  version   Print binary version
  doctor    Environment checks (stub until leaf commands land)
  help      Show this help

Repo: https://github.com/turgs/rstar-cli
Board: https://github.com/turgs/rstar-issues/issues/502
`)
}
