package cli_test

import (
	"bytes"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/cli"
)

const (
	toolAuditHealth  = "linode_audit_health"
	toolAuditSummary = "linode_audit_summary"
	outputTable      = "table"
)

// TestCallTableRendersScalarsAndNestedValues drives the health tool through
// --output table. Its payload mixes a bool, whole-number counters, an empty
// string, and an array, so one render proves each cell shape: bools print
// as true/false, whole numbers print without a fraction, an empty string
// prints as an empty cell, and a nested value collapses to compact JSON on
// one line. No audit tool emits a JSON null, so the nil cell shape is not
// pinned here.
func TestCallTableRendersScalarsAndNestedValues(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolAuditHealth, flagOutput, outputTable}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	out := stdout.String()
	wantMatch(t, "bool cell", out, `(?m)^active_log_exists\s+(true|false)$`)
	wantMatch(t, "whole-number cell", out, `(?m)^dropped_events\s+0$`)
	wantMatch(t, "whole-number cell", out, `(?m)^disk_bytes\s+\d+$`)
	wantMatch(t, "empty string cell", out, `(?m)^oldest_rotated_date\s*$`)
	wantMatch(t, "array cell", out, `(?m)^warnings\s+\[\]$`)

	if countLines(out) < 2 {
		t.Fatalf("table has no data rows:\n%s", out)
	}
}

// TestCallTablePrintsPlainTextPayloadVerbatim checks the table renderer
// steps aside for a payload that is not JSON: an audit tool's validation
// error is prose, so --output table prints it as-is (no FIELD/VALUE
// header) and the exit code still reports the error result.
func TestCallTablePrintsPlainTextPayloadVerbatim(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand(
		[]string{toolAuditSummary, flagArg, "since=yesterday", flagOutput, outputTable},
		&stdout, &stderr,
	)
	if code != 1 {
		t.Fatalf("exit code = %d (stderr: %s), want 1", code, stderr.String())
	}

	out := stdout.String()
	wantContains(t, "stdout", out, "invalid 'since' timestamp")
	wantNotContains(t, "stdout", out, "FIELD")
	wantContains(t, "stderr", stderr.String(), "tool returned an error result")
}
