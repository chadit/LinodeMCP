#!/usr/bin/env python3
"""Shared reader for the tool_route options the proto contract carries.

Which Linode API operation each MCP tool calls used to live in a hand-kept text
file; it lives in the proto now. Every non-meta tool's *Input message carries a
`linode.mcp.v1.tool_route` option holding the tool name, the HTTP method, and
the path template, so the descriptors alone yield the whole tool-to-route map.
The tool name rides in the option because the only other thing in the repo that
can say which tool a message belongs to is a Go AST dumper with no tests.

Reading descriptors needs the protobuf runtime and the generated modules, so
this resolves through python/.venv/bin/python when the running interpreter
cannot import them, keeping callers working under the plain `python3` the
Makefile uses. `make proto` must have run; inside `make check` it always has.

Every public reader aborts on an empty result: a missing options block or an
unrun `make proto` otherwise leaves a gate comparing nothing and passing.

Path templates name their parameters after the fields of the message they sit
on: "/databases/postgresql/instances/{instance_id}" where the message declares
`int32 instance_id`. That binding lets a consumer fill a route from the
descriptor alone, and `make field-location` keeps it true. Linode documents
some parameters under another name (postgresqlInstanceId here), which is why
the binding is to the field rather than to the documentation.

Both route scanners collapse every parameter to _routescan.PATH_PARAM, since
they read code that builds URLs out of variables where no name survives, so
comparing a declared route against a resolved one matches on shape through
norm_template.
"""

from __future__ import annotations

import importlib
import json
import pkgutil
import subprocess
import sys
from pathlib import Path
from typing import TYPE_CHECKING, Any, NamedTuple

import _routescan

if TYPE_CHECKING:
    from collections.abc import Iterator
    from types import ModuleType

    from google.protobuf.descriptor import Descriptor

REPO_ROOT = Path(__file__).resolve().parents[1]
VENV_PYTHON = REPO_ROOT / "python" / ".venv" / "bin" / "python"

GENERATED_PACKAGE = "linodemcp.genpb.linode.mcp.v1"

METHODS = ("GET", "POST", "PUT", "DELETE")

# The enum's zero value, which is what a message declaring no tier reads back as.
UNSPECIFIED_CAPABILITY = "TOOL_CAPABILITY_UNSPECIFIED"


def norm_template(path: str) -> str:
    """Collapse a path to its shape: '/a/{whatever}/b?q=1' to '/a/{p}/b'.

    Every comparison crosses here. A resolved route carries no parameter names
    and the spec names the same parameter differently again (linodeId where a
    handler says encoded_instance_id), so matching on shape is what lets a
    declared route keep a real name without any of that agreeing. The
    placeholder comes from the scanner so the two cannot drift apart. Query
    strings never take part in a route.
    """
    bare = path.split("?", maxsplit=1)[0]
    segments = [
        _routescan.PATH_PARAM if segment.startswith("{") else segment
        for segment in bare.strip("/").split("/")
    ]
    return "/" + "/".join(segments)


class ToolRoute(NamedTuple):
    """One tool_route option, with the message it was read from."""

    message: str
    tool: str
    method: str
    path: str


def _iter_messages() -> Iterator[tuple[ModuleType, Descriptor]]:
    """Every generated message descriptor, paired with the options module.

    The options module has to come from the same import as the descriptors, or
    the extension identities differ and every HasExtension call reports False.
    """
    options = importlib.import_module(f"{GENERATED_PACKAGE}.options_pb2")
    package = importlib.import_module(GENERATED_PACKAGE)

    for info in pkgutil.iter_modules(package.__path__):
        if not info.name.endswith("_pb2"):
            continue
        module = importlib.import_module(f"{GENERATED_PACKAGE}.{info.name}")
        for descriptor in module.DESCRIPTOR.message_types_by_name.values():
            yield options, descriptor


def _read_descriptors() -> list[ToolRoute]:
    """Every tool_route option in the generated descriptors, message-sorted.

    Only reachable under an interpreter that can import the generated tree.
    """
    found: list[ToolRoute] = []
    for options, descriptor in _iter_messages():
        declared = descriptor.GetOptions()
        if not declared.HasExtension(options.tool_route):
            continue
        route = declared.Extensions[options.tool_route]
        found.append(
            ToolRoute(descriptor.full_name, route.tool, route.method, route.path)
        )
    return sorted(found)


def _read_field_locations() -> dict[str, dict[str, str]]:
    """Tool input message full name to {field name: FieldLocation enum name}.

    An unannotated field reads back as the enum's zero value rather than being
    dropped: the gate has to see the gap, and a missing key cannot be told
    apart from a message that failed to load.

    Meta inputs are read alongside the routed ones, because both halves of the
    gate apply to them: a meta tool's own arguments are FIELD_LOCATION_TOOL and
    an unannotated one is as unreadable there as anywhere, and the one confirm
    the meta surface carries is LOCAL, which the `// system param` marker is
    locked to in both directions.
    """
    found: dict[str, dict[str, str]] = {}
    for options, descriptor in _iter_messages():
        declared = descriptor.GetOptions()
        if not declared.HasExtension(options.tool_route) and not declared.HasExtension(
            options.tool_meta
        ):
            continue
        located: dict[str, str] = {}
        for field in descriptor.fields:
            location = field.GetOptions().Extensions[options.field_location]
            located[field.name] = str(options.FieldLocation.Name(location))
        found[descriptor.full_name] = located
    return found


class ToolDeclaration(NamedTuple):
    """What one message's options say about a tool.

    The tool name lives in exactly one marker: `tool_route` for a tool that
    reaches the Linode API, `tool_meta` for one that works on local state. Both
    are read here, so the gate can hold a message to naming its tool once.

    The last eight say what the tool answers with rather than what it calls.
    Each reads back as "" or False when the message does not declare it, which
    is what lets a gate tell "not declared" from a declared empty value.

    hooks names the steps of the tool's handler that are hand-written, which is
    what the hand-validator ratchet counts and what tells a generated handler
    which functions to call.

    api_surface is the declared value's enum name, "" when the message declares
    none. The two readings differ: an absent option means v4, and an option
    written out as v4 is a redundant second spelling the gate refuses.
    """

    message: str
    route_tool: str
    meta_tool: str
    capability: str
    response: str = ""
    confirm_message: str = ""
    success_message: str = ""
    warning_message: str = ""
    resource_type: str = ""
    retry_disabled: bool = False
    description: str = ""
    error_message: str = ""
    hooks: tuple[str, ...] = ()
    api_surface: str = ""
    # The message's own field names, which is what holds the surface to being a
    # declaration rather than something a caller can pass.
    arguments: tuple[str, ...] = ()


def _read_declarations() -> list[ToolDeclaration]:
    """Every message that declares a tool, message-sorted.

    A message declaring none of the options is not a tool input, which is how
    every response and nested type falls out here.
    """
    found: list[ToolDeclaration] = []
    for options, descriptor in _iter_messages():
        declared = descriptor.GetOptions()
        entry = ToolDeclaration(
            message=descriptor.full_name,
            route_tool=str(declared.Extensions[options.tool_route].tool),
            meta_tool=str(declared.Extensions[options.tool_meta].tool),
            capability=str(
                options.ToolCapability.Name(
                    declared.Extensions[options.tool_capability]
                )
            ),
            response=str(declared.Extensions[options.tool_response]),
            confirm_message=str(declared.Extensions[options.confirm_message]),
            success_message=str(declared.Extensions[options.success_message]),
            warning_message=str(declared.Extensions[options.warning_message]),
            resource_type=str(declared.Extensions[options.resource_type]),
            retry_disabled=bool(declared.Extensions[options.retry_disabled]),
            description=str(declared.Extensions[options.tool_description]),
            error_message=str(declared.Extensions[options.error_message]),
            hooks=tuple(str(kind) for kind in declared.Extensions[options.tool_hooks]),
            api_surface=(
                str(
                    options.ApiSurface.Name(
                        declared.Extensions[options.tool_api_surface]
                    )
                )
                if declared.HasExtension(options.tool_api_surface)
                else ""
            ),
            arguments=tuple(str(field.name) for field in descriptor.fields),
        )
        # A stray surface with nothing beside it still has to reach the gate, or
        # an annotation on a response message would be invisible.
        if (
            entry.route_tool
            or entry.meta_tool
            or entry.capability != UNSPECIFIED_CAPABILITY
            or entry.api_surface
        ):
            found.append(entry)
    return sorted(found)


def _read_message_fields() -> dict[str, dict[str, str]]:
    """Message full name to {field name: field's message type, or ""}.

    The success-message placeholders name fields, so the gate needs the field
    set of every message rather than only of the routed ones, and it needs the
    sub-message type to follow a placeholder into the object an envelope wraps.
    """
    found: dict[str, dict[str, str]] = {}
    for _, descriptor in _iter_messages():
        found[descriptor.full_name] = {
            field.name: field.message_type.full_name if field.message_type else ""
            for field in descriptor.fields
        }
    return found


def _read_capability_values() -> list[str]:
    """Every value name the ToolCapability enum defines, in declared order.

    The gate maps each one to a capability-manifest tier, and a value it has no
    mapping for has to fail rather than be skipped, so it needs the enum's own
    value set rather than only the values currently in use.
    """
    options = importlib.import_module(f"{GENERATED_PACKAGE}.options_pb2")
    return [value.name for value in options.ToolCapability.DESCRIPTOR.values]


def _payload() -> dict[str, Any]:
    """Every reading in one descriptor walk, for the venv re-exec below."""
    return {
        "routes": [list(entry) for entry in _read_descriptors()],
        "fields": _read_field_locations(),
        "declarations": [list(entry) for entry in _read_declarations()],
        "capability_values": _read_capability_values(),
        "message_fields": _read_message_fields(),
    }


def _read_through_venv() -> dict[str, Any]:
    """Re-run this module under the venv interpreter and parse what it prints."""
    if not VENV_PYTHON.exists():
        msg = (
            "python/.venv missing; the tool_route options are read from the"
            " generated descriptors. Run `make -C python install-dev`."
        )
        raise SystemExit(msg)

    # Fixed argv, no shell; both paths are repo-owned.
    result = subprocess.run(
        [str(VENV_PYTHON), str(Path(__file__).resolve())],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        msg = f"reading tool_route options failed:\n{result.stderr.strip()}"
        raise SystemExit(msg)

    parsed: dict[str, Any] = json.loads(result.stdout)
    return parsed


def field_locations() -> dict[str, dict[str, str]]:
    """Field-location annotations for every tool input message.

    Empty means the annotations are gone or `make proto` has not run, which
    would let the gate below pass by measuring nothing.
    """
    try:
        found = _read_field_locations()
    except ImportError:
        located: dict[str, dict[str, str]] = _read_through_venv()["fields"]
        found = located

    if not found:
        msg = "no tool inputs found in the generated descriptors; run `make proto`"
        raise SystemExit(msg)

    return found


def tool_routes() -> list[ToolRoute]:
    """Every tool_route option the proto declares.

    An empty result means `make proto` has not run or the annotations are gone,
    either of which would make every consumer report the whole surface as
    missing, so it fails here instead.
    """
    try:
        found = _read_descriptors()
    except ImportError:
        declared: list[list[str]] = _read_through_venv()["routes"]
        found = [ToolRoute(*entry) for entry in declared]

    if not found:
        msg = (
            "no tool_route options found in the generated descriptors; run `make proto`"
        )
        raise SystemExit(msg)

    return found


def declarations() -> list[ToolDeclaration]:
    """Every tool declaration the proto carries.

    An empty result means `make proto` has not run or the options are gone,
    either of which would let a gate report the whole surface as agreeing with
    the contract by comparing nothing, so it fails here instead.
    """
    try:
        found = _read_declarations()
    except ImportError:
        # retry_disabled rides in the same row as the strings, so the row is
        # heterogeneous once it crosses the JSON boundary.
        declared: list[list[Any]] = _read_through_venv()["declarations"]
        found = [ToolDeclaration(*entry) for entry in declared]

    if not found:
        msg = "no tool declarations in the generated descriptors; run `make proto`"
        raise SystemExit(msg)

    return found


def capability_values() -> list[str]:
    """Every value name the ToolCapability enum defines."""
    try:
        found = _read_capability_values()
    except ImportError:
        values: list[str] = _read_through_venv()["capability_values"]
        found = values

    if not found:
        msg = "ToolCapability defines no values; run `make proto`"
        raise SystemExit(msg)

    return found


def message_fields() -> dict[str, dict[str, str]]:
    """Field set of every generated message, for resolving field references.

    Empty means the generated tree is gone, which would let a consumer decide
    every field reference is fine by finding nothing to contradict it.
    """
    try:
        found = _read_message_fields()
    except ImportError:
        read: dict[str, dict[str, str]] = _read_through_venv()["message_fields"]
        found = read

    if not found:
        msg = "no messages found in the generated descriptors; run `make proto`"
        raise SystemExit(msg)

    return found


def routes() -> dict[str, tuple[str, str]]:
    """Tool name to its one (METHOD, path template).

    Two messages claiming one tool is a broken contract rather than drift, so
    it aborts instead of one of them silently winning.
    """
    out: dict[str, tuple[str, str]] = {}
    seen: dict[str, str] = {}
    for entry in tool_routes():
        if entry.tool in seen:
            msg = (
                f"tool {entry.tool} is claimed by both {seen[entry.tool]}"
                f" and {entry.message}"
            )
            raise SystemExit(msg)
        seen[entry.tool] = entry.message
        out[entry.tool] = (entry.method, entry.path)
    return out


def main() -> int:
    """Print the options as JSON, for the venv re-exec above."""
    json.dump(_payload(), sys.stdout)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
