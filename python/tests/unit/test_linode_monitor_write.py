"""Tests for monitor write tools."""

import json
from typing import Any, cast
from unittest.mock import AsyncMock

import pytest

from linodemcp.genpb.linode.mcp.v1 import monitor_pb2
from linodemcp.tools import linode_monitor_write as monitor_write
from linodemcp.tools.proto_response import serialize_api_response


class _FakeClient:
    def __init__(self) -> None:
        self.update_monitor_alert_definition = AsyncMock(
            return_value={"id": 42, "label": "cpu high"}
        )


def _text(result: list[Any]) -> str:
    return str(result[0].text)


def _json(result: list[Any]) -> dict[str, Any]:
    parsed: dict[str, Any] = json.loads(result[0].text)
    return parsed


async def _run_call(
    monkeypatch: pytest.MonkeyPatch,
    handler: Any,
    arguments: dict[str, Any],
    client: Any,
    *,
    expected_action: str | None = None,
) -> list[Any]:
    """Drive a handler with a fake execute_tool that runs the inner _call and
    serializes the resulting dict to JSON text, mirroring the real transport.
    """

    async def fake_execute_tool(
        cfg: object, args: dict[str, Any], action: str, call: Any
    ) -> list[Any]:
        if expected_action is not None:
            assert action == expected_action
        payload = await call(client)
        return [type("Text", (), {"text": json.dumps(payload)})()]

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute_tool)
    return cast("list[Any]", await handler(arguments, cast("Any", object())))


def test_update_alert_definition_tool_schema_requires_confirm() -> None:
    tool, capability = (
        monitor_write.create_linode_monitor_service_alert_definition_update_tool()
    )
    assert tool.name == "linode_monitor_service_alert_definition_update"
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
    result = await monitor_write.handle_linode_monitor_service_alert_definition_update(
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
    result = await monitor_write.handle_linode_monitor_service_alert_definition_update(
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
    result = await monitor_write.handle_linode_monitor_service_alert_definition_update(
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
    result = await monitor_write.handle_linode_monitor_service_alert_definition_update(
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
    result = await monitor_write.handle_linode_monitor_service_alert_definition_update(
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


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("bad_args", "expected"),
    [
        ({"page": True}, "page must be an integer"),
        ({"page": "2"}, "page must be an integer"),
        ({"page": 0}, "page must be at least 1"),
        ({"page_size": True}, "page_size must be an integer"),
        ({"page_size": 10}, "page_size must be at least 25"),
        ({"page_size": 999}, "page_size must be at most 500"),
    ],
)
async def test_alert_definition_list_validates_pagination(
    monkeypatch: pytest.MonkeyPatch, bad_args: dict[str, Any], expected: str
) -> None:
    async def fake_execute_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called on bad pagination")

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute_tool)
    result = await monitor_write.handle_linode_monitor_alert_definition_list(
        bad_args, cast("Any", object())
    )
    assert expected in _text(result)


@pytest.mark.asyncio
async def test_service_list_emits_proto_envelope(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    client = AsyncMock()
    client.list_monitor_services = AsyncMock(
        return_value={"data": [{"label": "Databases", "service_type": "dbaas"}]}
    )

    async def fake_execute_tool(
        cfg: object, arguments: dict[str, Any], action: str, call: Any
    ) -> list[Any]:
        assert action == "list monitor services"
        payload = await call(client)
        return [type("Text", (), {"text": str(payload)})()]

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute_tool)
    result = await monitor_write.handle_linode_monitor_service_list(
        {}, cast("Any", object())
    )
    text = _text(result)
    assert "'count': 1" in text
    assert "'services'" in text
    assert "dbaas" in text
    assert "page" not in text


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "handler",
    [
        monitor_write.handle_linode_monitor_alert_definition_list,
        monitor_write.handle_linode_monitor_alert_channel_list,
        monitor_write.handle_linode_monitor_dashboard_list,
    ],
)
async def test_list_handlers_reject_invalid_pagination(
    monkeypatch: pytest.MonkeyPatch, handler: Any
) -> None:
    async def fake_execute_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called on bad pagination")

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute_tool)
    result = await handler({"page": 0}, cast("Any", object()))
    assert "page must be at least 1" in _text(result)


@pytest.mark.asyncio
async def test_alert_channel_list_emits_proto_envelope(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    client = AsyncMock()
    client.list_monitor_alert_channels = AsyncMock(
        return_value={
            "data": [
                {
                    "id": 10000,
                    "label": "Email Ops",
                    "channel_type": "email",
                    "content": {"email": {"email_addresses": ["ops@example.com"]}},
                }
            ]
        }
    )

    async def fake_execute_tool(
        cfg: object, arguments: dict[str, Any], action: str, call: Any
    ) -> list[Any]:
        payload = await call(client)
        return [type("Text", (), {"text": str(payload)})()]

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute_tool)
    result = await monitor_write.handle_linode_monitor_alert_channel_list(
        {"page": 2, "page_size": 50}, cast("Any", object())
    )
    text = _text(result)
    assert "'count': 1" in text
    assert "'alert_channels'" in text
    assert "ops@example.com" in text
    client.list_monitor_alert_channels.assert_awaited_once_with(page=2, page_size=50)


@pytest.mark.asyncio
async def test_service_metric_definition_list_emits_proto_envelope(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    client = AsyncMock()
    client.list_monitor_service_metric_definitions = AsyncMock(
        return_value={
            "data": [{"label": "CPU Utilization", "metric": "cpu_usage"}],
            "page": 1,
            "pages": 1,
            "results": 1,
        }
    )

    async def fake_execute_tool(
        cfg: object, arguments: dict[str, Any], action: str, call: Any
    ) -> list[Any]:
        assert action == "list monitor service metric definitions"
        payload = await call(client)
        return [type("Text", (), {"text": str(payload)})()]

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute_tool)
    result = await monitor_write.handle_linode_monitor_service_metric_definition_list(
        {"service_type": "dbaas"}, cast("Any", object())
    )
    text = _text(result)
    assert "'count': 1" in text
    assert "'metric_definitions'" in text
    assert "cpu_usage" in text
    assert "'metric_type': ''" in text
    assert "page" not in text
    client.list_monitor_service_metric_definitions.assert_awaited_once_with("dbaas")


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "handler",
    [
        monitor_write.handle_linode_monitor_service_metric_definition_list,
        monitor_write.handle_linode_monitor_service_alert_definition_list,
    ],
)
async def test_service_definition_list_rejects_bad_service_type(
    monkeypatch: pytest.MonkeyPatch, handler: Any
) -> None:
    async def fake_execute_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not run on bad service_type")

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute_tool)
    result = await handler({"service_type": "bad/type"}, cast("Any", object()))
    assert "service_type is required" in _text(result)


@pytest.mark.asyncio
async def test_metric_query_emits_proto_envelope(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    metrics = {"data": [{"label": "cpu", "value": 1.5}], "status": "success"}
    client = AsyncMock()
    client.read_monitor_service_metrics = AsyncMock(return_value=metrics)

    result = await _run_call(
        monkeypatch,
        monitor_write.handle_linode_monitor_service_metric_query,
        {"service_type": "dbaas"},
        client,
        expected_action="read monitor service metrics",
    )

    expected = serialize_api_response(
        {
            "message": "Monitor service metrics read for 'dbaas'",
            "service_type": "dbaas",
            "metrics": metrics,
        },
        monitor_pb2.MonitorServiceMetricQueryResponse(),
    )
    assert _json(result) == expected
    client.read_monitor_service_metrics.assert_awaited_once_with("dbaas")


@pytest.mark.asyncio
async def test_alert_definition_create_emits_proto_envelope(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    definition = {"id": 67890, "label": "CPU high", "service_type": "linode"}
    client = AsyncMock()
    client.create_monitor_service_alert_definition = AsyncMock(return_value=definition)

    result = await _run_call(
        monkeypatch,
        monitor_write.handle_linode_monitor_service_alert_definition_create,
        {
            "service_type": "linode",
            "label": "CPU high",
            "severity": 1,
            "rule_criteria": {"rules": [{"metric": "cpu_usage"}]},
            "trigger_conditions": {"criteria_condition": "ALL"},
            "channel_ids": [10000],
            "confirm": True,
        },
        client,
        expected_action="create monitor service alert definition",
    )

    expected = serialize_api_response(
        {
            "message": "Monitor service alert definition created for 'linode'",
            "alert_definition": definition,
        },
        monitor_pb2.MonitorAlertDefinitionWriteResponse(),
    )
    body = _json(result)
    assert body == expected
    # The create envelope drops the top-level service_type echo per the ruling.
    assert "service_type" not in body


@pytest.mark.asyncio
async def test_alert_definition_update_emits_proto_envelope(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    definition = {"id": 42, "label": "cpu high", "status": "enabled"}
    client = AsyncMock()
    client.update_monitor_alert_definition = AsyncMock(return_value=definition)

    result = await _run_call(
        monkeypatch,
        monitor_write.handle_linode_monitor_service_alert_definition_update,
        {
            "service_type": "linode",
            "alert_id": 42,
            "label": "cpu high",
            "status": "enabled",
            "confirm": True,
        },
        client,
        expected_action="update monitor alert definition",
    )

    expected = serialize_api_response(
        {
            "message": "Monitor alert definition 42 updated",
            "alert_definition": definition,
        },
        monitor_pb2.MonitorAlertDefinitionWriteResponse(),
    )
    body = _json(result)
    assert body == expected
    # service_type and alert_id top-level echoes are dropped per the ruling.
    assert "service_type" not in body
    assert "alert_id" not in body


@pytest.mark.asyncio
async def test_alert_definition_delete_emits_proto_envelope(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    client = AsyncMock()
    client.delete_monitor_service_alert_definition = AsyncMock(return_value={})

    result = await _run_call(
        monkeypatch,
        monitor_write.handle_linode_monitor_service_alert_definition_delete,
        {"service_type": "dbaas", "alert_id": 20000, "confirm": True},
        client,
        expected_action="delete monitor service alert definition",
    )

    expected = serialize_api_response(
        {
            "message": "Monitor service alert definition 20000 deleted for 'dbaas'",
            "service_type": "dbaas",
            "alert_id": 20000,
        },
        monitor_pb2.MonitorAlertDefinitionDeleteResponse(),
    )
    assert _json(result) == expected
    client.delete_monitor_service_alert_definition.assert_awaited_once_with(
        "dbaas", 20000
    )
