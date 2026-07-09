// Command hand-list-dump AST-extracts three hand-maintained validation
// value-sets from the Go source under go/internal/tools and prints them as a
// single JSON object on stdout, keyed by stable logical name.
//
// It feeds the Python sync gate (scripts/verify_sync_enums.py), which diffs
// these hand-lists against the live Linode OpenAPI spec, mirroring how
// scripts/verify_write_proto.py shells out to go/cmd/write-proto-dump.
//
// The tool reads .go source as text only with go/parser and go/ast. It never
// imports internal/tools or builds the package: the genpb generated tree is
// gitignored and may be absent, so any real import would fail the gate for the
// wrong reason. Zero third-party dependencies.
//
// Hard-fail contract: if any target symbol is missing, renamed, or extracts to
// an empty value set, the tool exits non-zero and names the offending target on
// stderr. A silently empty extraction must never look like success, because the
// Python gate would then diff an empty hand-list against the live spec and pass
// when it should have caught the drift.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// targetKind selects how a target's value-set is pulled out of the AST.
type targetKind int

const (
	// kindSwitch collects every string case label in the switch statements of
	// the named function.
	kindSwitch targetKind = iota
	// kindConst reads the string literal value of the named package const.
	kindConst
)

// target pairs a stable logical output key with the Go symbol that owns the
// hand-maintained values. The logical key is what the Python gate keys on, so
// it stays fixed even when the symbol or its file moves.
type target struct {
	logical string
	symbol  string
	kind    targetKind
}

// extractionTargets lists the three value-sets the sync gate needs. Symbols are
// found by name across every file, so moving a symbol between files does not
// break extraction. It is a function rather than a package global to satisfy
// gochecknoglobals.
func extractionTargets() []target {
	return []target{
		{logical: "bucket_acl", symbol: "validateBucketACL", kind: kindSwitch},
		{logical: "placement_group_type", symbol: "placementGroupTypeAntiAffinity", kind: kindConst},
		{logical: "config_device_slot", symbol: "validConfigDeviceSlot", kind: kindSwitch},
	}
}

func main() {
	toolsDir := flag.String("tools-dir", "internal/tools",
		"path to internal/tools, resolved relative to the working directory")

	flag.Parse()

	if err := run(*toolsDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run parses the tools package at toolsDir, extracts every target, and prints
// the result. It returns any error so main owns the sole os.Exit and flag.Parse
// (revive deep-exit).
func run(toolsDir string) error {
	files, err := parseGoFiles(toolsDir)
	if err != nil {
		return err
	}

	targets := extractionTargets()

	// Every target must succeed before anything prints so a partial run can
	// never emit a half-populated object that a downstream diff would trust.
	result := make(map[string][]string, len(targets))

	for _, tgt := range targets {
		values, extractErr := extract(tgt, files)
		if extractErr != nil {
			return extractErr
		}

		result[tgt.logical] = values
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if encErr := enc.Encode(result); encErr != nil {
		return fmt.Errorf("encode: %w", encErr)
	}

	return nil
}

// parseGoFiles walks dir and parses every non-test .go file. Test files are
// skipped so fixture strings in *_test.go can never leak into a value-set.
func parseGoFiles(dir string) ([]*ast.File, error) {
	fset := token.NewFileSet()

	var files []*ast.File

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		astFile, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", path, parseErr)
		}

		files = append(files, astFile)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}

	return files, nil
}

// extract pulls a target's value-set out of the parsed files, then sorts and
// de-duplicates it. An empty result is the renamed/deleted-symbol tripwire.
func extract(tgt target, files []*ast.File) ([]string, error) {
	var values []string

	switch tgt.kind {
	case kindSwitch:
		values = switchCaseStrings(tgt.symbol, files)
	case kindConst:
		values = constStrings(tgt.symbol, files)
	}

	values = sortedUnique(values)
	if len(values) == 0 {
		return nil, fmt.Errorf("target %q: symbol %q: %w", tgt.logical, tgt.symbol, errTargetMissing)
	}

	return values, nil
}

// switchCaseStrings collects every string case label inside the switch
// statements of the function named funcName, across all files.
func switchCaseStrings(funcName string, files []*ast.File) []string {
	var out []string

	for _, file := range files {
		funcDecl := findFunc(file, funcName)
		if funcDecl == nil {
			continue
		}

		ast.Inspect(funcDecl.Body, func(node ast.Node) bool {
			switchStmt, isSwitch := node.(*ast.SwitchStmt)
			if !isSwitch {
				return true
			}

			for _, stmt := range switchStmt.Body.List {
				clause, isClause := stmt.(*ast.CaseClause)
				if !isClause {
					continue
				}

				for _, expr := range clause.List {
					if value, ok := stringLit(expr); ok {
						out = append(out, value)
					}
				}
			}

			return true
		})
	}

	return out
}

// constStrings returns the string literal value of the package const named
// constName. Returns nil when the const is absent or not a string.
func constStrings(constName string, files []*ast.File) []string {
	var out []string

	for _, file := range files {
		for _, decl := range file.Decls {
			genDecl, isGen := decl.(*ast.GenDecl)
			if !isGen || genDecl.Tok != token.CONST {
				continue
			}

			for _, spec := range genDecl.Specs {
				valueSpec, isValue := spec.(*ast.ValueSpec)
				if !isValue {
					continue
				}

				for i, ident := range valueSpec.Names {
					if ident.Name != constName || i >= len(valueSpec.Values) {
						continue
					}

					if value, ok := stringLit(valueSpec.Values[i]); ok {
						out = append(out, value)
					}
				}
			}
		}
	}

	return out
}

// findFunc returns the first top-level function declaration named name in file,
// or nil. Only bodied declarations qualify so an interface method or forward
// declaration cannot masquerade as the target.
func findFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		funcDecl, isFunc := decl.(*ast.FuncDecl)
		if isFunc && funcDecl.Name != nil && funcDecl.Name.Name == name && funcDecl.Body != nil {
			return funcDecl
		}
	}

	return nil
}

// stringLit unquotes a string-literal expression. strconv.Unquote handles the
// raw-string and escaped forms so a value like public-read arrives clean.
func stringLit(expr ast.Expr) (string, bool) {
	lit, isLit := expr.(*ast.BasicLit)
	if !isLit || lit.Kind != token.STRING {
		return "", false
	}

	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}

	return value, true
}

// sortedUnique sorts ascending and drops duplicates so the JSON output is
// stable regardless of source ordering or accidental repeated case labels.
func sortedUnique(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	slices.Sort(values)

	return slices.Compact(values)
}
