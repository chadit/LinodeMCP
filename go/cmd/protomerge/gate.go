package main

import (
	"fmt"
	"os"
	"strings"
)

// The report shape scripts/_hardgate.py defines, written here because this gate
// is a Go binary: no baseline to accept a finding into, and an empty scan fails.

// say emits gate output rather than logging; a failed write leaves nothing to
// report the failure on.
func say(line string) {
	_, _ = os.Stdout.WriteString(line + "\n")
}

// measured refuses an empty scan, naming it so the reader knows which walk
// stopped: a moved module path, a renamed contract file, a walk that gave up.
func measured(what string, count int) error {
	if count > 0 {
		return nil
	}

	return fmt.Errorf("%w: %s", errScannedNothing, what)
}

// report prints every finding and answers the exit code; there is no accepted
// subset to diff against.
func report(title string, findings []string, fix string) int {
	if len(findings) == 0 {
		return 0
	}

	say(fmt.Sprintf("%s (%d):", title, len(findings)))

	for _, finding := range findings {
		say("  " + finding)
	}

	say("")
	say(fix)

	return 1
}

// quote makes a trailing-whitespace difference visible in a finding.
func quote(line string) string {
	return `"` + strings.ReplaceAll(line, `"`, `\"`) + `"`
}
