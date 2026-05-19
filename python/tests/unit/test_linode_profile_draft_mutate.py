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

from linodemcp.profiles import Capability
from linodemcp.profiles.builder import (
    DraftNotFoundError,
    Registry,
)
from linodemcp.profiles.builtin import ToolDescriptor
from linodemcp.tools.linode_profile_draft import (
    DraftNameMissingError,
    set_draft_registry,
)
from linodemcp.tools.linode_profile_draft_mutate import (
    create_linode_profile_draft_add_tools_tool,
    create_linode_profile_draft_remove_tools_tool,
    create_linode_profile_draft_set_tool,
    handle_linode_profile_draft_add_tools,
    handle_linode_profile_draft_remove_tools,
    handle_linode_profile_draft_set,
    set_mutator_catalog_provider,
)

if TYPE_CHECKING:
    from collections.abc import Iterator


_MUTATE_DRAFT_NAME = "my-draft"
_TOOL_INSTANCE_BOOT = "linode_instance_boot"
_TOOL_INSTANCE_REBOOT = "linode_instance_reboot"
_TOOL_HELLO = "hello"
_PROD_ENV = "prod"


def fixture_catalog() -> list[ToolDescriptor]:
    """Static catalog mirrored from the Go-side mutateFixtureCatalog."""
    return [
        ToolDescriptor(name=_TOOL_INSTANCE_BOOT, capability=Capability.Write),
        ToolDescriptor(name=_TOOL_INSTANCE_REBOOT, capability=Capability.Write),
        ToolDescriptor(name="linode_instance_shutdown", capability=Capability.Write),
        ToolDescriptor(name="linode_domain_get", capability=Capability.Read),
        ToolDescriptor(name=_TOOL_HELLO, capability=Capability.Meta),
    ]


@pytest.fixture(autouse=True)
def install_fixtures() -> Iterator[Registry]:
    """Install registry + catalog bridges; reset after each test."""
    registry = Registry()
    set_draft_registry(registry)
    set_mutator_catalog_provider(fixture_catalog)

    yield registry

    set_draft_registry(None)
    set_mutator_catalog_provider(None)


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
        {"name": _MUTATE_DRAFT_NAME, "tools": ["linode_instance_*"]},
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
        {"name": _MUTATE_DRAFT_NAME, "tools": [_TOOL_HELLO]}
    )
    response = await handle_linode_profile_draft_add_tools(
        {"name": _MUTATE_DRAFT_NAME, "tools": [_TOOL_HELLO]}
    )

    payload = _parse_response(response[0].text)
    assert payload["added"] == []

    draft = install_fixtures.get(_MUTATE_DRAFT_NAME)
    assert draft is not None
    assert draft.allowed_tools == [_TOOL_HELLO]


@pytest.mark.asyncio
async def test_add_tools_refuses_unknown_draft() -> None:
    """Add on a nonexistent draft raises DraftNotFoundError."""
    with pytest.raises(DraftNotFoundError):
        await handle_linode_profile_draft_add_tools(
            {"name": "nonexistent", "tools": [_TOOL_HELLO]}
        )


@pytest.mark.asyncio
async def test_add_tools_refuses_missing_name() -> None:
    """Empty name raises DraftNameMissingError."""
    with pytest.raises(DraftNameMissingError):
        await handle_linode_profile_draft_add_tools({"tools": [_TOOL_HELLO]})


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
        {"name": _MUTATE_DRAFT_NAME, "tools": [_TOOL_HELLO]}
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
        {"name": _MUTATE_DRAFT_NAME, "tools": ["linode_instance_*"]}
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
        {"name": _MUTATE_DRAFT_NAME, "tools": ["nonexistent-tool"]}
    )

    payload = _parse_response(response[0].text)
    assert payload["removed"] == []

    updated = install_fixtures.get(_MUTATE_DRAFT_NAME)
    assert updated is not None
    assert updated.allowed_tools == [_TOOL_HELLO]


@pytest.mark.asyncio
async def test_remove_tools_refuses_unknown_draft() -> None:
    """Remove on a nonexistent draft raises DraftNotFoundError."""
    with pytest.raises(DraftNotFoundError):
        await handle_linode_profile_draft_remove_tools(
            {"name": "nonexistent", "tools": [_TOOL_HELLO]}
        )


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
        {"name": _MUTATE_DRAFT_NAME, "allowed_environments": [_PROD_ENV]}
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
        {"name": _MUTATE_DRAFT_NAME, "allow_yolo": True}
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
        }
    )

    payload = _parse_response(response[0].text)
    changes = payload["changes"]
    assert isinstance(changes, dict)
    assert len(cast("dict[str, object]", changes)) == 3


@pytest.mark.asyncio
async def test_set_empty_call_no_ops(install_fixtures: Registry) -> None:
    """Call with just name returns empty changes and writes no fields."""
    install_fixtures.create(_MUTATE_DRAFT_NAME)

    response = await handle_linode_profile_draft_set({"name": _MUTATE_DRAFT_NAME})

    payload = _parse_response(response[0].text)
    changes = payload["changes"]
    assert isinstance(changes, dict)
    assert changes == {}


@pytest.mark.asyncio
async def test_set_refuses_unknown_draft() -> None:
    """Set on a nonexistent draft raises DraftNotFoundError."""
    with pytest.raises(DraftNotFoundError):
        await handle_linode_profile_draft_set(
            {"name": "nonexistent", "allow_yolo": True}
        )


@pytest.mark.asyncio
async def test_set_refuses_missing_name() -> None:
    """Empty name raises DraftNameMissingError."""
    with pytest.raises(DraftNameMissingError):
        await handle_linode_profile_draft_set({"allow_yolo": True})
