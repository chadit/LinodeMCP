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
const manifestPath = "../../../docs/tools-manifest.txt"

// loadManifest parses docs/tools-manifest.txt into the set of tool names.
// Every listed tool must exist in BOTH implementations, so any tab-separated
// annotation on a line (the retired go-only/py-only mechanism) is a fatal
// failure. Comment lines (#) and blanks are skipped.
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
// the Go catalog must equal EXACTLY the manifest's names. A failure names
// every missing and extra tool so drift is obvious. The Python twin
// (tests/unit/test_tools_manifest.py) enforces the same full set, so a tool
// cannot ship in one implementation without the other.
func TestToolSurfaceMatchesManifest(t *testing.T) {
	t.Parallel()

	expected := loadManifest(t)

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
		if !actual[name] {
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
			"tools in docs/tools-manifest.txt but not registered by the Go server: %s",
			strings.Join(missing, ", "),
		)
	}

	if len(extra) > 0 {
		t.Errorf(
			"tools registered by the Go server but missing from docs/tools-manifest.txt: %s",
			strings.Join(extra, ", "),
		)
	}
}
