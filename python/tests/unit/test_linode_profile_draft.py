"""Phase 8.3 draft lifecycle builder tool tests.

Mirrors ``go/internal/tools/linode_profile_draft_test.go``. Tests
define behavior contracts (CapMeta tag, error sentinels, idempotency,
JSON shape) rather than just exercising the current code path.

Each test installs a reproducible Registry + resolver via the
module-level bridge. The ``install_fixtures`` autouse fixture resets
both bridges after each test so state never bleeds across files (the
production server module touches the same singletons).
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, cast

import pytest

from linodemcp.profiles import Capability
from linodemcp.profiles.builder import DraftExistsError, Registry
from linodemcp.profiles.profile import Profile
from linodemcp.tools.linode_profile_draft import (
    CloneSourceMissingError,
    DraftNameMissingError,
    DraftNotFoundError,
    create_linode_profile_draft_discard_tool,
    create_linode_profile_draft_new_tool,
    create_linode_profile_draft_show_tool,
    handle_linode_profile_draft_discard,
    handle_linode_profile_draft_new,
    handle_linode_profile_draft_show,
    set_draft_registry,
    set_profile_resolver,
)

if TYPE_CHECKING:
    from collections.abc import Iterator


_DRAFT_FIXTURE_NAME = "dns-readall"
_CLONE_SOURCE_NAME = "compute-admin"


def fixture_source_profile() -> Profile:
    """Canonical Profile the resolver returns for the clone source.

    Distinct values per field so assertions catch field-level
    mistakes (e.g. a regression that swaps two field assignments).
    """
    return Profile(
        name=_CLONE_SOURCE_NAME,
        description="Compute admin clone source",
        allowed_tools=("linode_instance_boot", "linode_instance_list"),
        allowed_environments=("prod",),
        required_token_scopes=("linodes:read_write",),
        allow_yolo=False,
    )


@pytest.fixture(autouse=True)
def install_fixtures() -> Iterator[Registry]:
    """Install a reproducible Registry + resolver for each test.

    Yields the Registry so individual tests can inspect post-call
    state (e.g. confirm a draft was created or removed). The bridge
    state is reset after each test so cross-file state doesn't leak.
    """
    registry = Registry()
    set_draft_registry(registry)

    def resolver(name: str) -> Profile | None:
        if name == _CLONE_SOURCE_NAME:
            return fixture_source_profile()
        return None

    set_profile_resolver(resolver)

    yield registry

    set_draft_registry(None)
    set_profile_resolver(None)


def _parse_response(text: str) -> dict[str, object]:
    """Parse JSON payload from a TextContent's text field.

    json.loads returns Any; the cast collapses it into the shape the
    assertions need so pyright strict has the right types.
    """
    parsed: object = json.loads(text)
    assert isinstance(parsed, dict)
    return cast("dict[str, object]", parsed)


def test_draft_new_registration() -> None:
    """Static contract: name, description, CapMeta tag.

    CapMeta is what makes builder tools always-available regardless of
    the active profile. A regression on the tag would silently break
    the builder UX under the read-only default profile.
    """
    tool, capability = create_linode_profile_draft_new_tool()

    assert tool.name == "linode_profile_draft_new"
    assert tool.description
    assert capability is Capability.Meta


@pytest.mark.asyncio
async def test_draft_new_creates_empty_draft(install_fixtures: Registry) -> None:
    """No-clone-from happy path: empty draft created and registered."""
    response = await handle_linode_profile_draft_new({"name": _DRAFT_FIXTURE_NAME})

    payload = _parse_response(response[0].text)
    assert payload["name"] == _DRAFT_FIXTURE_NAME
    assert payload["description"] == ""
    assert payload["allowed_tools"] == []
    assert payload["allowed_environments"] == []
    assert payload["required_token_scopes"] == []
    assert payload["allow_yolo"] is False

    assert install_fixtures.get(_DRAFT_FIXTURE_NAME) is not None, (
        "draft must be registered after _new returns"
    )


@pytest.mark.asyncio
async def test_draft_new_clones_from_source() -> None:
    """Clone path: every field on the source profile lands on the draft."""
    response = await handle_linode_profile_draft_new(
        {"name": _DRAFT_FIXTURE_NAME, "clone_from": _CLONE_SOURCE_NAME}
    )

    payload = _parse_response(response[0].text)
    src = fixture_source_profile()
    assert payload["name"] == _DRAFT_FIXTURE_NAME
    assert payload["description"] == src.description
    assert payload["allowed_tools"] == list(src.allowed_tools)
    assert payload["allowed_environments"] == list(src.allowed_environments)
    assert payload["required_token_scopes"] == list(src.required_token_scopes)
    assert payload["allow_yolo"] is src.allow_yolo


@pytest.mark.asyncio
async def test_draft_new_refuses_missing_name() -> None:
    """Empty name raises DraftNameMissingError (validation guard)."""
    with pytest.raises(DraftNameMissingError):
        await handle_linode_profile_draft_new({})


@pytest.mark.asyncio
async def test_draft_new_refuses_unknown_clone_source(
    install_fixtures: Registry,
) -> None:
    """Unknown clone_from raises CloneSourceMissingError, leaves nothing behind."""
    with pytest.raises(CloneSourceMissingError) as excinfo:
        await handle_linode_profile_draft_new(
            {"name": _DRAFT_FIXTURE_NAME, "clone_from": "nonexistent-profile"}
        )

    assert excinfo.value.profile_name == "nonexistent-profile"
    assert install_fixtures.get(_DRAFT_FIXTURE_NAME) is None, (
        "failed _new must not leave a draft behind"
    )


@pytest.mark.asyncio
async def test_draft_new_refuses_duplicate_name() -> None:
    """Second create with the same name surfaces DraftExistsError."""
    await handle_linode_profile_draft_new({"name": _DRAFT_FIXTURE_NAME})

    with pytest.raises(DraftExistsError):
        await handle_linode_profile_draft_new({"name": _DRAFT_FIXTURE_NAME})


def test_draft_show_registration() -> None:
    """Static contract for the show tool."""
    tool, capability = create_linode_profile_draft_show_tool()

    assert tool.name == "linode_profile_draft_show"
    assert tool.description
    assert capability is Capability.Meta


@pytest.mark.asyncio
async def test_draft_show_returns_live_draft_state(
    install_fixtures: Registry,
) -> None:
    """Show reads the draft back with all fields populated.

    Mirrors the conversation flow where the model creates a draft,
    mutates it (Phase 8.4), then re-reads to confirm.
    """
    install_fixtures.create(_DRAFT_FIXTURE_NAME, fixture_source_profile())

    response = await handle_linode_profile_draft_show({"name": _DRAFT_FIXTURE_NAME})

    payload = _parse_response(response[0].text)
    src = fixture_source_profile()
    assert payload["name"] == _DRAFT_FIXTURE_NAME
    assert payload["description"] == src.description
    assert payload["allowed_tools"] == list(src.allowed_tools)


@pytest.mark.asyncio
async def test_draft_show_refuses_unknown() -> None:
    """Unknown name raises DraftNotFoundError (no silent empty response)."""
    with pytest.raises(DraftNotFoundError) as excinfo:
        await handle_linode_profile_draft_show({"name": "nonexistent-draft"})

    assert excinfo.value.draft_name == "nonexistent-draft"


@pytest.mark.asyncio
async def test_draft_show_refuses_missing_name() -> None:
    """Empty name raises DraftNameMissingError."""
    with pytest.raises(DraftNameMissingError):
        await handle_linode_profile_draft_show({})


def test_draft_discard_registration() -> None:
    """Static contract for the discard tool."""
    tool, capability = create_linode_profile_draft_discard_tool()

    assert tool.name == "linode_profile_draft_discard"
    assert tool.description
    assert capability is Capability.Meta


@pytest.mark.asyncio
async def test_draft_discard_removes_draft(install_fixtures: Registry) -> None:
    """Happy path: discard returns discarded=True and removes from registry."""
    install_fixtures.create(_DRAFT_FIXTURE_NAME)

    response = await handle_linode_profile_draft_discard({"name": _DRAFT_FIXTURE_NAME})

    payload = _parse_response(response[0].text)
    assert payload["name"] == _DRAFT_FIXTURE_NAME
    assert payload["discarded"] is True
    assert install_fixtures.get(_DRAFT_FIXTURE_NAME) is None


@pytest.mark.asyncio
async def test_draft_discard_idempotent() -> None:
    """Discarding an absent draft returns discarded=False, not an error.

    Tool handlers should be safe to call on cleanup paths without
    first checking existence.
    """
    response = await handle_linode_profile_draft_discard({"name": "nonexistent-draft"})

    payload = _parse_response(response[0].text)
    assert payload["name"] == "nonexistent-draft"
    assert payload["discarded"] is False


@pytest.mark.asyncio
async def test_draft_discard_refuses_missing_name() -> None:
    """Empty name raises DraftNameMissingError (mirrors _new and _show)."""
    with pytest.raises(DraftNameMissingError):
        await handle_linode_profile_draft_discard({})


def test_profile_builder_tools_registered_with_server() -> None:
    """Every profile-builder meta tool must be exported and server-registered.

    These create/handle pairs existed in the tools package but were missing
    from ``tools.__all__``, so the registry scan never picked them up and the
    server silently shipped without them. This pins the full set.
    """
    from linodemcp import tools as tools_mod
    from linodemcp.server import get_tool_registry

    builder_tools = [
        "linode_profile_draft_add_tools",
        "linode_profile_draft_discard",
        "linode_profile_draft_new",
        "linode_profile_draft_remove_tools",
        "linode_profile_draft_save",
        "linode_profile_draft_set",
        "linode_profile_draft_show",
        "linode_profile_list_categories",
        "linode_profile_list_tools",
    ]

    registered = {entry.name for entry in get_tool_registry()}
    for name in builder_tools:
        assert f"create_{name}_tool" in tools_mod.__all__, name
        assert f"handle_{name}" in tools_mod.__all__, name
        assert name in registered, f"{name} is not registered with the server"
