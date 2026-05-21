"""Tests for monitor write tools."""

from typing import Any, cast
from unittest.mock import AsyncMock

import pytest

from linodemcp.tools import linode_monitor_write as monitor_write


class _FakeClient:
    def __init__(self) -> None:
        self.update_monitor_alert_definition = AsyncMock(
            return_value={"id": 42, "label": "cpu high"}
        )


def _text(result: list[Any]) -> str:
    return str(result[0].text)


def test_update_alert_definition_tool_schema_requires_confirm() -> None:
    tool, capability = (
        monitor_write.create_linode_monitor_alert_definition_update_tool()
    )
    assert tool.name == "linode_monitor_alert_definition_update"
    assert capability.name == "Write"
    assert "confirm" in tool.inputSchema["required"]
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_update_alert_definition_requires_explicit_boolean_confirm(
    monkeypatch: pytest.MonkeyPatch, confirm: object
) -> None:
    async def fake_execute_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute_tool)
    arguments: dict[str, Any] = {"service_type": "linode", "alert_id": 42}
    if confirm is not None:
        arguments["confirm"] = confirm
    result = await monitor_write.handle_linode_monitor_alert_definition_update(
        arguments, cast("Any", object())
    )
    assert "confirm=true" in _text(result)


@pytest.mark.asyncio
@pytest.mark.parametrize("service_type", ["bad/type", "bad?type", "..", "bad type"])
async def test_update_alert_definition_rejects_malformed_service_type(
    monkeypatch: pytest.MonkeyPatch, service_type: str
) -> None:
    async def fake_execute_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute_tool)
    result = await monitor_write.handle_linode_monitor_alert_definition_update(
        {"service_type": service_type, "alert_id": 42, "confirm": True},
        cast("Any", object()),
    )
    assert "service_type" in _text(result)


@pytest.mark.asyncio
@pytest.mark.parametrize("alert_id", [True, "42", 4.2, 0, -1])
async def test_update_alert_definition_rejects_invalid_alert_id(
    monkeypatch: pytest.MonkeyPatch, alert_id: object
) -> None:
    async def fake_execute_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute_tool)
    result = await monitor_write.handle_linode_monitor_alert_definition_update(
        {"service_type": "linode", "alert_id": alert_id, "confirm": True},
        cast("Any", object()),
    )
    assert "alert_id" in _text(result)


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"service_type": "linode", "alert_id": 42, "confirm": True},
        {
            "service_type": "linode",
            "alert_id": 42,
            "confirm": True,
            "label": None,
            "status": None,
        },
    ],
)
async def test_update_alert_definition_rejects_empty_update_payload(
    monkeypatch: pytest.MonkeyPatch, arguments: dict[str, Any]
) -> None:
    async def fake_execute_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute_tool)
    result = await monitor_write.handle_linode_monitor_alert_definition_update(
        arguments,
        cast("Any", object()),
    )
    assert "update field" in _text(result)


@pytest.mark.asyncio
async def test_update_alert_definition_calls_client_once_without_retry(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = _FakeClient()

    async def fake_execute_tool(
        cfg: object, arguments: dict[str, Any], action: str, call: Any
    ) -> list[Any]:
        assert action == "update monitor alert definition"
        payload = await call(fake)
        return [type("Text", (), {"text": str(payload)})()]

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute_tool)
    result = await monitor_write.handle_linode_monitor_alert_definition_update(
        {
            "service_type": "linode",
            "alert_id": 42,
            "label": "cpu high",
            "status": "enabled",
            "confirm": True,
        },
        cast("Any", object()),
    )
    fake.update_monitor_alert_definition.assert_awaited_once_with(
        "linode", 42, label="cpu high", status="enabled"
    )
    assert "Monitor alert definition 42 updated" in _text(result)
