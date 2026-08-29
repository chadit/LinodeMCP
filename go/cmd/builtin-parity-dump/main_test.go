package main_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// fixture is the catalog both halves of the dump are checked against: one meta
// tool, one categorized mutator, and one tool whose category moved during the
// resolver reconciliation.
const fixture = `[
	{"name": "hello", "capability": "Meta"},
	{"name": "linode_volume_create", "capability": "Write"},
	{"name": "linode_longview_client_get", "capability": "Read"}
]`

// dumped is the shape scripts/verify_profile_resolution.py parses.
type dumped struct {
	Categories map[string][]string `json:"categories"`
	Profiles   []struct {
		Name         string   `json:"name"`
		AllowedTools []string `json:"allowed_tools"`
	} `json:"profiles"`
}

// runDump executes the command the way the gate does, black-box: JSON on
// stdout, non-zero exit on a catalog it cannot read.
func runDump(t *testing.T, stdin string) ([]byte, []byte, error) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "go", "run", ".")
	cmd.Stdin = strings.NewReader(stdin)

	var out, errBuf bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	err := cmd.Run()

	return out.Bytes(), errBuf.Bytes(), err
}

func TestDumpCarriesBothHalvesOfTheContract(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := runDump(t, fixture)
	if err != nil {
		t.Fatalf("builtin-parity-dump failed: %v\nstderr: %s", err, stderr)
	}

	var got dumped
	if unmarshalErr := json.Unmarshal(stdout, &got); unmarshalErr != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", unmarshalErr, stdout)
	}

	if len(got.Categories) != 3 {
		t.Errorf("categories = %v, want one entry per catalog tool", got.Categories)
	}

	for toolName, want := range map[string]string{
		"hello":                      "core",
		"linode_volume_create":       "block_storage",
		"linode_longview_client_get": "longview",
	} {
		if cats := got.Categories[toolName]; len(cats) != 1 || cats[0] != want {
			t.Errorf("categories[%q] = %v, want [%s]", toolName, cats, want)
		}
	}

	allowed := make(map[string][]string, len(got.Profiles))
	for _, profile := range got.Profiles {
		allowed[profile.Name] = profile.AllowedTools
	}

	if len(allowed) != 9 {
		t.Errorf("resolved %d profiles, want the 9 built-ins", len(allowed))
	}

	if !slices.Contains(allowed["storage-admin"], "linode_volume_create") {
		t.Errorf("storage-admin = %v, want the volume write", allowed["storage-admin"])
	}

	if slices.Contains(allowed["default"], "linode_volume_create") {
		t.Errorf("default = %v, want no volume write", allowed["default"])
	}
}

func TestDumpRefusesACapabilityItCannotRead(t *testing.T) {
	t.Parallel()

	_, stderr, err := runDump(t, `[{"name": "x", "capability": "Superuser"}]`)
	if err == nil {
		t.Fatal("expected a non-zero exit for an unknown capability")
	}

	if !bytes.Contains(stderr, []byte("Superuser")) {
		t.Errorf("stderr = %s, want it to name the capability it refused", stderr)
	}
}
