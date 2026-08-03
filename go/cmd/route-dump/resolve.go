package main

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// pathParam is what every path parameter collapses to. A declared route
// shape-matches parameters, so the name a route gives a segment never has to
// agree across languages and the resolver never has to recover it.
const pathParam = "{p}"

// resolveRoot resolves an expression that produces a whole endpoint. The false
// result means "not statically resolvable", which the caller reports rather
// than guessing at.
func (pkg *clientPackage) resolveRoot(expr ast.Expr, locals map[string]string) (string, bool) {
	switch node := expr.(type) {
	case *ast.ParenExpr:
		return pkg.resolveRoot(node.X, locals)
	case *ast.BasicLit:
		return stringLit(node)
	case *ast.Ident:
		return pkg.lookup(node.Name, locals)
	case *ast.BinaryExpr:
		return pkg.resolveConcat(node, locals)
	case *ast.CallExpr:
		return pkg.resolveCall(node, locals)
	}

	return "", false
}

// resolveConcat resolves a + chain. The left side carries the endpoint built so
// far and is resolved as a root; the right side is a single segment appended to
// it, where a value computed at request time is a path parameter.
func (pkg *clientPackage) resolveConcat(node *ast.BinaryExpr, locals map[string]string) (string, bool) {
	if node.Op != token.ADD {
		return "", false
	}

	left, leftOK := pkg.resolveRoot(node.X, locals)
	if !leftOK {
		return "", false
	}

	right, rightOK := pkg.resolveSegment(node.Y, locals)
	if !rightOK {
		return "", false
	}

	return left + right, true
}

// resolveSegment resolves an expression concatenated into a path. A value the
// client computes per request (url.PathEscape(...), a local holding an encoded
// id) is a path parameter by construction, so it collapses to pathParam rather
// than failing the endpoint around it.
//
// The trade-off is deliberate. A helper that returns a multi-segment subpath
// and resists resolution collapses to a single {p} here and yields a route that
// matches no declared route, which fails the gate out loud. That is
// the direction to be wrong in: the alternative, treating it as no evidence at
// all, is the silent miss this whole tool exists to remove.
func (pkg *clientPackage) resolveSegment(expr ast.Expr, locals map[string]string) (string, bool) {
	if value, ok := pkg.resolveRoot(expr, locals); ok {
		return value, true
	}

	switch expr.(type) {
	case *ast.Ident, *ast.CallExpr, *ast.SelectorExpr, *ast.IndexExpr:
		return pathParam, true
	}

	return "", false
}

// resolveCall resolves a call that produces an endpoint: a format call, or a
// helper in this package whose own return value resolves.
func (pkg *clientPackage) resolveCall(node *ast.CallExpr, locals map[string]string) (string, bool) {
	name, named := calleeName(node.Fun)
	if !named {
		return "", false
	}

	if isFormatCall(node.Fun) {
		return pkg.resolveFormat(node, locals)
	}

	decl, declared := pkg.function(name)
	if !declared {
		return "", false
	}

	return pkg.resolveResult(name, decl, node.Args, locals)
}

// isFormatCall reports whether the call is fmt.Sprintf, the one formatter the
// client builds endpoints with.
func isFormatCall(expr ast.Expr) bool {
	selector, isSelector := expr.(*ast.SelectorExpr)
	if !isSelector || selector.Sel.Name != "Sprintf" {
		return false
	}

	ident, isIdent := selector.X.(*ast.Ident)

	return isIdent && ident.Name == "fmt"
}

// resolveFormat resolves a Sprintf call into the path it formats.
//
// Both spellings appear in the client and mean the same route: the base
// constant concatenated into the format string, and the base constant passed as
// the first argument (fmt.Sprintf("%s/%s", endpointLKEVersions, id)). Resolving
// the arguments rather than blanking every verb is what keeps the second form
// from collapsing its own base path into a parameter.
func (pkg *clientPackage) resolveFormat(node *ast.CallExpr, locals map[string]string) (string, bool) {
	if len(node.Args) == 0 {
		return "", false
	}

	format, ok := pkg.resolveRoot(node.Args[0], locals)
	if !ok {
		return "", false
	}

	return pkg.expandVerbs(format, node.Args[1:], locals), true
}

// resolveResult resolves what a package helper returns for one call site, with
// its parameters bound to the resolved arguments.
//
// Every return has to agree once the query string is dropped. That is what lets
// withPaginationQuery resolve: it returns the endpoint with a query on one path
// and without one on the other, and both are the same route.
func (pkg *clientPackage) resolveResult(
	name string,
	decl *ast.FuncDecl,
	args []ast.Expr,
	locals map[string]string,
) (string, bool) {
	if decl.Body == nil || pkg.resolving[name] {
		return "", false
	}

	pkg.resolving[name] = true
	defer delete(pkg.resolving, name)

	bound := pkg.bindParams(decl, args, locals)

	var (
		resolved string
		found    bool
	)

	for _, result := range returnExprs(decl) {
		value, isKnown := pkg.resolveRoot(result, bound)
		if !isKnown {
			return "", false
		}

		value = stripQuery(value)
		if found && value != resolved {
			return "", false
		}

		resolved, found = value, true
	}

	return resolved, found
}

// bindParams binds a helper's parameters to the values its call site passes. An
// argument that does not resolve is left unbound, so inside the helper it
// behaves as a per-request value: a path parameter in segment position, and
// unresolvable in root position.
func (pkg *clientPackage) bindParams(
	decl *ast.FuncDecl,
	args []ast.Expr,
	locals map[string]string,
) map[string]string {
	bound := make(map[string]string, len(args))

	for index, name := range paramNames(decl) {
		if name == "" || index >= len(args) {
			continue
		}

		if value, ok := pkg.resolveRoot(args[index], locals); ok {
			bound[name] = value
		}
	}

	return bound
}

// returnExprs collects every single-value return in a function body, including
// the ones inside a nested literal. A helper that returns an endpoint has no
// nested literals today, and counting one would only make the resolver
// disagree with itself and give up, never invent a route.
func returnExprs(decl *ast.FuncDecl) []ast.Expr {
	var results []ast.Expr

	ast.Inspect(decl.Body, func(node ast.Node) bool {
		stmt, ok := node.(*ast.ReturnStmt)
		if ok && len(stmt.Results) == 1 {
			results = append(results, stmt.Results[0])
		}

		return true
	})

	return results
}

// lookup resolves an identifier against the enclosing function's locals first,
// then the package constants.
func (pkg *clientPackage) lookup(name string, locals map[string]string) (string, bool) {
	if value, ok := locals[name]; ok {
		return value, true
	}

	value, ok := pkg.constants[name]

	return value, ok
}

// stringLit reads an untyped string literal's value.
func stringLit(node *ast.BasicLit) (string, bool) {
	if node.Kind != token.STRING {
		return "", false
	}

	value, err := strconv.Unquote(node.Value)

	return value, err == nil
}

// expandVerbs replaces every format verb with what its argument contributes to
// the path. Flag, width, and precision characters sit between the % and the
// verb letter, so the scan runs to the first letter instead of assuming %s.
func (pkg *clientPackage) expandVerbs(format string, args []ast.Expr, locals map[string]string) string {
	var out strings.Builder

	// A verb is shorter than what replaces it, so the format's own length is a
	// floor rather than a target.
	out.Grow(len(format))

	rest := format

	var consumed int

	for {
		before, after, found := strings.Cut(rest, "%")

		out.WriteString(before)

		if !found {
			return out.String()
		}

		// %% is an escaped percent sign, not a verb, so it consumes no
		// argument.
		if strings.HasPrefix(after, "%") {
			out.WriteString("%")

			rest = after[1:]

			continue
		}

		out.WriteString(pkg.verbValue(args, consumed, locals))

		consumed++

		rest = trimVerb(after)
	}
}

// verbValue is what one verb expands to: its argument's value when that
// resolves to something static, and the path placeholder otherwise, which is
// what an id escaped at request time amounts to.
func (pkg *clientPackage) verbValue(args []ast.Expr, index int, locals map[string]string) string {
	if index >= len(args) {
		return pathParam
	}

	if value, ok := pkg.resolveRoot(args[index], locals); ok {
		return value
	}

	return pathParam
}

// trimVerb drops a verb's flag, width, and precision characters along with the
// verb letter itself, returning what follows.
func trimVerb(spec string) string {
	for index, char := range spec {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			return spec[index+1:]
		}
	}

	return ""
}

// stripQuery drops the query string. A route is its method and path; page and
// page_size ride in the query and belong to the pagination gate, not this one.
func stripQuery(endpoint string) string {
	path, _, _ := strings.Cut(endpoint, "?")

	return path
}
