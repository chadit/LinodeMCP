"""Argument-coercion parity with the Go accessors.

Every case here is a call whose arguments are well formed enough to reach a
handler but wrongly typed for the field they name. Go reads each one through
mcp-go's typed getters and coerces to the default; Python used to reach into
the arguments dict per tool and answer something else. These tests hold both
languages to the Go reading, tool by tool, so the divergence cannot come back
one handler at a time.

Mirrors the Go-side cases in
``go/internal/tools/argument_coercion_test.go``.
"""

from __future__ import annotations

import json
from datetime import UTC, datetime
from typing import TYPE_CHECKING, Any, cast

import pytest

from linodemcp.audit import (
    Capability as AuditCapability,
)
from linodemcp.audit import (
    Event,
    Mode,
    Status,
    event_timestamp,
)
from linodemcp.config import Config
from linodemcp.genlocal import (
    record_audit_event,
)
from linodemcp.gentools.audit import (
    handle_linode_audit_export,
    handle_linode_audit_recent,
)
from linodemcp.gentools.profile_builder import (
    handle_linode_profile_draft_add_tools,
    handle_linode_profile_draft_new,
    handle_linode_profile_draft_set,
)
from linodemcp.profiles import Capability
from linodemcp.profiles.builder import Registry
from linodemcp.profiles.builtin import ToolDescriptor
from linodemcp.profiles.profile import Profile
from linodemcp.tools.argreader import (
    tool_bool,
    tool_int,
    tool_string,
    tool_string_list,
)
from linodemcp.tools.builderstate import (
    BuilderState,
    reset_builder_state,
    set_builder_state,
)

if TYPE_CHECKING:
    from collections.abc import Iterator
    from pathlib import Path

_DRAFT_NAME = "coercion-draft"
_TOOL_INSTANCE_BOOT = "linode_instance_boot"
_NAME_MISSING = "Error: name argument is required"


def _catalog() -> list[ToolDescriptor]:
    """A two-entry catalog, enough for one literal add to match."""
    return [
        ToolDescriptor(name=_TOOL_INSTANCE_BOOT, capability=Capability.Write),
        ToolDescriptor(name="hello", capability=Capability.Meta),
    ]


def _no_profile() -> Profile:
    """The active-profile reader for tools that never read one."""
    return Profile(name="test", description="", allowed_tools=())


def _read_event(second: int) -> Event:
    """One exportable read event, at a distinct second so order is stable."""
    ts = datetime(2026, 5, 20, 0, 0, second, tzinfo=UTC)
    return Event(
        ts=event_timestamp(ts),
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id=f"evt_{second}",
        tool="linode_instance_list",
        tool_capability=AuditCapability.READ.value,
        environment="default",
        profile="operator",
        mode=Mode.NORMAL.value,
        plan_id=None,
        args={},
        args_redacted=[],
        status=Status.SUCCESS.value,
        latency_ms=0,
        result_summary="",
        error=None,
        linodemcp_version="0.1.0",
        session_id="session-1",
        credential_generation=1,
    )


@pytest.fixture
def registry() -> Iterator[Registry]:
    """Publish a builder state carrying registry plus catalog; reset after."""
    drafts = Registry()
    token = set_builder_state(
        BuilderState(
            drafts=drafts,
            catalog=_catalog,
            active_profile=_no_profile,
            config=Config(),
        )
    )

    yield drafts

    reset_builder_state(token)


def _payload(text: str) -> dict[str, object]:
    """Parse a handler's JSON answer into a typed dict."""
    parsed: object = json.loads(text)
    assert isinstance(parsed, dict)
    return cast("dict[str, object]", parsed)


def test_tool_string_refuses_a_non_string() -> None:
    """A wrongly typed value reads as the default, the way GetString does."""
    assert tool_string({"name": 123}, "name") == ""
    assert tool_string({"name": None}, "name") == ""
    assert tool_string({}, "name", "fallback") == "fallback"
    assert tool_string({"name": "draft"}, "name") == "draft"


@pytest.mark.parametrize(
    ("value", "want"),
    [
        (5, 5),
        (5.7, 5),
        (-5.7, -5),
        ("5000", 5000),
        ("+7", 7),
        ("abc", 0),
        ("1_0", 0),
        (" 5 ", 0),
        (True, 0),
        (None, 0),
        (str(1 << 63), 0),
    ],
)
def test_tool_int_matches_the_go_reading(value: Any, want: int) -> None:
    """Each spelling reads the way GetInt reads it, including the refusals."""
    assert tool_int({"limit": value}, "limit") == want


@pytest.mark.parametrize(
    ("value", "want"),
    [
        (True, True),
        (False, False),
        ("false", False),
        ("False", False),
        ("f", False),
        ("true", True),
        ("1", True),
        ("0", False),
        ("yes", False),
        (1, True),
        (0, False),
        (None, False),
    ],
)
def test_tool_bool_matches_the_go_reading(value: Any, want: bool) -> None:
    """Each spelling reads the way GetBool reads it, ParseBool set included."""
    assert tool_bool({"allow_yolo": value}, "allow_yolo") is want


def test_tool_string_list_drops_a_non_string_entry() -> None:
    """A non-string entry is dropped rather than coerced with str()."""
    assert tool_string_list({"tools": ["a", 42, None, "b"]}, "tools") == ["a", "b"]
    assert tool_string_list({"tools": "a"}, "tools") == []
    assert tool_string_list({}, "tools") == []


@pytest.mark.asyncio
async def test_draft_new_refuses_a_non_string_name(registry: Registry) -> None:
    """A non-string name refuses rather than keying a draft on the value."""
    result = await handle_linode_profile_draft_new({"name": 123}, Config())

    assert result[0].text == _NAME_MISSING
    assert registry.get("123") is None
    assert not registry.names()


@pytest.mark.asyncio
async def test_draft_set_reads_the_string_false_as_false(registry: Registry) -> None:
    """allow_yolo: "false" sets the flag False, not the True truthiness gives."""
    registry.create(_DRAFT_NAME)

    result = await handle_linode_profile_draft_set(
        {"name": _DRAFT_NAME, "allow_yolo": "false"}, Config()
    )

    changes = _payload(result[0].text)["changes"]
    assert isinstance(changes, dict)
    assert changes["allow_yolo"] is False

    draft = registry.get(_DRAFT_NAME)
    assert draft is not None
    assert draft.allow_yolo is False


@pytest.mark.asyncio
async def test_draft_set_drops_a_non_string_list_entry(registry: Registry) -> None:
    """A non-string entry is dropped, so no coerced value reaches the draft."""
    registry.create(_DRAFT_NAME)

    result = await handle_linode_profile_draft_set(
        {"name": _DRAFT_NAME, "allowed_environments": ["prod", 42]}, Config()
    )

    changes = _payload(result[0].text)["changes"]
    assert isinstance(changes, dict)
    assert changes["allowed_environments"] == ["prod"]

    draft = registry.get(_DRAFT_NAME)
    assert draft is not None
    assert list(draft.allowed_environments) == ["prod"]


@pytest.mark.asyncio
async def test_add_tools_drops_a_non_string_pattern(registry: Registry) -> None:
    """A non-string entry names no pattern, so only the named tool lands."""
    registry.create(_DRAFT_NAME)

    result = await handle_linode_profile_draft_add_tools(
        {"name": _DRAFT_NAME, "tools": [_TOOL_INSTANCE_BOOT, 42]}, Config()
    )

    added = _payload(result[0].text)["added"]
    assert added == [_TOOL_INSTANCE_BOOT]

    draft = registry.get(_DRAFT_NAME)
    assert draft is not None
    assert list(draft.allowed_tools) == [_TOOL_INSTANCE_BOOT]


@pytest.mark.asyncio
async def test_audit_recent_defaults_an_unparseable_limit(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """limit: "abc" answers the window rather than a Python parse failure."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    (tmp_path / "linodemcp").mkdir(parents=True)

    result = await handle_linode_audit_recent({"limit": "abc"}, Config())

    assert not result[0].text.startswith("Error: ")
    assert _payload(result[0].text)["count"] == 0


@pytest.mark.asyncio
async def test_audit_export_accepts_a_numeric_string_max_records(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """max_records: "1" caps the export at one record the way GetInt does."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)
    (audit_dir / "audit.log").write_text(
        "".join(
            record_audit_event(_read_event(second), "", "") + "\n" for second in (1, 2)
        ),
        encoding="utf-8",
    )

    result = await handle_linode_audit_export(
        {"format": "json", "max_records": "1"}, Config()
    )

    assert _payload(result[0].text)["record_count"] == 1
