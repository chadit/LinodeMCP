"""Phase 8.5 draft save builder tool tests.

Mirrors ``go/internal/tools/linode_profile_draft_save_test.go``.
Tests define the contract: confirm gate, built-in name refusal,
diff shape, atomic write side-effect, error sentinels.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, cast

import pytest

from linodemcp.config import load_from_file
from linodemcp.profiles import Capability
from linodemcp.profiles.builder import DraftNotFoundError, Registry
from linodemcp.tools.linode_profile_draft import (
    DraftNameMissingError,
    set_draft_registry,
)
from linodemcp.tools.linode_profile_draft_save import (
    ConfirmRequiredError,
    SaveBuiltinNameError,
    create_linode_profile_draft_save_tool,
    handle_linode_profile_draft_save,
    set_save_config_path_provider,
)

if TYPE_CHECKING:
    from collections.abc import Iterator
    from pathlib import Path


_SAVE_DRAFT_NAME = "my-saved"
_TOOL_HELLO = "hello"
_TOOL_INSTANCE_BOOT = "linode_instance_boot"


_MINIMAL_YAML = """\
server:
  name: "Test"
  logLevel: "info"
  transport: "stdio"
  host: "127.0.0.1"
  port: 8080
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
"""


@pytest.fixture
def writable_config(tmp_path: Path) -> Path:
    """Stage a minimal config file and return its path."""
    path = tmp_path / "config.yml"
    path.write_text(_MINIMAL_YAML)
    return path


@pytest.fixture(autouse=True)
def install_fixtures() -> Iterator[Registry]:
    """Install registry bridge; reset after each test."""
    registry = Registry()
    set_draft_registry(registry)

    yield registry

    set_draft_registry(None)
    set_save_config_path_provider(None)


def _parse_response(text: str) -> dict[str, object]:
    """Parse JSON payload into a typed dict."""
    parsed: object = json.loads(text)
    assert isinstance(parsed, dict)
    return cast("dict[str, object]", parsed)


def test_save_registration() -> None:
    """Static contract: name, description, CapMeta tag."""
    tool, capability = create_linode_profile_draft_save_tool()

    assert tool.name == "linode_profile_draft_save"
    assert tool.description
    assert capability is Capability.Meta


@pytest.mark.asyncio
async def test_save_creates_new_profile(
    install_fixtures: Registry,
    writable_config: Path,
) -> None:
    """Happy path for a brand-new user-defined profile."""
    draft = install_fixtures.create(_SAVE_DRAFT_NAME)
    draft.description = "saved via test"
    draft.allowed_tools = [_TOOL_HELLO, _TOOL_INSTANCE_BOOT]

    set_save_config_path_provider(lambda: str(writable_config))

    response = await handle_linode_profile_draft_save(
        {"name": _SAVE_DRAFT_NAME, "confirm": True}
    )

    payload = _parse_response(response[0].text)
    assert payload["name"] == _SAVE_DRAFT_NAME
    assert payload["is_new"] is True

    added = payload["added_tools"]
    assert isinstance(added, list)
    assert sorted(cast("list[str]", added)) == [_TOOL_HELLO, _TOOL_INSTANCE_BOOT]

    assert payload["removed_tools"] == []

    reloaded = load_from_file(writable_config)
    stored = reloaded.profiles[_SAVE_DRAFT_NAME]
    assert stored.description == "saved via test"
    assert sorted(stored.allowed_tools) == [_TOOL_HELLO, _TOOL_INSTANCE_BOOT]


@pytest.mark.asyncio
async def test_save_updates_existing_profile(
    install_fixtures: Registry,
    writable_config: Path,
) -> None:
    """Round-trip update: diff reports added + removed + changed description."""
    from linodemcp.config import UserProfileConfig, write_atomic

    prior = load_from_file(writable_config)
    prior.profiles[_SAVE_DRAFT_NAME] = UserProfileConfig(
        description="prior",
        allowed_tools=(_TOOL_HELLO,),
    )
    write_atomic(writable_config, prior)

    draft = install_fixtures.create(_SAVE_DRAFT_NAME)
    draft.description = "updated"
    draft.allowed_tools = [_TOOL_INSTANCE_BOOT]

    set_save_config_path_provider(lambda: str(writable_config))

    response = await handle_linode_profile_draft_save(
        {"name": _SAVE_DRAFT_NAME, "confirm": True}
    )

    payload = _parse_response(response[0].text)
    assert payload["is_new"] is False
    assert payload["added_tools"] == [_TOOL_INSTANCE_BOOT]
    assert payload["removed_tools"] == [_TOOL_HELLO]

    changes = payload["changed_fields"]
    assert isinstance(changes, dict)
    typed = cast("dict[str, object]", changes)
    assert "description" in typed
    desc_change = typed["description"]
    assert isinstance(desc_change, dict)
    desc_typed = cast("dict[str, object]", desc_change)
    assert desc_typed["old"] == "prior"
    assert desc_typed["new"] == "updated"


@pytest.mark.asyncio
async def test_save_refuses_missing_confirm(
    install_fixtures: Registry,
    writable_config: Path,
) -> None:
    """Without confirm=true, raises ConfirmRequiredError and writes nothing."""
    install_fixtures.create(_SAVE_DRAFT_NAME)
    set_save_config_path_provider(lambda: str(writable_config))

    original = writable_config.read_text()

    with pytest.raises(ConfirmRequiredError):
        await handle_linode_profile_draft_save({"name": _SAVE_DRAFT_NAME})

    assert writable_config.read_text() == original, (
        "refused save must not write to disk"
    )


@pytest.mark.asyncio
async def test_save_refuses_builtin_name(
    install_fixtures: Registry,
    writable_config: Path,
) -> None:
    """Save target name matching a built-in is refused."""
    install_fixtures.create("compute-admin")
    set_save_config_path_provider(lambda: str(writable_config))

    with pytest.raises(SaveBuiltinNameError) as excinfo:
        await handle_linode_profile_draft_save(
            {"name": "compute-admin", "confirm": True}
        )

    assert excinfo.value.profile_name == "compute-admin"


@pytest.mark.asyncio
async def test_save_refuses_unknown_draft(writable_config: Path) -> None:
    """Save on a nonexistent draft raises DraftNotFoundError."""
    set_save_config_path_provider(lambda: str(writable_config))

    with pytest.raises(DraftNotFoundError):
        await handle_linode_profile_draft_save(
            {"name": "nonexistent-draft", "confirm": True}
        )


@pytest.mark.asyncio
async def test_save_refuses_missing_name() -> None:
    """Empty name raises DraftNameMissingError."""
    with pytest.raises(DraftNameMissingError):
        await handle_linode_profile_draft_save({"confirm": True})


@pytest.mark.asyncio
async def test_save_response_has_expected_shape(
    install_fixtures: Registry,
    writable_config: Path,
) -> None:
    """JSON response carries every top-level field defined by the wire contract."""
    draft = install_fixtures.create(_SAVE_DRAFT_NAME)
    draft.allowed_tools = [_TOOL_HELLO]

    set_save_config_path_provider(lambda: str(writable_config))

    response = await handle_linode_profile_draft_save(
        {"name": _SAVE_DRAFT_NAME, "confirm": True}
    )

    payload = _parse_response(response[0].text)
    for key in ("name", "is_new", "added_tools", "removed_tools", "changed_fields"):
        assert key in payload, f"response must include {key}"
