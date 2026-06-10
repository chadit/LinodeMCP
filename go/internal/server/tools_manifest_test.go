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

const (
	annotationGoOnly = "go-only"
	annotationPyOnly = "py-only"
)

// loadManifest parses docs/tools-manifest.txt into a name -> annotation map.
// Annotation is "" for tools both implementations register, or one of the
// one-sided markers. Comment lines (#) and blanks are skipped.
func loadManifest(t *testing.T) map[string]string {
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

	entries := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, annotation, _ := strings.Cut(line, "\t")

		name = strings.TrimSpace(name)
		annotation = strings.TrimSpace(annotation)

		if annotation != "" && annotation != annotationGoOnly && annotation != annotationPyOnly {
			t.Fatalf("manifest line %q has unknown annotation %q", name, annotation)
		}

		if _, dup := entries[name]; dup {
			t.Fatalf("manifest lists %q twice", name)
		}

		entries[name] = annotation
	}

	if scanErr := scanner.Err(); scanErr != nil {
		t.Fatalf("read manifest: %v", scanErr)
	}

	return entries
}

// TestToolSurfaceMatchesManifest enforces cross-language tool-name parity:
// the Go catalog must equal exactly the manifest's names minus the py-only
// lines. A failure names every missing and extra tool so drift is obvious.
// The Python twin (tests/unit/test_tools_manifest.py) enforces the same
// manifest minus the go-only lines.
func TestToolSurfaceMatchesManifest(t *testing.T) {
	t.Parallel()

	manifest := loadManifest(t)

	expected := make(map[string]bool, len(manifest))

	for name, annotation := range manifest {
		if annotation == annotationPyOnly {
			continue
		}

		expected[name] = true
	}

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
