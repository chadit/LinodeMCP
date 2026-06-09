"""Phase 8.2 read-only builder tools.

Two MCP tools enumerate the server's registerable tool surface for the
profile builder workflow:

- ``linode_profile_list_tools``: tool catalog with name, capability,
  and the list of categories each tool belongs to. Optional ``category``
  and ``capability`` filters.
- ``linode_profile_list_categories``: deduplicated category list with
  per-category tool counts, sorted by name.

Both tools carry ``Capability.Meta`` so the profile filter always
admits them, even under the read-only default profile. They never
touch the Linode API.

The handlers read the live tool catalog through a module-level bridge
(``_catalog_provider``) the server installs at startup. Tests inject
reproducible fixtures via :func:`set_tool_catalog_provider`.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.profiles.builtin import categories as resolve_categories

if TYPE_CHECKING:
    from collections.abc import Callable

    from linodemcp.profiles.builtin import ToolDescriptor


# Bridge module state. The Phase 4 server installs the provider during
# __init__; tests install a reproducible stand-in via
# set_tool_catalog_provider. ``None`` is the default; handlers that
# fire before the bridge is wired return an empty catalog rather than
# raising, so unit tests against the handlers can opt into providing a
# fixture or accept "no tools".
_catalog_provider: Callable[[], list[ToolDescriptor]] | None = None


def set_tool_catalog_provider(
    provider: Callable[[], list[ToolDescriptor]] | None,
) -> None:
    """Register the function that returns the live tool catalog.

    Pass ``None`` to clear (used by tests during teardown to avoid
    state bleeding across cases).
    """
    global _catalog_provider  # noqa: PLW0603 - process-wide bridge
    _catalog_provider = provider


def _resolve_catalog() -> list[ToolDescriptor]:
    """Return the live catalog or an empty list when no bridge is set."""
    if _catalog_provider is None:
        return []
    return _catalog_provider()


# Argument-key constants. These are the JSON property names the model
# passes through MCP; hoisted so the handler and schema agree without
# stringly-typed drift.
_ARG_CATEGORY = "category"
_ARG_CAPABILITY = "capability"


def create_linode_profile_list_tools_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_profile_list_tools`` MCP tool definition.

    Schema mirrors the Go side: two optional string filters
    (``category`` and ``capability``), both exact-match (capability
    accepts case-insensitive short or long form, e.g. ``read`` or
    ``CapRead``).
    """
    return (
        Tool(
            name="linode_profile_list_tools",
            description=(
                "List every registerable tool with its capability tag and "
                "categories. Used by the profile builder to enumerate the "
                "full menu before composing a user-defined profile. "
                "Optional filters: category, capability."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    _ARG_CATEGORY: {
                        "type": "string",
                        "description": (
                            "Filter to tools whose categories include this "
                            'exact name (e.g. "compute", "dns").'
                        ),
                    },
                    _ARG_CAPABILITY: {
                        "type": "string",
                        "description": (
                            "Filter to tools with this capability. Accepts "
                            "the short form (read, write, destroy, admin, "
                            "meta) or the long form (CapRead, CapWrite, ...)."
                        ),
                    },
                },
            },
        ),
        Capability.Meta,
    )


def _capability_matches(capability: Capability, filter_value: str) -> bool:
    """Case-insensitive match against the long ("CapRead") or short ("read") form.

    Mirrors the Go ``capabilityMatches`` helper so cross-language tools
    accept the same filter strings.
    """
    long_form = capability.name  # Read, Write, Destroy, Admin, Meta
    cap_form = f"Cap{long_form}"

    needle = filter_value.lower()
    return needle in {long_form.lower(), cap_form.lower()}


async def handle_linode_profile_list_tools(
    arguments: dict[str, Any],
) -> list[TextContent]:
    """Return the filtered tool catalog as JSON text.

    The response shape per entry is ``{name, capability, categories}``
    where ``capability`` is the Capability stringified (``CapRead`` etc.)
    and ``categories`` is the list returned by
    :func:`linodemcp.profiles.builtin.categories`. Both filters narrow
    the result; missing/empty filters do nothing.
    """
    category_filter = arguments.get(_ARG_CATEGORY, "")
    capability_filter = arguments.get(_ARG_CAPABILITY, "")

    entries = _resolve_catalog()
    out: list[dict[str, Any]] = []

    for entry in entries:
        cats = resolve_categories(entry.name)
        if category_filter and category_filter not in cats:
            continue

        if capability_filter and not _capability_matches(
            entry.capability, capability_filter
        ):
            continue

        out.append(
            {
                "name": entry.name,
                "capability": f"Cap{entry.capability.name}",
                "categories": cats,
            }
        )

    return [TextContent(type="text", text=json.dumps(out))]


def create_linode_profile_list_categories_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_profile_list_categories`` MCP tool definition.

    No input arguments; the response is the deduplicated category list
    with tool counts, sorted by name.
    """
    return (
        Tool(
            name="linode_profile_list_categories",
            description=(
                "List tool categories with the number of tools each covers. "
                "Used by the profile builder to discover available "
                "categories before drilling into a category with "
                "linode_profile_list_tools."
            ),
            inputSchema={"type": "object", "properties": {}},
        ),
        Capability.Meta,
    )


async def handle_linode_profile_list_categories(
    arguments: dict[str, Any],  # noqa: ARG001 - no inputs per spec
) -> list[TextContent]:
    """Return ``[{name, tool_count}]`` for every category in the catalog.

    Counts include every category a tool carries (a tool that appears
    in two categories contributes 1 to each). Sorted by name so the
    output is reproducible and the cross-language parity test can
    compare directly.
    """
    entries = _resolve_catalog()
    counts: dict[str, int] = {}

    for entry in entries:
        for cat in resolve_categories(entry.name):
            counts[cat] = counts.get(cat, 0) + 1

    out = [{"name": name, "tool_count": counts[name]} for name in sorted(counts)]

    return [TextContent(type="text", text=json.dumps(out))]
