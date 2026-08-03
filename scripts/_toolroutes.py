#!/usr/bin/env python3
"""Shared reader for the tool_route options the proto contract carries.

Which Linode API operation each MCP tool calls used to live in a hand-kept text
file. It lives in the proto now: every non-meta tool's *Input message carries a
`linode.mcp.v1.tool_route` option holding the tool name, the HTTP method, and
the path template. That makes an Input message the equivalent of an OpenAPI
operation object, and it means the descriptors alone yield the whole
tool-to-route map with no separate tool-to-message mapping to keep in sync.

The tool NAME is part of the option on purpose. Without it, every consumer
would need its own way to decide which tool an Input message belongs to, and
the only thing in the repo that can answer that is a Go AST dumper with no
tests behind it.

Reading descriptors needs the protobuf runtime and the generated modules, so
this resolves through python/.venv/bin/python when the running interpreter
cannot import them. Callers keep working under plain `python3`, which is how
the Makefile invokes the offline gates. `make proto` must have run; inside
`make check` it always has, since proto is the first prerequisite.

Path templates name their parameters in snake_case, the way the Linode API
documentation names them: "/databases/postgresql/instances/{postgresql_instance_id}".
A name is there so a reader can bind the parameter to the documented one; no
comparison in this repo depends on it. Both route scanners collapse every
parameter to _routescan.PATH_PARAM because they read code that builds a URL out
of variables, where no name exists, so anything comparing a declared route
against a resolved one runs it through norm_template first and matches on shape.
"""

from __future__ import annotations

import importlib
import json
import pkgutil
import subprocess
import sys
from pathlib import Path
from typing import NamedTuple

import _routescan

REPO_ROOT = Path(__file__).resolve().parents[1]
VENV_PYTHON = REPO_ROOT / "python" / ".venv" / "bin" / "python"

GENERATED_PACKAGE = "linodemcp.genpb.linode.mcp.v1"

METHODS = ("GET", "POST", "PUT", "DELETE")


def norm_template(path: str) -> str:
    """Collapse a path to its shape: '/a/{whatever}/b?q=1' to '/a/{p}/b'.

    This is the boundary every comparison crosses. The scanners resolve routes
    from code that builds URLs out of variables, so a resolved route carries no
    parameter names, and the spec names the same parameter differently again
    (linodeId where a handler says encoded_instance_id). Matching on shape is
    what lets a declared route keep a real name without any of that agreeing.

    The placeholder comes from the scanner rather than being written out again
    here, so the two cannot drift apart. Query strings never take part in a
    route.
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


def _read_descriptors() -> list[ToolRoute]:
    """Every tool_route option in the generated descriptors, message-sorted.

    Only reachable under an interpreter that can import the generated tree.
    """
    options = importlib.import_module(f"{GENERATED_PACKAGE}.options_pb2")
    package = importlib.import_module(GENERATED_PACKAGE)

    found: list[ToolRoute] = []
    for info in pkgutil.iter_modules(package.__path__):
        if not info.name.endswith("_pb2"):
            continue
        module = importlib.import_module(f"{GENERATED_PACKAGE}.{info.name}")
        for descriptor in module.DESCRIPTOR.message_types_by_name.values():
            declared = descriptor.GetOptions()
            if not declared.HasExtension(options.tool_route):
                continue
            route = declared.Extensions[options.tool_route]
            found.append(
                ToolRoute(descriptor.full_name, route.tool, route.method, route.path)
            )
    return sorted(found)


def _read_through_venv() -> list[ToolRoute]:
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

    return [ToolRoute(*entry) for entry in json.loads(result.stdout)]


def tool_routes() -> list[ToolRoute]:
    """Every tool_route option the proto declares.

    An empty result means `make proto` has not run or the annotations are gone,
    either of which would make every consumer report the whole surface as
    missing, so it fails here instead.
    """
    try:
        found = _read_descriptors()
    except ImportError:
        found = _read_through_venv()

    if not found:
        msg = (
            "no tool_route options found in the generated descriptors; run `make proto`"
        )
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
    json.dump([list(entry) for entry in _read_descriptors()], sys.stdout)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
