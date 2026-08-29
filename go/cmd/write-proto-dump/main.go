// Command write-proto-dump AST-analyzes the tool factories and handlers in
// go/internal/tools and go/internal/gentools and prints a JSON object on stdout
// mapping tool name to classification. It tells the generated-form gate which
// tools are proto-canonical and which still use the legacy
// map[string]any / MarshalToolResponse path.
//
// -surface write (default) classifies the mutating surface (CapWrite,
// CapDestroy, CapAdmin) and -surface read the CapRead surface, both by success
// path: "proto", "legacy", or "review". The same analysis serves both; the
// destroy-wrapper rules never fire for read handlers.
//
// -surface input classifies the request-schema surface of every tool
// regardless of capability: "generated" when the factory builds its MCP input
// schema from the proto contract (it reaches mcp.NewToolWithRawSchema or
// toolschemas.Schema), "hand" when it builds the schema from mcp.With* option
// builders. This drives the input surface of scripts/verify_generated_form.py.
//
// Detection strategy (identifier-name reachability, no go/types):
//
//  1. Build the server with a throwaway config (same as parity-dump) and call
//     AllToolInfos() for the authoritative tool list.
//
//  2. Parse the non-test .go files of both packages into one name to handler
//     map: a factory (func New*Tool) names its tool with a "linode_"-shaped
//     string literal or a string const and wires a handle* function, either
//     bare or through a delegating closure. Consts resolve first, including
//     "a" + "b" concatenations; mcp.With* option calls are skipped so a param
//     name like "linode_id" is not mistaken for the tool name.
//
//  3. Build a call graph for the whole package: for every top-level func decl
//     (and its nested func-lit bodies), record all called function names.
//
//  4. Classify each handler by transitive reachability:
//     - "MarshalProtoToolResponse", either of its null-restoring variants, or
//     "MarshalProtoJSON" on any path => proto.
//     Passing MarshalProtoJSON into a reachable helper counts too, since that
//     is how handlers preserve documented JSON nulls after canonical proto
//     serialization.
//     - A "RunDestructiveActionWithID" / "RunDestructiveActionByTwoIDs" /
//     "RunDestructiveActionByRegionLabel" call whose literal sets SuccessProto
//     => proto, because the wrapper then routes the body through the proto
//     marshaller. Without it the wrapper builds its legacy map => legacy, and a
//     bare identifier reference (which cannot set the field) is legacy too.
//     - "RunDestructiveAction" with a DestructiveAction literal => proto when
//     its Success closure returns a &linodev1.* proto pointer, legacy when it
//     hands back a map[string]any{}.
//     - "MarshalToolResponse" or "marshalDestroySuccess" with no proto sink
//     reachable => legacy.
//     - Nothing of the above reachable => review.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

const (
	placeholderTokenLen = 16

	// sink function names.
	sinkProto = "MarshalProtoToolResponse"
	// The variant a mutation answers through when it restores the explicit
	// nulls its decode dropped, which is still the proto marshaller.
	sinkProtoNulls = "MarshalProtoToolResponseRestoringNulls"
	// The same variant for a page, which restores each element's nulls rather
	// than the response's own. Still the proto marshaller.
	sinkProtoListNulls = "MarshalProtoListResponseRestoringNulls"
	sinkProtoJSON      = "MarshalProtoJSON"
	sinkLegacy         = "MarshalToolResponse"
	sinkMarshalDestroy = "marshalDestroySuccess"
	sinkWithID         = "RunDestructiveActionWithID"
	sinkByTwoIDs       = "RunDestructiveActionByTwoIDs"
	sinkByRegionLabel  = "RunDestructiveActionByRegionLabel"
	sinkRunDestructive = "RunDestructiveAction"

	// linodev1Prefix is the import alias for the genpb package used in
	// proto composite literals.
	linodev1Prefix = "linodev1"

	// fieldSuccessProto is the DestructiveAction* literal field whose presence
	// marks a destroy wrapper as proto-routed.
	fieldSuccessProto = "SuccessProto"

	// successProtoParam is the parameter name a shared destroy helper uses for a
	// proto.Message it forwards from its Success closure. Returning it is
	// proto-routed because marshalDestroySuccess routes any proto.Message
	// through the proto marshaller.
	successProtoParam = "successProto"

	// successBuilderPrefix names the answer builder a generated destroy calls
	// from its Success closure. go/cmd/toolgen writes one per tool and returns
	// its call rather than a literal, so the classifier reads the call the same
	// way it reads the SuccessProto field the single-id wrapper carries.
	successBuilderPrefix = "success"

	classifyProto  = "proto"
	classifyLegacy = "legacy"
	classifyReview = "review"

	// surfaceInput selects the request-schema classification (generated vs
	// hand) over every tool, independent of the write/read success-path modes.
	surfaceInput = "input"

	// input-surface sinks: reaching any of these marks a factory as building its
	// MCP input schema from the proto contract rather than mcp.With* builders.
	// Both qualified and bare forms are listed because callExprName keeps the
	// package qualifier only when the call site writes one.
	sinkRawSchemaMCP = "mcp.NewToolWithRawSchema"
	sinkRawSchema    = "NewToolWithRawSchema"
	sinkToolschemas  = "toolschemas.Schema"

	classifyGenerated = "generated"
	classifyHand      = "hand"

	// toolsQualifier is how a generated factory in internal/gentools names a
	// driver that lives in internal/tools. Both packages share one call graph,
	// so the qualifier has to be stripped: otherwise a generated tool's path to
	// MarshalProtoToolResponse ends at a name nothing in the graph declares and
	// every generated tool classifies as review.
	toolsQualifier = "tools."
)

// callRecord records a single outgoing call from a function.
type callRecord struct {
	// successIsProto is non-nil only for destroy-wrapper calls whose Success
	// closure or SuccessProto field could be inspected: true for a proto
	// pointer, false for a map.
	successIsProto *bool
	name           string
}

// packageCallGraph maps function name to the list of outgoing calls it makes
// (direct only; transitive closure is resolved in classify).
type packageCallGraph map[string][]callRecord

func main() {
	surface := flag.String("surface", "write",
		"tool surface to classify: write (CapWrite/CapDestroy/CapAdmin), read (CapRead), meta (CapMeta), or input (all tools)")

	flag.Parse()

	toolDirs, err := locateToolDirs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "locate tools dir: %v\n", err)
		os.Exit(1)
	}

	tools, err := buildToolSet(*surface)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build server: %v\n", err)
		os.Exit(1)
	}

	fset := token.NewFileSet()

	files, err := parseToolPackages(toolDirs, fset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse tools package: %v\n", err)
		os.Exit(1)
	}

	stringConsts := buildStringConstMap(files)
	graph := buildCallGraph(files)

	var result map[string]string

	switch *surface {
	case surfaceInput:
		result = classifyInputSurface(tools, files, stringConsts, graph)
	default:
		result = classifySuccessSurface(tools, buildNameToHandler(files, stringConsts), graph)
	}

	keys := make([]string, 0, len(result))
	for toolName := range result {
		keys = append(keys, toolName)
	}

	sort.Strings(keys)

	ordered := make(map[string]string, len(result))
	for _, toolName := range keys {
		ordered[toolName] = result[toolName]
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if encErr := enc.Encode(ordered); encErr != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", encErr)
		os.Exit(1)
	}
}

// locateToolDirs returns the absolute paths of the packages a tool's answer is
// assembled across: the hand-written internal/tools and the generated
// internal/gentools. Both are read as one call graph, because a generated
// handler's path to the message it serializes can run through either.
//
// A missing generated tree is not an error, since before `make proto` has run
// there are no generated tools to classify and buildToolSet would have failed
// first if the server needed them.
func locateToolDirs() ([]string, error) {
	toolsDir, err := locateToolsDir()
	if err != nil {
		return nil, err
	}

	dirs := []string{toolsDir}

	candidate := filepath.Join(filepath.Dir(toolsDir), generatedToolsPackage)
	if _, statErr := os.Stat(candidate); statErr == nil {
		dirs = append(dirs, candidate)
	}

	return dirs, nil
}

// generatedToolsPackage is the directory name of the emitted tool package,
// a sibling of internal/tools.
const generatedToolsPackage = "gentools"

// locateToolsDir returns the absolute path to go/internal/tools. It tries
// executable-relative resolution first (works for built binaries), then falls
// back to CWD-relative candidates for common `go run` invocation points.
func locateToolsDir() (string, error) {
	if abs, found := exeRelativeToolsDir(); found {
		return abs, nil
	}

	return toolsDirFromCWD()
}

// exeRelativeToolsDir attempts to resolve go/internal/tools relative to the
// running executable. Returns ("", false) when the executable path is
// unavailable or the candidate directory does not exist.
func exeRelativeToolsDir() (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", false
	}

	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", false
	}

	// Executable lives at go/cmd/write-proto-dump/write-proto-dump;
	// walk up three dirs to reach go/, then descend into internal/tools.
	candidate := filepath.Join(filepath.Dir(resolved), "..", "..", "..", "internal", "tools")
	if _, statErr := os.Stat(candidate); statErr != nil {
		return "", false
	}

	abs, absErr := filepath.Abs(candidate)
	if absErr != nil {
		return "", false
	}

	return abs, true
}

// toolsDirFromCWD searches for go/internal/tools relative to the process
// working directory. Covers the two common invocation points: from go/ and
// from the repo root.
func toolsDirFromCWD() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}

	for _, rel := range []string{"internal/tools", "go/internal/tools"} {
		abs := filepath.Join(cwd, rel)
		if _, statErr := os.Stat(abs); statErr == nil {
			return abs, nil
		}
	}

	return "", &toolsDirNotFoundError{cwd: cwd}
}

// toolsDirNotFoundError is returned when internal/tools cannot be located.
type toolsDirNotFoundError struct {
	cwd string
}

func (e *toolsDirNotFoundError) Error() string {
	return "cannot locate internal/tools from " + e.cwd
}

// buildToolSet returns the sorted names of all tools on the requested surface.
// "input" selects every tool regardless of capability because the input-schema
// surface is capability-blind.
func buildToolSet(surface string) ([]string, error) {
	switch surface {
	case "write":
		return buildToolNames(func(capability string) bool {
			return capability == "CapWrite" || capability == "CapDestroy" || capability == "CapAdmin"
		})
	case "read":
		return buildToolNames(func(capability string) bool { return capability == "CapRead" })
	case "meta":
		return buildToolNames(func(capability string) bool { return capability == "CapMeta" })
	case surfaceInput:
		return buildToolNames(func(string) bool { return true })
	default:
		return nil, fmt.Errorf("%w: %q (want write, read, meta, or input)", errUnknownSurface, surface)
	}
}

// buildToolNames builds the server with a throwaway config and returns the
// sorted names of all tools whose capability string satisfies include.
func buildToolNames(include func(capability string) bool) ([]string, error) {
	placeholderToken := strings.Repeat("0", placeholderTokenLen)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:      "write-proto-dump",
			LogLevel:  "error",
			Transport: "stdio",
			Host:      "127.0.0.1",
			Port:      8080,
		},
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label: "default",
				Linode: config.LinodeConfig{
					APIURL: "https://api.linode.com/v4",
					Token:  placeholderToken,
				},
			},
		},
	}

	srv, err := server.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("server.New: %w", err)
	}

	infos := srv.AllToolInfos()
	names := make([]string, 0, len(infos))

	for _, info := range infos {
		if include(info.Capability.String()) {
			names = append(names, info.Name)
		}
	}

	sort.Strings(names)

	return names, nil
}

// parseToolPackages parses every directory's non-test .go files into one AST
// file list, so both tool packages share a single call graph.
func parseToolPackages(dirs []string, fset *token.FileSet) ([]*ast.File, error) {
	files := make([]*ast.File, 0)

	for _, dir := range dirs {
		parsed, err := parseToolsPackage(dir, fset)
		if err != nil {
			return nil, err
		}

		files = append(files, parsed...)
	}

	return files, nil
}

// parseToolsPackage parses all non-test .go files in dir and returns the AST
// file list.
func parseToolsPackage(dir string, fset *token.FileSet) ([]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	files := make([]*ast.File, 0, len(entries))

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		path := filepath.Join(dir, name)

		astFile, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil, fmt.Errorf("parse %s: %w", path, parseErr)
		}

		files = append(files, astFile)
	}

	return files, nil
}

// buildStringConstMap collects every package-level string constant so a
// factory that names its tool with a const identifier can be resolved to the
// literal tool name the const holds.
func buildStringConstMap(files []*ast.File) map[string]string {
	// The tools package declares a few hundred string consts; a per-file hint
	// keeps the map from rehashing repeatedly as they are collected.
	const constsPerFileHint = 8

	consts := make(map[string]string, len(files)*constsPerFileHint)

	for _, astFile := range files {
		for _, decl := range astFile.Decls {
			genDecl, isGen := decl.(*ast.GenDecl)
			if !isGen || genDecl.Tok != token.CONST {
				continue
			}

			for _, spec := range genDecl.Specs {
				valueSpec, isValue := spec.(*ast.ValueSpec)
				if !isValue {
					continue
				}

				for i, name := range valueSpec.Names {
					if i >= len(valueSpec.Values) {
						continue
					}

					if val := constStringValue(valueSpec.Values[i]); val != "" {
						consts[name.Name] = val
					}
				}
			}
		}
	}

	return consts
}

// buildNameToHandler walks every func New*Tool declaration and records the
// tool name to handler function name mapping.
func buildNameToHandler(files []*ast.File, consts map[string]string) map[string]string {
	result := make(map[string]string, len(files))

	for _, astFile := range files {
		for _, decl := range astFile.Decls {
			funcDecl, isFuncDecl := decl.(*ast.FuncDecl)
			if !isFuncDecl || funcDecl.Name == nil || funcDecl.Body == nil {
				continue
			}

			if !strings.HasPrefix(funcDecl.Name.Name, "New") || !strings.HasSuffix(funcDecl.Name.Name, "Tool") {
				continue
			}

			toolName, handlerName := extractToolAndHandler(funcDecl, consts)
			if toolName == "" {
				continue
			}

			// Fully inline handler closure (raw-schema factories): the call
			// graph credits the closure's calls to the enclosing New*Tool, so
			// reachability can run through the factory itself.
			if handlerName == "" {
				handlerName = funcDecl.Name.Name
			}

			result[toolName] = handlerName
		}
	}

	return result
}

// toolNameArg resolves an argument to a tool name: a tool-name-shaped string
// literal, or a const identifier whose value is one. Returns "" otherwise.
func toolNameArg(expr ast.Expr, consts map[string]string) string {
	if lit := stringLiteral(expr); isToolName(lit) {
		return lit
	}

	if ident, isIdent := expr.(*ast.Ident); isIdent {
		if val := consts[ident.Name]; isToolName(val) {
			return val
		}
	}

	return ""
}

// isToolName reports whether s has the shape of a registered tool name: the
// "linode_" prefix, plus the two meta tools that predate it. The shape check
// keeps param-name literals inside a factory, mcp.WithString("label", ...),
// from being read as the tool name.
func isToolName(s string) bool {
	return strings.HasPrefix(s, "linode_") || s == "hello" || s == "version"
}

// handlerArg resolves an argument to a handler function name: a bare handle*
// identifier, or a closure that delegates to one. Returns "" otherwise.
func handlerArg(expr ast.Expr) string {
	switch arg := expr.(type) {
	case *ast.Ident:
		if strings.HasPrefix(arg.Name, "handle") || strings.HasPrefix(arg.Name, "Handle") {
			return arg.Name
		}
	case *ast.FuncLit:
		return singleDelegateCall(arg)
	}

	return ""
}

// extractToolAndHandler inspects a factory function body and returns the tool
// name and the handler function name that serves it. It matches any call
// carrying both a tool-name argument and a handle* argument, which covers
// newToolWithHandler, the per-family constructors like
// newDatabaseInstanceCreateTool, and factories that assign the handler
// separately after an mcp.NewTool call.
func extractToolAndHandler(funcDecl *ast.FuncDecl, consts map[string]string) (string, string) {
	var foundTool string

	var foundHandler string

	var foundFactory string

	ast.Inspect(funcDecl.Body, func(node ast.Node) bool {
		callExpr, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}

		callee := callExprName(callExpr)

		if callee == "NewTool" || callee == "mcp.NewTool" ||
			callee == "NewToolWithRawSchema" || callee == "mcp.NewToolWithRawSchema" {
			if len(callExpr.Args) >= 1 {
				if name := toolNameArg(callExpr.Args[0], consts); name != "" {
					foundTool = name
				}
			}

			return true
		}

		// Skip the mcp option builders: their first string arg is a param name,
		// and mcp.WithNumber("linode_id", ...) would otherwise be read as a
		// tool name because it shares the linode_ prefix.
		if strings.HasPrefix(callee, "mcp.") {
			return true
		}

		name, handler := toolAndHandlerFromArgs(callExpr.Args, consts)
		if name != "" {
			foundTool = name

			// A package-local factory (newProtoListTool and friends) returns
			// the handler instead of taking a handle* argument. Remember the
			// callee: if no handler surfaces any other way, reachability runs
			// through the factory body, which hits the same marshal sinks the
			// returned closure would.
			if handler == "" && !strings.Contains(callee, ".") {
				foundFactory = callee
			}
		}

		if handler != "" {
			foundHandler = handler
		}

		return true
	})

	// The handler may be wired in a separate assignment (mcp.NewTool factories).
	if foundHandler == "" && foundTool != "" {
		foundHandler = extractHandlerFromAssignment(funcDecl)
	}

	// Last resort: classify through the factory that received the tool name.
	if foundHandler == "" {
		foundHandler = foundFactory
	}

	return foundTool, foundHandler
}

// toolAndHandlerFromArgs scans a constructor call's arguments for a tool-name
// value and a handler function, returning whichever it finds.
func toolAndHandlerFromArgs(args []ast.Expr, consts map[string]string) (string, string) {
	var name, handler string

	for _, arg := range args {
		if name == "" {
			if candidate := toolNameArg(arg, consts); candidate != "" {
				name = candidate
			}
		}

		if handler == "" {
			if candidate := handlerArg(arg); candidate != "" {
				handler = candidate
			}
		}
	}

	return name, handler
}

// extractHandlerFromAssignment scans a factory body for an assignment of the
// form: handler := func(ctx, request) { return handleXRequest(ctx, &request, cfg) }.
// Returns the called function name or "".
func extractHandlerFromAssignment(funcDecl *ast.FuncDecl) string {
	var found string

	ast.Inspect(funcDecl.Body, func(node ast.Node) bool {
		if found != "" {
			return false
		}

		assign, isAssign := node.(*ast.AssignStmt)
		if !isAssign {
			return true
		}

		for _, rhs := range assign.Rhs {
			funcLit, isFuncLit := rhs.(*ast.FuncLit)
			if !isFuncLit {
				continue
			}

			delegated := singleDelegateCall(funcLit)
			if delegated == "" {
				continue
			}

			for _, lhs := range assign.Lhs {
				ident, isIdent := lhs.(*ast.Ident)
				if isIdent && strings.Contains(strings.ToLower(ident.Name), "handler") {
					found = delegated

					return false
				}
			}
		}

		return true
	})

	return found
}

// singleDelegateCall returns the function name when a func-lit body contains
// a single return statement delegating to a top-level handler function.
// Returns "" otherwise.
func singleDelegateCall(funcLit *ast.FuncLit) string {
	if funcLit.Body == nil {
		return ""
	}

	for _, stmt := range funcLit.Body.List {
		retStmt, isReturn := stmt.(*ast.ReturnStmt)
		if !isReturn || len(retStmt.Results) != 1 {
			continue
		}

		callExpr, isCall := retStmt.Results[0].(*ast.CallExpr)
		if !isCall {
			continue
		}

		name := callExprName(callExpr)
		if strings.HasPrefix(name, "handle") || strings.HasPrefix(name, "Handle") {
			return name
		}
	}

	return ""
}

// callExprName returns the function name for a call expression.
// For "pkg.Func(...)" it returns "pkg.Func"; for "Func(...)" it returns "Func".
func callExprName(callExpr *ast.CallExpr) string {
	switch funcExpr := callExpr.Fun.(type) {
	case *ast.Ident:
		return funcExpr.Name
	case *ast.SelectorExpr:
		if pkg, isPkg := funcExpr.X.(*ast.Ident); isPkg {
			return pkg.Name + "." + funcExpr.Sel.Name
		}

		return funcExpr.Sel.Name
	}

	return ""
}

// constStringValue evaluates a const's value expression to a string: a plain
// string literal, or a "+"-concatenation of string literals (some tool-name
// consts are written as "linode_x_" + "y_create"). Returns "" for anything
// else.
func constStringValue(expr ast.Expr) string {
	if lit := stringLiteral(expr); lit != "" {
		return lit
	}

	binExpr, isBin := expr.(*ast.BinaryExpr)
	if !isBin || binExpr.Op != token.ADD {
		return ""
	}

	left := constStringValue(binExpr.X)
	right := constStringValue(binExpr.Y)

	if left == "" || right == "" {
		return ""
	}

	return left + right
}

// stringLiteral returns the unquoted value of a string literal node, or "".
func stringLiteral(expr ast.Expr) string {
	lit, isLit := expr.(*ast.BasicLit)
	if !isLit || lit.Kind != token.STRING {
		return ""
	}

	val := lit.Value
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		return val[1 : len(val)-1]
	}

	return val
}

// buildCallGraph returns a map from function name to all outgoing calls inside
// that function (including calls inside nested func-lit bodies).
func buildCallGraph(files []*ast.File) packageCallGraph {
	graph := make(packageCallGraph)

	for _, astFile := range files {
		for _, decl := range astFile.Decls {
			funcDecl, isFuncDecl := decl.(*ast.FuncDecl)
			if !isFuncDecl || funcDecl.Name == nil || funcDecl.Body == nil {
				continue
			}

			graph[funcDecl.Name.Name] = collectCalls(funcDecl.Body)
		}
	}

	return graph
}

// collectCalls walks a function body and collects all outgoing calls. It also
// records MarshalProtoJSON when passed as a helper argument, since the helper
// then invokes that canonical serializer indirectly, and inspects each destroy
// wrapper's literal to decide proto vs. map output.
func collectCalls(body *ast.BlockStmt) []callRecord {
	var records []callRecord

	ast.Inspect(body, func(node ast.Node) bool {
		callExpr, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}

		name := callExprName(callExpr)
		if name == "" {
			return true
		}

		// A generated factory reaches the shared drivers through the tools
		// package. Record the call as written, then continue with the bare
		// name so the sink comparisons below see the same function a call from
		// inside internal/tools would.
		if bare, qualified := strings.CutPrefix(name, toolsQualifier); qualified {
			records = append(records, callRecord{name: name})
			name = bare
		}

		for _, arg := range callExpr.Args {
			ident, isIdent := arg.(*ast.Ident)
			if isIdent && ident.Name == sinkProtoJSON {
				records = append(records, callRecord{name: sinkProtoJSON})
			}
		}

		if name == sinkRunDestructive {
			protoFlag := detectDestructiveActionSuccessProto(callExpr)
			records = append(records, callRecord{name: name, successIsProto: protoFlag})

			return true
		}

		if name == sinkWithID {
			protoFlag := detectByIDSuccessProto(callExpr)
			records = append(records, callRecord{name: name, successIsProto: protoFlag})

			return true
		}

		if name == sinkByTwoIDs {
			protoFlag := detectByTwoIDsSuccessProto(callExpr)
			records = append(records, callRecord{name: name, successIsProto: protoFlag})

			return true
		}

		if name == sinkByRegionLabel {
			protoFlag := detectByRegionLabelSuccessProto(callExpr)
			records = append(records, callRecord{name: name, successIsProto: protoFlag})

			return true
		}

		records = append(records, callRecord{name: name})

		return true
	})

	return records
}

// detectByIDSuccessProto inspects a RunDestructiveActionWithID call for a
// &DestructiveActionByID{...} literal that sets SuccessProto: true routes the
// success body through the proto marshaller, false leaves the wrapper building
// its legacy id-echo map. Returns nil when the argument is not that literal.
func detectByIDSuccessProto(callExpr *ast.CallExpr) *bool {
	protoResult := true

	var mapResult bool

	for _, arg := range callExpr.Args {
		unaryExpr, isUnary := arg.(*ast.UnaryExpr)
		if !isUnary || unaryExpr.Op.String() != "&" {
			continue
		}

		litExpr, isLit := unaryExpr.X.(*ast.CompositeLit)
		if !isLit || compositeLitTypeName(litExpr) != "DestructiveActionByID" {
			continue
		}

		for _, elt := range litExpr.Elts {
			kvExpr, isKV := elt.(*ast.KeyValueExpr)
			if !isKV {
				continue
			}

			keyIdent, isIdent := kvExpr.Key.(*ast.Ident)
			if isIdent && keyIdent.Name == fieldSuccessProto {
				return &protoResult
			}
		}

		return &mapResult
	}

	return nil
}

// detectByTwoIDsSuccessProto is detectByIDSuccessProto for
// RunDestructiveActionByTwoIDs and its &DestructiveActionByTwoIDs{...} literal.
func detectByTwoIDsSuccessProto(callExpr *ast.CallExpr) *bool {
	protoResult := true

	var mapResult bool

	for _, arg := range callExpr.Args {
		unaryExpr, isUnary := arg.(*ast.UnaryExpr)
		if !isUnary || unaryExpr.Op.String() != "&" {
			continue
		}

		litExpr, isLit := unaryExpr.X.(*ast.CompositeLit)
		if !isLit || compositeLitTypeName(litExpr) != "DestructiveActionByTwoIDs" {
			continue
		}

		for _, elt := range litExpr.Elts {
			kvExpr, isKV := elt.(*ast.KeyValueExpr)
			if !isKV {
				continue
			}

			keyIdent, isIdent := kvExpr.Key.(*ast.Ident)
			if isIdent && keyIdent.Name == fieldSuccessProto {
				return &protoResult
			}
		}

		return &mapResult
	}

	return nil
}

// detectByRegionLabelSuccessProto is detectByIDSuccessProto for
// RunDestructiveActionByRegionLabel and its
// &DestructiveActionByRegionLabel{...} literal, whose legacy shape is a
// {message, region, <key>} map.
func detectByRegionLabelSuccessProto(callExpr *ast.CallExpr) *bool {
	protoResult := true

	var mapResult bool

	for _, arg := range callExpr.Args {
		unaryExpr, isUnary := arg.(*ast.UnaryExpr)
		if !isUnary || unaryExpr.Op.String() != "&" {
			continue
		}

		litExpr, isLit := unaryExpr.X.(*ast.CompositeLit)
		if !isLit || compositeLitTypeName(litExpr) != "DestructiveActionByRegionLabel" {
			continue
		}

		for _, elt := range litExpr.Elts {
			kvExpr, isKV := elt.(*ast.KeyValueExpr)
			if !isKV {
				continue
			}

			keyIdent, isIdent := kvExpr.Key.(*ast.Ident)
			if isIdent && keyIdent.Name == fieldSuccessProto {
				return &protoResult
			}
		}

		return &mapResult
	}

	return nil
}

// detectDestructiveActionSuccessProto inspects a RunDestructiveAction call
// for a &DestructiveAction{...} literal argument and determines whether its
// Success closure returns a proto pointer (true), a map (false), or is
// indeterminate (nil).
func detectDestructiveActionSuccessProto(callExpr *ast.CallExpr) *bool {
	protoResult := true

	var mapResult bool

	for _, arg := range callExpr.Args {
		unaryExpr, isUnary := arg.(*ast.UnaryExpr)
		if !isUnary || unaryExpr.Op.String() != "&" {
			continue
		}

		litExpr, isLit := unaryExpr.X.(*ast.CompositeLit)
		if !isLit {
			continue
		}

		litTypeName := compositeLitTypeName(litExpr)
		if litTypeName != "DestructiveAction" {
			continue
		}

		for _, elt := range litExpr.Elts {
			kvExpr, isKV := elt.(*ast.KeyValueExpr)
			if !isKV {
				continue
			}

			keyIdent, isIdent := kvExpr.Key.(*ast.Ident)
			if !isIdent || keyIdent.Name != "Success" {
				continue
			}

			funcLit, isFuncLit := kvExpr.Value.(*ast.FuncLit)
			if !isFuncLit {
				return nil
			}

			if successClosureIsProto(funcLit) {
				return &protoResult
			}

			return &mapResult
		}
	}

	return nil
}

// compositeLitTypeName returns the unqualified type name of a composite
// literal, handling both *ast.Ident ("DestructiveAction") and
// *ast.SelectorExpr ("tools.DestructiveAction").
func compositeLitTypeName(lit *ast.CompositeLit) string {
	switch typ := lit.Type.(type) {
	case *ast.Ident:
		return typ.Name
	case *ast.SelectorExpr:
		return typ.Sel.Name
	}

	return ""
}

// successClosureIsProto returns true when the Success closure body's return
// expression is a &linodev1.X{...} proto pointer, or the bare identifier
// successProto that a shared destroy helper forwards so two callers can pass
// different concrete response messages. The identifier counts because the
// runtime marshalDestroySuccess routes any proto.Message through the proto
// marshaller, same as an inline literal.
func successClosureIsProto(funcLit *ast.FuncLit) bool {
	var foundProto bool

	ast.Inspect(funcLit, func(node ast.Node) bool {
		retStmt, isReturn := node.(*ast.ReturnStmt)
		if !isReturn {
			return true
		}

		if slices.ContainsFunc(retStmt.Results, isProtoResultExpr) {
			foundProto = true

			return false
		}

		return true
	})

	return foundProto
}

// isProtoResultExpr reports whether a Success closure return expression is
// proto-routed: either a &linodev1.X{...} literal, or the bare successProto
// identifier a shared destroy helper forwards.
func isProtoResultExpr(expr ast.Expr) bool {
	if isProtoPointerExpr(expr) {
		return true
	}

	if call, isCall := expr.(*ast.CallExpr); isCall {
		callee, namedFunc := call.Fun.(*ast.Ident)

		return namedFunc && strings.HasPrefix(callee.Name, successBuilderPrefix)
	}

	ident, isIdent := expr.(*ast.Ident)

	return isIdent && ident.Name == successProtoParam
}

// isProtoPointerExpr reports whether expr is a &linodev1.X{...} composite
// literal pointer.
func isProtoPointerExpr(expr ast.Expr) bool {
	unaryExpr, isUnary := expr.(*ast.UnaryExpr)
	if !isUnary || unaryExpr.Op.String() != "&" {
		return false
	}

	lit, isLit := unaryExpr.X.(*ast.CompositeLit)
	if !isLit {
		return false
	}

	selExpr, isSel := lit.Type.(*ast.SelectorExpr)
	if !isSel {
		return false
	}

	pkgIdent, isPkg := selExpr.X.(*ast.Ident)

	return isPkg && pkgIdent.Name == linodev1Prefix
}

// classifySuccessSurface classifies the write/read success-path surface: every
// tool routed through its handler's proto/legacy classification.
func classifySuccessSurface(tools []string, nameToHandler map[string]string, graph packageCallGraph) map[string]string {
	result := make(map[string]string, len(tools))

	for _, toolName := range tools {
		handlerName, found := nameToHandler[toolName]
		if !found {
			fmt.Fprintf(os.Stderr, "no handler found for tool %q\n", toolName)

			result[toolName] = classifyReview

			continue
		}

		result[toolName] = classify(handlerName, graph, make(map[string]bool))
	}

	return result
}

// classifyInputSurface classifies the input-schema surface: every tool routed
// through its factory's constructor form (generated vs hand).
func classifyInputSurface(tools []string, files []*ast.File, consts map[string]string, graph packageCallGraph) map[string]string {
	nameToFactory := buildNameToFactory(files, consts)
	result := make(map[string]string, len(tools))

	for _, toolName := range tools {
		factoryName, found := nameToFactory[toolName]
		if !found {
			fmt.Fprintf(os.Stderr, "no factory found for tool %q\n", toolName)

			result[toolName] = classifyReview

			continue
		}

		result[toolName] = classifyInput(factoryName, graph)
	}

	return result
}

// buildNameToFactory maps each tool name to the func New*Tool that declares it.
// The input surface needs the factory rather than the handler buildNameToHandler
// resolves, because the constructor call it makes (mcp.NewTool vs
// mcp.NewToolWithRawSchema) is what decides hand-built vs proto-generated.
func buildNameToFactory(files []*ast.File, consts map[string]string) map[string]string {
	result := make(map[string]string, len(files))

	for _, astFile := range files {
		for _, decl := range astFile.Decls {
			funcDecl, isFuncDecl := decl.(*ast.FuncDecl)
			if !isFuncDecl || funcDecl.Name == nil || funcDecl.Body == nil {
				continue
			}

			if !strings.HasPrefix(funcDecl.Name.Name, "New") || !strings.HasSuffix(funcDecl.Name.Name, "Tool") {
				continue
			}

			toolName, _ := extractToolAndHandler(funcDecl, consts)
			if toolName == "" {
				continue
			}

			result[toolName] = funcDecl.Name.Name
		}
	}

	return result
}

// classifyInput returns "generated" when the factory (or a helper it calls)
// builds the tool's MCP input schema from the proto contract, "hand" otherwise.
func classifyInput(factoryName string, graph packageCallGraph) string {
	if walkInputReachability(factoryName, graph, make(map[string]bool)) {
		return classifyGenerated
	}

	return classifyHand
}

// walkInputReachability reports whether name (or a function it transitively
// calls) reaches a raw-schema sink. Only tool factories and the
// newSimpleProtoGetTool-style helpers call those, never a request handler, so
// walking the whole factory body including the handler closure cannot yield a
// false generated verdict for a hand-built factory.
func walkInputReachability(name string, graph packageCallGraph, visited map[string]bool) bool {
	if visited[name] {
		return false
	}

	visited[name] = true

	for _, rec := range graph[name] {
		switch rec.name {
		case sinkRawSchemaMCP, sinkRawSchema, sinkToolschemas:
			return true
		default:
			if walkInputReachability(rec.name, graph, visited) {
				return true
			}
		}
	}

	return false
}

// classify returns the success-path classification for handlerName. It computes
// both reachability flags in one walk, so call-graph iteration order never
// decides the result, then lets proto win: once a proto sink is reachable the
// legacy signals only ever sit on error or dry-run branches.
func classify(handlerName string, graph packageCallGraph, visited map[string]bool) string {
	reach := &reachability{}
	walkReachability(handlerName, graph, visited, reach)

	switch {
	case reach.proto:
		return classifyProto
	case reach.legacy:
		return classifyLegacy
	default:
		return classifyReview
	}
}

// reachability accumulates whether a proto sink or a legacy sink is reachable
// from a handler over the transitive call graph.
type reachability struct {
	proto  bool
	legacy bool
}

// walkReachability sets reach.proto / reach.legacy by walking the call graph
// from name; visited guards cycles. The destroy wrappers and inspected Success
// closures are terminal signals, never recursed into, so a wrapper's internal
// MarshalProtoToolResponse branch (which never fires for a map Success) cannot
// leak a false proto.
func walkReachability(name string, graph packageCallGraph, visited map[string]bool, reach *reachability) {
	if visited[name] {
		return
	}

	visited[name] = true

	for _, rec := range graph[name] {
		switch rec.name {
		case sinkProto, sinkProtoNulls, sinkProtoListNulls, sinkProtoJSON:
			reach.proto = true

		case sinkLegacy, sinkMarshalDestroy:
			reach.legacy = true

		case sinkWithID, sinkByTwoIDs, sinkByRegionLabel:
			// The by-ID, by-two-IDs, and by-region-label wrappers are
			// proto-routed only when the caller set SuccessProto on the literal
			// (successIsProto true); otherwise they build the legacy map. A bare
			// identifier reference (nil) cannot set the field, so it is legacy.
			switch {
			case rec.successIsProto != nil && *rec.successIsProto:
				reach.proto = true
			default:
				reach.legacy = true
			}

		case sinkRunDestructive:
			switch {
			case rec.successIsProto == nil:
				walkReachability(sinkRunDestructive, graph, visited, reach)
			case *rec.successIsProto:
				reach.proto = true
			default:
				reach.legacy = true
			}

		default:
			walkReachability(rec.name, graph, visited, reach)
		}
	}
}
