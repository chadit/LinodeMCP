package main

// Phase 7a profile subcommand dispatch. The actual logic lives in
// internal/cli so it can be unit-tested with external-package tests
// (project policy disallows internal test packages on main).
//
// Mutation (use, clone, delete, enable, disable) lands in 7b/7c with
// atomic config writes.

import (
	"os"

	"github.com/chadit/LinodeMCP/internal/cli"
)

// runProfileCommand is the thin main-side wrapper that forwards to the
// cli package's exported entry point. main() calls this when the first
// positional argument is "profile".
func runProfileCommand(args []string) int {
	return cli.RunProfileCommand(args, os.Stdout, os.Stderr)
}
