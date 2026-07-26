package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// methodParam and endpointParam name the parameters a request primitive carries
// the HTTP method and the path in. Seeding by parameter name rather than by
// position means a reordered signature trips the hard fail instead of silently
// resolving every route with the arguments swapped.
const (
	methodParam   = "method"
	endpointParam = "endpoint"
)

// requestBuilder is the constructor prefix that marks a body as reaching the
// wire: http.NewRequest and http.NewRequestWithContext both start here.
const requestBuilder = "NewRequest"

const (
	// argResolved marks an argSource whose value is known.
	argResolved = -1
	// argUnknown marks an argSource the resolver could not follow.
	argUnknown = -2
)

// argSource is one argument of a request: a value already resolved, or the
// index of the parameter the enclosing function takes it from.
type argSource struct {
	value string
	param int
}

func resolvedArg(value string) argSource {
	return argSource{value: value, param: argResolved}
}

func (arg argSource) isResolved() bool {
	return arg.param == argResolved
}

func (arg argSource) isUnknown() bool {
	return arg.param == argUnknown
}

// request is one HTTP call a function can issue.
type request struct {
	method argSource
	path   argSource
}

// surface accumulates what the passes over the package find. grew is what
// drives another pass: a wrapper only becomes visible as a request issuer once
// the function it wraps has.
type surface struct {
	issuers    map[string][]request
	routes     map[string]bool
	unresolved map[string]bool
	grew       bool
}

// addIssuer records that a function issues a request whose method or path it
// takes from its own caller.
func (found *surface) addIssuer(name string, req request) {
	if slices.Contains(found.issuers[name], req) {
		return
	}

	found.issuers[name] = append(found.issuers[name], req)
	found.grew = true
}

// routeSurface resolves every route the package can build, along with the call
// sites it could not follow.
func (pkg *clientPackage) routeSurface() (dump, error) {
	issuers, err := pkg.seedIssuers()
	if err != nil {
		return dump{}, err
	}

	found := &surface{
		issuers:    issuers,
		routes:     map[string]bool{},
		unresolved: map[string]bool{},
	}

	for {
		found.grew = false

		pkg.expandIssuers(found)

		if !found.grew {
			break
		}
	}

	return dump{
		Routes:     sortedKeys(found.routes),
		Unresolved: sortedKeys(found.unresolved),
	}, nil
}

// seedIssuers builds the starting point: every function that puts a request on
// the wire from a method and an endpoint.
//
// Discovery is structural rather than a list of names. A function qualifies
// when its body constructs an http.Request and its signature declares the two
// parameters that carry the route. Two functions qualify today, the plain one
// and the variant that sets a Content-Type for the binary upload routes, and a
// third would be picked up here without an edit.
func (pkg *clientPackage) seedIssuers() (map[string][]request, error) {
	issuers := make(map[string][]request, len(pkg.functions))

	for name, decl := range pkg.functions {
		method, hasMethod := paramIndex(decl, methodParam)
		endpoint, hasEndpoint := paramIndex(decl, endpointParam)

		if !hasMethod || !hasEndpoint || !buildsRequest(decl) {
			continue
		}

		issuers[name] = []request{{
			method: argSource{param: method},
			path:   argSource{param: endpoint},
		}}
	}

	if len(issuers) == 0 {
		return nil, fmt.Errorf(
			"no function builds a request from %q and %q parameters: %w",
			methodParam, endpointParam, errRequestFuncMissing,
		)
	}

	return issuers, nil
}

// buildsRequest reports whether a body constructs an http.Request, which is
// what separates the functions that reach the wire from the ones that only pass
// an endpoint along.
func buildsRequest(decl *ast.FuncDecl) bool {
	if decl.Body == nil {
		return false
	}

	var builds bool

	ast.Inspect(decl.Body, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}

		if name, named := calleeName(call.Fun); named && strings.HasPrefix(name, requestBuilder) {
			builds = true
		}

		return true
	})

	return builds
}

// expandIssuers walks every function once, resolving each call it makes to a
// request-issuing function.
func (pkg *clientPackage) expandIssuers(found *surface) {
	for _, decl := range pkg.functions {
		pkg.walkCalls(decl, found)
	}
}

// walkCalls resolves one function's request call sites, tracking its string
// locals in source order as the walk reaches them.
func (pkg *clientPackage) walkCalls(decl *ast.FuncDecl, found *surface) {
	if decl.Body == nil {
		return
	}

	locals := map[string]string{}

	ast.Inspect(decl.Body, func(node ast.Node) bool {
		pkg.trackAssign(node, locals)

		if call, isCall := node.(*ast.CallExpr); isCall {
			pkg.visitCall(call, decl, locals, found)
		}

		return true
	})
}

// trackAssign records string locals as the walk reaches them, in source order,
// so a statement that rebuilds a local from its own previous value
// (endpoint = withPaginationQuery(endpoint, page, pageSize)) resolves against
// the value the local actually held at that point. A local that stops resolving
// is dropped rather than left holding an earlier value that no longer applies.
func (pkg *clientPackage) trackAssign(node ast.Node, locals map[string]string) {
	assign, isAssign := node.(*ast.AssignStmt)
	if !isAssign || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return
	}

	name, isIdent := assign.Lhs[0].(*ast.Ident)
	if !isIdent {
		return
	}

	value, resolved := pkg.resolveRoot(assign.Rhs[0], locals)

	// A compound assignment extends the value the local already holds, which is
	// how the client appends a query string to an endpoint it already built.
	// Without this the local would be replaced by the query alone.
	if resolved && assign.Tok == token.ADD_ASSIGN {
		previous, known := locals[name.Name]
		value, resolved = previous+value, known
	}

	if !resolved {
		delete(locals, name.Name)

		return
	}

	locals[name.Name] = value
}

// visitCall resolves one call against every request the callee is known to
// issue.
func (pkg *clientPackage) visitCall(
	call *ast.CallExpr,
	caller *ast.FuncDecl,
	locals map[string]string,
	found *surface,
) {
	name, ok := calleeName(call.Fun)
	if !ok {
		return
	}

	// Copied before iterating: resolving a self-recursive call can append to
	// the same slice.
	for _, req := range slices.Clone(found.issuers[name]) {
		pkg.resolveRequest(req, call, caller, locals, found)
	}
}

// resolveRequest translates one of the callee's requests into the caller's
// terms and files the outcome: a route when both parts are known, a new issuer
// when the caller supplies one of them from its own parameters, and an
// unresolved entry when neither holds.
func (pkg *clientPackage) resolveRequest(
	req request,
	call *ast.CallExpr,
	caller *ast.FuncDecl,
	locals map[string]string,
	found *surface,
) {
	method := substitute(req.method, call, caller, locals, pkg.resolveMethod)
	path := substitute(req.path, call, caller, locals, pkg.resolveRoot)

	if method.isResolved() && path.isResolved() {
		found.routes[method.value+" "+stripQuery(path.value)] = true

		return
	}

	if method.isUnknown() || path.isUnknown() {
		found.unresolved[describe(pkg.position(call), caller.Name.Name, method, path)] = true

		return
	}

	found.addIssuer(caller.Name.Name, request{method: method, path: path})
}

// substitute restates one argument of the callee's request in the caller's
// terms: a value when the caller passes something resolvable, a parameter index
// when it passes its own parameter straight through, unknown otherwise.
func substitute(
	arg argSource,
	call *ast.CallExpr,
	caller *ast.FuncDecl,
	locals map[string]string,
	resolve func(ast.Expr, map[string]string) (string, bool),
) argSource {
	if arg.isResolved() {
		return arg
	}

	if arg.param < 0 || arg.param >= len(call.Args) {
		return argSource{param: argUnknown}
	}

	actual := call.Args[arg.param]

	// Resolution runs first so a local shadowing a parameter name resolves to
	// the value it holds rather than to the parameter it hides.
	if value, ok := resolve(actual, locals); ok {
		return resolvedArg(value)
	}

	if index, ok := paramPassThrough(actual, caller, resolve); ok {
		return argSource{param: index}
	}

	return argSource{param: argUnknown}
}

// paramPassThrough reports which of the caller's parameters an argument carries
// to the request unchanged. It probes rather than pattern-matches: each
// parameter is bound to a unique marker and the argument is resolved against
// those bindings, so the marker coming back alone means that parameter reached
// the request.
//
// Probing is what sees through a wrapper that only decorates the value.
// listProtoElementsPaginated hands its endpoint to withPaginationQuery on the
// way in, and the query string it adds is not part of the route, so the marker
// survives and the parameter is still recognized as the path.
func paramPassThrough(
	actual ast.Expr,
	caller *ast.FuncDecl,
	resolve func(ast.Expr, map[string]string) (string, bool),
) (int, bool) {
	names := paramNames(caller)
	probes := make(map[string]string, len(names))

	for index, name := range names {
		if name != "" {
			probes[name] = probeMarker(index)
		}
	}

	value, ok := resolve(actual, probes)
	if !ok {
		return 0, false
	}

	value = stripQuery(value)

	for index, name := range names {
		if name != "" && value == probes[name] {
			return index, true
		}
	}

	return 0, false
}

// probeMarker is the stand-in value one parameter carries through resolution.
// It is uppercase because the method resolver upper-cases what it resolves, and
// a marker that changed shape on the way through would never match.
func probeMarker(index int) string {
	return fmt.Sprintf("\x00PARAM%d\x00", index)
}

// resolveMethod resolves an HTTP method argument: the http.MethodX constants
// the client uses, or a plain string literal.
func (pkg *clientPackage) resolveMethod(expr ast.Expr, locals map[string]string) (string, bool) {
	if selector, isSelector := expr.(*ast.SelectorExpr); isSelector {
		return methodFromSelector(selector)
	}

	value, ok := pkg.resolveRoot(expr, locals)
	if !ok {
		return "", false
	}

	return strings.ToUpper(value), true
}

func methodFromSelector(selector *ast.SelectorExpr) (string, bool) {
	ident, isIdent := selector.X.(*ast.Ident)
	if !isIdent || ident.Name != "http" || !strings.HasPrefix(selector.Sel.Name, "Method") {
		return "", false
	}

	return strings.ToUpper(strings.TrimPrefix(selector.Sel.Name, "Method")), true
}

// describe renders one unresolved call site, naming which part failed so the
// report says what to teach the resolver next.
func describe(position, caller string, method, path argSource) string {
	part := "path"

	switch {
	case method.isUnknown() && path.isUnknown():
		part = "method and path"
	case method.isUnknown():
		part = "method"
	}

	return fmt.Sprintf("%s %s: unresolved %s", position, caller, part)
}

// sortedKeys renders a set as a sorted slice, never nil, so an empty result
// encodes as [] rather than null.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}
