package linode_test

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestHTTPListInstanceInterfacesProtoDynamicRouteEvidence(t *testing.T) {
	t.Parallel()

	routes, err := os.ReadFile(filepath.Join(
		"..", "..", "..", "docs", "contracts", "tool-routes.txt",
	))
	if err != nil {
		t.Fatalf("read tool routes: %v", err)
	}

	const route = "linode_instance_interface_list: GET /linode/instances/{p}/interfaces"
	if !strings.Contains(string(routes), route) {
		t.Fatalf("tool routes do not contain %q", route)
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read linode package: %v", err)
	}

	const functionName = "httpListInstanceInterfacesProto"

	fset := token.NewFileSet()

	var declaration *ast.FuncDecl

	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		matches, err := build.Default.MatchFile(".", entry.Name())
		if err != nil {
			t.Fatalf("match Go file %s: %v", entry.Name(), err)
		}

		if !matches {
			continue
		}

		file, err := parser.ParseFile(
			fset,
			entry.Name(),
			nil,
			parser.SkipObjectResolution,
		)
		if err != nil {
			t.Fatalf("parse Go file %s: %v", entry.Name(), err)
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == functionName {
				declaration = fn
			}
		}
	}

	if declaration == nil {
		t.Fatalf("linode package does not contain %s", functionName)
	}

	const pathToken = "/%s/interfaces"

	var found bool

	ast.Inspect(declaration.Body, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}

		value, err := strconv.Unquote(literal.Value)
		if err == nil && value == pathToken {
			found = true
		}

		return true
	})

	if !found {
		t.Fatalf("%s does not contain path literal %q", functionName, pathToken)
	}
}
