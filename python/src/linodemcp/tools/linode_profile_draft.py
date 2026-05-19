"""Phase 8.3 draft lifecycle builder tools.

Three MCP tools wrap the in-memory draft registry from Phase 8.1:

- ``linode_profile_draft_new``: create a draft, optionally seeded from
  an existing profile via ``clone_from``.
- ``linode_profile_draft_show``: read a draft's current state.
- ``linode_profile_draft_discard``: remove a draft. Idempotent.

All three carry ``Capability.Meta`` so the profile filter always
admits them; they never touch the Linode API. Handlers read the live
registry and profile resolver through module-level bridges the server
installs at startup. Tests inject deterministic stand-ins via
:func:`set_draft_registry` and :func:`set_profile_resolver`.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.profiles.builder import (
    Draft,
    DraftExistsError,
    Registry,
)

if TYPE_CHECKING:
    from collections.abc import Callable

    from linodemcp.profiles.profile import Profile


# Bridges. ``None`` is the default; the server installs concrete values
# during ``Server.__init__``; tests install stand-ins via the setters.
_draft_registry: Registry | None = None
_profile_resolver: Callable[[str], Profile | None] | None = None


def set_draft_registry(registry: Registry | None) -> None:
    """Register the live draft registry. Pass ``None`` to clear."""
    global _draft_registry  # noqa: PLW0603 - process-wide bridge
    _draft_registry = registry


def get_draft_registry() -> Registry | None:
    """Return the registered draft registry, or ``None`` if unset.

    Phase 8.4 mutator handlers read the registry through this getter
    rather than touching the module-private ``_draft_registry``
    directly; pyright strict's reportPrivateUsage flags the
    underscore-prefixed name when imported from outside this module.
    """
    return _draft_registry


def set_profile_resolver(
    resolver: Callable[[str], Profile | None] | None,
) -> None:
    """Register the profile-by-name resolver. Pass ``None`` to clear.

    The resolver should return the materialized ``Profile`` for the
    given name (built-in or user-defined) or ``None`` if no such
    profile exists. The Phase 8.3 ``_draft_new`` handler uses it to
    seed clones via the ``clone_from`` argument.
    """
    global _profile_resolver  # noqa: PLW0603 - process-wide bridge
    _profile_resolver = resolver


# Exception classes. Ruff N818 enforces the ``Error`` suffix.


class DraftNameMissingError(ValueError):
    """The ``name`` argument was empty."""

    def __init__(self) -> None:
        super().__init__("name argument is required")


class CloneSourceMissingError(ValueError):
    """``clone_from`` named a profile that doesn't exist."""

    def __init__(self, name: str) -> None:
        super().__init__(f"clone_from profile not found: {name}")
        self.profile_name = name


class DraftNotFoundError(LookupError):
    """The named draft is not in the registry.

    Distinct from :class:`linodemcp.profiles.builder.DraftExistsError`
    which fires on Create. This one fires on Show.
    """

    def __init__(self, name: str) -> None:
        super().__init__(f"draft not found: {name}")
        self.draft_name = name


class BuilderUnconfiguredError(RuntimeError):
    """No draft registry is wired.

    The server installs the bridge in ``Server.__init__``. This
    exception fires only when the handlers are invoked before the
    bridge is set; in production paths it should never trigger.
    """

    def __init__(self) -> None:
        super().__init__("draft registry not configured")


# Argument-key constants. Used both in the schema and the handler so
# they can't drift.
_ARG_NAME = "name"
_ARG_CLONE_FROM = "clone_from"


def _require_registry() -> Registry:
    """Return the live registry or raise BuilderUnconfiguredError."""
    if _draft_registry is None:
        raise BuilderUnconfiguredError
    return _draft_registry


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


def create_linode_profile_draft_new_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_profile_draft_new`` MCP tool definition.

    Schema mirrors the Go side: required ``name`` and optional
    ``clone_from``.
    """
    return (
        Tool(
            name="linode_profile_draft_new",
            description=(
                "Start a new profile draft in the server's in-memory "
                "builder registry. Optional clone_from seeds the draft "
                "from an existing built-in or user-defined profile. The "
                "draft persists only for this server's lifetime; use "
                "linode_profile_draft_save (Phase 8.5) to write it to "
                "the config file."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    _ARG_NAME: {
                        "type": "string",
                        "description": (
                            "Name for the new draft. Must be unique "
                            "within the registry."
                        ),
                    },
                    _ARG_CLONE_FROM: {
                        "type": "string",
                        "description": (
                            "Optional profile name to seed the draft "
                            "from. Resolves against built-ins and "
                            "user-defined profiles; user-defined shadow "
                            "built-ins by name."
                        ),
                    },
                },
                "required": [_ARG_NAME],
            },
        ),
        Capability.Meta,
    )


async def handle_linode_profile_draft_new(
    arguments: dict[str, Any],
) -> list[TextContent]:
    """Create a new draft and return its JSON representation.

    Raises:
        DraftNameMissingError: ``name`` argument is empty.
        CloneSourceMissingError: ``clone_from`` is non-empty but no
            profile by that name exists.
        DraftExistsError: a draft with that name already lives in the
            registry. The user must discard first.
    """
    name = arguments.get(_ARG_NAME, "")
    if not name:
        raise DraftNameMissingError

    clone_from = arguments.get(_ARG_CLONE_FROM, "")

    source: Profile | None = None

    if clone_from:
        if _profile_resolver is None:
            raise BuilderUnconfiguredError

        source = _profile_resolver(clone_from)
        if source is None:
            raise CloneSourceMissingError(clone_from)

    registry = _require_registry()
    draft = registry.create(name, source)

    payload = json.dumps(_draft_to_payload(draft))
    return [TextContent(type="text", text=payload)]


def create_linode_profile_draft_show_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_profile_draft_show`` MCP tool definition."""
    return (
        Tool(
            name="linode_profile_draft_show",
            description=(
                "Show the current state of a profile draft. Returns "
                "name, description, allowed tools, allowed "
                "environments, required token scopes, and the "
                "allow_yolo flag."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    _ARG_NAME: {
                        "type": "string",
                        "description": "Draft name to show.",
                    },
                },
                "required": [_ARG_NAME],
            },
        ),
        Capability.Meta,
    )


async def handle_linode_profile_draft_show(
    arguments: dict[str, Any],
) -> list[TextContent]:
    """Read a draft and return its JSON representation.

    Raises:
        DraftNameMissingError: ``name`` argument is empty.
        DraftNotFoundError: no draft by that name exists.
    """
    name = arguments.get(_ARG_NAME, "")
    if not name:
        raise DraftNameMissingError

    registry = _require_registry()
    draft = registry.get(name)
    if draft is None:
        raise DraftNotFoundError(name)

    payload = json.dumps(_draft_to_payload(draft))
    return [TextContent(type="text", text=payload)]


def create_linode_profile_draft_discard_tool() -> tuple[Tool, Capability]:
    """Build the ``linode_profile_draft_discard`` MCP tool definition."""
    return (
        Tool(
            name="linode_profile_draft_discard",
            description=(
                "Discard a profile draft. Idempotent: returns "
                '{"discarded": false} when the draft does not exist '
                "(no error), so the model can call it from cleanup "
                "paths without first checking existence."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    _ARG_NAME: {
                        "type": "string",
                        "description": "Draft name to discard.",
                    },
                },
                "required": [_ARG_NAME],
            },
        ),
        Capability.Meta,
    )


async def handle_linode_profile_draft_discard(
    arguments: dict[str, Any],
) -> list[TextContent]:
    """Remove a draft. Returns ``{name, discarded}``.

    ``discarded`` is True if the draft existed; False if not. Either
    case returns a normal response (no exception).

    Raises:
        DraftNameMissingError: ``name`` argument is empty.
    """
    name = arguments.get(_ARG_NAME, "")
    if not name:
        raise DraftNameMissingError

    registry = _require_registry()
    removed = registry.discard(name)

    payload = json.dumps({"name": name, "discarded": removed})
    return [TextContent(type="text", text=payload)]


# Re-export DraftExistsError so tests can match without importing from
# the builder package directly. The handler propagates it from
# Registry.create unchanged.
__all__ = [
    "BuilderUnconfiguredError",
    "CloneSourceMissingError",
    "DraftExistsError",
    "DraftNameMissingError",
    "DraftNotFoundError",
    "create_linode_profile_draft_discard_tool",
    "create_linode_profile_draft_new_tool",
    "create_linode_profile_draft_show_tool",
    "get_draft_registry",
    "handle_linode_profile_draft_discard",
    "handle_linode_profile_draft_new",
    "handle_linode_profile_draft_show",
    "set_draft_registry",
    "set_profile_resolver",
]
