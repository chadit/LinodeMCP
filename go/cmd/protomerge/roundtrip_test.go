package main_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Relative to the package directory, which is go test's cwd.
const repoRoot = "../../.."

// The line every clean run prints, and the two words a finding is anchored on.
const (
	roundTripLine = "round-tripped "
	treeHas       = "tree has"
	overlayLine   = "overlay renders"
)

// Written rather than copied, so a case pins one construct instead of whatever
// the contract says today.
const (
	sampleWorkspace = `version: v2
modules:
  - path: proto
lint:
  ignore:
    - proto/vendor
`

	sampleProto = `syntax = "proto3";

package sample;

// Sample carries one field of each tail shape.
message Sample {
  // The label.
  string label = 1 [(linode.mcp.v1.field_location) = FIELD_LOCATION_PATH]; // system param
  optional int32 page = 2 [
    (linode.mcp.v1.field_location) = FIELD_LOCATION_QUERY,
    // The page number is read off the argument map.
    (linode.mcp.v1.argument_reader) = ARGUMENT_READER_POSITIVE_ID
  ];
}
`
)

// runTool runs one protomerge invocation and answers its output and exit code.
func runTool(t *testing.T, args ...string) (string, int) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "go", append([]string{"run", "."}, args...)...)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), 0
	}

	exit, ranAndFailed := errors.AsType[*exec.ExitError](err)
	if !ranAndFailed {
		t.Fatalf("protomerge did not run: %v\n%s", err, output)
	}

	return string(output), exit.ExitCode()
}

// runGate runs the gate the way `make overlay-roundtrip` does.
func runGate(t *testing.T, repo string) (string, int) {
	t.Helper()

	return runTool(t, "-repo", repo)
}

// sampleTree writes the synthetic workspace into a fresh directory.
func sampleTree(t *testing.T) string {
	t.Helper()

	return treeWith(t, sampleProto)
}

// treeWith writes the synthetic workspace around one proto file.
func treeWith(t *testing.T, content string) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, workspaceName), sampleWorkspace)
	writeFile(t, filepath.Join(root, sampleRel), content)

	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestRepoTreeRoundTripsByteIdentically(t *testing.T) {
	t.Parallel()

	output, code := runGate(t, repoRoot)
	if code != 0 {
		t.Fatalf("the contract does not round-trip (exit %d):\n%s", code, output)
	}

	if !strings.Contains(output, roundTripLine) {
		t.Fatalf("no scan report in output:\n%s", output)
	}

	if strings.Contains(output, roundTripLine+"0 ") {
		t.Fatalf("the walk covered no proto file:\n%s", output)
	}
}

func TestSampleTreeRoundTripsByteIdentically(t *testing.T) {
	t.Parallel()

	output, code := runGate(t, sampleTree(t))
	if code != 0 {
		t.Fatalf("the synthetic workspace does not round-trip (exit %d):\n%s", code, output)
	}

	if !strings.Contains(output, roundTripLine+"1 ") {
		t.Fatalf("expected a one-file scan, got:\n%s", output)
	}
}

// A vendored tree following another project's conventions cannot fail this gate.
func TestDisownedDirectoryIsNotWalked(t *testing.T) {
	t.Parallel()

	root := sampleTree(t)
	writeFile(t, filepath.Join(root, "proto", "vendor", "other.proto"), "message  Broken {\n}\n")

	output, code := runGate(t, root)
	if code != 0 {
		t.Fatalf("the disowned tree reached the walk (exit %d):\n%s", code, output)
	}

	if !strings.Contains(output, roundTripLine+"1 ") {
		t.Fatalf("expected a one-file scan, got:\n%s", output)
	}
}
