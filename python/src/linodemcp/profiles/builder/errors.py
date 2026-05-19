"""Exception classes for the draft registry.

Ruff N818 requires the ``Error`` suffix on exception class names; that's
why the Go ``ErrDraftExists`` sentinel becomes ``DraftExistsError`` here.
"""

from __future__ import annotations


class DraftNameEmptyError(ValueError):
    """Raised by :meth:`Registry.create` when ``name`` is empty."""

    def __init__(self) -> None:
        super().__init__("draft name cannot be empty")


class DraftExistsError(ValueError):
    """Raised by :meth:`Registry.create` when the registry already has a draft.

    Refusing silent overwrite makes a stray re-create surface to the
    caller. Discard the existing draft first or pick a different name.
    """

    def __init__(self, name: str) -> None:
        super().__init__(f"draft already exists: {name}")
        self.name = name


class DraftNotFoundError(LookupError):
    """Raised by Phase 8.4 mutator methods when the draft is not in the registry.

    The Phase 8.3 ``Registry.get`` returns ``None`` for misses; the
    mutators raise this instead so callers can match the same shape
    they use for the show/save error path.
    """

    def __init__(self, name: str) -> None:
        super().__init__(f"draft not found: {name}")
        self.draft_name = name
