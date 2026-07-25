package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/cli"
)

// subverbReport is the audit subcommand under test; a constant because
// goconst flags the repeated literal across the report test cases.
const subverbReport = "report"

// writeTestConfigWithReport writes the shared minimal config plus one
// named report under audit.reports, so the report subcommand has a
// resolvable name. Separate from writeTestConfigFile because only the
// report tests need the audit block.
func writeTestConfigWithReport(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	contents := `
server:
  name: "Test"
  logLevel: "info"
  transport: "stdio"
  host: "127.0.0.1"
  port: 8080
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
audit:
  reports:
    daily-writes:
      description: "write ops in the last day"
      filter:
        capability: "write"
        since_offset: "24h"
      output: "summary"
`

	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	return path
}

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

// TestAuditReportSucceeds checks `audit report <name>` resolves a report
// defined under audit.reports and prints its JSON result. The log is
// empty, so the summary report returns zero rows but still succeeds.
func TestAuditReportSucceeds(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigWithReport(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunAuditCommand([]string{subverbReport, "daily-writes"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	decodeJSONObject(t, stdout.String())
}

// TestAuditReportUnknownNameReturnsError checks an undefined report name
// reaches the tool, whose error result maps to exit 1 (not a usage
// error: the CLI cannot know the config's report names).
func TestAuditReportUnknownNameReturnsError(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunAuditCommand([]string{subverbReport, "no-such-report"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d (stderr: %s), want 1", code, stderr.String())
	}

	wantContains(t, "stderr", stderr.String(), "error result")
}

// TestAuditReportMissingNameExitsUsage checks `audit report` with no
// name (or more than one) is a usage error, caught before any dispatch.
func TestAuditReportMissingNameExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunAuditCommand([]string{"report"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}

	code = cli.RunAuditCommand([]string{subverbReport, "a", "b"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code with two names = %d, want %d", code, exitUsage)
	}
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
