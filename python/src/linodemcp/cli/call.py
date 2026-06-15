"""``linodemcp call <tool> [args]`` - run any registered tool from the shell.

The whole point is the one architectural rule: this never touches a tool
handler directly. It validates the tool name against the registry, builds an
arguments dict (from ``--json`` or ``--arg`` plus the safety flags), and feeds
it to ``Server.dispatch``. Every audit, profile-filter, dry-run, and two-stage
behavior an MCP request gets, a ``call`` invocation gets too, because it is the
same chokepoint.

Argument plumbing and result rendering live in ``_shared``; the runtime
(config load, server build, audit sink) lives in ``runtime``. This module is
the argparse surface and the exit-code policy.
"""

from __future__ import annotations

import argparse
from typing import TYPE_CHECKING, Any, TextIO

from linodemcp.cli._shared import (
    EXIT_SUCCESS,
    EXIT_TOOL_ERROR,
    EXIT_USAGE_ERROR,
    ArgError,
    build_arguments,
    is_error_result,
    render_output,
    result_text,
)
from linodemcp.cli.runtime import OneShotRuntime, open_runtime
from linodemcp.server import get_tool_registry

if TYPE_CHECKING:
    from pathlib import Path

    from mcp.types import Tool

_OUTPUT_CHOICES = ("json", "table")
_MODE_CHOICES = ("plan", "apply")


def _build_parser() -> argparse.ArgumentParser:
    """Construct the argparse parser for ``call``.

    ``--json`` and repeated ``--arg`` carry the tool arguments; they are
    mutually exclusive (enforced after parse so the error is a usage exit, not
    an argparse SystemExit). The safety flags mirror the MCP fields one to one.
    """
    parser = argparse.ArgumentParser(
        prog="linodemcp call",
        description="Run a registered tool non-interactively.",
        add_help=True,
    )
    parser.add_argument("tool", help="tool name (see `linodemcp tools`)")
    parser.add_argument(
        "--json",
        dest="json_args",
        metavar="OBJECT",
        help="arguments as a JSON object, e.g. '{\"instance_id\":123}'",
    )
    parser.add_argument(
        "--arg",
        dest="arg_pairs",
        action="append",
        default=[],
        metavar="KEY=VALUE",
        help="one argument, repeatable; typed by the tool schema",
    )
    parser.add_argument(
        "--output",
        choices=_OUTPUT_CHOICES,
        default="json",
        help="result rendering (default: json)",
    )
    _add_safety_flags(parser)
    return parser


def _add_safety_flags(parser: argparse.ArgumentParser) -> None:
    """Add the write-safety flags that fold into the arguments dict."""
    parser.add_argument("--dry-run", action="store_true", help="preview, no change")
    parser.add_argument("--confirm", action="store_true", help="confirm a mutation")
    parser.add_argument(
        "--mode",
        choices=_MODE_CHOICES,
        default=None,
        help="two-stage write mode",
    )
    parser.add_argument("--plan-id", default=None, help="plan id for mode=apply")
    parser.add_argument(
        "--confirmed-dry-run",
        action="store_true",
        help="assert a prior dry-run for a destroy",
    )
    parser.add_argument(
        "--yolo",
        action="store_true",
        help="bypass gating (only if the profile allows)",
    )
    parser.add_argument("--environment", default=None, help="named environment")


def _find_tool(name: str) -> Tool | None:
    """Look up a tool definition by name in the full registry.

    Uses the full registry (not the profile-filtered set) so an unknown name
    and a profile-filtered name get different, accurate messages: dispatch
    refuses a filtered tool, but the name itself is real.
    """
    for entry in get_tool_registry():
        if entry.name == name:
            return entry.tool
    return None


def callable_tool_names() -> frozenset[str]:
    """Return the set of tool names ``call`` can invoke.

    This is exactly the registry: ``call`` keeps no allow list of its own, so a
    name is callable iff it is registered. Public so the CLI-to-registry parity
    test can assert the surface matches without reaching into private lookups,
    mirroring the tool-manifest gate.
    """
    return frozenset(entry.name for entry in get_tool_registry())


def _known_tool_names() -> list[str]:
    """Sorted registry tool names, for the unknown-tool error hint."""
    return sorted(callable_tool_names())


def _parse_and_validate(
    argv: list[str],
    stderr: TextIO,
) -> tuple[argparse.Namespace, Tool] | None:
    """Parse argv and resolve the tool, or print a usage error and return None.

    Returns the parsed namespace plus the resolved ``Tool`` on success. Returns
    None for any pre-dispatch problem (argparse error, unknown tool, both arg
    sources given); the caller maps None to ``EXIT_USAGE_ERROR``.
    """
    parser = _build_parser()
    try:
        ns = parser.parse_args(argv)
    except SystemExit:
        # argparse already wrote its own message to stderr.
        return None

    if ns.json_args is not None and ns.arg_pairs:
        print("--json and --arg are mutually exclusive", file=stderr)
        return None

    tool = _find_tool(ns.tool)
    if tool is None:
        print(f"unknown tool: {ns.tool}", file=stderr)
        print(_unknown_tool_hint(ns.tool), file=stderr)
        return None

    return ns, tool


def _unknown_tool_hint(name: str) -> str:
    """Build a recovery hint for an unknown tool name.

    Suggests registry names that share the unknown name's prefix segment (the
    scope, e.g. ``linode_instance``) so a typo in the verb is easy to spot;
    falls back to the catalog pointer when nothing is close.
    """
    prefix = name.rsplit("_", 1)[0] if "_" in name else name
    near = [n for n in _known_tool_names() if n.startswith(prefix)]
    if near:
        listed = ", ".join(near[:10])
        return f"did you mean one of: {listed}"
    return "Run `linodemcp tools --all` to list every tool."


def _assemble_arguments(
    ns: argparse.Namespace,
    tool: Tool,
    stderr: TextIO,
) -> dict[str, Any] | None:
    """Build the dispatch arguments dict from the namespace, or None on error.

    Wraps ``build_arguments`` so an ``ArgError`` (bad json, bad key=value, a
    value that does not fit its schema type) becomes a stderr message and a
    None return the caller maps to ``EXIT_USAGE_ERROR``.
    """
    try:
        return build_arguments(
            tool,
            json_args=ns.json_args,
            arg_pairs=ns.arg_pairs,
            dry_run=ns.dry_run,
            confirm=ns.confirm,
            mode=ns.mode,
            plan_id=ns.plan_id,
            confirmed_dry_run=ns.confirmed_dry_run,
            yolo=ns.yolo,
            environment=ns.environment,
        )
    except ArgError as exc:
        print(str(exc), file=stderr)
        return None


async def run_call_command(
    argv: list[str],
    stdout: TextIO,
    stderr: TextIO,
    config_path: Path | None = None,
) -> int:
    """Entry point for ``linodemcp call``. Async: dispatch is async.

    Validates and assembles before building the runtime so a usage error never
    pays the server-construction cost. ``config_path`` overrides the standard
    config path (tests pass a temp file). Output streams are parameters so
    tests assert on captured text and the exit code.
    """
    parsed = _parse_and_validate(argv, stderr)
    if parsed is None:
        return EXIT_USAGE_ERROR
    ns, tool = parsed

    arguments = _assemble_arguments(ns, tool, stderr)
    if arguments is None:
        return EXIT_USAGE_ERROR

    return await _dispatch_call(
        ns.tool, arguments, ns.output, stdout, stderr, config_path
    )


async def _dispatch_call(
    name: str,
    arguments: dict[str, Any],
    output: str,
    stdout: TextIO,
    stderr: TextIO,
    config_path: Path | None,
) -> int:
    """Open the runtime, dispatch the call, render, and pick the exit code.

    A construction failure (bad config, unknown active profile) prints to
    stderr and exits 2. A tool-level error result prints its message to stderr
    and the raw payload to stdout, exiting 1. Success prints the rendered
    payload and exits 0.
    """
    try:
        async with open_runtime(config_path) as runtime:
            return await _run_and_render(
                runtime, name, arguments, output, stdout, stderr
            )
    except Exception as exc:
        print(f"failed to start runtime: {exc}", file=stderr)
        return EXIT_USAGE_ERROR


async def _run_and_render(
    runtime: OneShotRuntime,
    name: str,
    arguments: dict[str, Any],
    output: str,
    stdout: TextIO,
    stderr: TextIO,
) -> int:
    """Dispatch one call through the runtime's server and render the result.

    A ``ValueError`` from dispatch is the refused-tool path (profile filtered
    the tool out, or an unknown name slipped past validation): exit 2. Anything
    else propagating is an unexpected handler crash: surface it and exit 1.
    """
    try:
        result = await runtime.server.dispatch(name, arguments)
    except ValueError as exc:
        print(str(exc), file=stderr)
        return EXIT_USAGE_ERROR
    except Exception as exc:
        print(f"tool dispatch failed: {exc}", file=stderr)
        return EXIT_TOOL_ERROR

    text = result_text(result)
    if is_error_result(text):
        print(text, file=stderr)
        print(text, file=stdout)
        return EXIT_TOOL_ERROR

    print(render_output(text, output), file=stdout)
    return EXIT_SUCCESS
