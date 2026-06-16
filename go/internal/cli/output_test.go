package cli_test

import (
	"bytes"
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

	code := cli.RunCallCommand([]string{toolVersion, "--output", "table"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	out := stdout.String()
	wantContains(t, "stdout", out, "FIELD")
	wantContains(t, "stdout", out, "VALUE")
	wantContains(t, "stdout", out, "version")
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
