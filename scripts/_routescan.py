"""Python route resolution for the route-evidence gate.

This sits beside the gate rather than inside it because the two jobs are
separate: the gate owns the contract, the baseline, and the language registry,
while this owns reading one Python source tree and saying which routes it can
build. go/cmd/route-dump is the same job for Go, written in Go because Go source
needs a Go parser.

Resolution mirrors that dumper so both languages are judged the same way. Find
the functions that build a request out of a method and an endpoint, then work
outward to their callers until nothing new resolves, rather than carrying a list
of wrappers that goes stale the first time someone adds one. A helper that only
decorates an endpoint is resolved through its own return values, which is how
paginated_path resolves to the path it was handed: the query string it appends
is not part of the route.

Names are what tie a call site to a declaration, and a name declared more than
once contributes the union of what its declarations issue. That union is what
makes the retry twin resolve: RetryableClient.get_raw forwards through a
callable it is handed rather than calling anything by name, so only the shared
name connects it to the plain get_raw the tool layer reaches the API through.
Resolution itself is stricter than issuing: two declarations of one name that
resolve to different paths resolve to nothing, so a coincidental name collision
costs evidence rather than inventing it.
"""

from __future__ import annotations

import ast
import re
from dataclasses import dataclass
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Callable, Iterator
    from pathlib import Path

# What a path parameter collapses to. tool-routes.txt shape-matches parameters,
# so the name a route gives a segment never has to agree across languages.
PATH_PARAM = "{p}"

# The HTTP library call that puts a request on the wire, taking the method
# first and the URL second.
_REQUEST_BUILDER = "request"

# The attribute holding the API root the client was configured with. It is the
# server, not the route, so it contributes nothing to a path and resolving it
# away is what lets a URL built as base + endpoint be read as its endpoint.
_BASE_URL_ATTR = "base_url"

# A resolved argument holds its value; an unknown one is a call site the scanner
# could not follow. Anything else is an index into the enclosing function's
# parameters, in call-argument order.
_ARG_RESOLVED = -1
_ARG_UNKNOWN = -2

if TYPE_CHECKING:
    # One argument of a request: its value paired with its parameter index.
    ArgSource = tuple[str, int]
    # One request a function can issue: its method argument, then its path.
    Request = tuple[ArgSource, ArgSource]
    FunctionDef = ast.FunctionDef | ast.AsyncFunctionDef
    Declarations = dict[str, list[tuple[str, FunctionDef]]]
    Resolver = Callable[[ast.expr, dict[str, str]], str | None]

_UNKNOWN: ArgSource = ("", _ARG_UNKNOWN)

_FUNCTION_NODES = (ast.FunctionDef, ast.AsyncFunctionDef)

# A parameter resolves to a marker rather than to a value, so an endpoint that
# reaches the wire unchanged can be recognized however many locals and helpers
# it passed through on the way. The marker is uppercase because method
# resolution upper-cases what it resolves, and one that changed shape in
# transit would never match. NUL cannot occur in source text.
_MARKER_PREFIX = "\x00PARAM"
_MARKER_SUFFIX = "\x00"
_MARKER_PATTERN = re.compile(rf"^{re.escape(_MARKER_PREFIX)}(\d+){_MARKER_SUFFIX}$")
_MARKER_ANYWHERE = re.compile(rf"{re.escape(_MARKER_PREFIX)}\d+{_MARKER_SUFFIX}")


def _marker(index: int) -> str:
    """The stand-in value one parameter carries through resolution."""
    return f"{_MARKER_PREFIX}{index}{_MARKER_SUFFIX}"


def _marker_index(value: str) -> int | None:
    """The parameter a value is, when it is a marker and nothing else."""
    match = _MARKER_PATTERN.match(value)

    return int(match.group(1)) if match else None


@dataclass(frozen=True)
class Evidence:
    """The routes one language can build, and the call sites it could not follow.

    Both halves are reported because both hide the same thing. A route with no
    evidence is either unimplemented or built in a shape the scanner cannot
    read, and an unresolved call site is where that shape lives.
    """

    routes: set[str]
    unresolved: list[str]


class ScannerError(RuntimeError):
    """The scanner could not start: no function builds a request at all.

    That is always a broken scanner, never a client with no routes, and it must
    not reach the gate as an empty result that reads like a contract break.
    """


def scan_python(root: Path, repo_root: Path) -> Evidence:
    """Resolve every route the Python tree under root can build."""
    return _Scan(root, repo_root).run()


def _display_path(path: Path, root: Path, repo_root: Path) -> str:
    """Repo-relative when the tree sits in the repo, root-relative otherwise.

    The fallback keeps the scanner usable against any tree (a test fixture, a
    checkout mounted elsewhere) instead of raising.
    """
    try:
        return path.relative_to(repo_root).as_posix()
    except ValueError:
        return path.relative_to(root).as_posix()


def _is_test_path(relative: Path) -> bool:
    """Whether a file is test code, which never counts as route evidence.

    The Go dumper skips _test.go for the same reason: a fixture endpoint in a
    test would otherwise stand in as proof the client builds that route.
    """
    if any(part in {"tests", "test"} for part in relative.parts):
        return True

    return relative.name.startswith("test_") or relative.name.endswith("_test.py")


def _param_names(func: FunctionDef) -> list[str]:
    """A function's parameters in call-argument order.

    The bound receiver is dropped because it never appears in the arguments at a
    call site, so a method's first argument is its second parameter.
    """
    names = [arg.arg for arg in (*func.args.posonlyargs, *func.args.args)]
    if names and names[0] in {"self", "cls"}:
        return names[1:]

    return names


def _callee_name(func: ast.expr) -> str | None:
    """Reduce a call's function expression to the declared name."""
    if isinstance(func, ast.Name):
        return func.id
    if isinstance(func, ast.Attribute):
        return func.attr

    return None


def _calls_request(func: FunctionDef) -> bool:
    """Whether a body puts a request on the wire.

    Only the tripwire uses this: a tree where nothing reaches the HTTP library
    is a scanner that has stopped working, not a client with no routes.
    """
    return any(
        isinstance(node, ast.Call) and _callee_name(node.func) == _REQUEST_BUILDER
        for node in ast.walk(func)
    )


def _strip_query(endpoint: str) -> str:
    """Drop the query string.

    A route is its method and path. page and page_size ride in the query and
    belong to the pagination gate, not this one.
    """
    return endpoint.split("?", 1)[0]


def _returned(func: FunctionDef) -> list[ast.expr]:
    """Every value a function returns."""
    return [
        node.value
        for node in ast.walk(func)
        if isinstance(node, ast.Return) and node.value is not None
    ]


def _ordered_nodes(node: ast.AST) -> Iterator[ast.AST]:
    """Every node below one, depth first in source order.

    Source order is what lets a local be read with the value it held at that
    point rather than its last value anywhere in the function. Nested functions
    are descended into on purpose: a handler builds its endpoint in the outer
    body and calls the client from an inner closure, so the two share a scope
    here as they do at runtime.
    """
    for child in ast.iter_child_nodes(node):
        yield child
        yield from _ordered_nodes(child)


class _Expressions:
    """Resolves what an expression contributes to a path."""

    def __init__(self, declarations: Declarations) -> None:
        self.declarations = declarations
        # Guards a helper that reaches itself while its return is resolved.
        self.resolving: set[str] = set()

    def root(self, node: ast.expr, known: dict[str, str]) -> str | None:
        """Resolve an expression that produces a whole endpoint, or None."""
        if isinstance(node, ast.Constant):
            return node.value if isinstance(node.value, str) else None
        if isinstance(node, ast.Name):
            return known.get(node.id)
        if isinstance(node, ast.Attribute):
            return "" if node.attr == _BASE_URL_ATTR else None

        return self._composite(node, known)

    def _composite(self, node: ast.expr, known: dict[str, str]) -> str | None:
        """Resolve the shapes that are built out of other expressions."""
        if isinstance(node, ast.JoinedStr):
            return self._fstring(node, known)
        if isinstance(node, ast.BinOp) and isinstance(node.op, ast.Add):
            return self._concat(node, known)
        if isinstance(node, ast.IfExp):
            return self._branches(node, known)
        if isinstance(node, ast.Call):
            return self._call(node, known)

        return None

    def _branches(self, node: ast.IfExp, known: dict[str, str]) -> str | None:
        """Resolve a conditional expression.

        Both branches have to name the same route once the query string is
        dropped, which is what "the path with a query when there is one"
        amounts to.
        """
        body = self.root(node.body, known)
        other = self.root(node.orelse, known)
        if body is None or other is None or _strip_query(body) != _strip_query(other):
            return None

        return _strip_query(body)

    def segment(self, node: ast.expr, known: dict[str, str]) -> str | None:
        """Resolve an expression interpolated into a path.

        A value computed per request (quote(str(id)), a local holding an encoded
        id) is a path parameter by construction, so it collapses to the
        placeholder rather than failing the endpoint around it. A helper
        returning a multi-segment subpath would collapse the same way and yield
        a route matching nothing in the contract, which fails the gate out loud.
        That is the direction to be wrong in.
        """
        resolved = self.root(node, known)
        if resolved is not None:
            return resolved

        if isinstance(node, ast.Name | ast.Call | ast.Attribute | ast.Subscript):
            return PATH_PARAM

        return None

    def method(self, node: ast.expr, known: dict[str, str]) -> str | None:
        """Resolve an HTTP method argument."""
        resolved = self.root(node, known)

        return resolved.upper() if resolved is not None else None

    def _concat(self, node: ast.BinOp, known: dict[str, str]) -> str | None:
        """Resolve a + chain: the left side carries the endpoint built so far,
        the right side is one segment appended to it.
        """
        left = self.root(node.left, known)
        right = self.segment(node.right, known)
        if left is None or right is None:
            return None

        return left + right

    def _fstring(self, node: ast.JoinedStr, known: dict[str, str]) -> str | None:
        """Resolve an f-string: literal parts as themselves, interpolations as
        whatever they hold.
        """
        parts: list[str] = []
        for value in node.values:
            if isinstance(value, ast.FormattedValue):
                resolved = self.segment(value.value, known)
            else:
                resolved = self.root(value, known)

            if resolved is None:
                return None

            parts.append(resolved)

        return "".join(parts)

    def _call(self, node: ast.Call, known: dict[str, str]) -> str | None:
        """Resolve a call to a helper that returns an endpoint."""
        name = _callee_name(node.func)
        if name is None or name in self.resolving:
            return None

        self.resolving.add(name)
        try:
            return self._declared_result(name, node, known)
        finally:
            self.resolving.discard(name)

    def _declared_result(
        self, name: str, node: ast.Call, known: dict[str, str]
    ) -> str | None:
        """What every declaration of a name returns for this call site, when
        they agree.
        """
        results = {
            self._result(func, node.args, known)
            for _, func in self.declarations.get(name, ())
        }
        if len(results) != 1:
            return None

        return results.pop()

    def _result(
        self, func: FunctionDef, args: list[ast.expr], known: dict[str, str]
    ) -> str | None:
        """What one helper returns for one call site, with its parameters bound
        to the resolved arguments.

        Every return has to agree once the query string is dropped, which is
        what lets paginated_path resolve: it returns the path with a query on
        one branch and without one on the other, and both are the same route.
        """
        bound = self._bind(func, args, known)

        resolved: set[str] = set()
        for value in _returned(func):
            result = self.root(value, bound)
            if result is None:
                return None

            resolved.add(_strip_query(result))

        if len(resolved) != 1:
            return None

        return resolved.pop()

    def _bind(
        self, func: FunctionDef, args: list[ast.expr], known: dict[str, str]
    ) -> dict[str, str]:
        """Bind a helper's parameters to the values its call site passes.

        An argument that does not resolve is left unbound, so inside the helper
        it behaves as a per-request value.
        """
        bound: dict[str, str] = {}
        for index, name in enumerate(_param_names(func)):
            if index >= len(args):
                break

            value = self.root(args[index], known)
            if value is not None:
                bound[name] = value

        return bound


class _Scan:
    """One structural pass over a Python tree, resolving outward from the
    request primitives until no new issuer appears.
    """

    def __init__(self, root: Path, repo_root: Path) -> None:
        self.declarations = self._index(root, repo_root)
        self.expressions = _Expressions(self.declarations)
        self.issuers = self._seed()
        self.routes: set[str] = set()
        self.unresolved: set[str] = set()
        self.grew = False

    def run(self) -> Evidence:
        while True:
            self.grew = False
            self._expand()
            if not self.grew:
                break

        return Evidence(routes=self.routes, unresolved=sorted(self.unresolved))

    def _index(self, root: Path, repo_root: Path) -> Declarations:
        """Every named declaration by name, with the file it came from.

        Only module-level and class-level functions are indexed. A nested
        function is walked as part of the body that encloses it, both because it
        shares that scope and because nothing calls it by name.
        """
        found: Declarations = {}
        for path in sorted(root.rglob("*.py")):
            relative = path.relative_to(root)
            if any(part.startswith(".") for part in relative.parts) or _is_test_path(
                relative
            ):
                continue

            display = _display_path(path, root, repo_root)
            tree = ast.parse(path.read_text(encoding="utf-8"))
            for func in _declared_functions(tree):
                found.setdefault(func.name, []).append((display, func))

        return found

    def _seed(self) -> dict[str, set[Request]]:
        """The starting point: the HTTP library call itself, method first and
        URL second.

        Python seeds one hop closer to the wire than the Go dumper does. Go
        stops at the function that constructs an http.Request, because the URL
        arriving there has been through url.Parse; here the client joins its
        base URL to the endpoint with a plain concatenation that resolution
        reads straight through. Seeding at the call is what covers the two
        thumbnail methods, which send raw PNG bytes and skip the shared
        make_request path the way Go's second primitive does.
        """
        if not any(
            _calls_request(func)
            for declarations in self.declarations.values()
            for _, func in declarations
        ):
            msg = f"no call to {_REQUEST_BUILDER!r} anywhere in the tree"
            raise ScannerError(msg)

        return {_REQUEST_BUILDER: {(("", 0), ("", 1))}}

    def _expand(self) -> None:
        for name, declarations in self.declarations.items():
            for display, func in declarations:
                self._walk_calls(name, display, func)

    def _walk_calls(self, name: str, display: str, func: FunctionDef) -> None:
        """Resolve one function's request call sites, tracking its string locals
        in source order as the walk reaches them.
        """
        # Parameters start bound to markers so an endpoint the function was
        # handed stays recognizable wherever it surfaces later.
        known = {
            param: _marker(index)
            for index, param in enumerate(_param_names(func))
            if param
        }
        for node in _ordered_nodes(func):
            self._track_assign(node, known)
            if isinstance(node, ast.Call):
                self._visit_call(node, name, display, known)

    def _visit_call(
        self,
        call: ast.Call,
        caller: str,
        display: str,
        known: dict[str, str],
    ) -> None:
        callee = _callee_name(call.func)
        if callee is None:
            return

        # Copied before iterating: resolving a self-recursive call can add to
        # the same set.
        for req in list(self.issuers.get(callee, ())):
            self._resolve_request(req, call, caller, display, known)

    def _resolve_request(
        self,
        req: Request,
        call: ast.Call,
        caller: str,
        display: str,
        known: dict[str, str],
    ) -> None:
        method = self._substitute(req[0], call, known, self.expressions.method)
        path = self._substitute(req[1], call, known, self.expressions.root)

        if method[1] == _ARG_RESOLVED and path[1] == _ARG_RESOLVED:
            self._record(method[0], path[0], call, caller, display)

            return

        if method[1] == _ARG_UNKNOWN or path[1] == _ARG_UNKNOWN:
            self.unresolved.add(_describe(call, caller, display, method, path))

            return

        self._add_issuer(caller, (method, path))

    def _record(
        self, method: str, path: str, call: ast.Call, caller: str, display: str
    ) -> None:
        """File a fully resolved request as a route.

        A parameter interpolated into a path is a path parameter by
        construction, so a marker left in the value collapses to the placeholder
        here. A marker left in the method is a method the caller derives rather
        than names, which resolution cannot carry forward.

        A path that does not start with a slash is a resolution that went wrong
        rather than a route, so it is reported instead of recorded.
        """
        if _MARKER_ANYWHERE.search(method):
            self.unresolved.add(f"{display}:{call.lineno} {caller}: unresolved method")

            return

        route = _strip_query(_MARKER_ANYWHERE.sub(PATH_PARAM, path))
        if not route.startswith("/"):
            self.unresolved.add(
                f"{display}:{call.lineno} {caller}: resolved to a non-path {route!r}"
            )

            return

        self.routes.add(f"{method} {route}")

    def _substitute(
        self,
        arg: ArgSource,
        call: ast.Call,
        known: dict[str, str],
        resolve: Resolver,
    ) -> ArgSource:
        """Restate one argument of the callee's request in the caller's terms.

        The caller's own parameters resolve to markers, so an argument that
        arrives as nothing but a marker is one the caller passes through, however
        many locals and helpers it crossed on the way. A value that merely
        contains a marker is left for _record, which knows whether an
        interpolated parameter is a path segment or a method it cannot follow.
        """
        if arg[1] == _ARG_RESOLVED:
            return arg

        if arg[1] < 0 or arg[1] >= len(call.args):
            return _UNKNOWN

        resolved = resolve(call.args[arg[1]], known)
        if resolved is None:
            return _UNKNOWN

        index = _marker_index(_strip_query(resolved))
        if index is not None:
            return ("", index)

        return (resolved, _ARG_RESOLVED)

    def _track_assign(self, node: ast.AST, known: dict[str, str]) -> None:
        """Record string locals as the walk reaches them.

        A compound assignment extends the value the local already holds, which
        is how a handler appends a query string to an endpoint it already built.
        A local that stops resolving is dropped rather than left holding an
        earlier value that no longer applies.
        """
        target, value = self._assigned(node, known)
        if target is None:
            return

        if value is None:
            known.pop(target, None)

            return

        known[target] = value

    def _assigned(
        self, node: ast.AST, known: dict[str, str]
    ) -> tuple[str | None, str | None]:
        """The name and resolved value one assignment binds."""
        if isinstance(node, ast.Assign):
            if len(node.targets) != 1 or not isinstance(node.targets[0], ast.Name):
                return (None, None)

            return (node.targets[0].id, self.expressions.root(node.value, known))

        if isinstance(node, ast.AugAssign):
            return self._augmented(node, known)

        return (None, None)

    def _augmented(
        self, node: ast.AugAssign, known: dict[str, str]
    ) -> tuple[str | None, str | None]:
        if not isinstance(node.target, ast.Name) or not isinstance(node.op, ast.Add):
            return (None, None)

        addition = self.expressions.root(node.value, known)
        previous = known.get(node.target.id)
        if addition is None or previous is None:
            return (node.target.id, None)

        return (node.target.id, previous + addition)

    def _add_issuer(self, name: str, req: Request) -> None:
        pending = self.issuers.setdefault(name, set())
        if req not in pending:
            pending.add(req)
            self.grew = True


def _declared_functions(tree: ast.Module) -> Iterator[FunctionDef]:
    """Module-level and class-level function declarations."""
    for node in tree.body:
        if isinstance(node, _FUNCTION_NODES):
            yield node
        elif isinstance(node, ast.ClassDef):
            for member in node.body:
                if isinstance(member, _FUNCTION_NODES):
                    yield member


def _describe(
    call: ast.Call, caller: str, display: str, method: ArgSource, path: ArgSource
) -> str:
    """Render one unresolved call site, naming which part failed."""
    if method[1] == _ARG_UNKNOWN and path[1] == _ARG_UNKNOWN:
        part = "method and path"
    elif method[1] == _ARG_UNKNOWN:
        part = "method"
    else:
        part = "path"

    return f"{display}:{call.lineno} {caller}: unresolved {part}"
