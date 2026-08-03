#!/usr/bin/env python3
"""Offline gate: every non-meta tool declares its route in the proto.

Which Linode API operation a tool calls is a `linode.mcp.v1.tool_route` option
on that tool's proto *Input message. The option carries the tool name too, so
the descriptors alone answer "which route does this tool call" for every
consumer; nothing else in the repo has to hold a tool-to-message mapping.

An annotation nothing checks rots the same way the hand-kept text file it
replaced could, so this pins it from both sides and cross-checks the names
against docs/contracts/tools-manifest.txt, the surface the parity gates
already trust:

- a registered tool whose capability is not Meta must carry the option;
- a Meta tool (profile builder, audit, version, hello) reaches no Linode
  route, so it must not carry one;
- an option naming a tool the manifest does not list fails, so a typo cannot
  pass as a route for a tool that does not exist;
- an option must sit on the input message its tool actually registers, so an
  annotation cannot land on the wrong message and still look complete;
- method must be one of GET/POST/PUT/DELETE and the path must be a rooted
  template whose parameters are snake_case names, each used once.

Naming is checked here because nothing else can. A parameter name is
documentation for whoever reads the contract next: every comparison in the
repo normalizes names away and matches on shape, so a name that went camelCase
or got pasted twice would never show up anywhere else.

This gate is hard: the surface is fully annotated today, so there is no
baseline to ratchet and any violation is a regression.

Reading descriptors needs the generated modules, which scripts/_toolroutes.py
resolves through python/.venv/bin/python when the running interpreter cannot
import them. Run via `make tool-routes` (in `make check`, and so in the
pre-push hook and the CI gate on every branch).
"""

from __future__ import annotations

import re
import sys
from typing import TYPE_CHECKING

import _surface
import _toolroutes

if TYPE_CHECKING:
    from pathlib import Path

_MANIFEST = _surface.REPO_ROOT / "docs" / "contracts" / "tools-manifest.txt"

_META = "Meta"

# A path template: rooted, and every parameter segment is a snake_case name in
# braces. Names come from the Linode API documentation for the operation.
_PATH = re.compile(r"^(?:/(?:\{[a-z][a-z0-9_]*\}|[A-Za-z0-9._-]+))+$")

# One parameter of a path template, name included.
_PARAM = re.compile(r"\{([a-z][a-z0-9_]*)\}")


def read_manifest(path: Path = _MANIFEST) -> set[str]:
    """Every tool name the canonical surface manifest lists."""
    names = {
        stripped
        for raw in path.read_text(encoding="utf-8").splitlines()
        if (stripped := raw.strip()) and not stripped.startswith("#")
    }
    if not names:
        msg = f"{path.name} lists no tools"
        raise SystemExit(msg)
    return names


def annotation_violations(
    declared: list[_toolroutes.ToolRoute],
    capabilities: dict[str, str],
    manifest: set[str],
    messages: dict[str, str],
) -> tuple[list[str], list[str], list[str]]:
    """Missing annotations, unwanted ones, and malformed or misplaced ones."""
    routed = {entry.tool: entry for entry in declared}

    missing = [
        f"{tool} ({messages.get(tool, 'no input message')}): no tool_route option"
        for tool, capability in sorted(capabilities.items())
        if capability != _META and tool not in routed
    ]
    unwanted = [
        f"{tool} ({routed[tool].message}): Meta tools reach no Linode route"
        for tool, capability in sorted(capabilities.items())
        if capability == _META and tool in routed
    ]

    malformed: list[str] = []
    for entry in sorted(declared):
        if entry.tool not in manifest:
            malformed.append(f"{entry.message}: names unregistered tool {entry.tool}")
            continue
        expected = messages.get(entry.tool)
        if expected is not None and not entry.message.endswith(f".{expected}"):
            malformed.append(
                f"{entry.message}: annotates {entry.tool}, which registers {expected}"
            )
        if entry.method not in _toolroutes.METHODS:
            malformed.append(f"{entry.message}: method {entry.method!r} is not a verb")
        if not _PATH.match(entry.path):
            malformed.append(f"{entry.message}: path {entry.path!r} is not a template")
            continue
        repeated = _repeated_params(entry.path)
        if repeated:
            malformed.append(
                f"{entry.message}: path {entry.path!r} names"
                f" {', '.join(repeated)} more than once"
            )

    return missing, unwanted, malformed


def _repeated_params(path: str) -> list[str]:
    """Parameter names a template uses more than once, in first-use order.

    Two segments of one route are two different values, so one name standing
    for both cannot be read back to either. It is what a copy-paste of a
    neighbouring route leaves behind.
    """
    seen: list[str] = []
    repeated: list[str] = []
    for name in _PARAM.findall(path):
        if name in seen and name not in repeated:
            repeated.append(name)
        seen.append(name)
    return repeated


def main() -> int:
    """Report every annotation gap; zero when the surface is fully annotated."""
    declared = _toolroutes.tool_routes()
    capabilities = _surface.read_capabilities()
    missing, unwanted, malformed = annotation_violations(
        declared, capabilities, read_manifest(), _surface.tool_input_messages()
    )

    if missing:
        print("tools whose proto input declares no route:", file=sys.stderr)
        for entry in missing:
            print(f"  {entry}", file=sys.stderr)
        print(
            "  (add `option (linode.mcp.v1.tool_route)` to the input message,"
            " then run `make proto`)",
            file=sys.stderr,
        )
    if unwanted:
        print("Meta tools whose proto input declares a route:", file=sys.stderr)
        for entry in unwanted:
            print(f"  {entry}", file=sys.stderr)
        print(
            "  (drop the option, or fix the tier in"
            " docs/contracts/tools-capabilities.txt)",
            file=sys.stderr,
        )
    if malformed:
        print("tool_route options that do not describe a real route:", file=sys.stderr)
        for entry in malformed:
            print(f"  {entry}", file=sys.stderr)
        print(
            "  (tool must be a name in docs/contracts/tools-manifest.txt, method"
            " one of GET/POST/PUT/DELETE, path a rooted template whose"
            " parameters are distinct snake_case names)",
            file=sys.stderr,
        )
    if missing or unwanted or malformed:
        return 1

    print(f"tool-routes gate OK: {len(declared)} tool(s) declare their route")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
