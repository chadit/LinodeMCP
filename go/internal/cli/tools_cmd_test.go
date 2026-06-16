package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/cli"
)

// TestToolsListPrintsNamesAndCapabilities checks the default `tools`
// listing emits tool names alongside a capability tag. version is a meta
// tool present in every profile, so it must appear with CapMeta.
func TestToolsListPrintsNamesAndCapabilities(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunToolsCommand(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	wantContains(t, "stdout", stdout.String(), toolVersion)
	wantContains(t, "stdout", stdout.String(), "CapMeta")
}

// TestToolsListAllIsAtLeastAsLargeAsProfile checks --all lists the full
// catalog: its line count is at least the active-profile listing's, since
// the catalog is a superset of any one profile's surface.
func TestToolsListAllIsAtLeastAsLargeAsProfile(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var profileOut, allOut, stderr bytes.Buffer

	if code := cli.RunToolsCommand(nil, &profileOut, &stderr); code != 0 {
		t.Fatalf("tools list exit = %d (stderr: %s)", code, stderr.String())
	}

	if code := cli.RunToolsCommand([]string{"--all"}, &allOut, &stderr); code != 0 {
		t.Fatalf("tools --all exit = %d (stderr: %s)", code, stderr.String())
	}

	profileLines := countLines(profileOut.String())
	allLines := countLines(allOut.String())

	if allLines < profileLines {
		t.Errorf("--all listed %d tools, fewer than the profile's %d", allLines, profileLines)
	}

	if allLines == 0 {
		t.Error("--all listed no tools")
	}
}

// TestToolsShowPrintsSchema checks `tools show` prints the capability and
// the argument schema. hello declares a single string arg "name", so the
// detail must name the tool, its capability, and that argument.
func TestToolsShowPrintsSchema(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunToolsCommand([]string{"show", toolHello}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, stderr.String())
	}

	out := stdout.String()
	wantContains(t, "stdout", out, "Tool: hello")
	wantContains(t, "stdout", out, "CapMeta")
	wantContains(t, "stdout", out, "name")
}

// TestToolsShowUnknownExitsUsage checks `tools show` of a missing tool
// exits with the usage code.
func TestToolsShowUnknownExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunToolsCommand([]string{"show", "linode_nope"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

// TestToolsUnknownFlagExitsUsage checks an unrecognized argument to the
// list path is a usage error.
func TestToolsUnknownFlagExitsUsage(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer

	code := cli.RunToolsCommand([]string{"--nope"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

// countLines counts non-empty lines in s, so a listing's tool count can
// be compared without trailing-newline noise.
func countLines(s string) int {
	var count int

	for line := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}

	return count
}
