package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The workspace file both scope cases rewrite.
const workspaceName = "buf.yaml"

// plant refuses a case whose target text is not there to change.
func plant(t *testing.T, root, from, into string) {
	t.Helper()

	path := filepath.Join(root, sampleRel)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", sampleRel, err)
	}

	text := string(raw)
	if !strings.Contains(text, from) {
		t.Fatalf("%s does not carry %q, so the case would measure nothing", sampleRel, from)
	}

	writeFile(t, path, strings.Replace(text, from, into, 1))
}

// Copying an unmodellable declaration through would hide that the extractor
// does not cover it.
func TestUnmodelledDeclarationIsRefused(t *testing.T) {
	t.Parallel()

	root := sampleTree(t)
	plant(t, root, "  string label = 1 ", "  string  label = 1 ")

	output, code := runGate(t, root)
	if code == 0 {
		t.Fatalf("an unmodelled declaration passed:\n%s", output)
	}

	if !strings.Contains(output, "declaration does not parse") ||
		!strings.Contains(output, "proto/sample.proto:8") {
		t.Fatalf("the refusal does not name the declaration:\n%s", output)
	}
}

// A tail layout the model cannot re-render is refused for the same reason.
func TestUnmodelledTailIsRefused(t *testing.T) {
	t.Parallel()

	root := sampleTree(t)
	plant(t, root, "  optional int32 page = 2 [", "  optional int32 page = 2  [")

	output, code := runGate(t, root)
	if code == 0 {
		t.Fatalf("an unmodelled tail passed:\n%s", output)
	}

	if !strings.Contains(output, "tail does not parse") ||
		!strings.Contains(output, "proto/sample.proto:9") {
		t.Fatalf("the refusal does not name the tail:\n%s", output)
	}
}

// The second declaration would answer for the first everywhere.
func TestDuplicateDeclarationIsRefused(t *testing.T) {
	t.Parallel()

	root := sampleTree(t)
	writeFile(t, filepath.Join(root, "proto", "again.proto"), "message Sample {\n  string label = 1;\n}\n")

	output, code := runGate(t, root)
	if code == 0 {
		t.Fatalf("a duplicated surface name passed:\n%s", output)
	}

	if !strings.Contains(output, "two declarations claim one surface name") ||
		!strings.Contains(output, "Sample.label") {
		t.Fatalf("the refusal does not name the declaration:\n%s", output)
	}
}

// The finding names the file, the line, and both spellings.
func TestByteDifferenceNamesTheLine(t *testing.T) {
	t.Parallel()

	root := sampleTree(t)
	plant(t, root, "FIELD_LOCATION_QUERY,\n", "FIELD_LOCATION_QUERY, \n")

	output, code := runGate(t, root)
	if code == 0 {
		t.Fatalf("a byte the overlay drops passed:\n%s", output)
	}

	if !strings.Contains(output, "proto/sample.proto:10") ||
		!strings.Contains(output, treeHas) || !strings.Contains(output, overlayLine) {
		t.Fatalf("the finding does not name the line and both spellings:\n%s", output)
	}
}

// "No findings" and "nothing scanned" are the same line otherwise.
func TestEmptyModuleFailsRatherThanReportingClean(t *testing.T) {
	t.Parallel()

	root := sampleTree(t)

	if err := os.Remove(filepath.Join(root, "proto", "sample.proto")); err != nil {
		t.Fatalf("remove the sample: %v", err)
	}

	output, code := runGate(t, root)
	if code == 0 {
		t.Fatalf("an empty walk reported success:\n%s", output)
	}

	if !strings.Contains(output, "scan covered nothing") {
		t.Fatalf("the refusal does not say the scan was empty:\n%s", output)
	}
}

// The gate never falls back to a scope of its own.
func TestWorkspaceWithNoModuleIsRefused(t *testing.T) {
	t.Parallel()

	root := sampleTree(t)
	writeFile(t, filepath.Join(root, workspaceName), "version: v2\n")

	output, code := runGate(t, root)
	if code == 0 {
		t.Fatalf("a module-less workspace passed:\n%s", output)
	}

	if !strings.Contains(output, "declares no module") {
		t.Fatalf("the refusal does not name the missing declaration:\n%s", output)
	}
}
