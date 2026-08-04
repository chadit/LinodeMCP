package toolhooks_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

// manifestPath is the hook contract, relative to this package directory.
const manifestPath = "../../../docs/contracts/tool-hooks.txt"

// hookFields is the field count on a manifest line: tool, kind, language, function.
const hookFields = 4

// hookLanguage is the manifest column this package implements. A line naming
// another language declares a hook in that language's own package.
const hookLanguage = "go"

// hookLanguageField is the position of the language column on a manifest line.
const hookLanguageField = 2

// The two arguments linode_domain_record_get takes, and the sentences the hook
// answers with when either is not a positive integer.
const (
	domainIDArg  = "domain_id"
	recordIDArg  = "record_id"
	wantDomainID = "domain_id must be a positive integer"
	wantRecordID = "record_id must be a positive integer"
)

// TestManifestNamesEveryDeclaredHook covers the half of the hook contract the
// compiler cannot: a manifest-named hook missing here fails the build, but a
// function implemented here that no manifest line names has no such backstop.
// The check parses this package's source so no hand-kept list can go stale.
func TestManifestNamesEveryDeclaredHook(t *testing.T) {
	t.Parallel()

	declared := manifestFunctions(t)
	implemented := exportedFunctions(t)

	for _, name := range declared {
		if !slices.Contains(implemented, name) {
			t.Errorf("the manifest names %s, which this package does not implement", name)
		}
	}

	for _, name := range implemented {
		if !slices.Contains(declared, name) {
			t.Errorf("%s is implemented here but no manifest line names it", name)
		}
	}
}

// TestDomainRecordGetValidateRejectsNonPositiveIds guards wording that is fixed
// in both languages: the read surface elsewhere answers "domain_id is required"
// for a missing id, this tool answers "must be a positive integer" instead.
func TestDomainRecordGetValidateRejectsNonPositiveIds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{
			name: "missing domain id",
			args: map[string]any{recordIDArg: 7},
			want: wantDomainID,
		},
		{
			name: "zero domain id",
			args: map[string]any{domainIDArg: 0, recordIDArg: 7},
			want: wantDomainID,
		},
		{
			name: "negative domain id",
			args: map[string]any{domainIDArg: -1, recordIDArg: 7},
			want: wantDomainID,
		},
		{
			name: "missing record id",
			args: map[string]any{domainIDArg: 5},
			want: wantRecordID,
		},
		{
			name: "negative record id",
			args: map[string]any{domainIDArg: 5, recordIDArg: -2},
			want: wantRecordID,
		},
		{
			name: "both ids positive",
			args: map[string]any{domainIDArg: 5, recordIDArg: 7},
			want: "",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := requestWith(testCase.args)

			if got := toolhooks.DomainRecordGetValidate(&request); got != testCase.want {
				t.Errorf("DomainRecordGetValidate = %q, want %q", got, testCase.want)
			}
		})
	}
}

func requestWith(args map[string]any) mcp.CallToolRequest {
	var request mcp.CallToolRequest

	request.Params.Arguments = args

	return request
}

// manifestFunctions reads the Go function names the hook contract declares.
func manifestFunctions(t *testing.T) []string {
	t.Helper()

	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	names := make([]string, 0)

	for line := range strings.SplitSeq(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		parts := strings.Fields(trimmed)
		if len(parts) != hookFields {
			t.Fatalf("manifest line is malformed: %q", trimmed)
		}

		if parts[hookLanguageField] != hookLanguage {
			continue
		}

		names = append(names, parts[hookFields-1])
	}

	if len(names) == 0 {
		t.Fatal("manifest declares no hooks, so the comparison would prove nothing")
	}

	return names
}

// exportedFunctions parses this package's own source for its exported functions.
func exportedFunctions(t *testing.T) []string {
	t.Helper()

	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list package files: %v", err)
	}

	fset := token.NewFileSet()
	names := make([]string, 0, len(entries))

	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}

		names = append(names, exportedFuncNames(file)...)
	}

	return names
}

// exportedFuncNames lists the exported top-level functions one file declares.
func exportedFuncNames(file *ast.File) []string {
	names := make([]string, 0)

	for _, decl := range file.Decls {
		funcDecl, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || funcDecl.Recv != nil || funcDecl.Name == nil {
			continue
		}

		if ast.IsExported(funcDecl.Name.Name) {
			names = append(names, funcDecl.Name.Name)
		}
	}

	return names
}
