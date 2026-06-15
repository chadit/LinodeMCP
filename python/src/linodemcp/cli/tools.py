"""``linodemcp tools`` - discover the tool surface without reading source.

Two read-only views over the same registry the server filters by profile:

- ``tools`` (optionally ``--all``) lists tool name + capability. The default is
  the active-profile-filtered set (what ``call`` can actually reach right now);
  ``--all`` shows the full registry.
- ``tools show <tool>`` prints one tool's description, capability, and argument
  schema (name, type, required) so a user can build a ``call`` by hand.

This reads the registry and the resolved active profile directly; it does not
build a server or touch the Linode API. Output streams are parameters so the
views are unit-testable the way the profile subcommands are.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, TextIO

from linodemcp.cli._shared import (
    EXIT_SUCCESS,
    EXIT_USAGE_ERROR,
    required_args,
    schema_properties,
)
from linodemcp.config import get_config_path, load_from_file
from linodemcp.profiles import Capability, ToolDescriptor, resolve_active_profile
from linodemcp.server import get_tool_registry

if TYPE_CHECKING:
    from mcp.types import Tool

TOOLS_USAGE = """\
Usage: linodemcp tools [--all]
       linodemcp tools show <tool>

  (no args)     List tools available under the active profile.
  --all         List every registered tool, ignoring the profile filter.
  show <tool>   Show a tool's description, capability, and argument schema.\
"""

# Column widths for the `tools` list. Extracted so header and rows align.
_COL_NAME = 48
_COL_CAP = 8


def run_tools_command(args: list[str], stdout: TextIO, stderr: TextIO) -> int:
    """Dispatch ``linodemcp tools ...`` to the list or show view.

    ``show`` routes to the detail view; ``--all`` or no args route to the list.
    Any other token is a usage error so a typo does not silently list.
    """
    if args and args[0] == "show":
        return _run_tools_show(args[1:], stdout, stderr)
    if not args:
        return _run_tools_list(show_all=False, stdout=stdout, stderr=stderr)
    if args == ["--all"]:
        return _run_tools_list(show_all=True, stdout=stdout, stderr=stderr)

    print(f"unknown tools arguments: {args}\n\n{TOOLS_USAGE}", file=stderr)
    return EXIT_USAGE_ERROR


def _capability_label(capability: Capability) -> str:
    """Lowercase capability name for display (read/write/destroy/admin/meta)."""
    return capability.name.lower()


def _active_profile_tool_names(stderr: TextIO) -> frozenset[str] | None:
    """Resolve the active profile's allowed tool names from the config.

    Returns None on a config-load or profile-resolution failure after writing a
    friendly message; the caller maps None to a non-zero exit. The descriptor
    list is built from the same registry the server uses, so the filter matches
    what dispatch would allow.
    """
    try:
        cfg = load_from_file(get_config_path())
    except Exception as exc:
        print(f"load config: {exc}", file=stderr)
        return None

    descriptors = [
        ToolDescriptor(name=entry.name, capability=entry.capability)
        for entry in get_tool_registry()
    ]
    try:
        profile = resolve_active_profile(cfg, descriptors)
    except Exception as exc:
        print(f"resolve active profile: {exc}", file=stderr)
        return None

    return frozenset(profile.allowed_tools)


def _run_tools_list(*, show_all: bool, stdout: TextIO, stderr: TextIO) -> int:
    """List tool name + capability, filtered by the active profile by default.

    With ``show_all`` the full registry prints. Otherwise only tools the active
    profile allows are shown, which is exactly the set ``call`` can dispatch.
    """
    allowed: frozenset[str] | None = None
    if not show_all:
        allowed = _active_profile_tool_names(stderr)
        if allowed is None:
            return EXIT_USAGE_ERROR

    header = f"{'name':<{_COL_NAME}} {'capability':<{_COL_CAP}}"
    print(header, file=stdout)

    count = 0
    for entry in sorted(get_tool_registry(), key=lambda e: e.name):
        if allowed is not None and entry.name not in allowed:
            continue
        cap = _capability_label(entry.capability)
        print(f"{entry.name:<{_COL_NAME}} {cap:<{_COL_CAP}}", file=stdout)
        count += 1

    scope = "all" if show_all else "active profile"
    print(f"\n{count} tools ({scope})", file=stdout)
    return EXIT_SUCCESS


def _find_tool(name: str) -> tuple[Tool, Capability] | None:
    """Look up a tool's definition and capability by name in the registry."""
    for entry in get_tool_registry():
        if entry.name == name:
            return entry.tool, entry.capability
    return None


def _run_tools_show(args: list[str], stdout: TextIO, stderr: TextIO) -> int:
    """Print one tool's description, capability, and argument schema.

    Unknown names exit 2 (a usage problem: the requested tool does not exist).
    The schema rows mark required arguments so the user knows the minimum
    ``call`` needs.
    """
    if len(args) != 1:
        print("Usage: linodemcp tools show <tool>", file=stderr)
        return EXIT_USAGE_ERROR

    name = args[0]
    found = _find_tool(name)
    if found is None:
        print(f"unknown tool: {name}", file=stderr)
        return EXIT_USAGE_ERROR

    tool, capability = found
    _print_tool_detail(stdout, tool, capability)
    return EXIT_SUCCESS


def _print_tool_detail(stdout: TextIO, tool: Tool, capability: Capability) -> None:
    """Write one tool's detail in a stable, human-readable shape.

    Exported-style helper kept separate so a test can assert on the formatting
    without the lookup. Required arguments are flagged; absent description or
    empty schema print explicit placeholders rather than blank lines.
    """
    print(f"Tool: {tool.name}", file=stdout)
    print(f"Capability: {_capability_label(capability)}", file=stdout)
    print(f"Description: {tool.description or '(none)'}", file=stdout)

    properties = schema_properties(tool)
    required = set(required_args(tool))
    if not properties:
        print("Arguments: (none)", file=stdout)
        return

    print(f"Arguments ({len(properties)}):", file=stdout)
    for arg_name in sorted(properties):
        prop = properties[arg_name]
        arg_type = str(prop.get("type", "any"))
        flag = "required" if arg_name in required else "optional"
        print(f"  {arg_name} ({arg_type}, {flag})", file=stdout)
