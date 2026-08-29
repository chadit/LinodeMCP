package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The merge flags, and the two anchors its findings are read for.
const (
	surfaceFlag   = "-surface"
	outFlag       = "-out"
	repoFlag      = "-repo"
	emitFlag      = "-emit-surface"
	danglingLine  = "overlay anchors a field the surface does not carry"
	uncoveredWord = "upstream field has no overlay entry"
)

// One documented field and one enum covers both refusal directions.
const mergeProto = `syntax = "proto3";

package sample;

// Kind is what a sample carries.
enum Kind {
  KIND_UNSPECIFIED = 0;
  KIND_PRIMARY = 1;
}

message Sample {
  // The label.
  string label = 1 [(linode.mcp.v1.field_location) = FIELD_LOCATION_PATH];
  // The kind.
  Kind kind = 2;
}
`

// emit answers the descriptor path; at zero drift upstream would send this one.
func emit(t *testing.T, root string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "surface.json")

	output, code := runTool(t, repoFlag, root, emitFlag, path)
	if code != 0 {
		t.Fatalf("emitting the surface failed (exit %d):\n%s", code, output)
	}

	return path
}

// reshape plants upstream drift without touching the tree the overlay reads.
func reshape(t *testing.T, path string, change func(fields, doc map[string]any)) {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the descriptor: %v", err)
	}

	var doc map[string]any
	if unmarshalErr := json.Unmarshal(raw, &doc); unmarshalErr != nil {
		t.Fatalf("parse the descriptor: %v", unmarshalErr)
	}

	fields, ok := doc["fields"].(map[string]any)
	if !ok {
		t.Fatalf("the descriptor carries no field map, so the case would measure nothing")
	}

	change(fields, doc)

	rewritten, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("encode the descriptor: %v", err)
	}

	writeFile(t, path, string(rewritten)+"\n")
}

// mergeInto answers the output, the exit code, and the fresh output directory.
func mergeInto(t *testing.T, root, surface string, extra ...string) (string, int, string) {
	t.Helper()

	out := filepath.Join(t.TempDir(), "tree")
	args := append([]string{repoFlag, root, surfaceFlag, surface, outFlag, out}, extra...)

	output, code := runTool(t, args...)

	return output, code, out
}

// The control: at zero drift the merged tree is the tree the overlay came from.
func TestMergeAtZeroDriftReproducesTheRepoTree(t *testing.T) {
	t.Parallel()

	output, code, out := mergeInto(t, repoRoot, emit(t, repoRoot))
	if code != 0 {
		t.Fatalf("the merge refused the tree it read (exit %d):\n%s", code, output)
	}

	compared := compareTrees(t, filepath.Join(repoRoot, "proto"), filepath.Join(out, "proto"))
	if compared == 0 {
		t.Fatalf("the comparison covered no file, so it proves nothing:\n%s", output)
	}
}

// compareTrees answers how many files it read, so an empty walk cannot pass.
func compareTrees(t *testing.T, want, got string) int {
	t.Helper()

	var compared int

	err := filepath.WalkDir(want, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}

		rel, relErr := filepath.Rel(want, path)
		if relErr != nil {
			return fmt.Errorf("relative path for %s: %w", path, relErr)
		}

		compared++

		compareFile(t, path, filepath.Join(got, rel), rel)

		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", want, err)
	}

	return compared
}

func compareFile(t *testing.T, wantPath, gotPath, rel string) {
	t.Helper()

	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read %s: %v", wantPath, err)
	}

	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("the merge did not write %s: %v", rel, err)
	}

	if !bytes.Equal(want, got) {
		t.Fatalf("the merged %s is not the tree's", rel)
	}
}

// REQ-D4, dropped: an anchor the descriptor stopped carrying is refused by line.
func TestMergeRefusesAnAnchorTheUpstreamDropped(t *testing.T) {
	t.Parallel()

	root := treeWith(t, mergeProto)
	surface := emit(t, root)

	reshape(t, surface, func(fields, _ map[string]any) {
		delete(fields, "Sample.label")
	})

	output, code, out := mergeInto(t, root, surface)
	if code == 0 {
		t.Fatalf("a dropped field passed:\n%s", output)
	}

	if !strings.Contains(output, danglingLine) ||
		!strings.Contains(output, "Sample.label") ||
		!strings.Contains(output, sampleRel+":13") {
		t.Fatalf("the refusal does not name the anchor and its line:\n%s", output)
	}

	if _, err := os.Stat(out); err == nil {
		t.Fatalf("a refused merge left a tree behind at %s", out)
	}
}

// REQ-D4, added: unclaimed surface is refused, since the field would reach the
// tree with no wire number and none of the MCP semantics this repo states.
func TestMergeRefusesSurfaceTheOverlayDoesNotCover(t *testing.T) {
	t.Parallel()

	root := treeWith(t, mergeProto)
	surface := emit(t, root)

	reshape(t, surface, func(fields, _ map[string]any) {
		fields["Sample.title"] = map[string]any{"type": "string", "location": "FIELD_LOCATION_QUERY"}
	})

	output, code, _ := mergeInto(t, root, surface)
	if code == 0 {
		t.Fatalf("an uncovered upstream field passed:\n%s", output)
	}

	if !strings.Contains(output, uncoveredWord) || !strings.Contains(output, "Sample.title") {
		t.Fatalf("the refusal does not name the uncovered field:\n%s", output)
	}
}

// A rename is a drop and an add the merger cannot pair, so both halves refuse.
func TestMergeRefusesBothHalvesOfARename(t *testing.T) {
	t.Parallel()

	root := treeWith(t, mergeProto)
	surface := emit(t, root)

	reshape(t, surface, func(fields, _ map[string]any) {
		fields["Sample.vlan_label"] = fields["Sample.label"]
		delete(fields, "Sample.label")
	})

	output, code, _ := mergeInto(t, root, surface)
	if code == 0 {
		t.Fatalf("a renamed field passed:\n%s", output)
	}

	if !strings.Contains(output, danglingLine) || !strings.Contains(output, uncoveredWord) {
		t.Fatalf("a rename did not fail in both directions:\n%s", output)
	}
}

// Both directions cover enum values, whose names are contract too.
func TestMergeRefusesEnumValueDriftInBothDirections(t *testing.T) {
	t.Parallel()

	root := treeWith(t, mergeProto)
	surface := emit(t, root)

	reshape(t, surface, func(_, doc map[string]any) {
		doc["values"] = []any{"Kind.KIND_UNSPECIFIED", "Kind.KIND_SECONDARY"}
	})

	output, code, _ := mergeInto(t, root, surface)
	if code == 0 {
		t.Fatalf("enum value drift passed:\n%s", output)
	}

	if !strings.Contains(output, "overlay anchors an enum value the surface does not carry") ||
		!strings.Contains(output, "Kind.KIND_PRIMARY") {
		t.Fatalf("the dropped value is not named:\n%s", output)
	}

	if !strings.Contains(output, "upstream enum value has no overlay entry") ||
		!strings.Contains(output, "Kind.KIND_SECONDARY") {
		t.Fatalf("the added value is not named:\n%s", output)
	}
}

// Retirement is what the refusal hands the human: the declaration and its
// prose go, its number and name stay spent.
func TestRetirementSpendsTheNumberAndTheName(t *testing.T) {
	t.Parallel()

	root := treeWith(t, mergeProto)
	surface := emit(t, root)

	reshape(t, surface, func(fields, _ map[string]any) {
		delete(fields, "Sample.label")
	})

	output, code, out := mergeInto(t, root, surface, "-retire", "Sample.label")
	if code != 0 {
		t.Fatalf("the retirement was refused (exit %d):\n%s", code, output)
	}

	merged := readMerged(t, out)

	for _, want := range []string{"  reserved 1;", `  reserved "label";`} {
		if !strings.Contains(merged, want) {
			t.Fatalf("the merged file does not spend the field with %q:\n%s", want, merged)
		}
	}

	for _, gone := range []string{"string label = 1", "// The label."} {
		if strings.Contains(merged, gone) {
			t.Fatalf("the merged file still carries %q:\n%s", gone, merged)
		}
	}
}

// The merger's output goes back through the extractor, so a retirement survives
// the next cycle instead of failing the round-trip gate.
func TestARetiredTreeRoundTrips(t *testing.T) {
	t.Parallel()

	root := treeWith(t, mergeProto)
	surface := emit(t, root)

	reshape(t, surface, func(fields, _ map[string]any) {
		delete(fields, "Sample.label")
	})

	output, code, out := mergeInto(t, root, surface, "-retire", "Sample.label")
	if code != 0 {
		t.Fatalf("the retirement was refused (exit %d):\n%s", code, output)
	}

	writeFile(t, filepath.Join(out, workspaceName), sampleWorkspace)

	output, code = runGate(t, out)
	if code != 0 {
		t.Fatalf("the merged tree does not round-trip (exit %d):\n%s", code, output)
	}
}

// A retirement naming nothing would report a number the file never spends.
func TestRetiringAnAbsentDeclarationIsRefused(t *testing.T) {
	t.Parallel()

	root := treeWith(t, mergeProto)

	output, code, _ := mergeInto(t, root, emit(t, root), "-retire", "Sample.nothing")
	if code == 0 {
		t.Fatalf("a retirement of nothing passed:\n%s", output)
	}

	if !strings.Contains(output, "no declaration to retire under that name") ||
		!strings.Contains(output, "Sample.nothing") {
		t.Fatalf("the refusal does not name the key:\n%s", output)
	}
}

// An empty descriptor would dangle every anchor and read as total drift.
func TestEmptySurfaceDescriptorIsRefused(t *testing.T) {
	t.Parallel()

	root := treeWith(t, mergeProto)
	surface := emit(t, root)
	writeFile(t, surface, "{\"fields\": {}, \"values\": [], \"routes\": {}}\n")

	output, code, _ := mergeInto(t, root, surface)
	if code == 0 {
		t.Fatalf("an empty descriptor passed:\n%s", output)
	}

	if !strings.Contains(output, "states no field, enum value, or route") {
		t.Fatalf("the refusal does not say the descriptor was empty:\n%s", output)
	}
}

// A descriptor the merge cannot parse is refused rather than half-read.
func TestUnparsedSurfaceDescriptorIsRefused(t *testing.T) {
	t.Parallel()

	root := treeWith(t, mergeProto)
	surface := emit(t, root)
	writeFile(t, surface, "{not json\n")

	output, code, _ := mergeInto(t, root, surface)
	if code == 0 {
		t.Fatalf("an unparsed descriptor passed:\n%s", output)
	}

	if !strings.Contains(output, "surface descriptor does not parse") {
		t.Fatalf("the refusal does not name the descriptor:\n%s", output)
	}
}

// Rendering into memory would report a clean merge nobody can review.
func TestMergeWithoutAnOutputDirectoryIsRefused(t *testing.T) {
	t.Parallel()

	root := treeWith(t, mergeProto)

	output, code := runTool(t, repoFlag, root, surfaceFlag, emit(t, root))
	if code == 0 {
		t.Fatalf("a merge with no output directory passed:\n%s", output)
	}

	if !strings.Contains(output, "needs -out") {
		t.Fatalf("the refusal does not name the missing flag:\n%s", output)
	}
}

// Emitting and merging in one run leaves what it measures undecided.
func TestEmittingAndMergingInOneRunIsRefused(t *testing.T) {
	t.Parallel()

	root := treeWith(t, mergeProto)
	surface := emit(t, root)

	output, code := runTool(t, repoFlag, root, emitFlag, surface, surfaceFlag, surface)
	if code == 0 {
		t.Fatalf("two modes in one run passed:\n%s", output)
	}

	if !strings.Contains(output, "two different runs") {
		t.Fatalf("the refusal does not name the conflict:\n%s", output)
	}
}

func readMerged(t *testing.T, out string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(out, sampleRel))
	if err != nil {
		t.Fatalf("read the merged file: %v", err)
	}

	return string(raw)
}
