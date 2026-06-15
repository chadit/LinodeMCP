package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/chadit/LinodeMCP/internal/cli"
)

// missingConfigPath returns a config path inside a fresh tempdir that is
// guaranteed not to exist, so a command run with it exercises the
// no-config fallback rather than reading a real file.
func missingConfigPath(t *testing.T) string {
	t.Helper()

	return filepath.Join(t.TempDir(), "absent-config.yml")
}

// TestToolsListAllNoConfig locks the parity behavior: with no config file
// present, `tools --all` still lists the catalog and exits 0, matching the
// Python CLI which lists its tools without a config. This guards against a
// regression where a missing config hard-fails tool discovery.
func TestToolsListAllNoConfig(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", missingConfigPath(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunToolsCommand([]string{"--all"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	if got := countLines(stdout.String()); got == 0 {
		t.Fatal("tools --all listed no tools with a missing config")
	}

	wantContains(t, "stdout", stdout.String(), toolVersion)
}

// TestCallVersionNoConfig checks that a meta-tool call works offline: with
// no config file, `call version` falls back to the in-memory default
// config, dispatches, and exits 0. version touches no Linode API, so it
// succeeds without any environment configured.
func TestCallVersionNoConfig(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", missingConfigPath(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{toolVersion}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	wantContains(t, "stdout", stdout.String(), `"version"`)
}

// TestCallUnknownToolNoConfigExitsUsage is the regression guard for the
// ordering bug: with no config, an unknown tool must still be detected and
// exit 2, not exit 0. Before the fallback, config-load failed before the
// unknown-tool check ran, so the command exited 0 and silently did
// nothing. The fallback lets the catalog build offline, so the check runs.
func TestCallUnknownToolNoConfigExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", missingConfigPath(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{"linode_does_not_exist"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}

	wantContains(t, "stderr", stderr.String(), "unknown tool")
}

// TestCallLinodeAPIToolNoConfigFailsAtCall checks the boundary: a Linode
// API read tool, with no environment configured, fails at call time with a
// clear message and exit 1, not at config load. The error names the
// missing environment so the user knows what to configure.
func TestCallLinodeAPIToolNoConfigFailsAtCall(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", missingConfigPath(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunCallCommand([]string{"linode_instance_list"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d (stderr: %s), want 1", code, stderr.String())
	}

	wantContains(t, "stdout", stdout.String(), "environment")
}

// TestVersionCommandNoConfig checks the top-level `version` command works
// with no config: it prints appinfo and exits 0, never touching config.
func TestVersionCommandNoConfig(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", missingConfigPath(t))

	var stdout, stderr bytes.Buffer

	code := cli.RunVersionCommand(&stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	wantContains(t, "stdout", stdout.String(), `"version"`)
}
