"""Diff helper for Phase 8.5 draft save.

Mirrors ``go/internal/profiles/builder/diff.go``. The save handler
constructs a draft-as-UserProfileConfig, looks up the existing entry
in ``cfg.profiles`` (if any), and calls :func:`compute_diff` to
build the response payload.

JSON shape per :class:`Diff` matches the Go side so cross-language
tooling produces identical save responses for the same input.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from linodemcp.config import UserProfileConfig
    from linodemcp.profiles.builder import Draft


@dataclass
class FieldDiff:
    """Old/new pair for one scalar or list field in a save diff."""

    old: Any
    new: Any

    def to_payload(self) -> dict[str, Any]:
        """Serialize to ``{"old": ..., "new": ...}``."""
        return {"old": self.old, "new": self.new}


@dataclass
class Diff:
    """Change set produced by a draft save.

    ``is_new`` true means no prior user-defined profile with this name
    existed. ``added_tools`` / ``removed_tools`` describe the
    AllowedTools delta. ``changed_fields`` carries the scalar/list
    fields that differ.
    """

    name: str
    is_new: bool = False
    added_tools: list[str] = field(default_factory=list[str])
    removed_tools: list[str] = field(default_factory=list[str])
    changed_fields: dict[str, FieldDiff] = field(default_factory=dict[str, FieldDiff])

    def to_payload(self) -> dict[str, Any]:
        """Serialize to the wire shape consumed by the save tool response."""
        return {
            "name": self.name,
            "is_new": self.is_new,
            "added_tools": list(self.added_tools),
            "removed_tools": list(self.removed_tools),
            "changed_fields": {
                key: change.to_payload() for key, change in self.changed_fields.items()
            },
        }


def draft_as_user_profile(draft: Draft) -> UserProfileConfig:
    """Convert a Draft into the config-file shape.

    Slices are copied so later mutation of the draft doesn't propagate
    into the saved config.
    """
    from linodemcp.config import UserProfileConfig  # noqa: PLC0415 - avoid import cycle

    return UserProfileConfig(
        description=draft.description,
        allowed_tools=tuple(draft.allowed_tools),
        denied_tools=(),
        allowed_environments=tuple(draft.allowed_environments),
        required_token_scopes=tuple(draft.required_token_scopes),
        allow_yolo=draft.allow_yolo,
    )


def compute_diff(
    name: str,
    draft_cfg: UserProfileConfig,
    existing: UserProfileConfig | None,
) -> Diff:
    """Return a Diff comparing ``draft_cfg`` against ``existing``.

    When ``existing`` is None, every non-zero field shows up in
    ``changed_fields`` with ``old`` as the zero value and ``is_new``
    is True. AddedTools/RemovedTools always reflect the AllowedTools
    delta; for a new profile that means added_tools is the draft's
    full allowed_tools and removed_tools is empty.
    """
    from linodemcp.config import UserProfileConfig  # noqa: PLC0415 - avoid import cycle

    diff = Diff(name=name, is_new=existing is None)
    prev = existing if existing is not None else UserProfileConfig()

    diff.added_tools = _subtract_sorted(
        list(draft_cfg.allowed_tools), list(prev.allowed_tools)
    )
    diff.removed_tools = _subtract_sorted(
        list(prev.allowed_tools), list(draft_cfg.allowed_tools)
    )

    if draft_cfg.description != prev.description:
        diff.changed_fields["description"] = FieldDiff(
            old=prev.description, new=draft_cfg.description
        )

    if list(draft_cfg.allowed_environments) != list(prev.allowed_environments):
        diff.changed_fields["allowed_environments"] = FieldDiff(
            old=list(prev.allowed_environments),
            new=list(draft_cfg.allowed_environments),
        )

    if list(draft_cfg.required_token_scopes) != list(prev.required_token_scopes):
        diff.changed_fields["required_token_scopes"] = FieldDiff(
            old=list(prev.required_token_scopes),
            new=list(draft_cfg.required_token_scopes),
        )

    if draft_cfg.allow_yolo != prev.allow_yolo:
        diff.changed_fields["allow_yolo"] = FieldDiff(
            old=prev.allow_yolo, new=draft_cfg.allow_yolo
        )

    return diff


def _subtract_sorted(source: list[str], minus: list[str]) -> list[str]:
    """Return source elements not in minus, sorted ascending."""
    minus_set = set(minus)
    return sorted(item for item in source if item not in minus_set)
