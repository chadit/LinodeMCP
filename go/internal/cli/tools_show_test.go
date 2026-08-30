package cli_test

import (
	"bytes"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/cli"
)

// TestToolsShowWithoutNameExitsUsage checks `tools show` with no tool
// name prints the show usage line and exits 2 before building a runtime.
func TestToolsShowWithoutNameExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunToolsCommand([]string{subverbShow}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}

	wantContains(t, "stderr", stderr.String(), "Usage: linodemcp tools show <tool>")
}

// TestToolsShowMarksRequiredArguments checks the schema's required list
// reaches the listing: instance_get requires instance_id but not
// environment, and only the required one carries the marker.
func TestToolsShowMarksRequiredArguments(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunToolsCommand([]string{subverbShow, "linode_instance_get"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	out := stdout.String()
	wantMatch(t, "required argument", out, `(?m)^\s+instance_id\s+integer \(required\)$`)
	wantMatch(t, "optional argument", out, `(?m)^\s+environment\s+string$`)
}

// TestToolsShowOmitsDescriptionForProfileHiddenTool checks a write tool the
// read-only default profile hides still shows its capability and
// arguments, and skips the Description line the hidden registration
// cannot supply, rather than failing as an unknown tool.
func TestToolsShowOmitsDescriptionForProfileHiddenTool(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunToolsCommand([]string{subverbShow, "linode_tag_create"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	out := stdout.String()
	wantContains(t, "stdout", out, "Tool: linode_tag_create\n")
	wantContains(t, "stdout", out, "Capability: CapWrite\n")
	wantNotContains(t, "stdout", out, "Description:")
	wantMatch(t, "required label", out, `(?m)^\s+label\s+string \(required\)$`)
}

// TestToolsShowDefaultsUntypedArgumentToString checks an argument whose
// schema declares no single type (instance_clone's metadata is a nested
// message) is listed as string, the same permissive default argument
// coercion applies, instead of an empty type column.
func TestToolsShowDefaultsUntypedArgumentToString(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunToolsCommand([]string{subverbShow, "linode_instance_clone"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	wantMatch(t, "untyped argument", stdout.String(), `(?m)^\s+metadata\s+string$`)
}
