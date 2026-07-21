// Command write-proto-dump AST-analyzes the tool handlers in
// go/internal/tools and prints a JSON object on stdout mapping every mutating
// tool name (CapWrite, CapDestroy, CapAdmin) to its success-path
// classification: "proto", "legacy", or "review". With -surface read it
// classifies the read surface (CapRead) instead, using the same reachability
// analysis; the destroy-wrapper rules simply never fire for read handlers.
//
// The classification tells the proto-everywhere workstream which handlers
// already emit proto-canonical output and which still use the legacy
// map[string]any / MarshalToolResponse path.
//
// With -surface input it classifies the INPUT (request-schema) surface for
// every tool regardless of capability: "generated" when the tool's factory
// builds its MCP input schema from the proto contract (it reaches
// mcp.NewToolWithRawSchema / toolschemas.Schema), "hand" when it builds the
// schema from mcp.With* option builders. This drives the input-proto ratchet
// gate (scripts/verify_input_proto.py) the same way read/write drive theirs.
//
// Detection strategy (identifier-name reachability, no go/types):
//
//  1. Build the server with a throwaway config (same as parity-dump) and call
//     AllToolInfos() to get the authoritative mutating tool list.
//
//  2. Parse all non-test .go files in internal/tools with go/parser to build
//     a name to handler map: every factory function (func New*Tool) declares
//     the tool name (a "linode_"-shaped string literal or a string const, in
//     mcp.NewTool, newToolWithHandler, or a per-family constructor like
//     newDatabaseInstanceCreateTool) and wires a handler (a closure that
//     delegates to a top-level handle* function, or a bare handle* identifier).
//     String consts are resolved first, including "a" + "b" concatenations;
//     mcp.With* option calls are skipped so a param name like "linode_id" is
//     not mistaken for the tool name.
//
//  3. Build a call graph for the whole package: for every top-level func decl
//     (and its nested func-lit bodies), record all called function names.
//
//  4. Classify each handler by transitive reachability:
//     - Reaching "MarshalProtoToolResponse" or "MarshalProtoJSON" on any path
//     => proto. Passing MarshalProtoJSON into a reachable helper also counts.
//     This covers handlers that preserve documented JSON nulls after canonical
//     proto serialization before wrapping the MCP result.
//     - Reaching "RunDestructiveActionWithID" with a DestructiveActionByID
//     literal that sets SuccessProto => proto (the wrapper routes the body
//     through the proto marshaller); without SuccessProto it builds the legacy
//     id-echo map => legacy. A bare identifier reference is legacy.
//     - Reaching "RunDestructiveActionByTwoIDs" with a DestructiveActionByTwoIDs
//     literal that sets SuccessProto => proto (same routing as the by-ID
//     wrapper); without SuccessProto it builds the legacy id-echo map => legacy.
//     - Reaching "RunDestructiveActionByRegionLabel" with a
//     DestructiveActionByRegionLabel literal that sets SuccessProto => proto
//     (same routing as the by-ID wrapper); without SuccessProto it builds the
//     legacy {message, region, <key>} map => legacy.
//     - Reaching "RunDestructiveAction" directly with a DestructiveAction
//     literal whose Success closure returns a &linodev1.* proto pointer =>
//     proto; a map[string]any{} return => legacy.
//     - Reaching "MarshalToolResponse" or "marshalDestroySuccess" without
//     reaching a proto sink => legacy.
//     - If none of the above is reachable => review.
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
	sinkProto          = "MarshalProtoToolResponse"
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
	// proto.Message it forwards from its Success closure. Returning it counts as
	// proto-routed since marshalDestroySuccess routes any proto.Message through
	// the proto marshaller.
	successProtoParam = "successProto"

	classifyProto  = "proto"
	classifyLegacy = "legacy"
	classifyReview = "review"

	// surfaceInput selects the request-schema classification (generated vs
	// hand) over every tool, independent of the write/read success-path modes.
	surfaceInput = "input"

	// input-surface sinks: reaching either call marks a factory as building its
	// MCP input schema from the proto contract rather than mcp.With* builders.
	// callExprName renders mcp.NewToolWithRawSchema(...) as "mcp.NewToolWithRawSchema"
	// and toolschemas.Schema(...) as "toolschemas.Schema".
	sinkRawSchemaMCP = "mcp.NewToolWithRawSchema"
	sinkRawSchema    = "NewToolWithRawSchema"
	sinkToolschemas  = "toolschemas.Schema"

	classifyGenerated = "generated"
	classifyHand      = "hand"
)

// callRecord records a single outgoing call from a function. For
// RunDestructiveAction calls it also carries the result of inspecting the
// DestructiveAction literal's Success closure.
type callRecord struct {
	name string
	// successIsProto is non-nil only for RunDestructiveAction calls where the
	// Success closure was successfully inspected. true = returns a proto
	// pointer; false = returns a map.
	successIsProto *bool
}

// packageCallGraph maps function name to the list of outgoing calls it makes
// (direct only; transitive closure is resolved in classify).
type packageCallGraph map[string][]callRecord

func main() {
	surface := flag.String("surface", "write",
		"tool surface to classify: write (CapWrite/CapDestroy/CapAdmin), read (CapRead), meta (CapMeta), or input (all tools)")

	flag.Parse()

	toolsDir, err := locateToolsDir()
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

	files, err := parseToolsPackage(toolsDir, fset)
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

// buildToolSet builds the server with a throwaway config and returns the
// sorted names of all tools on the requested surface: "write" selects
// CapWrite/CapDestroy/CapAdmin, "read" selects CapRead, "meta" selects
// CapMeta, "input" selects every tool regardless of capability (the
// input-schema surface is capability-blind).
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
// factory that names its tool with a const identifier (for example the monitor
// tools' monitorServiceAlertDefinitionCreateToolName) can be resolved to the
// literal tool name the const holds.
func buildStringConstMap(files []*ast.File) map[string]string {
	// The tools package declares a few hundred string consts; a hint per file
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
			// graph credits the closure's calls to the enclosing New*Tool
			// function, so reachability runs through the factory itself.
			if handlerName == "" {
				handlerName = funcDecl.Name.Name
			}

			result[toolName] = handlerName
		}
	}

	return result
}

// toolNameArg resolves an argument to a tool name: a tool-name-shaped string
// literal, or a const identifier whose value is one. Returns "" otherwise. It
// insists on the tool-name shape so param names in mcp.WithString("label", ...)
// calls inside a factory body are not mistaken for the tool name.
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

// isToolName reports whether s has the shape of a registered tool name. Every
// tool name is prefixed "linode_" except the two meta tools, so the shape check
// keys on that prefix plus those names. This keeps param-name string literals
// (mcp.WithString("label", ...)) inside a factory from being read as the name.
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
// name and the handler function name that serves it. It matches any call in
// the factory that carries both a tool-name argument (string literal or const
// identifier) and a trailing handle* argument. That covers newToolWithHandler,
// the per-family constructors like newDatabaseInstanceCreateTool, and factories
// that assign the handler separately after an mcp.NewTool call.
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

		// Skip the mcp option builders (mcp.WithString("label", ...),
		// mcp.WithNumber("linode_id", ...)). Their first string arg is a param
		// name, not the tool name, and "linode_id" would otherwise be read as a
		// tool name because it shares the linode_ prefix.
		if strings.HasPrefix(callee, "mcp.") {
			return true
		}

		// Any other constructor call: look across its args for a tool name and a
		// handler. Handles newToolWithHandler and the per-family constructors.
		name, handler := toolAndHandlerFromArgs(callExpr.Args, consts)
		if name != "" {
			foundTool = name

			// A package-local factory (newProtoListTool and friends) RETURNS
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
// records MarshalProtoJSON when passed as a helper argument because the helper
// invokes that canonical serializer indirectly. For RunDestructiveAction calls
// it additionally inspects the DestructiveAction literal's Success closure to
// determine proto vs. map output.
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
// &DestructiveActionByID{...} literal that sets the SuccessProto field. When
// present the tool routes its success body through the proto-canonical
// marshaller (true); otherwise the wrapper builds the legacy id-echo map
// (false). Returns nil when the argument is not the expected literal.
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

// detectByTwoIDsSuccessProto inspects a RunDestructiveActionByTwoIDs call for a
// &DestructiveActionByTwoIDs{...} literal that sets the SuccessProto field. When
// present the tool routes its success body through the proto-canonical
// marshaller (true); otherwise the wrapper builds the legacy id-echo map
// (false). Returns nil when the argument is not the expected literal.
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

// detectByRegionLabelSuccessProto inspects a RunDestructiveActionByRegionLabel
// call for a &DestructiveActionByRegionLabel{...} literal that sets the
// SuccessProto field. When present the tool routes its success body through the
// proto-canonical marshaller (true); otherwise the wrapper builds the legacy
// {message, region, <key>} map (false). Returns nil when the argument is not the
// expected literal.
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
// successProto. A shared destroy helper that takes a proto.Message argument
// named successProto (so two callers can pass different concrete response
// messages) returns that value from its Success closure; the runtime
// marshalDestroySuccess routes any proto.Message through the proto marshaller,
// so this identifier return is proto-routed just like an inline literal.
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

// buildNameToFactory walks every func New*Tool declaration and records the tool
// name to the factory function that declares it. Unlike buildNameToHandler,
// which resolves the handler that serves the tool, the input surface needs the
// factory itself: its constructor call (mcp.NewTool vs mcp.NewToolWithRawSchema)
// is what decides whether the input schema is hand-built or proto-generated.
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
// calls) reaches a raw-schema sink: mcp.NewToolWithRawSchema or
// toolschemas.Schema. Only tool factories and the newSimpleProtoGetTool-style
// helpers call those, and no request handler does, so walking the whole factory
// body (handler closure included) cannot yield a false generated verdict for a
// hand-built factory.
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

// classify returns the success-path classification for handlerName.
//
// It computes two independent reachability flags over the whole transitive
// call graph, then applies proto-wins. Proto wins because reaching
// MarshalProtoToolResponse or MarshalProtoJSON anywhere means the success body
// is proto; the legacy signals (MarshalToolResponse, the destroy wrappers, a map
// Success closure) only ever sit on error or dry-run branches once proto is
// present. The two flags are computed together so iteration order never decides
// the result.
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
// from name. visited guards cycles. The destroy wrappers and inspected Success
// closures are treated as terminal legacy/proto signals and never recursed
// into, so a wrapper's internal MarshalProtoToolResponse branch (which never
// fires for a map Success) cannot leak a false proto.
func walkReachability(name string, graph packageCallGraph, visited map[string]bool, reach *reachability) {
	if visited[name] {
		return
	}

	visited[name] = true

	for _, rec := range graph[name] {
		switch rec.name {
		case sinkProto, sinkProtoJSON:
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
