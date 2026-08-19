"""Ambient builder-state access for the profile-builder tools.

Python tool handlers take ``(arguments, config)`` with no context parameter, so
the server publishes the state the builder tools read through a ``ContextVar``
they read back. This mirrors the Go side's context injection
(``WithBuilderState`` / ``BuilderStateFromContext`` in ``internal/tools``) and
the two-stage plan store next to it.

The state carries the draft registry the tools mutate, the catalog they compose
a profile against, and the active profile a pre-check answers for. Catalog and
active_profile are callables read at call time rather than snapshots, so a
profile reload reaches an already-registered tool.
"""

from __future__ import annotations

from contextvars import ContextVar
from dataclasses import dataclass
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Callable
    from contextvars import Token

    from linodemcp.profiles import Profile
    from linodemcp.profiles.builder import Registry
    from linodemcp.profiles.builtin import ToolDescriptor


# The sentence every builder tool answers when it was called with no state
# attached. Go answers the same words.
BUILDER_UNCONFIGURED = "draft registry not configured"


@dataclass(frozen=True)
class BuilderState:
    """What the profile-builder tools read off the call."""

    drafts: Registry
    catalog: Callable[[], list[ToolDescriptor]]
    active_profile: Callable[[], Profile]


_BUILDER_STATE: ContextVar[BuilderState | None] = ContextVar(
    "builder_state", default=None
)


def set_builder_state(state: BuilderState | None) -> Token[BuilderState | None]:
    """Publish the state the builder tools read, returning a resetting token."""
    return _BUILDER_STATE.set(state)


def reset_builder_state(token: Token[BuilderState | None]) -> None:
    """Restore the state the matching set_builder_state call replaced."""
    _BUILDER_STATE.reset(token)


def builder_state_from_context() -> BuilderState | None:
    """Return the state the server published, or None when unset.

    None means no server is behind the handler (a unit test calling it
    directly), so the tool refuses rather than answering from nothing.
    """
    return _BUILDER_STATE.get()
