#!/usr/bin/env python3
"""Offline gate: every route proto/ declares must still exist upstream.

The repo half of the TechDocs loop. The scraping half runs weekly
(.github/workflows/techdocs-drift.yml) and lands a reviewed snapshot,
docs/contracts/api-techdocs-routes-baseline.txt. This gate never fetches: it
reads that snapshot and the local proto tree, so `make check` stays offline.

It answers what `make wire-breaking` cannot. wire-breaking compares proto/
against a proto-derived image and catches a wire break WE made; this snapshot
is TechDocs-derived, so it catches a route THEY dropped, renamed, or moved
between API surfaces.

Two refusals, both by name:

    dropped     a tool_route the snapshot no longer states, which means the
                route the tool calls is gone from the documented API
    surface     a tool declaring v4 whose route the snapshot restricts to
                v4beta, or the reverse, which means upstream withdrew the
                surface the tool is built on

Deprecated routes are counted and named rather than failed; REQ-D7 owns that
worklist. A documented route no tool declares is the comparator's
route_missing_from_proto, not this gate's business.

Scope comes from buf.yaml's modules, so a module added later is walked without
touching this file.

Stdlib only. Run via `make techdocs-routes` (in `make check`, and so the
pre-push hook and the CI gate on every branch).

Usage: verify_techdocs_routes.py
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import NamedTuple

import _hardgate

_REPO_ROOT = Path(__file__).resolve().parents[1]
_SNAPSHOT = _REPO_ROOT / "docs" / "contracts" / "api-techdocs-routes-baseline.txt"
_WORKSPACE = _REPO_ROOT / "buf.yaml"

_MESSAGE_RE = re.compile(r"^message (\S+) \{$")
_ROUTE_OPEN_RE = re.compile(r"^\s*option \(linode\.mcp\.v1\.tool_route\) = \{$")
_ROUTE_ENTRY_RE = re.compile(r'^\s*(tool|method|path): "([^"]*)"$')
_BETA_RE = re.compile(
    r"^\s*option \(linode\.mcp\.v1\.tool_api_surface\) = API_SURFACE_V4BETA;$"
)
_PLACEHOLDER_RE = re.compile(r"\{[^{}]*\}")
_MODULE_RE = re.compile(r"^\s*-\s*path:\s*(\S+)\s*$")

# The surfaces a snapshot line can state, and which of them a tool built on the
# v4 or the v4beta surface can still be called through.
_SERVES = {
    "v4": {"both", "v4_only"},
    "v4beta": {"both", "v4beta_only"},
}


class Declared(NamedTuple):
    """One tool_route as the proto tree states it."""

    tool: str
    method: str
    shape: str
    surface: str
    file: str
    line: int


class Route(NamedTuple):
    """One route as the reviewed snapshot states it."""

    surface: str
    deprecated: bool


def modules(workspace: Path) -> list[str]:
    """The module paths buf.yaml declares, which is the tree's own scope."""
    paths: list[str] = []
    inside = False
    for raw in workspace.read_text(encoding="utf-8").splitlines():
        if not raw.startswith(" "):
            inside = raw.strip() == "modules:"
            continue
        match = _MODULE_RE.match(raw)
        if inside and match:
            paths.append(match.group(1))
    return paths


def snapshot_routes(path: Path) -> dict[tuple[str, str], Route]:
    """(METHOD, path shape) to the surface and status TechDocs state."""
    routes: dict[tuple[str, str], Route] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if not stripped or stripped.startswith("#"):
            continue
        parts = stripped.split()
        if len(parts) != 4:
            continue
        surface = parts[2].removeprefix("surface=")
        status = parts[3].removeprefix("status=")
        routes[(parts[0], parts[1])] = Route(surface, status == "deprecated")
    return routes


def path_shape(path: str) -> str:
    """Collapse placeholder names, which the two sides spell differently."""
    return _PLACEHOLDER_RE.sub("{}", path)


def _entries(lines: list[str], start: int) -> dict[str, str]:
    """The tool, method, and path a route option states, empty when malformed."""
    found: dict[str, str] = {}
    for raw in lines[start + 1 : start + 4]:
        match = _ROUTE_ENTRY_RE.match(raw)
        if match is None:
            return {}
        found[match.group(1)] = match.group(2)
    return found if len(found) == 3 else {}


def _file_routes(path: Path, rel: str) -> list[Declared]:
    """Every tool_route one proto file declares, with its API surface."""
    lines = path.read_text(encoding="utf-8").splitlines()
    declared: list[Declared] = []
    message = ""
    beta: set[str] = set()
    opens: dict[str, tuple[dict[str, str], int]] = {}
    for index, raw in enumerate(lines):
        opened = _MESSAGE_RE.match(raw)
        if opened is not None:
            message = opened.group(1)
            continue
        if _BETA_RE.match(raw):
            beta.add(message)
            continue
        if not _ROUTE_OPEN_RE.match(raw):
            continue
        found = _entries(lines, index)
        if found:
            opens[message] = (found, index + 1)
    for name, (found, line) in opens.items():
        surface = "v4beta" if name in beta else "v4"
        shape = path_shape(found["path"])
        declared.append(
            Declared(found["tool"], found["method"].upper(), shape, surface, rel, line)
        )
    return declared


def declared_routes() -> list[Declared]:
    """Every tool_route in the modules buf.yaml declares."""
    declared: list[Declared] = []
    for module in modules(_WORKSPACE):
        root = _REPO_ROOT / module
        for path in sorted(root.rglob("*.proto")):
            declared.extend(_file_routes(path, str(path.relative_to(_REPO_ROOT))))
    return declared


class Verdict(NamedTuple):
    """The drift found, plus what the gate reached and what it is waiting on."""

    violations: list[str]
    judged: int
    deprecated: list[str]


def current_violations() -> Verdict:
    """One entry per declared route the reviewed snapshot no longer supports."""
    snapshot = snapshot_routes(_SNAPSHOT)
    if not snapshot:
        raise SystemExit(
            f"{_SNAPSHOT} is missing or states no route, so this gate would"
            " measure nothing. Refresh it from a comparator run before"
            " running the gate again."
        )

    violations: list[str] = []
    deprecated: list[str] = []
    for route in sorted(declared_routes()):
        upstream = snapshot.get((route.method, route.shape))
        if upstream is None:
            violations.append(
                f"{route.file}:{route.line}: {route.tool} declares"
                f" {route.method} {route.shape}, which TechDocs no longer state"
            )
            continue
        if upstream.surface not in _SERVES[route.surface]:
            violations.append(
                f"{route.file}:{route.line}: {route.tool} is built on the"
                f" {route.surface} surface, but TechDocs serve"
                f" {route.method} {route.shape} as {upstream.surface}"
            )
            continue
        if upstream.deprecated:
            deprecated.append(f"{route.tool}: {route.method} {route.shape}")

    return Verdict(violations, len(snapshot), deprecated)


def main(argv: list[str]) -> int:
    del argv  # no options: a hard gate has nothing to record

    result = current_violations()

    _hardgate.measured("the declared-route comparison", len(declared_routes()))

    if result.violations:
        print("routes the rendered TechDocs no longer support:", file=sys.stderr)
        for entry in result.violations:
            print(f"  {entry}", file=sys.stderr)
        print(
            "\nUpstream moved and proto/ did not follow. Merge the refreshed"
            " surface (go/cmd/protomerge -surface ... -out ...), or retire the"
            " tool; never edit the snapshot to match the tree.",
            file=sys.stderr,
        )
        return 1

    print(
        f"techdocs-route gate OK: {len(declared_routes())} declared route(s)"
        f" against {result.judged} snapshot route(s)"
    )
    if result.deprecated:
        print(
            f"  {len(result.deprecated)} route(s) TechDocs mark deprecated,"
            " tracked by REQ-D7 rather than failed here:"
        )
        for entry in result.deprecated:
            print(f"    {entry}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
