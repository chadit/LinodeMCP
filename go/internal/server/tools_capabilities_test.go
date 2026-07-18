package server_test

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// capabilitiesManifestPath points at the shared cross-language capability
// manifest at the repo root; this test runs three levels below it.
const capabilitiesManifestPath = "../../../docs/contracts/tools-capabilities.txt"

// toolParityBaselinePath records accepted cross-language divergences,
// including per-language absences ("<tool>: missing in <language>").
const toolParityBaselinePath = "../../../docs/contracts/tool-parity-baseline.txt"

// loadCapabilityManifest parses docs/contracts/tools-capabilities.txt into a tool->tier
// map. Each non-comment line is "<tool>\t<Capability>"; the tier strings match
// profiles.Capability.String() with the "Cap" prefix stripped (Read, Write,
// Destroy, Admin, Meta).
func loadCapabilityManifest(t *testing.T) map[string]string {
	t.Helper()

	file, err := os.Open(filepath.Clean(capabilitiesManifestPath))
	if err != nil {
		t.Fatalf("open capability manifest: %v", err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close capability manifest: %v", closeErr)
		}
	}()

	entries := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if trimmed := strings.TrimSpace(line); trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		name, tier, found := strings.Cut(line, "\t")
		if !found {
			t.Fatalf("capability manifest line %q is not <tool>\\t<Capability>", line)
		}

		name = strings.TrimSpace(name)
		tier = strings.TrimSpace(tier)

		if _, dup := entries[name]; dup {
			t.Fatalf("capability manifest lists %q twice", name)
		}

		entries[name] = tier
	}

	if scanErr := scanner.Err(); scanErr != nil {
		t.Fatalf("read capability manifest: %v", scanErr)
	}

	return entries
}

// loadMissingInLanguage returns the tools whose absence in the given language
// is an accepted divergence in the tool-parity baseline. The manifests keep
// listing those tools (they describe the full surface, and the other
// languages' tiers stay pinned), so this language's registry tests skip
// exactly this set in their missing checks. Trailing "  # accepted ..."
// annotations are stripped; scripts/verify_tool_parity.py enforces their
// presence and format.
func loadMissingInLanguage(t *testing.T, language string) map[string]bool {
	t.Helper()

	file, err := os.Open(filepath.Clean(toolParityBaselinePath))
	if err != nil {
		t.Fatalf("open tool parity baseline: %v", err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close tool parity baseline: %v", closeErr)
		}
	}()

	suffix := ": missing in " + language
	tools := make(map[string]bool)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		entry, _, _ := strings.Cut(line, "  # ")
		if name, found := strings.CutSuffix(strings.TrimSpace(entry), suffix); found {
			tools[name] = true
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		t.Fatalf("read tool parity baseline: %v", scanErr)
	}

	return tools
}

// TestToolCapabilitiesMatchManifest enforces cross-language capability parity:
// every registered tool's capability tag must equal docs/contracts/tools-capabilities.txt.
// The Python twin (tests/unit/test_tools_capabilities.py) checks the same file,
// so a tool cannot carry a different capability, and thus appear in a different
// profile, across the two implementations.
func TestToolCapabilitiesMatchManifest(t *testing.T) {
	t.Parallel()

	expected := loadCapabilityManifest(t)
	absent := loadMissingInLanguage(t, "go")

	srv := newCapabilityTestServer(t)
	infos := srv.AllToolInfos()

	var mismatched, extra []string

	seen := make(map[string]bool, len(infos))

	for _, info := range infos {
		seen[info.Name] = true
		actualTier := strings.TrimPrefix(info.Capability.String(), "Cap")

		want, ok := expected[info.Name]
		if !ok {
			extra = append(extra, info.Name)

			continue
		}

		if actualTier != want {
			mismatched = append(mismatched, info.Name+" (manifest "+want+", registry "+actualTier+")")
		}
	}

	var missing []string

	for name := range expected {
		if !seen[name] && !absent[name] {
			missing = append(missing, name)
		}
	}

	sort.Strings(mismatched)
	sort.Strings(missing)
	sort.Strings(extra)

	if len(mismatched) > 0 {
		t.Errorf("tool capabilities differ from docs/contracts/tools-capabilities.txt: %s", strings.Join(mismatched, ", "))
	}

	if len(missing) > 0 {
		t.Errorf("tools in docs/contracts/tools-capabilities.txt but not registered: %s", strings.Join(missing, ", "))
	}

	if len(extra) > 0 {
		t.Errorf("tools registered but missing from docs/contracts/tools-capabilities.txt: %s", strings.Join(extra, ", "))
	}
}
