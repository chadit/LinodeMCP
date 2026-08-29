"""Phase 8.4 draft mutation builder tool tests.

Mirrors ``go/internal/tools/linode_profile_draft_mutate_test.go``.
Tests define behavior contracts (CapMeta tag, wildcard expansion,
dedup, idempotency, error sentinels) rather than just exercising the
current code path.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, cast

import pytest

from linodemcp.config import Config
from linodemcp.gentools import (
    create_linode_profile_draft_add_tools_tool,
    create_linode_profile_draft_remove_tools_tool,
    create_linode_profile_draft_set_tool,
)
from linodemcp.gentools.profile_builder import (
    handle_linode_profile_draft_add_tools,
    handle_linode_profile_draft_remove_tools,
    handle_linode_profile_draft_set,
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

if TYPE_CHECKING:
    from collections.abc import Iterator


_MUTATE_DRAFT_NAME = "my-draft"
_TOOL_INSTANCE_BOOT = "linode_instance_boot"
_TOOL_INSTANCE_REBOOT = "linode_instance_reboot"
_TOOL_HELLO = "hello"
_PROD_ENV = "prod"
_NAME_MISSING = "Error: name argument is required"
_DRAFT_MISSING = "Error: draft not found: nonexistent"


def fixture_catalog() -> list[ToolDescriptor]:
    """Static catalog mirrored from the Go-side mutateFixtureCatalog."""
    return [
        ToolDescriptor(name=_TOOL_INSTANCE_BOOT, capability=Capability.Write),
        ToolDescriptor(name=_TOOL_INSTANCE_REBOOT, capability=Capability.Write),
        ToolDescriptor(name="linode_instance_shutdown", capability=Capability.Write),
        ToolDescriptor(name="linode_domain_get", capability=Capability.Read),
        ToolDescriptor(name=_TOOL_HELLO, capability=Capability.Meta),
    ]


def _no_profile() -> Profile:
    """The active-profile reader for the tools that never read one."""
    return Profile(name="test", description="", allowed_tools=())


@pytest.fixture(autouse=True)
def install_fixtures() -> Iterator[Registry]:
    """Publish a builder state carrying registry + catalog; reset afterwards."""
    registry = Registry()
    token = set_builder_state(
        BuilderState(
            drafts=registry,
            catalog=fixture_catalog,
            active_profile=_no_profile,
            config=Config(),
        )
    )

    yield registry

    reset_builder_state(token)


def _parse_response(text: str) -> dict[str, object]:
    """Parse JSON payload into a typed dict.

    json.loads returns Any; cast collapses the ambiguity for pyright.
    """
    parsed: object = json.loads(text)
    assert isinstance(parsed, dict)
    return cast("dict[str, object]", parsed)


def test_add_tools_registration() -> None:
    """Static contract: name, description, CapMeta tag."""
    tool, capability = create_linode_profile_draft_add_tools_tool()

    assert tool.name == "linode_profile_draft_add_tools"
    assert tool.description
    assert capability is Capability.Meta


@pytest.mark.asyncio
async def test_add_tools_adds_literals(install_fixtures: Registry) -> None:
    """No-wildcard path: literal names match the catalog and land on the draft."""
    install_fixtures.create(_MUTATE_DRAFT_NAME)

    response = await handle_linode_profile_draft_add_tools(
        {"name": _MUTATE_DRAFT_NAME, "tools": [_TOOL_INSTANCE_BOOT, _TOOL_HELLO]},
        Config(),
    )

    payload = _parse_response(response[0].text)
    added = payload["added"]
    assert isinstance(added, list)
    assert sorted(cast("list[str]", added)) == [_TOOL_HELLO, _TOOL_INSTANCE_BOOT]

    draft = install_fixtures.get(_MUTATE_DRAFT_NAME)
    assert draft is not None
    assert sorted(draft.allowed_tools) == [_TOOL_HELLO, _TOOL_INSTANCE_BOOT]


@pytest.mark.asyncio
async def test_add_tools_expands_wildcards(install_fixtures: Registry) -> None:
    """Wildcard path: linode_instance_* expands to boot + reboot + shutdown."""
    install_fixtures.create(_MUTATE_DRAFT_NAME)

    response = await handle_linode_profile_draft_add_tools(
        {"name": _MUTATE_DRAFT_NAME, "tools": ["linode_instance_*"]}, Config()
    )

    payload = _parse_response(response[0].text)
    added = payload["added"]
    assert isinstance(added, list)
    assert sorted(cast("list[str]", added)) == [
        _TOOL_INSTANCE_BOOT,
        _TOOL_INSTANCE_REBOOT,
        "linode_instance_shutdown",
    ]


@pytest.mark.asyncio
async def test_add_tools_dedupes_against_existing(
    install_fixtures: Registry,
) -> None:
    """Second add of the same literal returns an empty added list."""
    install_fixtures.create(_MUTATE_DRAFT_NAME)

    await handle_linode_profile_draft_add_tools(
        {"name": _MUTATE_DRAFT_NAME, "tools": [_TOOL_HELLO]}, Config()
    )
    response = await handle_linode_profile_draft_add_tools(
        {"name": _MUTATE_DRAFT_NAME, "tools": [_TOOL_HELLO]}, Config()
    )

    payload = _parse_response(response[0].text)
    assert payload["added"] == []

    draft = install_fixtures.get(_MUTATE_DRAFT_NAME)
    assert draft is not None
    assert draft.allowed_tools == [_TOOL_HELLO]


@pytest.mark.asyncio
async def test_add_tools_refuses_unknown_draft() -> None:
    """Add on a draft the registry does not hold refuses by name."""
    response = await handle_linode_profile_draft_add_tools(
        {"name": "nonexistent", "tools": [_TOOL_HELLO]}, Config()
    )

    assert response[0].text == _DRAFT_MISSING


@pytest.mark.asyncio
async def test_add_tools_refuses_missing_name() -> None:
    """An absent name answers the shared refusal."""
    response = await handle_linode_profile_draft_add_tools(
        {"tools": [_TOOL_HELLO]}, Config()
    )

    assert response[0].text == _NAME_MISSING


def test_remove_tools_registration() -> None:
    """Static contract: name, description, CapMeta tag."""
    tool, capability = create_linode_profile_draft_remove_tools_tool()

    assert tool.name == "linode_profile_draft_remove_tools"
    assert tool.description
    assert capability is Capability.Meta


@pytest.mark.asyncio
async def test_remove_tools_removes_literals(install_fixtures: Registry) -> None:
    """Happy path: literal names matched against the draft's existing tools."""
    draft = install_fixtures.create(_MUTATE_DRAFT_NAME)
    draft.allowed_tools = [_TOOL_INSTANCE_BOOT, _TOOL_INSTANCE_REBOOT, _TOOL_HELLO]

    response = await handle_linode_profile_draft_remove_tools(
        {"name": _MUTATE_DRAFT_NAME, "tools": [_TOOL_HELLO]}, Config()
    )

    payload = _parse_response(response[0].text)
    assert payload["removed"] == [_TOOL_HELLO]

    updated = install_fixtures.get(_MUTATE_DRAFT_NAME)
    assert updated is not None
    assert sorted(updated.allowed_tools) == [_TOOL_INSTANCE_BOOT, _TOOL_INSTANCE_REBOOT]


@pytest.mark.asyncio
async def test_remove_tools_expands_wildcards_against_draft(
    install_fixtures: Registry,
) -> None:
    """Wildcards target the draft's state, not the live catalog."""
    draft = install_fixtures.create(_MUTATE_DRAFT_NAME)
    draft.allowed_tools = [_TOOL_INSTANCE_BOOT, _TOOL_INSTANCE_REBOOT, _TOOL_HELLO]

    response = await handle_linode_profile_draft_remove_tools(
        {"name": _MUTATE_DRAFT_NAME, "tools": ["linode_instance_*"]}, Config()
    )

    payload = _parse_response(response[0].text)
    removed = payload["removed"]
    assert isinstance(removed, list)
    assert sorted(cast("list[str]", removed)) == [
        _TOOL_INSTANCE_BOOT,
        _TOOL_INSTANCE_REBOOT,
    ]

    updated = install_fixtures.get(_MUTATE_DRAFT_NAME)
    assert updated is not None
    assert updated.allowed_tools == [_TOOL_HELLO]


@pytest.mark.asyncio
async def test_remove_tools_no_match_is_benign(install_fixtures: Registry) -> None:
    """No-match returns an empty removed list and leaves the draft unchanged."""
    draft = install_fixtures.create(_MUTATE_DRAFT_NAME)
    draft.allowed_tools = [_TOOL_HELLO]

    response = await handle_linode_profile_draft_remove_tools(
        {"name": _MUTATE_DRAFT_NAME, "tools": ["nonexistent-tool"]}, Config()
    )

    payload = _parse_response(response[0].text)
    assert payload["removed"] == []

    updated = install_fixtures.get(_MUTATE_DRAFT_NAME)
    assert updated is not None
    assert updated.allowed_tools == [_TOOL_HELLO]


@pytest.mark.asyncio
async def test_remove_tools_refuses_unknown_draft() -> None:
    """Remove on a draft the registry does not hold refuses by name."""
    response = await handle_linode_profile_draft_remove_tools(
        {"name": "nonexistent", "tools": [_TOOL_HELLO]}, Config()
    )

    assert response[0].text == _DRAFT_MISSING


@pytest.mark.asyncio
async def test_remove_tools_refuses_missing_name() -> None:
    """An absent name answers the shared refusal, matching its add sibling.

    The contract's name rule stops this call before the body at the generated
    handler, so the body's own guard is only reachable here.
    """
    response = await handle_linode_profile_draft_remove_tools(
        {"tools": [_TOOL_HELLO]}, Config()
    )

    assert response[0].text == _NAME_MISSING


def test_set_registration() -> None:
    """Static contract: name, description, CapMeta tag."""
    tool, capability = create_linode_profile_draft_set_tool()

    assert tool.name == "linode_profile_draft_set"
    assert tool.description
    assert capability is Capability.Meta


@pytest.mark.asyncio
async def test_set_environments_only(install_fixtures: Registry) -> None:
    """Only specified fields are written; others stay at their prior value."""
    draft = install_fixtures.create(_MUTATE_DRAFT_NAME)
    draft.allowed_environments = ["old-env"]
    draft.required_token_scopes = ["scope:read"]
    draft.allow_yolo = True

    response = await handle_linode_profile_draft_set(
        {"name": _MUTATE_DRAFT_NAME, "allowed_environments": [_PROD_ENV]}, Config()
    )

    payload = _parse_response(response[0].text)
    changes = payload["changes"]
    assert isinstance(changes, dict)
    typed_changes = cast("dict[str, object]", changes)
    assert "allowed_environments" in typed_changes
    assert "required_token_scopes" not in typed_changes
    assert "allow_yolo" not in typed_changes

    updated = install_fixtures.get(_MUTATE_DRAFT_NAME)
    assert updated is not None
    assert updated.allowed_environments == [_PROD_ENV]
    assert updated.required_token_scopes == ["scope:read"]
    assert updated.allow_yolo is True


@pytest.mark.asyncio
async def test_set_allow_yolo_flips_cleanly(install_fixtures: Registry) -> None:
    """allow_yolo=true on a draft that started false is a material change."""
    install_fixtures.create(_MUTATE_DRAFT_NAME)

    response = await handle_linode_profile_draft_set(
        {"name": _MUTATE_DRAFT_NAME, "allow_yolo": True}, Config()
    )

    payload = _parse_response(response[0].text)
    changes = payload["changes"]
    assert isinstance(changes, dict)
    assert cast("dict[str, object]", changes)["allow_yolo"] is True

    updated = install_fixtures.get(_MUTATE_DRAFT_NAME)
    assert updated is not None
    assert updated.allow_yolo is True


@pytest.mark.asyncio
async def test_set_multiple_fields_at_once(install_fixtures: Registry) -> None:
    """A single call can update every settable field."""
    install_fixtures.create(_MUTATE_DRAFT_NAME)

    response = await handle_linode_profile_draft_set(
        {
            "name": _MUTATE_DRAFT_NAME,
            "allowed_environments": [_PROD_ENV, "dev"],
            "required_token_scopes": ["linodes:read_write"],
            "allow_yolo": True,
        },
        Config(),
    )

    payload = _parse_response(response[0].text)
    changes = payload["changes"]
    assert isinstance(changes, dict)
    assert len(cast("dict[str, object]", changes)) == 3


@pytest.mark.asyncio
async def test_set_empty_call_no_ops(install_fixtures: Registry) -> None:
    """Call with just name returns empty changes and writes no fields."""
    install_fixtures.create(_MUTATE_DRAFT_NAME)

    response = await handle_linode_profile_draft_set(
        {"name": _MUTATE_DRAFT_NAME}, Config()
    )

    payload = _parse_response(response[0].text)
    changes = payload["changes"]
    assert isinstance(changes, dict)
    assert changes == {}


@pytest.mark.asyncio
async def test_set_refuses_unknown_draft() -> None:
    """Set on a draft the registry does not hold refuses by name."""
    response = await handle_linode_profile_draft_set(
        {"name": "nonexistent", "allow_yolo": True}, Config()
    )

    assert response[0].text == _DRAFT_MISSING


@pytest.mark.asyncio
async def test_set_refuses_missing_name() -> None:
    """An absent name answers the shared refusal."""
    response = await handle_linode_profile_draft_set({"allow_yolo": True}, Config())

    assert response[0].text == _NAME_MISSING
