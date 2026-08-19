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
touch the Linode API. The generator owns their registration; what is left
here is the body each one's answer hook calls.

The handlers read the live tool catalog off the builder state the server
publishes for the call (:mod:`linodemcp.tools.builderstate`), so a reload
reaches an already-registered tool and a handler reached without a server
refuses rather than answering from an empty catalog.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

from mcp.types import TextContent

from linodemcp.genpb.linode.mcp.v1 import profile_builder_pb2
from linodemcp.profiles.builtin import categories as resolve_categories
from linodemcp.tools.builderstate import (
    BUILDER_UNCONFIGURED,
    builder_state_from_context,
)
from linodemcp.tools.helpers import error_response
from linodemcp.tools.proto_response import serialize_api_response

if TYPE_CHECKING:
    from linodemcp.profiles import Capability

# Argument-key constants. These are the JSON property names the model
# passes through MCP; hoisted so the handler and schema agree without
# stringly-typed drift.
_ARG_CATEGORY = "category"
_ARG_CAPABILITY = "capability"


def _capability_matches(capability: Capability, filter_value: str) -> bool:
    """Case-insensitive match against the long ("CapRead") or short ("read") form.

    Mirrors the Go ``capabilityMatches`` helper so cross-language tools
    accept the same filter strings.
    """
    long_form = capability.name  # Read, Write, Destroy, Admin, Meta
    cap_form = f"Cap{long_form}"

    needle = filter_value.lower()
    return needle in {long_form.lower(), cap_form.lower()}


def profile_list_tools_result(arguments: dict[str, Any]) -> list[TextContent]:
    """Return the filtered tool catalog as JSON text.

    The response shape per entry is ``{name, capability, categories}``
    where ``capability`` is the Capability stringified (``CapRead`` etc.)
    and ``categories`` is the list returned by
    :func:`linodemcp.profiles.builtin.categories`. Both filters narrow
    the result; missing/empty filters do nothing.
    """
    state = builder_state_from_context()
    if state is None:
        return error_response(BUILDER_UNCONFIGURED)

    category_filter = arguments.get(_ARG_CATEGORY, "")
    capability_filter = arguments.get(_ARG_CAPABILITY, "")

    entries = state.catalog()
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

    # Name-sorted. The catalog itself is in registration order, which differs
    # per language, so sorting here is what makes one tool answer one order
    # whichever binary served it.
    out.sort(key=lambda entry: entry["name"])

    result = serialize_api_response(
        {"count": len(out), "tools": out},
        profile_builder_pb2.ProfileToolListResponse(),
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


def profile_list_categories_result(arguments: dict[str, Any]) -> list[TextContent]:
    """Return ``[{name, tool_count}]`` for every category in the catalog.

    Counts include every category a tool carries (a tool that appears
    in two categories contributes 1 to each). Sorted by name so the
    output is reproducible and the cross-language parity test can
    compare directly.
    """
    # The tool takes no inputs per spec; del marks the parameter used.
    del arguments

    state = builder_state_from_context()
    if state is None:
        return error_response(BUILDER_UNCONFIGURED)

    entries = state.catalog()
    counts: dict[str, int] = {}

    for entry in entries:
        for cat in resolve_categories(entry.name):
            counts[cat] = counts.get(cat, 0) + 1

    out = [{"name": name, "tool_count": counts[name]} for name in sorted(counts)]

    result = serialize_api_response(
        {"count": len(out), "categories": out},
        profile_builder_pb2.ProfileCategoryListResponse(),
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]
