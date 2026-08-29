#!/usr/bin/env python3
"""Offline gate: every tool input field declares where it goes in the request.

`tool_route` says which Linode operation a tool calls. On its own that is not
enough to build the call: a field could be a path segment, a query entry, or a
body member, and nothing in the descriptor said which. So each language spelled
the same URL out again in its own client, and the route ended up written five
times across the repo with only a gate to notice when a copy drifted.

`linode.mcp.v1.field_location` closes that. Every field of a message carrying a
tool_route declares one of:

    FIELD_LOCATION_PATH    substituted into the route template by field name
    FIELD_LOCATION_QUERY   appended to the query string when set
    FIELD_LOCATION_BODY    carried in the JSON body
    FIELD_LOCATION_LOCAL   consumed by the MCP layer, never sent
    FIELD_LOCATION_TOOL    the tool's own argument, reaching no request at all

The last one is what a meta tool's arguments carry: those tools answer from
local state, so nothing about them is a request part, and reusing LOCAL there
would break the bidirectional lock below. A routed message may declare one too,
but only when its declared execute_transport owns the call and reads it: an
upload's source_path names bytes the request never carries.

Four things are checked, and each one is a way the annotation could look
complete while being useless:

- an unannotated field reads back as FIELD_LOCATION_UNSPECIFIED, so a request
  builder would silently drop it;
- a path template naming a parameter no field declares as PATH cannot be
  filled, and a PATH field absent from the template would never be consumed.
  Both directions fail, because either one leaves a route that cannot be built;
- TOOL on a routed message needs a declared execute_transport to read it, or
  the value is advertised in the schema and dropped before the call;
- LOCAL must agree exactly with the trailing `// system param` marker that
  scripts/verify_system_params.py pins. Two records of the same fact drift, and
  the direction that matters is a field losing its marker and going on the wire:
  `dry_run` reaching Linode as a query parameter is a request nobody asked for.

The `mode` field is why the marker is the cross-check rather than a name list.
It is a system param as `optional string` (the two-stage selector) and a real
API body param as `optional NodeBalancerNodeMode.Value`, so name alone cannot
tell them apart.

Reading descriptors needs the generated modules, which scripts/_toolroutes.py
resolves through python/.venv/bin/python when the running interpreter cannot
import them. `make proto` must have run. Run via `make field-location` (in
`make check`, and so in the pre-push hook and the CI gate on every branch).
"""

from __future__ import annotations

import re
import sys

import _toolroutes
import verify_system_params

_PARAM = re.compile(r"\{([a-z][a-z0-9_]*)\}")

_UNSPECIFIED = "FIELD_LOCATION_UNSPECIFIED"
_PATH = "FIELD_LOCATION_PATH"
_LOCAL = "FIELD_LOCATION_LOCAL"
_TOOL = "FIELD_LOCATION_TOOL"

# Descriptor full names are package-qualified; proto sources name the message
# alone. Everything under proto/linode/mcp/v1/ shares this package.
_PACKAGE = "linode.mcp.v1."


def unannotated(located: dict[str, dict[str, str]]) -> list[str]:
    """Fields of a routed message that declare no location."""
    return [
        f"{message.removeprefix(_PACKAGE)}.{field}"
        for message, fields in sorted(located.items())
        for field, value in sorted(fields.items())
        if value == _UNSPECIFIED
    ]


def path_mismatches(
    routes: list[_toolroutes.ToolRoute],
    located: dict[str, dict[str, str]],
) -> list[str]:
    """Path parameters with no PATH field, and PATH fields with no slot."""
    found: list[str] = []
    for route in routes:
        fields = located.get(route.message, {})
        declared = {name for name, value in fields.items() if value == _PATH}
        slots = set(_PARAM.findall(route.path))
        short = route.message.removeprefix(_PACKAGE)
        found.extend(
            f"{short}: path names {{{name}}}, which no field declares as PATH"
            for name in sorted(slots - declared)
        )
        found.extend(
            f"{short}.{name}: declared PATH, but {route.path!r} has no such slot"
            for name in sorted(declared - slots)
        )
    return found


def local_mismatches(located: dict[str, dict[str, str]]) -> list[str]:
    """Fields where the LOCAL annotation and the `// system param` marker differ."""
    marked = {
        (field.message, field.name)
        for field in verify_system_params.all_fields()
        if field.marked
    }

    found: list[str] = []
    for message, fields in sorted(located.items()):
        short = message.removeprefix(_PACKAGE)
        for name, value in sorted(fields.items()):
            is_local = value == _LOCAL
            is_marked = (short, name) in marked
            if is_local and not is_marked:
                found.append(f"{short}.{name}: LOCAL but carries no `// system param`")
            elif is_marked and not is_local:
                found.append(f"{short}.{name}: `// system param` but declared {value}")
    return found


def tool_argument_mismatches(located: dict[str, dict[str, str]]) -> list[str]:
    """Routed messages declaring a TOOL argument with no transport to read it.

    TOOL is the location for an argument that reaches no request part. A meta
    tool's whole input carries it, and so does the local file path a byte-moving
    tool's declared transport reads. Anywhere else the value would be advertised
    in the schema and then dropped before the call, which reads to a caller as
    an argument the tool ignores. The emitter refuses the declaration for every
    language.
    """
    routed = {
        declared.message: declared
        for declared in _toolroutes.declarations()
        if declared.route_tool
    }

    found: list[str] = []
    for message, fields in sorted(located.items()):
        declared = routed.get(message)
        if declared is None or declared.transported:
            continue

        short = message.removeprefix(_PACKAGE)
        found.extend(
            f"{short}.{name}: TOOL on a routed message with no execute_transport"
            for name, value in sorted(fields.items())
            if value == _TOOL
        )
    return found


def _report(header: str, entries: list[str], remedy: str) -> None:
    """Print one violation group to stderr."""
    print(header, file=sys.stderr)
    for entry in entries:
        print(f"  {entry}", file=sys.stderr)
    print(f"  ({remedy})", file=sys.stderr)


def main() -> int:
    """Report every location gap; zero when the routed surface is fully declared."""
    routes = _toolroutes.tool_routes()
    located = _toolroutes.field_locations()

    missing = unannotated(located)
    paths = path_mismatches(routes, located)
    locals_ = local_mismatches(located)
    domain = tool_argument_mismatches(located)

    if missing:
        _report(
            "tool input fields that declare no location:",
            missing,
            "add `[(linode.mcp.v1.field_location) = FIELD_LOCATION_...]`"
            " to the field, then run `make proto`",
        )
    if paths:
        _report(
            "path templates and PATH fields that do not line up:",
            paths,
            "every {parameter} in a tool_route path names a field of that same"
            " message, and that field declares FIELD_LOCATION_PATH",
        )
    if locals_:
        _report(
            "FIELD_LOCATION_LOCAL and `// system param` disagree:",
            locals_,
            "a system param is LOCAL and carries the marker; anything else is"
            " PATH, QUERY, or BODY. See docs/contracts/system-params.txt",
        )
    if domain:
        _report(
            "FIELD_LOCATION_TOOL on a routed message with nothing to read it:",
            domain,
            "TOOL is for an argument that reaches no request part: a meta"
            " tool's input, or the local path a byte-moving tool's declared"
            " transport reads. Declare execute_transport, or give the field a"
            " request location",
        )
    if missing or paths or locals_ or domain:
        return 1

    total = sum(len(fields) for fields in located.values())
    print(
        f"field-location gate OK: {total} field(s) across {len(located)}"
        " tool input message(s) declare where they go"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
