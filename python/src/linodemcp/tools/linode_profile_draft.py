"""Phase 8.3 draft lifecycle builder tools.

Three MCP tools wrap the in-memory draft registry from Phase 8.1:

- ``linode_profile_draft_new``: create a draft, optionally seeded from
  an existing profile via ``clone_from``.
- ``linode_profile_draft_show``: read a draft's current state.
- ``linode_profile_draft_discard``: remove a draft. Idempotent.

All three carry ``Capability.Meta`` so the profile filter always
admits them; they never touch the Linode API. The generator owns their
registration; what is left here is the body each one's answer hook calls. They
read the live registry off the builder state the server publishes for the call
(:mod:`linodemcp.tools.builderstate`); the clone source resolves against
the running config and that same state's catalog.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

from mcp.types import TextContent

from linodemcp.genpb.linode.mcp.v1 import profile_builder_pb2
from linodemcp.profiles.builder import (
    Draft,
    DraftExistsError,
)
from linodemcp.profiles.loader import lookup_profile
from linodemcp.tools.builderstate import (
    BUILDER_UNCONFIGURED,
    builder_state_from_context,
)
from linodemcp.tools.helpers import error_response
from linodemcp.tools.proto_response import serialize_api_response

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.profiles.profile import Profile


# The sentences the draft tools answer. Go spells them the same way, so a
# caller reads one refusal whichever binary served it.
NAME_MISSING = "name argument is required"


def draft_not_found(name: str) -> str:
    """The sentence every builder tool answers for a name no draft carries."""
    return f"draft not found: {name}"


# Argument-key constants. Used both in the schema and the handler so
# they can't drift.
_ARG_NAME = "name"
_ARG_CLONE_FROM = "clone_from"


def _draft_to_payload(draft: Draft) -> dict[str, Any]:
    """Serialize a Draft into the wire shape.

    The JSON tags match the Go side so cross-language tooling sees
    identical payloads. Empty lists serialize as ``[]`` not ``null``;
    the Draft dataclass already initializes lists, so no substitution
    is needed.
    """
    return {
        "name": draft.name,
        "description": draft.description,
        "allowed_tools": list(draft.allowed_tools),
        "allowed_environments": list(draft.allowed_environments),
        "required_token_scopes": list(draft.required_token_scopes),
        "allow_yolo": draft.allow_yolo,
    }


def profile_draft_new_result(
    arguments: dict[str, Any],
    cfg: Config,
) -> list[TextContent]:
    """Create a new draft and return its JSON representation.

    Refuses with a tool result, the way every other tool reports a bad
    argument, when the name is empty, the clone source resolves to nothing, or
    a draft already holds the name.
    """
    state = builder_state_from_context()
    if state is None:
        return error_response(BUILDER_UNCONFIGURED)

    name = arguments.get(_ARG_NAME, "")
    if not name:
        return error_response(NAME_MISSING)

    clone_from = arguments.get(_ARG_CLONE_FROM, "")

    source: Profile | None = None

    if clone_from:
        source = lookup_profile(clone_from, cfg, state.catalog())
        if source is None:
            return error_response(f"clone_from profile not found: {clone_from}")

    try:
        draft = state.drafts.create(name, source)
    except DraftExistsError:
        return error_response(f"draft already exists: {name}")

    result = serialize_api_response(
        _draft_to_payload(draft), profile_builder_pb2.ProfileDraftResponse()
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


def profile_draft_show_result(arguments: dict[str, Any]) -> list[TextContent]:
    """Read a draft and return its JSON representation.

    A missing name and a name no draft carries both refuse with a tool result
    the model can correct from.
    """
    state = builder_state_from_context()
    if state is None:
        return error_response(BUILDER_UNCONFIGURED)

    name = arguments.get(_ARG_NAME, "")
    if not name:
        return error_response(NAME_MISSING)

    draft = state.drafts.get(name)
    if draft is None:
        return error_response(draft_not_found(name))

    result = serialize_api_response(
        _draft_to_payload(draft), profile_builder_pb2.ProfileDraftResponse()
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


def profile_draft_discard_result(arguments: dict[str, Any]) -> list[TextContent]:
    """Remove a draft. Returns ``{name, discarded}``.

    ``discarded`` is True if the draft existed; False if not. Either case
    returns a normal response, so only an absent name refuses.
    """
    state = builder_state_from_context()
    if state is None:
        return error_response(BUILDER_UNCONFIGURED)

    name = arguments.get(_ARG_NAME, "")
    if not name:
        return error_response(NAME_MISSING)

    removed = state.drafts.discard(name)

    result = serialize_api_response(
        {"name": name, "discarded": removed},
        profile_builder_pb2.ProfileDraftDiscardResponse(),
    )
    return [TextContent(type="text", text=json.dumps(result, indent=2))]


__all__ = [
    "NAME_MISSING",
    "draft_not_found",
    "profile_draft_discard_result",
    "profile_draft_new_result",
    "profile_draft_show_result",
]
