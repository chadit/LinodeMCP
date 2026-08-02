package toolschemas_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// schemaDir is the generated schema directory the package embeds. The test
// reads it from disk to enumerate every name the embedded FS carries, which the
// external test package cannot reach through the unexported embed.FS.
const schemaDir = "data"

// schemaSuffix is the strict, snake_case variant Schema serves; the generator
// also writes camelCase and bundle variants into the same directory.
const schemaSuffix = ".schema.strict.json"

// TestSchemaServesEveryGeneratedName walks the generated directory and asserts
// Schema returns the file's bytes verbatim for every name in it. This is the
// no-op proof for the panic added alongside it: every name that resolved before
// still resolves to the same bytes.
func TestSchemaServesEveryGeneratedName(t *testing.T) {
	t.Parallel()

	names := generatedNames(t)
	if len(names) == 0 {
		t.Fatal("len(names) = 0, want the generated schema directory to be populated")
	}

	for _, name := range names {
		want, err := os.ReadFile(filepath.Join(schemaDir, name+schemaSuffix))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := toolschemas.Schema(name)
		if string(got) != string(want) {
			t.Errorf("Schema(%q) = %s, want %s", name, got, want)
		}
	}
}

// TestSchemaReturnsObjectSchemaForToolInput spot-checks that a served schema is
// a usable MCP input schema rather than any old bytes: it parses as JSON and
// declares an object.
func TestSchemaReturnsObjectSchemaForToolInput(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"linode.mcp.v1.HelloInput",
		"linode.mcp.v1.InstanceGetInput",
		"linode.mcp.v1.AuditHealthInput",
	} {
		var decoded struct {
			Type string `json:"type"`
		}

		if err := json.Unmarshal(toolschemas.Schema(name), &decoded); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if decoded.Type != "object" {
			t.Errorf("Schema(%q) type = %v, want %v", name, decoded.Type, "object")
		}
	}
}

// TestSchemaReportsFallbackForUnknownName pins the miss path. A name with no
// generated schema is a build defect, so the lookup hands back a placeholder
// IsFallback recognizes. server.New turns that into a startup failure, matching
// the Python twin's FileNotFoundError, rather than registering a tool that
// accepts anything.
func TestSchemaReportsFallbackForUnknownName(t *testing.T) {
	t.Parallel()

	const missing = "linode.mcp.v1.NoSuchMessageInput"

	if got := toolschemas.Schema(missing); !toolschemas.IsFallback(got) {
		t.Errorf("IsFallback(Schema(%q)) = false, want true (got %s)", missing, got)
	}
}

// TestSchemaReportsFallbackForEmptyName covers the degenerate lookup: an empty
// name has no file behind it either, so it misses the same way rather than
// resolving to some directory-level artifact.
func TestSchemaReportsFallbackForEmptyName(t *testing.T) {
	t.Parallel()

	if got := toolschemas.Schema(""); !toolschemas.IsFallback(got) {
		t.Errorf("IsFallback(Schema(\"\")) = false, want true (got %s)", got)
	}
}

// TestIsFallbackRejectsGeneratedSchemas is the other half of the guard: the
// startup check is only meaningful if no real schema trips it. Every generated
// name must read as not-fallback, otherwise server.New would refuse to start on
// a schema that is perfectly good.
func TestIsFallbackRejectsGeneratedSchemas(t *testing.T) {
	t.Parallel()

	names := generatedNames(t)
	if len(names) == 0 {
		t.Fatal("len(names) = 0, want the generated schema directory to be populated")
	}

	for _, name := range names {
		if toolschemas.IsFallback(toolschemas.Schema(name)) {
			t.Errorf("IsFallback(Schema(%q)) = true, want false", name)
		}
	}
}

// generatedNames returns the proto full name behind every strict schema file in
// the generated directory.
func generatedNames(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := make([]string, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), schemaSuffix) {
			continue
		}

		names = append(names, strings.TrimSuffix(entry.Name(), schemaSuffix))
	}

	return names
}
