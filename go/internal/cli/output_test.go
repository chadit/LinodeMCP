package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/cli"
)

// TestCallTableRendersObject checks the --output table path: version's
// result is a flat JSON object, so the table view prints a FIELD/VALUE
// header and a row per field. The version value still appears, just in a
// table cell rather than raw JSON.
func TestCallTableRendersObject(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolVersion, flagOutput, "table"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	out := stdout.String()
	wantContains(t, "stdout", out, "FIELD")
	wantContains(t, "stdout", out, "VALUE")
	wantContains(t, "stdout", out, "version")
}

// TestCallTableRendersDryRunEnvelope drives a create tool's dry-run through the
// --output table path. The dry-run preview is a proto-serialized object with a
// nested would_execute object and dependency arrays; the table view must render
// it as a FIELD/VALUE table without crashing (nested values collapse to a
// compact-JSON cell). tag_create previews offline, so no fake API is needed.
func TestCallTableRendersDryRunEnvelope(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeFullAccessConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand(
		[]string{"linode_tag_create", flagArg, "label=example-tag", "--dry-run", flagOutput, "table"},
		&stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	out := stdout.String()
	wantContains(t, "stdout", out, "FIELD")
	wantContains(t, "stdout", out, "dry_run")
	wantContains(t, "stdout", out, "would_execute")
}

// writeFullAccessConfigFile writes a config selecting the full-access built-in
// profile so a write tool (tag_create) is registered, which lets the dry-run
// table case drive a preview through the CLI. The dry-run branch short-circuits
// before any API call, so the placeholder token is never used.
func writeFullAccessConfigFile(t *testing.T) string {
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
active_profile: "full-access"
profiles_builtin_overrides:
  full-access:
    disabled: false
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`

	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	return path
}

// TestCallJSONIsDefault checks that without --output the payload prints as
// JSON (an object with braces), not the table view.
func TestCallJSONIsDefault(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolVersion}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	out := stdout.String()
	wantContains(t, "stdout", out, "{")
	wantNotContains(t, "stdout", out, "FIELD")
}

// TestVersionCommand checks the standalone `version` subcommand prints
// JSON version info and exits 0.
func TestVersionCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	code := cli.RunVersionCommand(&stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	body := decodeJSONObject(t, stdout.String())
	if _, ok := body["version"]; !ok {
		t.Errorf("version payload missing version key: %v", body)
	}
}
