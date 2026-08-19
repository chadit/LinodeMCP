"""Phase 8.5 draft save builder tool tests.

Mirrors ``go/internal/tools/linode_profile_draft_save_test.go``.
Tests define the contract: confirm gate, built-in name refusal,
diff shape, atomic write side-effect, error sentinels.
"""

from __future__ import annotations

import errno
import json
from typing import TYPE_CHECKING, cast

import pytest

from linodemcp.config import Config, load_from_file
from linodemcp.gentools import (
    create_linode_profile_draft_save_tool,
    handle_linode_profile_draft_save,
)
from linodemcp.profiles import Capability
from linodemcp.profiles.builder import Registry
from linodemcp.profiles.profile import Profile
from linodemcp.tools.builderstate import (
    BuilderState,
    reset_builder_state,
    set_builder_state,
)

if TYPE_CHECKING:
    from collections.abc import Iterator
    from pathlib import Path


_SAVE_DRAFT_NAME = "my-saved"
_NAME_MISSING = "Error: name argument is required"
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


def _no_profile() -> Profile:
    """The active-profile reader for the tools that never read one."""
    return Profile(name="test", description="", allowed_tools=())


@pytest.fixture
def writable_config(tmp_path: Path) -> Path:
    """Stage a minimal config file and return its path."""
    path = tmp_path / "config.yml"
    path.write_text(_MINIMAL_YAML)
    return path


@pytest.fixture(autouse=True)
def install_fixtures() -> Iterator[Registry]:
    """Publish a builder state over a fresh registry; reset afterwards."""
    registry = Registry()
    token = set_builder_state(
        BuilderState(
            drafts=registry,
            catalog=list,
            active_profile=_no_profile,
        )
    )

    yield registry

    reset_builder_state(token)


def aim_config_at(monkeypatch: pytest.MonkeyPatch, path: Path) -> None:
    """Point the save at a staged config file.

    The handler reads the live path on every call, so the environment override
    is how a test aims it somewhere writable.
    """
    monkeypatch.setenv("LINODEMCP_CONFIG_PATH", str(path))


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
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Happy path for a brand-new user-defined profile."""
    draft = install_fixtures.create(_SAVE_DRAFT_NAME)
    draft.description = "saved via test"
    draft.allowed_tools = [_TOOL_HELLO, _TOOL_INSTANCE_BOOT]

    aim_config_at(monkeypatch, writable_config)

    response = await handle_linode_profile_draft_save(
        {"name": _SAVE_DRAFT_NAME, "confirm": True}, Config()
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
    monkeypatch: pytest.MonkeyPatch,
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

    aim_config_at(monkeypatch, writable_config)

    response = await handle_linode_profile_draft_save(
        {"name": _SAVE_DRAFT_NAME, "confirm": True}, Config()
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
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Without confirm=true the save refuses and writes nothing."""
    install_fixtures.create(_SAVE_DRAFT_NAME)
    aim_config_at(monkeypatch, writable_config)

    original = writable_config.read_text()

    response = await handle_linode_profile_draft_save(
        {"name": _SAVE_DRAFT_NAME}, Config()
    )

    assert response[0].text == "Error: confirm=true is required for draft save"

    assert writable_config.read_text() == original, (
        "refused save must not write to disk"
    )


@pytest.mark.asyncio
async def test_save_refuses_builtin_name(
    install_fixtures: Registry,
    writable_config: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Save target name matching a built-in is refused."""
    install_fixtures.create("compute-admin")
    aim_config_at(monkeypatch, writable_config)

    response = await handle_linode_profile_draft_save(
        {"name": "compute-admin", "confirm": True}, Config()
    )

    assert response[0].text == (
        "Error: cannot save over built-in profile name: compute-admin"
    )


@pytest.mark.asyncio
async def test_save_refuses_unknown_draft(
    writable_config: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Save on a draft the registry does not hold refuses by name."""
    aim_config_at(monkeypatch, writable_config)

    response = await handle_linode_profile_draft_save(
        {"name": "nonexistent-draft", "confirm": True}, Config()
    )

    assert response[0].text == "Error: draft not found: nonexistent-draft"


@pytest.mark.asyncio
async def test_save_refuses_missing_name() -> None:
    """An absent name answers the shared refusal."""
    response = await handle_linode_profile_draft_save({"confirm": True}, Config())

    assert response[0].text == _NAME_MISSING


@pytest.mark.asyncio
async def test_save_reports_load_failure(
    install_fixtures: Registry,
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A config path naming no readable file refuses instead of raising.

    The draft is real and confirmed by then, so a load failure that escaped the
    handler would reach the caller as a transport error rather than something
    the model can act on.
    """
    install_fixtures.create(_SAVE_DRAFT_NAME)
    missing = tmp_path / "absent.yml"
    aim_config_at(monkeypatch, missing)

    response = await handle_linode_profile_draft_save(
        {"name": _SAVE_DRAFT_NAME, "confirm": True}, Config()
    )

    assert response[0].text.startswith("Error: failed to load config from ")
    assert str(missing) in response[0].text


@pytest.mark.asyncio
async def test_save_reports_write_failure(
    install_fixtures: Registry,
    writable_config: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A config that loads but cannot be written back refuses.

    The draft has already been merged into the in-memory config by then, so a
    swallowed write error would report a saved profile that only exists in this
    process and vanishes on restart.
    """
    install_fixtures.create(_SAVE_DRAFT_NAME)
    aim_config_at(monkeypatch, writable_config)

    def refuse_write(*_args: object, **_kwargs: object) -> None:
        raise OSError(errno.EACCES, "permission denied")

    monkeypatch.setattr(
        "linodemcp.tools.linode_profile_draft_save.write_atomic", refuse_write
    )

    response = await handle_linode_profile_draft_save(
        {"name": _SAVE_DRAFT_NAME, "confirm": True}, Config()
    )

    assert response[0].text.startswith("Error: failed to write config to ")
    assert "permission denied" in response[0].text
    assert "profiles" not in writable_config.read_text(), (
        "a refused write must not reach disk"
    )


@pytest.mark.asyncio
async def test_save_response_has_expected_shape(
    install_fixtures: Registry,
    writable_config: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """JSON response carries every top-level field defined by the wire contract."""
    draft = install_fixtures.create(_SAVE_DRAFT_NAME)
    draft.allowed_tools = [_TOOL_HELLO]

    aim_config_at(monkeypatch, writable_config)

    response = await handle_linode_profile_draft_save(
        {"name": _SAVE_DRAFT_NAME, "confirm": True}, Config()
    )

    payload = _parse_response(response[0].text)
    for key in ("name", "is_new", "added_tools", "removed_tools", "changed_fields"):
        assert key in payload, f"response must include {key}"
