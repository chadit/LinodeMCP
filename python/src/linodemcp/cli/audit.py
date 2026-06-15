"""``linodemcp audit <recent|summary|health|export>`` - read the audit log.

A thin wrapper so an operator can read the audit log from the shell without a
separate MCP client. Each subcommand builds the matching ``linode_audit_*``
tool call and drives it through ``Server.dispatch``, the same chokepoint an MCP
request uses, then prints the result. No bespoke log parsing lives here: the
audit query tools already do that, and routing through dispatch means the read
is itself audited and profile-checked like any other call.

Common flags map onto the tools' schema arguments: ``--tool`` (glob),
``--since`` (RFC 3339), ``--limit``. ``export`` adds ``--format``.
"""

from __future__ import annotations

import argparse
import asyncio
from typing import TYPE_CHECKING, Any, TextIO

from linodemcp.cli._shared import (
    EXIT_SUCCESS,
    EXIT_TOOL_ERROR,
    EXIT_USAGE_ERROR,
    is_error_result,
    result_text,
)
from linodemcp.cli.runtime import open_runtime

if TYPE_CHECKING:
    from pathlib import Path

# Subcommand -> audit tool name. The CLI verbs are short; the tools they drive
# carry the linode_audit_ prefix the registry uses.
_AUDIT_TOOLS = {
    "recent": "linode_audit_recent",
    "summary": "linode_audit_summary",
    "health": "linode_audit_health",
    "export": "linode_audit_export",
}

_EXPORT_FORMATS = ("json", "csv", "ndjson")

AUDIT_USAGE = """\
Usage: linodemcp audit <subcommand> [flags]

  recent [--tool GLOB] [--since TS] [--limit N]
                        Most recent events, newest first.
  summary [--since TS]  Counts grouped by tool, status, capability, ...
  health                Audit subsystem state (paths, sizes, counters).
  export --format FMT [--tool GLOB] [--since TS]
                        Dump a filtered range to a temp file (json/csv/ndjson).\
"""


def run_audit_command(
    argv: list[str],
    stdout: TextIO,
    stderr: TextIO,
    config_path: Path | None = None,
) -> int:
    """Synchronous entry for ``linodemcp audit``; runs the async body.

    Kept separate from the async worker so ``main`` can call it the same way it
    calls the profile subcommand (without managing the event loop itself). The
    args/output contract matches the other subcommands.
    """
    return asyncio.run(_run_audit(argv, stdout, stderr, config_path))


def _build_audit_arguments(
    sub: str,
    ns: argparse.Namespace,
) -> dict[str, Any]:
    """Map the parsed flags onto the audit tool's schema arguments.

    Only set flags are written, so the tool sees absent rather than empty for
    an unused filter. ``export`` always carries its required ``format``; the
    others share the tool/since/limit subset where the tool supports it.
    """
    arguments: dict[str, Any] = {}
    tool_glob = getattr(ns, "tool", None)
    since = getattr(ns, "since", None)
    limit = getattr(ns, "limit", None)

    if tool_glob:
        arguments["tool"] = tool_glob
    if since:
        arguments["since"] = since
    if limit is not None:
        arguments["limit"] = limit
    if sub == "export":
        arguments["format"] = ns.format
    return arguments


def _parse_audit_args(
    sub: str,
    rest: list[str],
) -> argparse.Namespace | None:
    """Parse the flags for one audit subcommand, or None on a usage error.

    Each subcommand gets only the flags its tool understands. ``export``
    requires ``--format``; argparse enforces that and, on failure, has already
    written its own message, so a None return maps to ``EXIT_USAGE_ERROR``.
    """
    parser = argparse.ArgumentParser(prog=f"linodemcp audit {sub}", add_help=True)
    if sub in ("recent", "summary", "export"):
        parser.add_argument("--since", default=None, help="RFC 3339 lower bound")
    if sub in ("recent", "export"):
        parser.add_argument("--tool", default=None, help="tool-name glob filter")
    if sub == "recent":
        parser.add_argument("--limit", type=int, default=None, help="max events")
    if sub == "export":
        parser.add_argument(
            "--format",
            choices=_EXPORT_FORMATS,
            required=True,
            help="export format",
        )
    try:
        return parser.parse_args(rest)
    except SystemExit:
        return None


async def _run_audit(
    argv: list[str],
    stdout: TextIO,
    stderr: TextIO,
    config_path: Path | None,
) -> int:
    """Resolve the subcommand, build the call, dispatch it, and print.

    An unknown or missing subcommand prints usage and exits 2. Otherwise the
    matching ``linode_audit_*`` call runs through the shared dispatch and its
    result text prints; a tool-level error result exits 1.
    """
    if not argv:
        print(AUDIT_USAGE, file=stderr)
        return EXIT_USAGE_ERROR

    sub = argv[0]
    tool_name = _AUDIT_TOOLS.get(sub)
    if tool_name is None:
        print(f"unknown audit subcommand: {sub}\n\n{AUDIT_USAGE}", file=stderr)
        return EXIT_USAGE_ERROR

    ns = _parse_audit_args(sub, argv[1:])
    if ns is None:
        return EXIT_USAGE_ERROR

    arguments = _build_audit_arguments(sub, ns)
    return await _dispatch_audit(tool_name, arguments, stdout, stderr, config_path)


async def _dispatch_audit(
    tool_name: str,
    arguments: dict[str, Any],
    stdout: TextIO,
    stderr: TextIO,
    config_path: Path | None,
) -> int:
    """Open the runtime, run the audit tool, and render its result.

    A runtime-construction failure exits 2. The audit tools are CapMeta so the
    active profile never filters them; a ValueError here would still mean a bad
    name and exits 2. A tool-level error result exits 1; success exits 0.
    """
    try:
        async with open_runtime(config_path) as runtime:
            try:
                result = await runtime.server.dispatch(tool_name, arguments)
            except ValueError as exc:
                print(str(exc), file=stderr)
                return EXIT_USAGE_ERROR
    except Exception as exc:
        print(f"failed to start runtime: {exc}", file=stderr)
        return EXIT_USAGE_ERROR

    text = result_text(result)
    if is_error_result(text):
        print(text, file=stderr)
        print(text, file=stdout)
        return EXIT_TOOL_ERROR

    print(text, file=stdout)
    return EXIT_SUCCESS
