#!/usr/bin/env python3
"""Offline gate: the proto contract and the capability manifest agree, tool for tool.

Every tool's *Input message declares `linode.mcp.v1.tool_capability` and names
its tool in exactly one marker, `tool_route` for a tool that reaches the Linode
API and `tool_meta` for one that works on local state. Both clients enumerate
the surface from those declarations, so docs/contracts/tools-capabilities.txt is
a mirror, and an unchecked mirror drifts. Failures:

- a tool the proto declares that the manifest does not list, or the reverse;
- a tool whose declared tier is not the tier the manifest gives it;
- a message that names its tool twice (both markers) or not at all;
- a Meta tool carrying no meta marker, or a non-Meta tool carrying one. `make
  tool-routes` holds the route side of that split, and together they keep
  "meta" meaning one thing.

A capability value TIERS cannot translate is checked before all of that: an
unmapped tier drops out of the comparison rather than failing it, so the gate
would report OK while checking less than it claims.

Descriptor reads need the generated modules, which scripts/_toolroutes.py
resolves through python/.venv/bin/python when the running interpreter cannot
import them. `make proto` must have run. Run via `make tool-capability`, which
`make check` includes, and so the pre-push hook and the CI gate.
"""

from __future__ import annotations

import sys

import _surface
import _toolroutes

# Proto enum value to the spelling docs/contracts/tools-capabilities.txt uses.
# Mapped explicitly because the vocabularies are independent: deriving one from
# the other would mistranslate the first tier named with more than one word.
TIERS = {
    "TOOL_CAPABILITY_READ": "Read",
    "TOOL_CAPABILITY_WRITE": "Write",
    "TOOL_CAPABILITY_DESTROY": "Destroy",
    "TOOL_CAPABILITY_ADMIN": "Admin",
    "TOOL_CAPABILITY_META": "Meta",
}

META = "Meta"

_PACKAGE = "linode.mcp.v1."


def unmapped_values(values: list[str]) -> list[str]:
    """Capability values the tier table cannot translate.

    Read from the enum, not from the declarations in use, so a tier added to
    the proto and never mapped here fails on the commit that adds it.
    """
    return sorted(
        value
        for value in values
        if value != _toolroutes.UNSPECIFIED_CAPABILITY and value not in TIERS
    )


def marker_violations(declared: list[_toolroutes.ToolDeclaration]) -> list[str]:
    """Messages that name their tool twice, not at all, or in the wrong marker."""
    found: list[str] = []
    for entry in declared:
        short = entry.message.removeprefix(_PACKAGE)
        tier = TIERS.get(entry.capability, entry.capability)

        if entry.route_tool and entry.meta_tool:
            found.append(
                f"{short}: names {entry.route_tool} as a routed tool and"
                f" {entry.meta_tool} as a meta tool"
            )
        elif not entry.route_tool and not entry.meta_tool:
            found.append(f"{short}: declares {tier} but no marker names its tool")
        elif entry.meta_tool and tier != META:
            found.append(f"{short}: {entry.meta_tool} is {tier}, not a meta tool")
        elif entry.route_tool and tier == META:
            found.append(f"{short}: {entry.route_tool} is Meta, so it carries no route")

        if entry.capability == _toolroutes.UNSPECIFIED_CAPABILITY:
            found.append(f"{short}: names a tool but declares no capability")

    return sorted(found)


def declared_tiers(declared: list[_toolroutes.ToolDeclaration]) -> dict[str, str]:
    """Tool name to manifest-spelled tier, for every well-formed declaration.

    A message whose markers contradict each other is left out; marker_violations
    reports it, and the tool name it would be filed under may not be its own.
    """
    tiers: dict[str, str] = {}
    for entry in declared:
        name = entry.route_tool or entry.meta_tool
        if not name or (entry.route_tool and entry.meta_tool):
            continue
        if entry.capability in TIERS:
            tiers[name] = TIERS[entry.capability]
    return tiers


def manifest_violations(
    proto: dict[str, str], manifest: dict[str, str]
) -> tuple[list[str], list[str], list[str]]:
    """Tools the manifest is missing, the proto is missing, and tiers that differ."""
    unlisted = sorted(f"{tool} ({proto[tool]})" for tool in proto.keys() - manifest)
    undeclared = sorted(
        f"{tool} ({manifest[tool]})" for tool in manifest.keys() - proto
    )
    retiered = sorted(
        f"{tool}: proto says {proto[tool]}, manifest says {manifest[tool]}"
        for tool in proto.keys() & manifest
        if proto[tool] != manifest[tool]
    )
    return unlisted, undeclared, retiered


def _report(header: str, entries: list[str], remedy: str) -> None:
    """Print one violation group to stderr."""
    print(header, file=sys.stderr)
    for entry in entries:
        print(f"  {entry}", file=sys.stderr)
    print(f"  ({remedy})", file=sys.stderr)


def main() -> int:
    """Report every disagreement; zero when the proto mirrors the manifest."""
    values = _toolroutes.capability_values()
    if unmapped := unmapped_values(values):
        _report(
            "ToolCapability values with no manifest tier:",
            unmapped,
            "add the value to TIERS in this script, and the tier to"
            " docs/contracts/tools-capabilities.txt",
        )
        return 1

    declared = _toolroutes.declarations()
    markers = marker_violations(declared)
    manifest = _surface.read_capabilities()
    unlisted, undeclared, retiered = manifest_violations(
        declared_tiers(declared), manifest
    )

    if markers:
        _report(
            "messages whose tool declaration does not describe one tool:",
            markers,
            "a tool input carries tool_capability plus exactly one of tool_route"
            " or tool_meta, then run `make proto`",
        )
    if unlisted:
        _report(
            "tools the proto declares that the capability manifest does not list:",
            unlisted,
            "add the tool to docs/contracts/tools-capabilities.txt, or drop the"
            " declaration",
        )
    if undeclared:
        _report(
            "tools the capability manifest lists that no proto message declares:",
            undeclared,
            "add `option (linode.mcp.v1.tool_capability)` to the tool's input"
            " message, then run `make proto`",
        )
    if retiered:
        _report(
            "tools whose proto tier and manifest tier differ:",
            retiered,
            "the two are one fact; move both or neither",
        )
    if markers or unlisted or undeclared or retiered:
        return 1

    print(
        f"tool-capability gate OK: {len(declared)} tool(s) declare a tier"
        f" matching docs/contracts/tools-capabilities.txt"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
