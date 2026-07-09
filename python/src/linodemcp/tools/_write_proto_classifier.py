"""Static AST classifier for tool proto vs. legacy success paths.

Analyzes handler functions for every tool on a surface (default: the mutating
surface, capability Write/Destroy/Admin; ``surface="read"`` selects capability
Read) in the tools package without importing or executing any handler code.
No network calls are made.

Classification rules (per handler, via transitive intra-package call graph):

- "proto"  -- the handler (or a helper it calls) reaches
              ``serialize_api_response`` or ``serialize_list_response`` on
              any reachable code path.
- "legacy" -- no such reach; the success path builds a plain dict or calls
              ``execute_tool`` / ``execute_dry_run`` with a hand-built dict
              callback.
- "review" -- ``handle_<tool>`` was not found in the tools package; written
              to stderr.

``surface="input"`` classifies the INPUT (request-schema) surface instead,
over every tool regardless of capability. It inspects each
``create_<tool>_tool`` factory's ``Tool(...)`` constructor:

- "generated" -- ``inputSchema=schema(...)``; the MCP input schema is loaded
                 from the proto contract.
- "hand"      -- ``inputSchema={...}``; the schema is a hand-built dict.
- "review"    -- ``create_<tool>_tool`` was not found; written to stderr.

Usage::

    from linodemcp.tools._write_proto_classifier import classify
    result = classify()          # dict[str, str]
    result = classify("input")   # dict[str, str] over every tool

    python -m linodemcp.tools._write_proto_classifier         # write surface
    python -m linodemcp.tools._write_proto_classifier input   # input surface
"""

from __future__ import annotations

import ast
import json
import sys
from pathlib import Path

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

_TOOLS_DIR = Path(__file__).resolve().parent

_CAPABILITIES_PATH = _TOOLS_DIR.parents[3] / "docs" / "tools-capabilities.txt"

_PROTO_SINKS: frozenset[str] = frozenset(
    {"serialize_api_response", "serialize_list_response", "proto_to_canonical_dict"}
)

_MUTATING_CAPS: frozenset[str] = frozenset({"Write", "Destroy", "Admin"})

_READ_CAPS: frozenset[str] = frozenset({"Read"})

_META_CAPS: frozenset[str] = frozenset({"Meta"})

_SURFACE_CAPS: dict[str, frozenset[str]] = {
    "write": _MUTATING_CAPS,
    "read": _READ_CAPS,
    "meta": _META_CAPS,
}

# The input surface is capability-blind: it classifies every tool's factory.
_INPUT_SURFACE = "input"

# The proto-schema loader call name. A create_<tool>_tool factory that sets
# inputSchema=schema(...) builds its schema from the proto contract (generated);
# a dict-literal inputSchema is hand-built.
_SCHEMA_LOADER = "schema"

# A capabilities file line has exactly two tab-separated fields.
_CAP_FIELD_COUNT = 2


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _load_surface_tools(caps: frozenset[str]) -> list[str]:
    """Return sorted list of tool names whose capability is in *caps*."""
    tools: list[str] = []
    for raw_line in _CAPABILITIES_PATH.read_text(encoding="utf-8").splitlines():
        stripped = raw_line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        parts = stripped.split("\t")
        if len(parts) != _CAP_FIELD_COUNT:
            continue
        tool_name, cap = parts[0].strip(), parts[1].strip()
        if cap in caps:
            tools.append(tool_name)
    return sorted(tools)


def _load_all_tools() -> list[str]:
    """Return every tool name in the capabilities file, regardless of capability."""
    tools: list[str] = []
    for raw_line in _CAPABILITIES_PATH.read_text(encoding="utf-8").splitlines():
        stripped = raw_line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        parts = stripped.split("\t")
        if len(parts) != _CAP_FIELD_COUNT:
            continue
        tools.append(parts[0].strip())
    return sorted(tools)


def _is_schema_loader_call(value: ast.expr) -> bool:
    """Return True when *value* is a call to the proto-schema loader ``schema``."""
    if not isinstance(value, ast.Call):
        return False
    func = value.func
    if isinstance(func, ast.Name):
        return func.id == _SCHEMA_LOADER
    if isinstance(func, ast.Attribute):
        return func.attr == _SCHEMA_LOADER
    return False


def _input_status(create_node: ast.AST) -> str:
    """Classify a create_<tool>_tool factory as "generated" or "hand".

    "generated" means the ``Tool(...)`` constructor sets its ``inputSchema`` from
    the proto-schema loader (``inputSchema=schema(...)``); "hand" means the
    schema is a dict literal or any other non-loader expression. As a fallback
    for a factory that builds the schema in a local variable first, a call to
    the loader anywhere in the factory also counts as generated.
    """
    for node in ast.walk(create_node):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        is_tool = (isinstance(func, ast.Name) and func.id == "Tool") or (
            isinstance(func, ast.Attribute) and func.attr == "Tool"
        )
        if not is_tool:
            continue
        for keyword in node.keywords:
            if keyword.arg != "inputSchema":
                continue
            return "generated" if _is_schema_loader_call(keyword.value) else "hand"

    if _SCHEMA_LOADER in _collect_direct_calls(create_node):
        return "generated"
    return "hand"


def _collect_direct_calls(node: ast.AST) -> set[str]:
    """Return called names reachable inside *node*.

    Walks the entire subtree so nested defs, lambdas, comprehensions, and
    ``async for`` bodies are all included.  Only the final name component is
    recorded (e.g. ``foo.bar()`` records ``"bar"``; ``baz()`` records ``"baz"``).
    """
    called: set[str] = set()
    for child in ast.walk(node):
        if not isinstance(child, ast.Call):
            continue
        func = child.func
        if isinstance(func, ast.Name):
            called.add(func.id)
        elif isinstance(func, ast.Attribute):
            called.add(func.attr)
    return called


def _build_package_index(
    tools_dir: Path,
) -> dict[str, ast.FunctionDef | ast.AsyncFunctionDef]:
    """Parse every *.py in tools_dir and return a map of function name -> AST node.

    When two files define the same top-level function name the later one wins;
    in practice tool handler names are unique across the package.
    """
    index: dict[str, ast.FunctionDef | ast.AsyncFunctionDef] = {}
    for py_file in sorted(tools_dir.glob("*.py")):
        try:
            source = py_file.read_text(encoding="utf-8")
        except OSError:
            continue
        try:
            tree = ast.parse(source, filename=str(py_file))
        except SyntaxError:
            continue
        for child in ast.iter_child_nodes(tree):
            if isinstance(child, (ast.FunctionDef, ast.AsyncFunctionDef)):
                index[child.name] = child
    return index


def _reaches_proto_sink(
    start: str,
    package_index: dict[str, ast.FunctionDef | ast.AsyncFunctionDef],
    visited: set[str] | None = None,
) -> bool:
    """Return True if *start* or any intra-package helper it calls hits a sink.

    Uses depth-first traversal with a visited set to guard against cycles.
    """
    if visited is None:
        visited = set()
    if start in visited:
        return False
    visited.add(start)

    node = package_index.get(start)
    if node is None:
        return False

    direct = _collect_direct_calls(node)

    if direct & _PROTO_SINKS:
        return True

    # Recurse into intra-package helpers.
    for callee in direct:
        if callee not in package_index:
            continue
        if _reaches_proto_sink(callee, package_index, visited):
            return True

    return False


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------


def _classify_input() -> dict[str, str]:
    """Classify every tool's create_<tool>_tool factory as "generated"/"hand".

    Tools whose ``create_<name>_tool`` factory cannot be found are classified
    "review" and a warning is written to stderr.
    """
    tools = _load_all_tools()
    package_index = _build_package_index(_TOOLS_DIR)

    result: dict[str, str] = {}
    for tool_name in tools:
        factory_name = f"create_{tool_name}_tool"
        node = package_index.get(factory_name)
        if node is None:
            sys.stderr.write(f"WARNING: factory not found: {factory_name}\n")
            result[tool_name] = "review"
            continue
        result[tool_name] = _input_status(node)

    return result


def classify(surface: str = "write") -> dict[str, str]:
    """Classify every tool on *surface*.

    ``surface="write"`` covers capability Write/Destroy/Admin and ``"read"``
    covers capability Read, each classifying the handler success path as
    "proto"/"legacy"/"review". ``surface="input"`` covers every tool and
    classifies its factory's input schema as "generated"/"hand"/"review".
    Returns a dict mapping tool name to its classification.
    """
    if surface == _INPUT_SURFACE:
        return _classify_input()

    caps = _SURFACE_CAPS.get(surface)
    if caps is None:
        msg = f"unknown surface {surface!r} (want 'write', 'read', or 'input')"
        raise ValueError(msg)

    surface_tools = _load_surface_tools(caps)
    package_index = _build_package_index(_TOOLS_DIR)

    result: dict[str, str] = {}
    for tool_name in surface_tools:
        handler_name = f"handle_{tool_name}"
        if handler_name not in package_index:
            sys.stderr.write(f"WARNING: handler not found: {handler_name}\n")
            result[tool_name] = "review"
            continue

        if _reaches_proto_sink(handler_name, package_index):
            result[tool_name] = "proto"
        else:
            result[tool_name] = "legacy"

    return result


# ---------------------------------------------------------------------------
# __main__ entry point
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    _surface = sys.argv[1] if len(sys.argv) > 1 else "write"
    try:
        _output = classify(_surface)
    except Exception as exc:  # pragma: no cover
        sys.stderr.write(f"ERROR: {exc}\n")
        sys.exit(1)
    sys.stdout.write(json.dumps(_output, indent=2, sort_keys=True) + "\n")
