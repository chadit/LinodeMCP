"""In-memory draft registry used by the Phase 8 profile builder tools.

A ``Draft`` is a mutable, server-process-local snapshot of a ``Profile``
under construction. Drafts do not persist across restarts. Phase 8.5
``draft_save`` is the bridge from a ``Draft`` back into the config file.

This module is intentionally independent of the server and MCP wire
layers. Phase 8.2 onward wraps ``Registry`` operations in tool handlers;
the wrapping lives in ``linodemcp.tools``, not here.
"""

from __future__ import annotations

import threading
from dataclasses import dataclass, field
from typing import TYPE_CHECKING

from linodemcp.profiles.builder.errors import (
    DraftExistsError,
    DraftNameEmptyError,
    DraftNotFoundError,
)
from linodemcp.profiles.builder.match import match_patterns
from linodemcp.profiles.builtin import ToolDescriptor
from linodemcp.profiles.capability import Capability

if TYPE_CHECKING:
    from collections.abc import Sequence

    from linodemcp.profiles.profile import Profile


@dataclass
class Draft:
    """In-memory shape of a profile under construction.

    Field semantics mirror :class:`linodemcp.profiles.profile.Profile`
    so the Phase 8.5 save step can produce a ``UserProfileConfig``
    without translation. ``disabled`` is intentionally absent: drafts
    cannot be saved into a disabled state.

    Lists are used rather than tuples since drafts mutate field-by-field
    via builder tool handlers (Phase 8.4).
    """

    name: str
    description: str = ""
    allowed_tools: list[str] = field(default_factory=list[str])
    allowed_environments: list[str] = field(default_factory=list[str])
    required_token_scopes: list[str] = field(default_factory=list[str])
    allow_yolo: bool = False


class Registry:
    """Holds drafts keyed by name. Safe for concurrent use.

    The MCP server has exactly one ``Registry`` per process; each builder
    tool handler resolves it via ``Server.builder_registry``. Drafts
    share the registry across concurrent tool calls so a ``_show`` can
    race with an ``_add_tools``. The lock serializes mutations.
    """

    def __init__(self) -> None:
        self._lock = threading.RLock()
        self._drafts: dict[str, Draft] = {}

    def create(self, name: str, clone_from: Profile | None = None) -> Draft:
        """Start a new draft.

        If ``clone_from`` is provided, the draft seeds its fields from
        that profile. The seeded sequences are copied into fresh lists
        so later edits to the draft do not mutate the source profile.

        :raises DraftNameEmptyError: when ``name`` is empty.
        :raises DraftExistsError: when a draft by that name already
            lives in the registry. Refuse silent overwrite so a stray
            reroll doesn't lose work.
        """
        if not name:
            raise DraftNameEmptyError

        with self._lock:
            if name in self._drafts:
                raise DraftExistsError(name)

            if clone_from is None:
                draft = Draft(name=name)
            else:
                draft = Draft(
                    name=name,
                    description=clone_from.description,
                    allowed_tools=list(clone_from.allowed_tools),
                    allowed_environments=list(clone_from.allowed_environments),
                    required_token_scopes=list(clone_from.required_token_scopes),
                    allow_yolo=clone_from.allow_yolo,
                )

            self._drafts[name] = draft

            return draft

    def get(self, name: str) -> Draft | None:
        """Return the named draft or ``None`` if absent.

        The returned reference IS the registry's own draft; callers that
        mutate it hold no assumption of isolation. Phase 8.4 mutators
        acquire the registry's lock through dedicated methods rather
        than mutating the reference directly.
        """
        with self._lock:
            return self._drafts.get(name)

    def discard(self, name: str) -> bool:
        """Remove the named draft. Idempotent.

        Returns ``True`` if the draft was present, ``False`` if no draft
        by that name existed.
        """
        with self._lock:
            if name not in self._drafts:
                return False

            del self._drafts[name]

            return True

    def names(self) -> list[str]:
        """Return the names of every draft, sorted.

        Renamed from ``list`` (Phase 8.1) because mypy treats
        ``Registry.list`` as a type when ``list[str]`` annotations
        appear inside the class scope (the Phase 8.4 mutators added
        many such annotations). The method body is unchanged; only
        the name moved. Phase 8.1 tests are updated in lockstep.

        Returns an empty list (never ``None``) when the registry is
        empty. JSON marshaling of the Phase 8.3 ``_show`` response
        relies on this for ``[]`` output.
        """
        with self._lock:
            return sorted(self._drafts.keys())

    def add_tools(
        self,
        draft_name: str,
        patterns: Sequence[str],
        catalog: Sequence[ToolDescriptor],
    ) -> list[str]:
        """Expand patterns against the catalog and merge into the draft.

        Returns the sorted list of names actually added (those not
        already on the draft). The draft's ``allowed_tools`` is also
        sorted after the merge so the order is stable across calls.

        :raises DraftNotFoundError: when the draft is not in the registry.
        """
        with self._lock:
            draft = self._drafts.get(draft_name)
            if draft is None:
                raise DraftNotFoundError(draft_name)

            matched = match_patterns(patterns, catalog)
            existing = set(draft.allowed_tools)
            added: list[str] = []

            for name in matched:
                if name in existing:
                    continue

                existing.add(name)
                added.append(name)

            draft.allowed_tools = sorted(existing)
            added.sort()

            return added

    def remove_tools(self, draft_name: str, patterns: Sequence[str]) -> list[str]:
        """Expand patterns against the draft's current tools and remove matches.

        Patterns target the draft's state directly, not the live
        catalog, so a wildcard like ``linode_instance_*`` removes
        exactly the instance tools already on the draft regardless
        of what the live catalog contains.

        :raises DraftNotFoundError: when the draft is not in the registry.
        """
        with self._lock:
            draft = self._drafts.get(draft_name)
            if draft is None:
                raise DraftNotFoundError(draft_name)

            synthetic = [
                ToolDescriptor(name=name, capability=Capability.Read)
                for name in draft.allowed_tools
            ]
            matched = match_patterns(patterns, synthetic)
            remove_set = set(matched)
            draft.allowed_tools = [
                name for name in draft.allowed_tools if name not in remove_set
            ]

            return matched

    def set_allowed_environments(self, draft_name: str, envs: Sequence[str]) -> None:
        """Replace the draft's ``allowed_environments`` with the given list.

        :raises DraftNotFoundError: when the draft is not in the registry.
        """
        with self._lock:
            draft = self._drafts.get(draft_name)
            if draft is None:
                raise DraftNotFoundError(draft_name)

            draft.allowed_environments = list(envs)

    def set_required_token_scopes(self, draft_name: str, scopes: Sequence[str]) -> None:
        """Replace the draft's ``required_token_scopes`` with the given list.

        :raises DraftNotFoundError: when the draft is not in the registry.
        """
        with self._lock:
            draft = self._drafts.get(draft_name)
            if draft is None:
                raise DraftNotFoundError(draft_name)

            draft.required_token_scopes = list(scopes)

    def set_allow_yolo(self, draft_name: str, allow: bool) -> None:
        """Set the draft's ``allow_yolo`` flag.

        :raises DraftNotFoundError: when the draft is not in the registry.
        """
        with self._lock:
            draft = self._drafts.get(draft_name)
            if draft is None:
                raise DraftNotFoundError(draft_name)

            draft.allow_yolo = allow


__all__ = [
    "Draft",
    "DraftExistsError",
    "DraftNameEmptyError",
    "DraftNotFoundError",
    "Registry",
    "match_patterns",
]
