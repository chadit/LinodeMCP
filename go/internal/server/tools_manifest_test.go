package server_test

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// manifestPath points at the shared cross-language tool manifest. The file
// lives at the repo root so both implementations read the same source of
// truth; this test runs from go/internal/server, three levels below it.
const manifestPath = "../../../docs/contracts/tools-manifest.txt"

// loadManifest parses docs/contracts/tools-manifest.txt into the set of tool names.
// The manifest is the full canonical surface; per-language absences live in
// docs/contracts/tool-parity-baseline.txt, so any tab-separated annotation on a
// manifest line (the retired go-only/py-only mechanism) is a fatal failure.
// Comment lines (#) and blanks are skipped.
func loadManifest(t *testing.T) map[string]bool {
	t.Helper()

	file, err := os.Open(filepath.Clean(manifestPath))
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close manifest: %v", closeErr)
		}
	}()

	entries := make(map[string]bool)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if trimmed := strings.TrimSpace(line); trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.Contains(line, "\t") {
			t.Fatalf("manifest line %q carries a tab annotation; one-sided tools are not allowed", line)
		}

		name := strings.TrimSpace(line)
		if entries[name] {
			t.Fatalf("manifest lists %q twice", name)
		}

		entries[name] = true
	}

	if scanErr := scanner.Err(); scanErr != nil {
		t.Fatalf("read manifest: %v", scanErr)
	}

	return entries
}

// TestToolSurfaceMatchesManifest enforces cross-language tool-name parity:
// the Go catalog must equal the manifest's names minus the absences the
// tool-parity baseline accepts for Go ("missing in go" entries, each
// annotated with a tracking issue). A failure names every missing and extra
// tool so drift is obvious. Extra tools are never excused: a tool cannot
// register anywhere without entering the manifest. The Python twin
// (tests/unit/test_tools_manifest.py) enforces the same contract for its
// side, so a tool cannot ship in one implementation untracked.
func TestToolSurfaceMatchesManifest(t *testing.T) {
	t.Parallel()

	expected := loadManifest(t)
	absent := loadMissingInLanguage(t, "go")

	srv := newCapabilityTestServer(t)
	catalog := srv.ToolCatalog()
	actual := make(map[string]bool, len(catalog))

	for _, descriptor := range catalog {
		if actual[descriptor.Name] {
			t.Errorf("tool %q is registered twice", descriptor.Name)
		}

		actual[descriptor.Name] = true
	}

	var missing, extra []string

	for name := range expected {
		if !actual[name] && !absent[name] {
			missing = append(missing, name)
		}
	}

	for name := range actual {
		if !expected[name] {
			extra = append(extra, name)
		}
	}

	sort.Strings(missing)
	sort.Strings(extra)

	if len(missing) > 0 {
		t.Errorf(
			"tools in docs/contracts/tools-manifest.txt but not registered by the Go server: %s",
			strings.Join(missing, ", "),
		)
	}

	if len(extra) > 0 {
		t.Errorf(
			"tools registered by the Go server but missing from docs/contracts/tools-manifest.txt: %s",
			strings.Join(extra, ", "),
		)
	}
}
