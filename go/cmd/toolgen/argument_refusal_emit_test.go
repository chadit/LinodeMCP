package main_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Go tree `make proto` writes and the extension its files carry, relative
// to this package because go test runs with that as cwd.
const (
	goPackageDir = "../../internal/gentools"
	pythonSuffix = ".py"
)

// The engine call every tool has to reach, and the package the emitted tree
// reaches it through.
const (
	refusalFunc   = "CheckArgumentRefusals"
	enginePackage = "tools"
)

// probeUnknownInput is the message this file's probe declares, named here
// because the case spells it both as a message name and inside the emitted
// call it looks for.
const probeUnknownInput = "ProbeUnknownArgumentInput"

// A tool that declares nothing about refusals still gets the check: it is the
// emitter default rather than an opt-in. The call hands the engine two names
// and lets it derive the allowlist and the sentence, so this pins the shape.
// The words are pinned by the behavior fixtures instead.
func TestEmitsTheUndeclaredArgumentCheck(t *testing.T) {
	t.Parallel()

	const message = "linode.mcp.v1." + probeUnknownInput

	files, err := goProbe(probeRegistered(t, probeUnknownInput, getOptions(),
		pathInt(probeIDArg))).Emit()
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	want := `tools.CheckArgumentRefusals("` + message + `", "` + probeToolName +
		`", request.GetArguments())`

	emitted := armText(files, "go")
	if !strings.Contains(emitted, want) {
		t.Errorf("the Go arm wrote no %s in:\n%s", want, emitted)
	}
}

// The default reaches every tool in both trees, which is the claim the option
// this replaced could not make. The rules check has always been written for
// every tool, so holding the two counts equal says the refusal is too, and it
// would have caught the Go list tiers, whose handler the emitter never writes.
func TestBothTreesCheckEveryToolTheyValidate(t *testing.T) {
	t.Parallel()

	trees := map[string]struct {
		dir       string
		suffix    string
		validates string
		refuses   string
	}{
		"go": {
			dir: goPackageDir, suffix: ".gen.go",
			validates: "toolvalidate.Check(", refuses: "tools.CheckArgumentRefusals(",
		},
		"python": {
			dir: pythonPackageDir, suffix: pythonSuffix,
			validates: "check_constraints(", refuses: "or unknown_arguments(",
		},
	}

	for language, tree := range trees {
		t.Run(language, func(t *testing.T) {
			t.Parallel()

			text := treeText(t, tree.dir, tree.suffix)

			validated := strings.Count(text, tree.validates)
			if validated == 0 {
				t.Fatalf("%s tree carries no %s, so this measured nothing", language, tree.validates)
			}

			if refused := strings.Count(text, tree.refuses); refused != validated {
				t.Errorf("%s tree carries %d %s and %d %s",
					language, validated, tree.validates, refused, tree.refuses)
			}
		})
	}
}

// armText is everything one language's probe arm emitted, joined so a case can
// search it without knowing which file the tool landed in.
func armText(files map[string]string, language string) string {
	var parts []string

	for name, text := range files {
		if strings.HasPrefix(name, language+"/") {
			parts = append(parts, text)
		}
	}

	return strings.Join(parts, "\n")
}

// treeText is every emitted file in one tree, read as one string.
func treeText(t *testing.T, dir, suffix string) string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	var parts []string

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}

		text, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Name(), readErr)
		}

		parts = append(parts, string(text))
	}

	return strings.Join(parts, "\n")
}

// enginePackageDir is the hand-written engine the emitted handlers delegate to.
// A tier that answers the refusal inside a driver emits no call of its own, so
// proving the default reached every tool means reading both trees.
const enginePackageDir = "../../internal/tools"

// The count above balances because a tier emitting neither call moves both
// numbers by zero, which is how 25 single-id destroys sat outside the default
// while the counts still matched. This walks each factory the emitted registry
// lists to the refusal instead, through the package's own calls and into the
// engine, so a tool is measured by where its call lands rather than by what its
// handler writes.
func TestEveryGeneratedFactoryReachesTheRefusal(t *testing.T) {
	t.Parallel()

	engine := refusingEngineFunctions(t)
	if len(engine) == 0 {
		t.Fatal("no engine function calls CheckArgumentRefusals, so this measured nothing")
	}

	generated := packageFunctions(t, goPackageDir)

	factories := factoryNames(t, generated)
	if len(factories) == 0 {
		t.Fatal("the emitted registry lists no factories, so this measured nothing")
	}

	var unreached []string

	for _, factory := range factories {
		if !reachesRefusal(factory, generated, engine) {
			unreached = append(unreached, factory)
		}
	}

	if len(unreached) > 0 {
		t.Errorf("%d of %d generated factories never reach tools.CheckArgumentRefusals: %s",
			len(unreached), len(factories), strings.Join(unreached, ", "))
	}
}

// refusingEngineFunctions is the set of engine entry points that answer the
// refusal, directly or through another engine function. The transitive step is
// what NewGeneratedListTool needs: it holds no check of its own and hands the
// call to the subresource driver.
func refusingEngineFunctions(t *testing.T) map[string]bool {
	t.Helper()

	engine := packageFunctions(t, enginePackageDir)
	refusing := make(map[string]bool, len(engine))

	for name, decl := range engine {
		if name != refusalFunc && callsFunction(decl, "", refusalFunc) {
			refusing[name] = true
		}
	}

	for grew := true; grew; {
		grew = false

		for name, decl := range engine {
			if refusing[name] || !callsAnyOf(decl, refusing) {
				continue
			}

			refusing[name] = true
			grew = true
		}
	}

	return refusing
}

// callsAnyOf reports whether a body calls one of the same-package functions in
// the set.
func callsAnyOf(decl *ast.FuncDecl, names map[string]bool) bool {
	for name := range names {
		if callsFunction(decl, "", name) {
			return true
		}
	}

	return false
}

// factoryNames reads the identifiers the emitted Factories() slice lists, which
// is one per registered tool.
func factoryNames(t *testing.T, generated map[string]*ast.FuncDecl) []string {
	t.Helper()

	decl, listed := generated["Factories"]
	if !listed {
		t.Fatal("the emitted tree has no Factories function")
	}

	var names []string

	ast.Inspect(decl, func(node ast.Node) bool {
		ident, isIdent := node.(*ast.Ident)
		if isIdent && generated[ident.Name] != nil && ident.Name != "Factories" {
			names = append(names, ident.Name)
		}

		return true
	})

	return names
}

// reachesRefusal walks the generated call graph from one factory, stepping into
// the package's own functions and stopping at the engine entry points that
// refuse.
func reachesRefusal(factory string, generated map[string]*ast.FuncDecl, engine map[string]bool) bool {
	seen := map[string]bool{}
	queue := []string{factory}

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		if seen[name] {
			continue
		}

		seen[name] = true

		decl := generated[name]
		if decl == nil {
			continue
		}

		if callsFunction(decl, enginePackage, refusalFunc) || callsRefusingEngine(decl, engine) {
			return true
		}

		queue = append(queue, calledPackageFunctions(decl, generated)...)
	}

	return false
}

// callsRefusingEngine reports whether a body hands the call to an engine entry
// point that answers the refusal itself.
func callsRefusingEngine(decl *ast.FuncDecl, engine map[string]bool) bool {
	for name := range engine {
		if callsFunction(decl, enginePackage, name) {
			return true
		}
	}

	return false
}

// calledPackageFunctions is every function of the same package a body calls.
func calledPackageFunctions(decl *ast.FuncDecl, generated map[string]*ast.FuncDecl) []string {
	var called []string

	ast.Inspect(decl, func(node ast.Node) bool {
		ident, isIdent := node.(*ast.Ident)
		if isIdent && generated[ident.Name] != nil {
			called = append(called, ident.Name)
		}

		return true
	})

	return called
}

// callsFunction reports whether a body calls pkg.name, or name alone when pkg
// is blank.
func callsFunction(decl *ast.FuncDecl, pkg, name string) bool {
	var found bool

	ast.Inspect(decl, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}

		if calledName(call.Fun) == name && calledPackage(call.Fun) == pkg {
			found = true
		}

		return !found
	})

	return found
}

// calledName is the function name a call expression names, blank for a call
// through anything but an identifier or a package selector.
func calledName(fun ast.Expr) string {
	switch target := fun.(type) {
	case *ast.Ident:
		return target.Name
	case *ast.SelectorExpr:
		return target.Sel.Name
	}

	return ""
}

// calledPackage is the package qualifier a call expression carries, blank for
// an unqualified call.
func calledPackage(fun ast.Expr) string {
	selector, isSelector := fun.(*ast.SelectorExpr)
	if !isSelector {
		return ""
	}

	ident, isIdent := selector.X.(*ast.Ident)
	if !isIdent {
		return ""
	}

	return ident.Name
}

// packageFunctions parses one package directory into its top-level functions,
// keyed by name. Test files are skipped: what ships is what has to refuse.
func packageFunctions(t *testing.T, dir string) map[string]*ast.FuncDecl {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	fileSet := token.NewFileSet()
	functions := make(map[string]*ast.FuncDecl)

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, parseErr := parser.ParseFile(fileSet, filepath.Join(dir, name), nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}

		collectFunctions(file, functions)
	}

	return functions
}

// collectFunctions records the file's top-level functions, methods excluded:
// the refusal is reached through plain calls, and a method name can collide
// with one.
func collectFunctions(file *ast.File, into map[string]*ast.FuncDecl) {
	for _, decl := range file.Decls {
		function, isFunction := decl.(*ast.FuncDecl)
		if isFunction && function.Recv == nil {
			into[function.Name.Name] = function
		}
	}
}
