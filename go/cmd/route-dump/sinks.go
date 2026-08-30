package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// methodParam and endpointParam name the parameters a request primitive carries
// the method and path in. Seeding by name rather than position means a reordered
// signature trips the hard fail instead of silently swapping every route.
const (
	methodParam   = "method"
	endpointParam = "endpoint"
)

// toolParam names the parameter a contract-driven primitive carries the tool
// name in. Such a function resolves its method and path from the proto contract
// at runtime, so its body holds no path: the evidence lives at its callers.
const toolParam = "tool"

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
// the function it wraps has. builders pairs each contract-driven primitive with
// the argument position its tool name arrives in.
type surface struct {
	issuers    map[string][]request
	builders   map[string]int
	routes     map[string]bool
	contracted map[contractedSite]bool
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
		builders:   pkg.routeBuilders(),
		routes:     map[string]bool{},
		contracted: map[contractedSite]bool{},
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
		Contracted: sortedCalls(found.contracted),
		Unresolved: sortedKeys(found.unresolved),
	}, nil
}

// seedIssuers builds the starting point: every function that puts a request on
// the wire from a method and an endpoint. Discovery is structural rather than a
// list of names: a body that constructs an http.Request plus the two route
// parameters qualifies, so a third such function is picked up without an edit.
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

// routeBuilders finds the primitives that take a tool name where the others
// take a method and a path, paired with the argument position that name arrives
// in. Discovery is structural for the same reason seedIssuers is, so a renamed
// parameter surfaces as call sites nothing can follow rather than as routes
// resolved from the wrong argument. Handing the request on is what separates one
// of these from a method that merely takes a tool name, and the test is the
// callee's own signature because the issuer set is still growing when this runs.
func (pkg *clientPackage) routeBuilders() map[string]int {
	builders := make(map[string]int, len(pkg.functions))

	// The set is closed under handoff rather than found in one sweep: an
	// exported primitive can pass its tool to another primitive instead of
	// straight to a function carrying a path. Only an exported function chains,
	// since an unexported forwarder is reachable only from this package, where
	// every tool it carries is already named at a call site this scan reads, so
	// one with no such call site is the hole the unresolved report exists to
	// show.
	for {
		var grew bool

		for name, decl := range pkg.functions {
			if _, known := builders[name]; known {
				continue
			}

			index, carriesTool := paramIndex(decl, toolParam)
			if !carriesTool || !pkg.handsOffRoute(decl, chainable(name, builders)) {
				continue
			}

			builders[name] = index
			grew = true
		}

		if !grew {
			return builders
		}
	}
}

// chainable is the builder set one function may hand its tool to: the whole set
// for an exported function, and none of it otherwise. See routeBuilders for why
// the two differ.
func chainable(name string, builders map[string]int) map[string]int {
	if ast.IsExported(name) {
		return builders
	}

	return nil
}

// handsOffRoute reports whether a body passes its work to a function that
// carries the route the rest of the way, which separates a route-resolving
// primitive from a function that merely takes a tool name.
func (pkg *clientPackage) handsOffRoute(decl *ast.FuncDecl, builders map[string]int) bool {
	if decl.Body == nil {
		return false
	}

	var handsOff bool

	ast.Inspect(decl.Body, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}

		name, named := calleeName(call.Fun)
		if !named {
			return true
		}

		if _, isBuilder := builders[name]; isBuilder {
			handsOff = true

			return true
		}

		if callee, declared := pkg.function(name); declared && takesRoute(callee) {
			handsOff = true
		}

		return true
	})

	return handsOff
}

// takesRoute reports whether a signature carries an endpoint, which is how a
// function that sends one is recognized. The method is deliberately not
// required alongside it: the list fetchers only ever GET and hold that method
// as their own constant, so requiring it would leave every call site they serve
// with no evidence at all. A tool name is not accepted in its place either,
// since routeBuilders already knows which callees are primitives.
func takesRoute(decl *ast.FuncDecl) bool {
	_, hasEndpoint := paramIndex(decl, endpointParam)

	return hasEndpoint
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
	for name, decl := range pkg.functions {
		// A contract-driven primitive looks its route up at runtime, so its own
		// body resolves to nothing and would report unresolved on every pass.
		// The evidence lives at its callers, which name the tool.
		if _, isBuilder := found.builders[name]; isBuilder {
			continue
		}

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

// trackAssign records string locals in source order, so a statement that
// rebuilds a local from its own previous value resolves against the value it
// held at that point. A local that stops resolving is dropped rather than left
// holding a stale one.
//
// A two-name assignment binds its first name: the typed lookup answers a tool
// beside an error, and no other two-result call resolves, since helpers only
// resolve through single-value returns.
func (pkg *clientPackage) trackAssign(node ast.Node, locals map[string]string) {
	assign, isAssign := node.(*ast.AssignStmt)
	if !isAssign || len(assign.Lhs) == 0 || len(assign.Rhs) != 1 {
		return
	}

	name, isIdent := assign.Lhs[0].(*ast.Ident)
	if !isIdent {
		return
	}

	value, resolved := pkg.resolveRoot(assign.Rhs[0], locals)

	// A compound assignment extends what the local already holds, which is how
	// the client appends a query string to an endpoint it already built.
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

// visitCall resolves one call: against the contract when the callee resolves
// its own route, and otherwise against every request the callee is known to
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

	if index, isBuilder := found.builders[name]; isBuilder {
		pkg.resolveContracted(call, caller, locals, index, found)

		return
	}

	// Copied before iterating: resolving a self-recursive call can append to
	// the same slice.
	for _, req := range slices.Clone(found.issuers[name]) {
		pkg.resolveRequest(req, call, caller, locals, found)
	}
}

// resolveContracted records what one contract-driven call site names: a tool,
// or the generated message the typed lookup answers a tool for. The tool has
// to be a literal or a constant that reduces to one, and the message a type
// written at the call site: a tool assembled at runtime leaves no route an
// offline reader can check, so it is reported as unresolved rather than
// dropped.
func (pkg *clientPackage) resolveContracted(
	call *ast.CallExpr,
	caller *ast.FuncDecl,
	locals map[string]string,
	index int,
	found *surface,
) {
	site := pkg.position(call) + " " + caller.Name.Name

	if index >= len(call.Args) {
		found.unresolved[site+": unnamed tool"] = true

		return
	}

	tool, resolved := pkg.resolveRoot(call.Args[index], locals)
	if !resolved || tool == "" {
		found.unresolved[site+": unnamed tool"] = true

		return
	}

	if message, typed := strings.CutPrefix(tool, messageMarker); typed {
		found.contracted[contractedSite{Message: message, Site: site}] = true

		return
	}

	found.contracted[contractedSite{Tool: tool, Site: site}] = true
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
// the request. Probing is what sees through a wrapper that only decorates the
// value, such as withPaginationQuery, whose query string is not part of the
// route.
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
// It is uppercase because the method resolver upper-cases what it resolves, so
// a lowercase marker would never match on the way back.
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

// sortedCalls renders the contracted call sites in source order, never nil, so
// an empty result encodes as [] rather than null.
func sortedCalls(set map[contractedSite]bool) []contractedSite {
	calls := make([]contractedSite, 0, len(set))
	for call := range set {
		calls = append(calls, call)
	}

	slices.SortFunc(calls, func(left, right contractedSite) int {
		if order := strings.Compare(left.Site, right.Site); order != 0 {
			return order
		}

		if order := strings.Compare(left.Tool, right.Tool); order != 0 {
			return order
		}

		return strings.Compare(left.Message, right.Message)
	})

	return calls
}
