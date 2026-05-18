"""Phase 8.1 builder registry tests.

Mirrors ``go/internal/profiles/builder/builder_test.go``. Tests define
behavior contracts (sort order, source-isolation, no-silent-overwrite,
empty-slice not None) rather than just exercising current code paths.
"""

from __future__ import annotations

import pytest

from linodemcp.profiles.builder import (
    Draft,
    DraftExistsError,
    DraftNameEmptyError,
    Registry,
)
from linodemcp.profiles.profile import Profile


def fixture_profile() -> Profile:
    """Non-empty profile used as the clone source in clone-from tests.

    Every field is populated so each can be verified to copy
    independently and to confirm that later mutation of the draft
    doesn't leak back into the source.
    """
    return Profile(
        name="source",
        description="Source profile for clone tests",
        allowed_tools=("linode_instance_list", "linode_account"),
        allowed_environments=("prod",),
        required_token_scopes=("linodes:read_only", "account:read_only"),
        allow_yolo=True,
    )


def test_new_registry_starts_empty() -> None:
    """Freshly-built registries hold zero drafts.

    Phase 8.4 mutation handlers rely on ``list()`` being callable
    without seeding.
    """
    reg = Registry()
    assert reg.list() == []


def test_create_minimal_draft_from_scratch() -> None:
    """No-clone-from path: name set, everything else empty.

    Phase 8.3 ``_new`` without ``clone_from`` flows through this path.
    """
    reg = Registry()

    draft = reg.create("dns-readall")

    assert draft.name == "dns-readall"
    assert draft.description == ""
    assert draft.allowed_tools == []
    assert draft.allowed_environments == []
    assert draft.required_token_scopes == []
    assert draft.allow_yolo is False


def test_create_clones_all_fields_from_profile() -> None:
    """Copy fidelity: every Profile field lands on the Draft.

    Phase 8.3 ``_new`` with ``clone_from`` expects the new draft to
    mirror the source.
    """
    reg = Registry()
    src = fixture_profile()

    draft = reg.create("my-dns", src)

    assert draft.name == "my-dns"
    assert draft.description == src.description
    assert draft.allowed_tools == list(src.allowed_tools)
    assert draft.allowed_environments == list(src.allowed_environments)
    assert draft.required_token_scopes == list(src.required_token_scopes)
    assert draft.allow_yolo is src.allow_yolo


def test_create_cloned_draft_isolates_from_source() -> None:
    """Mutating the draft does not propagate to the source profile.

    Python's ``Profile`` uses tuples so the immutability is enforced at
    the type level for the source, but the draft accepts the tuple and
    must copy it into a list. This test guards against a regression
    where the copy step is removed.
    """
    reg = Registry()
    src = fixture_profile()
    original = list(src.allowed_tools)

    draft = reg.create("my-dns", src)
    draft.allowed_tools.append("linode_domain_list")

    assert list(src.allowed_tools) == original, (
        "draft mutation must not propagate to source profile"
    )
    assert len(draft.allowed_tools) == len(original) + 1


def test_create_refuses_empty_name() -> None:
    """Validation guard.

    An empty draft name would yield a config map entry with a blank key
    on save. Refuse at create time so the failure surfaces near the
    user's mistake.
    """
    reg = Registry()

    with pytest.raises(DraftNameEmptyError):
        reg.create("")


def test_create_refuses_duplicate_name() -> None:
    """No-silent-overwrite rule.

    A second ``create`` with the same name raises ``DraftExistsError``.
    The user must discard first or pick a different name.
    """
    reg = Registry()
    reg.create("dns-readall")

    with pytest.raises(DraftExistsError) as excinfo:
        reg.create("dns-readall")

    assert excinfo.value.name == "dns-readall"


def test_get_returns_live_draft() -> None:
    """Get returns the same object reference create produced.

    Phase 8.4 mutators rely on identity to locate and edit the draft.
    """
    reg = Registry()
    original = reg.create("dns-readall")

    got = reg.get("dns-readall")

    assert got is original, "get must return the registry's own draft reference"


def test_get_missing_returns_none() -> None:
    """Tool handlers rely on the None branch to produce a 'no such draft' error."""
    reg = Registry()

    assert reg.get("nonexistent") is None


def test_discard_removes_draft() -> None:
    """Happy path: discard removes the draft from list and get.

    A successful discard returns True; the discarded name disappears
    from both query surfaces.
    """
    reg = Registry()
    reg.create("dns-readall")

    removed = reg.discard("dns-readall")

    assert removed is True
    assert reg.list() == []
    assert reg.get("dns-readall") is None


def test_discard_missing_is_idempotent() -> None:
    """Discard on a name that was never created returns False, not an error.

    Tool handlers can call discard on tear-down paths without first
    checking existence.
    """
    reg = Registry()

    assert reg.discard("nonexistent") is False


def test_list_returns_sorted_names() -> None:
    """List returns names in sorted order.

    Stable output matters for Phase 8.3 ``_show`` and Phase 8.5 diff
    presentation; both compare draft names against existing profile
    names by exact match.
    """
    reg = Registry()
    reg.create("zebra")
    reg.create("alpha")
    reg.create("middle")

    assert reg.list() == ["alpha", "middle", "zebra"]


def test_list_empty_registry_returns_empty_list() -> None:
    """List on an empty registry returns ``[]``, not None.

    JSON marshaling of ``_show`` surfaces as ``[]`` not ``null`` per
    spec.
    """
    reg = Registry()

    result = reg.list()

    assert result is not None
    assert result == []


def test_draft_dataclass_is_mutable() -> None:
    """Confirm Draft is a non-frozen dataclass.

    Phase 8.4 mutates ``allowed_tools`` in place; if Draft were frozen
    those mutations would raise. Guard against an accidental
    ``@dataclass(frozen=True)`` decoration.
    """
    draft = Draft(name="test")
    draft.allowed_tools.append("some_tool")
    draft.allow_yolo = True

    assert draft.allowed_tools == ["some_tool"]
    assert draft.allow_yolo is True
