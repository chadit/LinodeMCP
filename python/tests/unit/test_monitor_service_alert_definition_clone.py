"""Focused tests for monitor alert-definition clone."""

import json
from typing import Any, cast
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient, RetryConfig
from linodemcp.profiles import Capability
from linodemcp.tools import linode_monitor_write as monitor_write


def _text(result: list[Any]) -> str:
    return str(result[0].text)


@pytest.mark.asyncio
async def test_client_clone_posts_escaped_path_and_preserves_empty_overrides() -> None:
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.status_code = 200
    response.json.return_value = {"id": 20001, "label": "CPU Clone"}

    with patch.object(client.client, "request", new_callable=AsyncMock) as request:
        request.return_value = response
        result = await client.clone_monitor_service_alert_definition(
            "dbaas/postgres",
            20000,
            label="  CPU Clone  ",
            channel_ids=[],
            description="",
            entity_ids=[],
            group_by=[],
            regions=[],
            rule_criteria={},
            severity=0,
            trigger_conditions={},
        )

    assert result == {"id": 20001, "label": "CPU Clone"}
    assert request.call_args[0][0] == "POST"
    assert request.call_args[0][1].endswith(
        "/monitor/services/dbaas%2Fpostgres/alert-definitions/20000/clone"
    )
    assert request.call_args[1]["json"] == {
        "label": "CPU Clone",
        "channel_ids": [],
        "description": "",
        "entity_ids": [],
        "group_by": [],
        "regions": [],
        "rule_criteria": {},
        "severity": 0,
        "trigger_conditions": {},
    }
    await client.close()


@pytest.mark.asyncio
async def test_client_clone_omits_absent_optional_fields() -> None:
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.status_code = 200
    response.json.return_value = {"id": 20001, "label": "CPU Clone"}

    with patch.object(client.client, "request", new_callable=AsyncMock) as request:
        request.return_value = response
        await client.clone_monitor_service_alert_definition(
            "dbaas", 20000, label="CPU Clone"
        )

    assert request.call_args[1]["json"] == {"label": "CPU Clone"}
    await client.close()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("kwargs", "message"),
    [
        ({"service_type": "", "alert_id": 1, "label": "clone"}, "service_type"),
        ({"service_type": "dbaas", "alert_id": True, "label": "clone"}, "alert_id"),
        ({"service_type": "dbaas", "alert_id": 0, "label": "clone"}, "positive"),
        ({"service_type": "dbaas", "alert_id": 1, "label": " "}, "label"),
        (
            {
                "service_type": "dbaas",
                "alert_id": 1,
                "label": "clone",
                "channel_ids": [True],
            },
            "channel_ids",
        ),
        (
            {
                "service_type": "dbaas",
                "alert_id": 1,
                "label": "clone",
                "channel_ids": "1",
            },
            "channel_ids",
        ),
        (
            {
                "service_type": "dbaas",
                "alert_id": 1,
                "label": "clone",
                "entity_ids": [1],
            },
            "entity_ids",
        ),
        (
            {
                "service_type": "dbaas",
                "alert_id": 1,
                "label": "clone",
                "entity_ids": [" "],
            },
            "entity_ids",
        ),
        (
            {
                "service_type": "dbaas",
                "alert_id": 1,
                "label": "clone",
                "regions": [1],
            },
            "regions",
        ),
        (
            {
                "service_type": "dbaas",
                "alert_id": 1,
                "label": "clone",
                "severity": 4,
            },
            "severity",
        ),
    ],
)
async def test_client_clone_rejects_invalid_inputs_before_request(
    kwargs: dict[str, Any], message: str
) -> None:
    client = Client("https://api.linode.com/v4", "test-token")
    with patch.object(client.client, "request", new_callable=AsyncMock) as request:
        with pytest.raises((TypeError, ValueError), match=message):
            await client.clone_monitor_service_alert_definition(**kwargs)
        request.assert_not_called()
    await client.close()


@pytest.mark.asyncio
async def test_retryable_clone_does_not_retry_transient_failure() -> None:
    retryable = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=2, base_delay=0.01),
    )
    with patch.object(
        retryable.client,
        "clone_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as clone:
        clone.side_effect = NetworkError(
            "CloneMonitorServiceAlertDefinition",
            httpx.TimeoutException("timeout"),
        )
        with pytest.raises(NetworkError):
            await retryable.clone_monitor_service_alert_definition(
                "dbaas", 20000, label="CPU Clone", rule_criteria={}
            )

    clone.assert_awaited_once_with("dbaas", 20000, label="CPU Clone", rule_criteria={})
    await retryable.close()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "failure",
    [
        httpx.ConnectTimeout("connect timeout"),
        httpx.ReadTimeout("read timeout"),
        httpx.HTTPStatusError(
            "server error",
            request=httpx.Request("POST", "https://api.linode.com/v4"),
            response=httpx.Response(
                500,
                request=httpx.Request("POST", "https://api.linode.com/v4"),
            ),
        ),
        httpx.HTTPError("transport error"),
    ],
)
async def test_client_clone_wraps_http_failures(failure: httpx.HTTPError) -> None:
    client = Client("https://api.linode.com/v4", "test-token")
    with (
        patch.object(
            client, "make_request", new_callable=AsyncMock, side_effect=failure
        ),
        pytest.raises(NetworkError, match="CloneMonitorServiceAlertDefinition"),
    ):
        await client.clone_monitor_service_alert_definition(
            "dbaas", 20000, label="CPU Clone"
        )
    await client.close()


def test_clone_tool_is_exported_with_write_capability_and_contract_schema() -> None:
    from linodemcp import tools as tools_module

    assert (
        "create_linode_monitor_service_alert_definition_clone_tool"
        in tools_module.__all__
    )
    assert (
        "handle_linode_monitor_service_alert_definition_clone" in tools_module.__all__
    )

    tool, capability = (
        monitor_write.create_linode_monitor_service_alert_definition_clone_tool()
    )
    assert tool.name == "linode_monitor_service_alert_definition_clone"
    assert capability is Capability.Write
    assert set(tool.inputSchema["required"]) == {
        "service_type",
        "alert_id",
        "label",
        "confirm",
    }


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_clone_requires_explicit_boolean_confirmation(
    monkeypatch: pytest.MonkeyPatch, confirm: object
) -> None:
    async def fail_execute(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(monitor_write, "execute_tool", fail_execute)
    arguments: dict[str, Any] = {
        "service_type": "dbaas",
        "alert_id": 20000,
        "label": "CPU Clone",
    }
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await monitor_write.handle_linode_monitor_service_alert_definition_clone(
        arguments, cast("Any", object())
    )

    assert _text(result) == (
        "Error: This clones a monitor alert definition. Set confirm=true to proceed."
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("field", "value", "message"),
    [
        ("service_type", "dbaas/postgres", "service_type"),
        ("service_type", "DBAAS", "service_type"),
        ("alert_id", True, "alert_id"),
        ("alert_id", 0, "positive"),
        ("label", " ", "label"),
        ("channel_ids", ["1"], "channel_ids"),
        ("channel_ids", "1", "channel_ids"),
        ("description", 1, "description"),
        ("entity_ids", [1], "entity_ids"),
        ("entity_ids", [" "], "entity_ids"),
        ("group_by", [1], "group_by"),
        ("regions", [1], "regions"),
        ("rule_criteria", [], "rule_criteria"),
        ("severity", 4, "severity"),
        ("trigger_conditions", "ALL", "trigger_conditions"),
    ],
)
async def test_clone_handler_rejects_invalid_fields(
    monkeypatch: pytest.MonkeyPatch, field: str, value: object, message: str
) -> None:
    async def fail_execute(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(monitor_write, "execute_tool", fail_execute)
    arguments: dict[str, Any] = {
        "service_type": "dbaas",
        "alert_id": 20000,
        "label": "CPU Clone",
        "confirm": True,
    }
    arguments[field] = value

    result = await monitor_write.handle_linode_monitor_service_alert_definition_clone(
        arguments, cast("Any", object())
    )
    assert message in _text(result)


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        (
            {"alert_id": 1, "label": "clone", "confirm": True},
            "service_type is required",
        ),
        (
            {
                "service_type": None,
                "alert_id": 1,
                "label": "clone",
                "confirm": True,
            },
            "service_type must be a string",
        ),
        (
            {
                "service_type": " dbaas ",
                "alert_id": 1,
                "label": "clone",
                "confirm": True,
            },
            "service_type must be a single non-empty service type slug",
        ),
        (
            {
                "service_type": "dbaas/postgres",
                "alert_id": 1,
                "label": "clone",
                "confirm": True,
            },
            "service_type must be a single non-empty service type slug",
        ),
        (
            {
                "service_type": "",
                "alert_id": 1,
                "label": "clone",
                "confirm": True,
            },
            "service_type must be a single non-empty service type slug",
        ),
        (
            {"service_type": "dbaas", "label": "clone", "confirm": True},
            "alert_id is required",
        ),
        (
            {
                "service_type": "dbaas",
                "alert_id": True,
                "label": "clone",
                "confirm": True,
            },
            "alert_id must be a positive integer",
        ),
        (
            {
                "service_type": "dbaas",
                "alert_id": 0,
                "label": "clone",
                "confirm": True,
            },
            "alert_id must be a positive integer",
        ),
        (
            {
                "service_type": "dbaas",
                "alert_id": 1.5,
                "label": "clone",
                "confirm": True,
            },
            "alert_id must be a positive integer",
        ),
        (
            {
                "service_type": "dbaas",
                "alert_id": float("inf"),
                "label": "clone",
                "confirm": True,
            },
            "alert_id must be a positive integer",
        ),
    ],
)
async def test_clone_target_validation_matches_go_exactly(
    monkeypatch: pytest.MonkeyPatch,
    arguments: dict[str, Any],
    message: str,
) -> None:
    async def fail_execute(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(monitor_write, "execute_tool", fail_execute)
    result = await monitor_write.handle_linode_monitor_service_alert_definition_clone(
        arguments, cast("Any", object())
    )

    assert _text(result) == f"Error: {message}"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("field", "value", "message"),
    [
        ("channel_ids", [True], "channel_ids must be an array of integers"),
        ("channel_ids", [1.5], "channel_ids must be an array of integers"),
        ("channel_ids", [float("nan")], "channel_ids must be an array of integers"),
        ("severity", True, "severity must be an integer from 0 through 3"),
        ("severity", 1.5, "severity must be an integer from 0 through 3"),
        ("severity", float("-inf"), "severity must be an integer from 0 through 3"),
    ],
)
async def test_clone_rejects_fractional_and_non_finite_numbers(
    monkeypatch: pytest.MonkeyPatch,
    field: str,
    value: object,
    message: str,
) -> None:
    async def fail_execute(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(monitor_write, "execute_tool", fail_execute)
    arguments: dict[str, Any] = {
        "service_type": "dbaas",
        "alert_id": 20000,
        "label": "CPU Clone",
        "confirm": True,
        field: value,
    }
    result = await monitor_write.handle_linode_monitor_service_alert_definition_clone(
        arguments, cast("Any", object())
    )

    assert _text(result) == f"Error: {message}"


@pytest.mark.asyncio
async def test_clone_success_coerces_integral_floats_and_preserves_group_by_response(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    client = AsyncMock()
    client.clone_monitor_service_alert_definition.return_value = {
        "id": 20001,
        "label": "CPU Clone",
        "group_by": ["entity_id"],
    }

    async def fake_execute(
        cfg: object, arguments: dict[str, Any], action: str, call: Any
    ) -> list[Any]:
        assert action == "clone monitor alert definition"
        payload = await call(client)
        return [type("Text", (), {"text": json.dumps(payload)})()]

    monkeypatch.setattr(monitor_write, "execute_tool", fake_execute)
    result = await monitor_write.handle_linode_monitor_service_alert_definition_clone(
        {
            "service_type": "db--aas",
            "alert_id": 20000.0,
            "label": "  CPU Clone  ",
            "channel_ids": [101.0, 202],
            "description": "",
            "entity_ids": ["13116", "13217"],
            "group_by": ["entity_id"],
            "regions": ["us-east", "us-iad"],
            "rule_criteria": {},
            "severity": 2.0,
            "trigger_conditions": {},
            "confirm": True,
        },
        cast("Any", object()),
    )

    client.clone_monitor_service_alert_definition.assert_awaited_once_with(
        "db--aas",
        20000,
        label="CPU Clone",
        channel_ids=[101, 202],
        description="",
        entity_ids=["13116", "13217"],
        group_by=["entity_id"],
        regions=["us-east", "us-iad"],
        rule_criteria={},
        severity=2,
        trigger_conditions={},
    )
    call = client.clone_monitor_service_alert_definition.await_args
    assert type(call.args[1]) is int
    assert all(type(value) is int for value in call.kwargs["channel_ids"])
    assert type(call.kwargs["severity"]) is int
    payload = json.loads(_text(result))
    assert payload["message"] == "Monitor alert definition 20000 cloned"
    assert payload["alert_definition"]["group_by"] == ["entity_id"]


@pytest.mark.asyncio
async def test_clone_dry_run_previews_post_body_and_fetches_source(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    client = AsyncMock()
    client.get_monitor_service_alert_definition.return_value = {
        "id": 20000,
        "label": "CPU",
    }

    async def fake_dry_run(
        cfg: object,
        arguments: dict[str, Any],
        tool_name: str,
        method: str,
        path: str,
        fetch_state: Any,
        *,
        request_body: Any,
    ) -> list[Any]:
        assert tool_name == "linode_monitor_service_alert_definition_clone"
        assert method == "POST"
        assert path == "/monitor/services/dbaas/alert-definitions/20000/clone"
        assert request_body == {
            "label": "CPU Clone",
            "channel_ids": [],
            "rule_criteria": {},
        }
        assert await fetch_state(client) == {"id": 20000, "label": "CPU"}
        return [type("Text", (), {"text": "preview"})()]

    monkeypatch.setattr(monitor_write, "execute_dry_run", fake_dry_run)
    result = await monitor_write.handle_linode_monitor_service_alert_definition_clone(
        {
            "service_type": "dbaas",
            "alert_id": 20000,
            "label": "CPU Clone",
            "channel_ids": [],
            "rule_criteria": {},
            "dry_run": True,
        },
        cast("Any", object()),
    )

    assert _text(result) == "preview"
    client.get_monitor_service_alert_definition.assert_awaited_once_with("dbaas", 20000)
