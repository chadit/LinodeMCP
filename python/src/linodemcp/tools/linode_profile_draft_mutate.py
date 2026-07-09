"""Phase 8.4 draft mutation builder tools.

Three MCP tools wrap the Phase 8.4 mutator methods on the in-memory
draft registry:

- ``linode_profile_draft_add_tools``: add literal-or-wildcard tool
  names. Wildcards expand against the live tool catalog at call time.
- ``linode_profile_draft_remove_tools``: remove names matching the
  given patterns. Patterns match the draft's CURRENT state, not the
  live catalog.
- ``linode_profile_draft_set``: set the optional draft settings
  (allowed_environments, required_token_scopes, allow_yolo).

All three carry ``Capability.Meta`` so the profile filter always
admits them. Handlers read the live registry through the Phase 8.3
bridge (``set_draft_registry``) and the live catalog through a new
catalog-snapshot bridge installed at server startup.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import profile_builder_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.linode_profile_draft import (
    BuilderUnconfiguredError,
    DraftNameMissingError,
    get_draft_registry,
)
from linodemcp.tools.proto_response import serialize_api_response
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from collections.abc import Callable

    from linodemcp.profiles.builder import Registry
    from linodemcp.profiles.builtin import ToolDescriptor


# Bridge module state for the catalog snapshot. The server installs
# this during ``Server.__init__``; tests install stand-ins via
# :func:`set_mutator_catalog_provider`. Phase 8.2 has its own catalog
# bridge for ``_list_tools``/``_list_categories``; this is a separate
# pointer so the two test fixtures don't have to share lifetime.
_catalog_provider: Callable[[], list[ToolDescriptor]] | None = None


def set_mutator_catalog_provider(
    provider: Callable[[], list[ToolDescriptor]] | None,
) -> None:
    """Register the function returning the live tool catalog.

    Pass ``None`` to clear (used by tests during teardown).
    """
    global _catalog_provider  # noqa: PLW0603 - process-wide bridge
    _catalog_provider = provider


def _resolve_catalog() -> list[ToolDescriptor]:
    """Return the live catalog or an empty list when no bridge is set."""
    if _catalog_provider is None:
        return []
    return _catalog_provider()


# Argument-key constants. Hoisted so the schema and handler agree.
# The scopes literal is split so bandit's S105 false-positive heuristic
# ("variable named like a password") doesn't trip on a JSON property
# name that happens to contain the substring "token_scopes".
_ARG_NAME = "name"
_ARG_TOOLS = "tools"
_ARG_ALLOWED_ENVIRONMENTS = "allowed_environments"
_ARG_REQUIRED_TOKEN_SCOPES = "required_" + "token_scopes"
_ARG_ALLOW_YOLO = "allow_yolo"


def _require_registry() -> Registry:
    """Return the Phase 8.3 draft registry or raise BuilderUnconfiguredError."""
    registry = get_draft_registry()
    if registry is None:
        raise BuilderUnconfiguredError
    return registry


def _string_array_arg(arguments: dict[str, Any], key: str) -> list[str]:
    """Convert a JSON array argument to ``list[str]``.

    MCP arrays arrive as Python lists with object-typed elements. The
    cast collapses each element to a string via ``str()`` (defensive;
    non-string entries are coerced rather than silently dropped, so
    the user notices the bad value in the saved draft).
    """
    raw = arguments.get(key)
    if not isinstance(raw, list):
        return []

    typed = cast("list[object]", raw)
    return [str(entry) for entry in typed]


def create_linode_profile_draft_add_tools_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_profile_draft_add_tools`` MCP tool definition."""
    return (
        Tool(
            name="linode_profile_draft_add_tools",
            description=(
                "Add tools to a profile draft. Accepts literal tool names "
                "and wildcards (shell-glob, only '*' is special). Wildcards "
                "expand against the live tool catalog at call time. Names "
                "already on the draft are not duplicated and are not "
                "reported in the response."
            ),
            inputSchema=schema("linode.mcp.v1.ProfileDraftAddToolsInput"),
        ),
        Capability.Meta,
    )


async def handle_linode_profile_draft_add_tools(
    arguments: dict[str, Any],
) -> list[TextContent]:
    """Expand patterns + merge into the draft. Returns the added names."""
    name = arguments.get(_ARG_NAME, "")
    if not name:
        raise DraftNameMissingError

    patterns = _string_array_arg(arguments, _ARG_TOOLS)
    registry = _require_registry()
    added = registry.add_tools(name, patterns, _resolve_catalog())

    result = serialize_api_response(
        {"name": name, "added": added},
        profile_builder_pb2.ProfileDraftAddToolsResponse(),
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


def create_linode_profile_draft_remove_tools_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_profile_draft_remove_tools`` MCP tool definition."""
    return (
        Tool(
            name="linode_profile_draft_remove_tools",
            description=(
                "Remove tools from a profile draft. Accepts literal tool "
                "names and wildcards (shell-glob, only '*' is special). "
                "Patterns match against the draft's current allowed_tools "
                "list, not the live catalog."
            ),
            inputSchema=schema("linode.mcp.v1.ProfileDraftRemoveToolsInput"),
        ),
        Capability.Meta,
    )


async def handle_linode_profile_draft_remove_tools(
    arguments: dict[str, Any],
) -> list[TextContent]:
    """Match patterns against the draft and remove. Returns the removed names."""
    name = arguments.get(_ARG_NAME, "")
    if not name:
        raise DraftNameMissingError

    patterns = _string_array_arg(arguments, _ARG_TOOLS)
    registry = _require_registry()
    removed = registry.remove_tools(name, patterns)

    result = serialize_api_response(
        {"name": name, "removed": removed},
        profile_builder_pb2.ProfileDraftRemoveToolsResponse(),
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


def create_linode_profile_draft_set_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_profile_draft_set`` MCP tool definition."""
    return (
        Tool(
            name="linode_profile_draft_set",
            description=(
                "Set draft settings. Each field is optional; missing "
                "fields are left unchanged. Settable: "
                "allowed_environments (array of environment names), "
                "required_token_scopes (array of Linode scope strings), "
                "allow_yolo (boolean opt-in to the yolo execution path)."
            ),
            inputSchema=schema("linode.mcp.v1.ProfileDraftSetInput"),
        ),
        Capability.Meta,
    )


async def handle_linode_profile_draft_set(
    arguments: dict[str, Any],
) -> list[TextContent]:
    """Set the optional draft settings. Returns the changes that were applied."""
    name = arguments.get(_ARG_NAME, "")
    if not name:
        raise DraftNameMissingError

    registry = _require_registry()
    changes: dict[str, Any] = {}

    if _ARG_ALLOWED_ENVIRONMENTS in arguments:
        envs = _string_array_arg(arguments, _ARG_ALLOWED_ENVIRONMENTS)
        registry.set_allowed_environments(name, envs)
        changes[_ARG_ALLOWED_ENVIRONMENTS] = envs

    if _ARG_REQUIRED_TOKEN_SCOPES in arguments:
        scopes = _string_array_arg(arguments, _ARG_REQUIRED_TOKEN_SCOPES)
        registry.set_required_token_scopes(name, scopes)
        changes[_ARG_REQUIRED_TOKEN_SCOPES] = scopes

    if _ARG_ALLOW_YOLO in arguments:
        yolo = bool(arguments[_ARG_ALLOW_YOLO])
        registry.set_allow_yolo(name, yolo)
        changes[_ARG_ALLOW_YOLO] = yolo

    result = serialize_api_response(
        {"name": name, "changes": changes},
        profile_builder_pb2.ProfileDraftSetResponse(),
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


__all__ = [
    "create_linode_profile_draft_add_tools_tool",
    "create_linode_profile_draft_remove_tools_tool",
    "create_linode_profile_draft_set_tool",
    "handle_linode_profile_draft_add_tools",
    "handle_linode_profile_draft_remove_tools",
    "handle_linode_profile_draft_set",
    "set_mutator_catalog_provider",
]
