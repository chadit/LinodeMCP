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
