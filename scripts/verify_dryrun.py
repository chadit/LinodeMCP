#!/usr/bin/env python3
"""Offline gate: dry-run must be set up per capability, with no drift.

Every mutating tool (Write, Admin, Destroy in tools-capabilities.txt) must
advertise `dry_run` in its proto input message, and no Read or Meta tool may
carry one. Both languages generate their input schemas from the same proto,
so this single check pins the whole surface; the runtime half (a fixture
case actually exercising the preview in both languages) is verify_behavior.py's
dry-run ratchet.

This gate is hard: the surface is fully compliant today, so there is no
baseline to ratchet and any violation is a regression. A tool that cannot be
mapped to its proto input also fails, because an unmapped tool is an
unchecked one.

Stdlib only, so no venv is needed. Run via `make dryrun` (in `make check`).
"""

from __future__ import annotations

import sys

import _surface

_MUTATING = ("Write", "Admin", "Destroy")
_READONLY = ("Read", "Meta")


def dryrun_violations() -> tuple[list[str], list[str], list[str]]:
    """Unmapped tools, mutators missing dry_run, read tools carrying it."""
    capabilities = _surface.read_capabilities()
    messages = _surface.tool_input_messages()
    bodies = _surface.proto_message_bodies()

    unmapped: list[str] = []
    missing: list[str] = []
    extra: list[str] = []
    for tool, capability in sorted(capabilities.items()):
        body = bodies.get(messages.get(tool, ""))
        if body is None:
            unmapped.append(f"{tool} ({capability})")
            continue
        advertises = _surface.message_has_field(body, "dry_run")
        if capability in _MUTATING and not advertises:
            missing.append(f"{tool} ({capability})")
        if capability in _READONLY and advertises:
            extra.append(f"{tool} ({capability})")
    return unmapped, missing, extra


def main() -> int:
    unmapped, missing, extra = dryrun_violations()

    if unmapped:
        print("tools with no resolvable proto input message:", file=sys.stderr)
        for entry in unmapped:
            print(f"  {entry}", file=sys.stderr)
        print(
            '  (fix the factory to the name= then schema("linode.mcp.v1...")'
            " shape, or teach scripts/_surface.py the new shape)",
            file=sys.stderr,
        )
    if missing:
        print("mutating tools whose input lacks dry_run:", file=sys.stderr)
        for entry in missing:
            print(f"  {entry}", file=sys.stderr)
        print(
            "  (add `optional bool dry_run` to the proto input and implement"
            " the preview in every registered language; docs/dry-run.md)",
            file=sys.stderr,
        )
    if extra:
        print("read-only tools whose input advertises dry_run:", file=sys.stderr)
        for entry in extra:
            print(f"  {entry}", file=sys.stderr)
        print(
            "  (a read tool has nothing to preview; drop the field or fix the"
            " capability tier in docs/contracts/tools-capabilities.txt)",
            file=sys.stderr,
        )
    if unmapped or missing or extra:
        return 1

    total = len(_surface.read_capabilities())
    print(f"dry-run gate OK: {total} tools advertise dry_run per capability")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
