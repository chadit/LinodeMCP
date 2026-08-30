package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/cli"
)

const (
	subverbRecent   = "recent"
	subverbSummary  = "summary"
	subverbExport   = "export"
	subverbHealth   = "health"
	flagIncludeMeta = "--include-meta"
	flagTool        = "--tool"
	flagSince       = "--since"
	flagBogus       = "--bogus"
)

// recordVersionCall runs `call version` so the audit log under
// XDG_STATE_HOME holds exactly one meta event for the audit verbs to find.
func recordVersionCall(t *testing.T) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	if code := cli.RunCallCommand([]string{toolVersion}, &stdout, &stderr); code != 0 {
		t.Fatalf("call version exit code = %d (stderr: %s), want 0", code, stderr.String())
	}
}

// auditLogPath returns the active JSONL log the sink writes under the
// given XDG state home, the same file the audit verbs read.
func auditLogPath(stateHome string) string {
	return filepath.Join(stateHome, "linodemcp", "audit.log")
}

// eventCount decodes an `audit recent` payload and returns how many
// events it lists.
func eventCount(t *testing.T, payload string) int {
	t.Helper()

	body := decodeJSONObject(t, payload)

	events, ok := body["events"].([]any)
	if !ok {
		t.Fatalf("recent payload has no events list: %v", body)
	}

	return len(events)
}

// TestAuditRecentFlagsReachTheQuery checks each recent flag changes what
// comes back: a meta event recorded by `call version` is hidden by
// default, shown with --include-meta, matched by a --tool glob, and
// excluded by a --since bound after it happened. Every audit query is
// itself audited, so the counting rows filter on the version tool to
// keep earlier rows' own events out of the count.
func TestAuditRecentFlagsReachTheQuery(t *testing.T) {
	configPath := writeTestConfigFile(t)
	stateHome := t.TempDir()

	t.Setenv("LINODEMCP_CONFIG_PATH", configPath)
	t.Setenv("XDG_STATE_HOME", stateHome)

	recordVersionCall(t)

	// One hour ahead is the shape a user types for "nothing yet"; a far
	// future year would overflow the reader's nanosecond comparison.
	futureSince := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

	cases := []struct {
		name string
		args []string
		want int
	}{
		{name: "meta event hidden without include-meta", args: []string{subverbRecent, flagTool, toolVersion}, want: 0},
		{name: "include-meta shows the version call", args: []string{subverbRecent, flagIncludeMeta, flagTool, toolVersion}, want: 1},
		{name: "tool glob matches the version call", args: []string{subverbRecent, flagIncludeMeta, flagTool, "vers*"}, want: 1},
		{name: "tool glob for an unused name matches nothing", args: []string{subverbRecent, flagIncludeMeta, flagTool, "nothing_*"}, want: 0},
		{
			name: "since after the call excludes it",
			args: []string{subverbRecent, flagIncludeMeta, flagTool, toolVersion, flagSince, futureSince},
			want: 0,
		},
	}

	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) {
			// Every row reads the one store the parent seeded, so the
			// subtests share its environment rather than running in parallel.
			t.Setenv("LINODEMCP_CONFIG_PATH", configPath)
			t.Setenv("XDG_STATE_HOME", stateHome)

			var stdout, stderr bytes.Buffer

			code := cli.RunAuditCommand(scenario.args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
			}

			if got := eventCount(t, stdout.String()); got != scenario.want {
				t.Fatalf("%v listed %d events, want %d", scenario.args, got, scenario.want)
			}
		})
	}
}

// TestAuditRecentSurvivesCorruptLogLine checks a hand-damaged audit.log
// does not sink the query: the corrupt line is skipped and the intact
// event before it is still listed.
func TestAuditRecentSurvivesCorruptLogLine(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", stateHome)

	recordVersionCall(t)

	logFile, err := os.OpenFile(auditLogPath(stateHome), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}

	if _, err := logFile.WriteString("{this is not an audit record\n"); err != nil {
		t.Fatalf("append corrupt line: %v", err)
	}

	if err := logFile.Close(); err != nil {
		t.Fatalf("close audit log: %v", err)
	}

	var stdout, stderr bytes.Buffer

	code := cli.RunAuditCommand([]string{subverbRecent, flagIncludeMeta, flagTool, toolVersion}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	if got := eventCount(t, stdout.String()); got != 1 {
		t.Fatalf("listed %d events, want the 1 intact event", got)
	}
}

// TestAuditSummaryCountsNothingOnEmptyStore checks `audit summary` on a
// store no call has written exits 0 with a zero total rather than
// treating the missing log as an error.
func TestAuditSummaryCountsNothingOnEmptyStore(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunAuditCommand([]string{subverbSummary}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	body := decodeJSONObject(t, stdout.String())

	total, ok := body["total_events"].(float64)
	if !ok || total != 0 {
		t.Fatalf("total_events = %v, want 0", body["total_events"])
	}
}

// TestAuditSummaryRejectsMalformedSince checks --since is passed to the
// summary tool, whose RFC 3339 check turns a bad value into an error
// result: the message prints and the exit code is 1.
func TestAuditSummaryRejectsMalformedSince(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunAuditCommand([]string{subverbSummary, flagSince, "yesterday"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d (stderr: %s), want 1", code, stderr.String())
	}

	wantContains(t, "stdout", stdout.String(), `invalid 'since' timestamp`)
	wantContains(t, "stdout", stdout.String(), "yesterday")
	wantContains(t, "stderr", stderr.String(), "error result")
}

// TestAuditVerbsRejectUnknownFlags checks each flag-taking verb turns an
// undefined flag into a usage exit that names the flag, before any
// runtime is built.
func TestAuditVerbsRejectUnknownFlags(t *testing.T) {
	for _, verb := range []string{subverbRecent, subverbSummary, subverbExport} {
		t.Run(verb, func(t *testing.T) {
			t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
			t.Setenv("XDG_STATE_HOME", t.TempDir())

			var stdout, stderr bytes.Buffer

			code := cli.RunAuditCommand([]string{verb, flagBogus}, &stdout, &stderr)
			if code != exitUsage {
				t.Fatalf("exit code = %d, want %d", code, exitUsage)
			}

			wantContains(t, "stderr", stderr.String(), "flag provided but not defined: -bogus")
		})
	}
}

// TestAuditHealthRejectsArguments checks `audit health extra` is a usage
// error instead of a health query that ignores the stray word.
func TestAuditHealthRejectsArguments(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunAuditCommand([]string{subverbHealth, "extra"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}

	wantContains(t, "stderr", stderr.String(), "takes no arguments")
}
