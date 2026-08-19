"""Phase 8.3 draft lifecycle builder tool tests.

Mirrors ``go/internal/tools/linode_profile_draft_test.go``. Tests
define behavior contracts (CapMeta tag, error sentinels, idempotency,
JSON shape) rather than just exercising the current code path.

Each test publishes a reproducible builder state for the call, the way the
server does per dispatch, and the autouse fixture resets it afterwards so no
state bleeds across tests.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, cast

import pytest

from linodemcp.config import Config, UserProfileConfig
from linodemcp.gentools import (
    create_linode_profile_draft_discard_tool,
    create_linode_profile_draft_new_tool,
    create_linode_profile_draft_show_tool,
)
from linodemcp.profiles import Capability
from linodemcp.profiles.builder import Registry
from linodemcp.profiles.builtin import ToolDescriptor
from linodemcp.profiles.profile import Profile
from linodemcp.tools.builderstate import (
    BuilderState,
    reset_builder_state,
    set_builder_state,
)
from linodemcp.tools.linode_profile_draft import (
    profile_draft_discard_result,
    profile_draft_new_result,
    profile_draft_show_result,
)

if TYPE_CHECKING:
    from collections.abc import Iterator


_DRAFT_FIXTURE_NAME = "dns-readall"
_CLONE_SOURCE_NAME = "compute-admin"
_NAME_MISSING = "Error: name argument is required"


def _no_profile() -> Profile:
    """The active-profile reader for the tools that never read one."""
    return Profile(name="test", description="", allowed_tools=())


def fixture_catalog() -> list[ToolDescriptor]:
    """The catalog a cloned profile's tool patterns expand against."""
    return [
        ToolDescriptor(name="linode_instance_boot", capability=Capability.Write),
        ToolDescriptor(name="linode_instance_list", capability=Capability.Read),
    ]


def fixture_config() -> Config:
    """A config carrying one user-defined profile to clone from.

    User-defined entries shadow built-ins, so the fixture's fields are the ones
    a clone lands on.
    """
    cfg = Config()
    cfg.profiles[_CLONE_SOURCE_NAME] = UserProfileConfig(
        description="Compute admin clone source",
        allowed_tools=("linode_instance_boot", "linode_instance_list"),
        allowed_environments=("prod",),
        required_token_scopes=("linodes:read_write",),
    )
    return cfg


def fixture_source_profile() -> Profile:
    """The Profile fixture_config resolves the clone source to."""
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
    """Publish a reproducible builder state for each test.

    Yields the Registry so individual tests can inspect post-call state (a
    draft created or removed). The state is reset after each test so nothing
    bleeds across them.
    """
    registry = Registry()
    token = set_builder_state(
        BuilderState(
            drafts=registry,
            catalog=fixture_catalog,
            active_profile=_no_profile,
        )
    )

    yield registry

    reset_builder_state(token)


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
    response = profile_draft_new_result({"name": _DRAFT_FIXTURE_NAME}, fixture_config())

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
    response = profile_draft_new_result(
        {"name": _DRAFT_FIXTURE_NAME, "clone_from": _CLONE_SOURCE_NAME},
        fixture_config(),
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
    """An absent name answers the shared refusal, not a transport failure."""
    response = profile_draft_new_result({}, fixture_config())

    assert response[0].text == _NAME_MISSING


@pytest.mark.asyncio
async def test_draft_new_refuses_unknown_clone_source(
    install_fixtures: Registry,
) -> None:
    """An unknown clone_from refuses by name and leaves nothing behind."""
    response = profile_draft_new_result(
        {"name": _DRAFT_FIXTURE_NAME, "clone_from": "nonexistent-profile"},
        fixture_config(),
    )

    assert response[0].text == (
        "Error: clone_from profile not found: nonexistent-profile"
    )
    assert install_fixtures.get(_DRAFT_FIXTURE_NAME) is None, (
        "failed _new must not leave a draft behind"
    )


@pytest.mark.asyncio
async def test_draft_new_refuses_duplicate_name() -> None:
    """A second create with the same name refuses rather than overwriting."""
    profile_draft_new_result({"name": _DRAFT_FIXTURE_NAME}, fixture_config())

    response = profile_draft_new_result({"name": _DRAFT_FIXTURE_NAME}, fixture_config())

    assert response[0].text == "Error: draft already exists: dns-readall"


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

    response = profile_draft_show_result({"name": _DRAFT_FIXTURE_NAME})

    payload = _parse_response(response[0].text)
    src = fixture_source_profile()
    assert payload["name"] == _DRAFT_FIXTURE_NAME
    assert payload["description"] == src.description
    assert payload["allowed_tools"] == list(src.allowed_tools)


@pytest.mark.asyncio
async def test_draft_show_refuses_unknown() -> None:
    """A name no draft carries refuses with the sentence Go answers too."""
    response = profile_draft_show_result({"name": "nonexistent-draft"})

    assert response[0].text == "Error: draft not found: nonexistent-draft"


@pytest.mark.asyncio
async def test_draft_show_refuses_missing_name() -> None:
    """An absent name answers the shared refusal."""
    response = profile_draft_show_result({})

    assert response[0].text == _NAME_MISSING


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

    response = profile_draft_discard_result({"name": _DRAFT_FIXTURE_NAME})

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
    response = profile_draft_discard_result({"name": "nonexistent-draft"})

    payload = _parse_response(response[0].text)
    assert payload["name"] == "nonexistent-draft"
    assert payload["discarded"] is False


@pytest.mark.asyncio
async def test_draft_discard_refuses_missing_name() -> None:
    """An absent name answers the shared refusal, as _new and _show do."""
    response = profile_draft_discard_result({})

    assert response[0].text == _NAME_MISSING


def test_profile_builder_tools_registered_with_server() -> None:
    """Every profile-builder meta tool must be exported and server-registered.

    These create/handle pairs existed in the tools package but were missing
    from ``tools.__all__``, so the registry scan never picked them up and the
    server silently shipped without them. This pins the full set.

    The registry scans ``linodemcp.tools`` and ``linodemcp.gentools`` the same
    way, so a tool moving from hand-written to generated changes which module
    exports the pair but never whether it reaches the server. The pair is
    looked up across both for that reason; the registration assertion is the
    one that must hold no matter which side owns the tool.
    """
    from linodemcp import gentools as gentools_mod
    from linodemcp import tools as tools_mod
    from linodemcp.server import get_tool_registry

    builder_tools = [
        "linode_profile_can_run",
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

    exported = set(tools_mod.__all__) | set(gentools_mod.__all__)
    registered = {entry.name for entry in get_tool_registry()}
    for name in builder_tools:
        assert f"create_{name}_tool" in exported, name
        assert f"handle_{name}" in exported, name
        assert name in registered, f"{name} is not registered with the server"
