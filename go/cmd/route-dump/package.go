package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

// clientPackage is the parsed client package plus the two lookup tables
// resolution needs: package-level string constants and every declared function
// by name.
//
// Functions are keyed by their own name, so a method and a plain function that
// share one name would collide. Rather than pick a winner, a shared name is
// recorded as ambiguous and resolves to nothing, which surfaces the affected
// call sites as unresolved instead of attributing a route to the wrong body.
type clientPackage struct {
	fset      *token.FileSet
	functions map[string]*ast.FuncDecl
	ambiguous map[string]bool
	constants map[string]string
	// resolving guards against a helper that reaches itself, directly or
	// through another helper, while its return value is being resolved.
	resolving map[string]bool
}

// parsePackage parses every non-test .go file under dir. Test files are skipped
// so a fixture endpoint in a _test.go can never enter the route surface as
// evidence that the client builds it.
func parsePackage(dir string) (*clientPackage, error) {
	pkg := &clientPackage{
		fset:      token.NewFileSet(),
		functions: map[string]*ast.FuncDecl{},
		ambiguous: map[string]bool{},
		constants: map[string]string{},
		resolving: map[string]bool{},
	}

	var files []*ast.File

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		astFile, parseErr := parser.ParseFile(pkg.fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", path, parseErr)
		}

		files = append(files, astFile)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}

	pkg.indexFunctions(files)
	pkg.indexConstants(files)

	return pkg, nil
}

// indexFunctions records every function declaration by name and flags the
// names more than one declaration claims.
func (pkg *clientPackage) indexFunctions(files []*ast.File) {
	for _, file := range files {
		for _, decl := range file.Decls {
			fnDecl, isFunc := decl.(*ast.FuncDecl)
			if !isFunc {
				continue
			}

			pkg.recordFunction(fnDecl)
		}
	}
}

func (pkg *clientPackage) recordFunction(decl *ast.FuncDecl) {
	name := decl.Name.Name
	if _, seen := pkg.functions[name]; seen {
		pkg.ambiguous[name] = true

		return
	}

	pkg.functions[name] = decl
}

// function returns the declaration a name resolves to, or false when the name
// is unknown or ambiguous.
func (pkg *clientPackage) function(name string) (*ast.FuncDecl, bool) {
	if pkg.ambiguous[name] {
		return nil, false
	}

	decl, ok := pkg.functions[name]

	return decl, ok
}

// indexConstants resolves the package's string constants, including the ones
// built from other constants (endpointRegions + "/%s/availability"). Each pass
// resolves whatever became resolvable in the previous one, so declaration
// order does not matter.
func (pkg *clientPackage) indexConstants(files []*ast.File) {
	pending := constantExprs(files)

	for changed := true; changed; {
		changed = false

		for name, expr := range pending {
			value, ok := pkg.resolveRoot(expr, nil)
			if !ok {
				continue
			}

			pkg.constants[name] = value

			delete(pending, name)

			changed = true
		}
	}
}

// constantExprs collects the value expression of every single-name constant.
// Non-string constants stay in the map and simply never resolve, which costs
// one wasted pass each and keeps the collector from having to know types.
func constantExprs(files []*ast.File) map[string]ast.Expr {
	exprs := map[string]ast.Expr{}

	for _, file := range files {
		for _, decl := range file.Decls {
			genDecl, isGen := decl.(*ast.GenDecl)
			if !isGen || genDecl.Tok != token.CONST {
				continue
			}

			collectConstSpecs(genDecl, exprs)
		}
	}

	return exprs
}

func collectConstSpecs(decl *ast.GenDecl, exprs map[string]ast.Expr) {
	for _, spec := range decl.Specs {
		valueSpec, isValue := spec.(*ast.ValueSpec)
		if !isValue || len(valueSpec.Names) != 1 || len(valueSpec.Values) != 1 {
			continue
		}

		exprs[valueSpec.Names[0].Name] = valueSpec.Values[0]
	}
}

// paramNames returns a function's parameter names in call-argument order. A
// grouped field ("operation, endpoint string") declares several names in one
// field, so a field index is not an argument index. An unnamed parameter
// yields an empty name and can never be bound.
func paramNames(decl *ast.FuncDecl) []string {
	if decl.Type.Params == nil {
		return nil
	}

	var names []string

	for _, field := range decl.Type.Params.List {
		if len(field.Names) == 0 {
			names = append(names, "")

			continue
		}

		for _, ident := range field.Names {
			names = append(names, ident.Name)
		}
	}

	return names
}

// paramIndex reports the argument position an identifier occupies in the
// enclosing function's parameter list.
func paramIndex(decl *ast.FuncDecl, name string) (int, bool) {
	for index, param := range paramNames(decl) {
		if param != "" && param == name {
			return index, true
		}
	}

	return 0, false
}

// calleeName reduces a call's function expression to the declared name, seeing
// through a method selector and through the explicit type arguments a generic
// helper may carry.
func calleeName(expr ast.Expr) (string, bool) {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name, true
	case *ast.SelectorExpr:
		return node.Sel.Name, true
	case *ast.IndexExpr:
		return calleeName(node.X)
	case *ast.IndexListExpr:
		return calleeName(node.X)
	}

	return "", false
}

// position renders a node's source location the way the gate reports it.
func (pkg *clientPackage) position(node ast.Node) string {
	pos := pkg.fset.Position(node.Pos())

	return fmt.Sprintf("%s:%d", filepath.ToSlash(pos.Filename), pos.Line)
}
