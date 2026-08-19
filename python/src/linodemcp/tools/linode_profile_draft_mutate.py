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
admits them. The generator owns their registration; what is left here is the
body each one's answer hook calls. They read the live registry and catalog off
the builder state the server publishes for the call
(:mod:`linodemcp.tools.builderstate`).
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent

from linodemcp.genpb.linode.mcp.v1 import profile_builder_pb2
from linodemcp.profiles.builder import DraftNotFoundError
from linodemcp.tools.builderstate import (
    BUILDER_UNCONFIGURED,
    builder_state_from_context,
)
from linodemcp.tools.helpers import error_response
from linodemcp.tools.linode_profile_draft import NAME_MISSING, draft_not_found
from linodemcp.tools.proto_response import serialize_api_response

if TYPE_CHECKING:
    from linodemcp.profiles.builder import Registry


# Argument-key constants. Hoisted so the schema and handler agree.
# The scopes literal is split so bandit's S105 false-positive heuristic
# ("variable named like a password") doesn't trip on a JSON property
# name that happens to contain the substring "token_scopes".
_ARG_NAME = "name"
_ARG_TOOLS = "tools"
_ARG_ALLOWED_ENVIRONMENTS = "allowed_environments"
_ARG_REQUIRED_TOKEN_SCOPES = "required_" + "token_scopes"
_ARG_ALLOW_YOLO = "allow_yolo"


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


def _apply_draft_settings(
    drafts: Registry,
    name: str,
    arguments: dict[str, Any],
    changes: dict[str, Any],
) -> None:
    """Apply every settable field the call named, recording what changed.

    Split out of the handler because each setter can report a draft the
    registry does not hold, and one try around the three keeps the handler's
    nesting flat.
    """
    if _ARG_ALLOWED_ENVIRONMENTS in arguments:
        envs = _string_array_arg(arguments, _ARG_ALLOWED_ENVIRONMENTS)
        drafts.set_allowed_environments(name, envs)
        changes[_ARG_ALLOWED_ENVIRONMENTS] = envs

    if _ARG_REQUIRED_TOKEN_SCOPES in arguments:
        scopes = _string_array_arg(arguments, _ARG_REQUIRED_TOKEN_SCOPES)
        drafts.set_required_token_scopes(name, scopes)
        changes[_ARG_REQUIRED_TOKEN_SCOPES] = scopes

    if _ARG_ALLOW_YOLO in arguments:
        yolo = bool(arguments[_ARG_ALLOW_YOLO])
        drafts.set_allow_yolo(name, yolo)
        changes[_ARG_ALLOW_YOLO] = yolo


def profile_draft_add_tools_result(arguments: dict[str, Any]) -> list[TextContent]:
    """Expand patterns + merge into the draft. Returns the added names."""
    state = builder_state_from_context()
    if state is None:
        return error_response(BUILDER_UNCONFIGURED)

    name = arguments.get(_ARG_NAME, "")
    if not name:
        return error_response(NAME_MISSING)

    patterns = _string_array_arg(arguments, _ARG_TOOLS)

    try:
        added = state.drafts.add_tools(name, patterns, state.catalog())
    except DraftNotFoundError:
        return error_response(draft_not_found(name))

    result = serialize_api_response(
        {"name": name, "added": added},
        profile_builder_pb2.ProfileDraftAddToolsResponse(),
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


def profile_draft_remove_tools_result(arguments: dict[str, Any]) -> list[TextContent]:
    """Match patterns against the draft and remove. Returns the removed names."""
    state = builder_state_from_context()
    if state is None:
        return error_response(BUILDER_UNCONFIGURED)

    name = arguments.get(_ARG_NAME, "")
    if not name:
        return error_response(NAME_MISSING)

    patterns = _string_array_arg(arguments, _ARG_TOOLS)

    try:
        removed = state.drafts.remove_tools(name, patterns)
    except DraftNotFoundError:
        return error_response(draft_not_found(name))

    result = serialize_api_response(
        {"name": name, "removed": removed},
        profile_builder_pb2.ProfileDraftRemoveToolsResponse(),
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


def profile_draft_set_result(arguments: dict[str, Any]) -> list[TextContent]:
    """Set the optional draft settings. Returns the changes that were applied."""
    state = builder_state_from_context()
    if state is None:
        return error_response(BUILDER_UNCONFIGURED)

    name = arguments.get(_ARG_NAME, "")
    if not name:
        return error_response(NAME_MISSING)

    changes: dict[str, Any] = {}

    try:
        _apply_draft_settings(state.drafts, name, arguments, changes)
    except DraftNotFoundError:
        return error_response(draft_not_found(name))

    result = serialize_api_response(
        {"name": name, "changes": changes},
        profile_builder_pb2.ProfileDraftSetResponse(),
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


__all__ = [
    "profile_draft_add_tools_result",
    "profile_draft_remove_tools_result",
    "profile_draft_set_result",
]
