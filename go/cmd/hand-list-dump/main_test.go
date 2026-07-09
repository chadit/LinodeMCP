package main_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"reflect"
	"testing"
)

// toolsDir is where the hand-maintained value-sets live, two levels up from this
// command's package directory (go test runs with that directory as cwd).
const toolsDir = "../../internal/tools"

// runDump executes the command as the Python gate does and returns its streams.
// Black-box on purpose: the gate depends on the real contract (JSON on stdout,
// non-zero exit on failure), so the test exercises that contract, not internals.
func runDump(t *testing.T, dir string) ([]byte, []byte, error) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "go", "run", ".", "-tools-dir", dir)

	var out, errBuf bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	err := cmd.Run()

	return out.Bytes(), errBuf.Bytes(), err
}

// TestDumpMatchesHandLists pins the extractor output to the current hand-lists.
// If a hand-list gains or loses a value, this test changes with it, so the
// extractor and the source it reads can never drift apart silently.
func TestDumpMatchesHandLists(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := runDump(t, toolsDir)
	if err != nil {
		t.Fatalf("hand-list-dump failed: %v\nstderr: %s", err, stderr)
	}

	var got map[string][]string

	if unmarshalErr := json.Unmarshal(stdout, &got); unmarshalErr != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", unmarshalErr, stdout)
	}

	want := map[string][]string{
		"bucket_acl":           {"authenticated-read", "private", "public-read", "public-read-write"},
		"placement_group_type": {"anti_affinity:local"},
		"config_device_slot":   {"sda", "sdb", "sdc", "sdd", "sde", "sdf", "sdg", "sdh"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("dump = %v, want %v", got, want)
	}
}

// TestDumpEmptyDirIsHardFail proves the renamed/deleted-symbol tripwire: an
// empty extraction must exit non-zero and name the target, never emit an empty
// set that a downstream diff would trust and pass on.
func TestDumpEmptyDirIsHardFail(t *testing.T) {
	t.Parallel()

	_, stderr, err := runDump(t, t.TempDir())
	if err == nil {
		t.Fatal("expected non-zero exit on empty tools dir, got success")
	}

	if !bytes.Contains(stderr, []byte("bucket_acl")) {
		t.Errorf("stderr should name the missing target; got: %s", stderr)
	}
}
