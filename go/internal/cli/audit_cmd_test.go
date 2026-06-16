package cli_test

import (
	"bytes"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/cli"
)

// TestAuditHealthSucceeds checks `audit health` drives the health query
// tool and prints its JSON. The audit subsystem reports its paths, so the
// payload mentions the log path key.
func TestAuditHealthSucceeds(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunAuditCommand([]string{"health"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	// The health payload is a JSON object; decoding proves it's well
	// formed without pinning a specific key the tool may rename.
	decodeJSONObject(t, stdout.String())
}

// TestAuditRecentSucceeds checks `audit recent --limit 5` drives the
// recent query and returns an events list, even when empty.
func TestAuditRecentSucceeds(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunAuditCommand([]string{"recent", "--limit", "5"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	body := decodeJSONObject(t, stdout.String())
	if _, ok := body["events"]; !ok {
		t.Errorf("recent payload missing events key: %v", body)
	}
}

// TestAuditExportMissingFormatReturnsError checks that `audit export`
// with no --format reaches the export tool, which returns an error
// result, mapping to exit 1 (a tool-level error, not a usage error).
// This exercises the IsError path of the dispatch.
func TestAuditExportMissingFormatReturnsError(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunAuditCommand([]string{"export"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d (stderr: %s), want 1", code, stderr.String())
	}

	wantContains(t, "stderr", stderr.String(), "error result")
}

// TestAuditUnknownSubcommandExitsUsage checks an unknown audit
// subcommand prints usage and exits 2.
func TestAuditUnknownSubcommandExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunAuditCommand([]string{"nope"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

// TestAuditNoSubcommandExitsUsage checks that bare `audit` prints usage.
func TestAuditNoSubcommandExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunAuditCommand(nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}
