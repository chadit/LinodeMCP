"""Tests for updating Linode instance configuration profile interfaces."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeVar, cast
from unittest.mock import AsyncMock

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry
from linodemcp.tools.linode_instances import (
    create_linode_instance_config_interface_update_tool,
    handle_linode_instance_config_interface_update,
)
from linodemcp.version import FEATURE_TOOLS_LIST

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable


T = TypeVar("T")


class _FailingRetryableClient(RetryableClient):
    """Retryable client test double that fails if replay retry is used."""

    def __init__(self) -> None:
        super().__init__("https://api.linode.com/v4", "test-token")
        self.retry_calls = 0

    async def _execute_with_retry(
        self, func: Callable[..., Awaitable[T]], *args: Any
    ) -> T:
        del func, args
        self.retry_calls += 1
        raise AssertionError("mutating update must not use replay retry")


@pytest.mark.asyncio
async def test_client_update_config_interface_sends_method_path_body() -> None:
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": 789, "primary": True})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.update_instance_config_interface(
            123,
            456,
            789,
            {
                "ip_ranges": ["192.0.2.0/24"],
                "ipv4": {"vpc": "10.0.0.2"},
                "primary": True,
            },
        )
    finally:
        await client.close()

    assert result == {"id": 789, "primary": True}
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "PUT"
    assert request.url.path == "/v4/linode/instances/123/configs/456/interfaces/789"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert json.loads(request.content) == {
        "ip_ranges": ["192.0.2.0/24"],
        "ipv4": {"vpc": "10.0.0.2"},
        "primary": True,
    }


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("linode_id", "config_id", "interface_id", "message"),
    [
        ("1/2", 456, 789, "linode_id must be a positive integer"),
        ("1?x=2", 456, 789, "linode_id must be a positive integer"),
        ("..", 456, 789, "linode_id must be a positive integer"),
        (0, 456, 789, "linode_id must be a positive integer"),
        (True, 456, 789, "linode_id must be a positive integer"),
        (123, "4/5", 789, "config_id must be a positive integer"),
        (123, "4?x=5", 789, "config_id must be a positive integer"),
        (123, "..", 789, "config_id must be a positive integer"),
        (123, 0, 789, "config_id must be a positive integer"),
        (123, False, 789, "config_id must be a positive integer"),
        (123, 456, "7/8", "interface_id must be a positive integer"),
        (123, 456, "7?x=8", "interface_id must be a positive integer"),
        (123, 456, "..", "interface_id must be a positive integer"),
        (123, 456, 0, "interface_id must be a positive integer"),
        (123, 456, True, "interface_id must be a positive integer"),
    ],
)
async def test_client_update_config_interface_rejects_bad_path_params(
    linode_id: Any, config_id: Any, interface_id: Any, message: str
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match=message):
            await client.update_instance_config_interface(
                linode_id, config_id, interface_id, {"primary": True}
            )
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_retryable_client_update_instance_config_interface_no_replay() -> None:
    retryable = _FailingRetryableClient()
    transient = httpx.ReadTimeout("timeout")
    mock_update = AsyncMock(
        side_effect=NetworkError("UpdateInstanceConfigInterface", transient)
    )
    cast("Any", retryable.client).update_instance_config_interface = mock_update

    try:
        with pytest.raises(NetworkError):
            await retryable.update_instance_config_interface(
                123, 456, 789, {"primary": True}
            )
    finally:
        await retryable.close()

    assert retryable.retry_calls == 0
    mock_update.assert_awaited_once_with(123, 456, 789, {"primary": True})


def test_create_linode_instance_config_interface_update_tool_schema() -> None:
    tool, capability = create_linode_instance_config_interface_update_tool()

    assert tool.name == "linode_instance_config_interface_update"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == [
        "linode_id",
        "config_id",
        "interface_id",
        "confirm",
    ]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["interface_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert "dry_run" not in tool.inputSchema["required"]
    assert {"required": ["ip_ranges"]} in tool.inputSchema["anyOf"]
    assert {"required": ["ipv4"]} in tool.inputSchema["anyOf"]
    assert {"required": ["primary"]} in tool.inputSchema["anyOf"]


@pytest.mark.asyncio
async def test_handle_linode_instance_config_interface_update_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.update_instance_config_interface.return_value = {
        "id": 789,
        "primary": True,
    }

    result = await handle_linode_instance_config_interface_update(
        {
            "linode_id": 123,
            "config_id": 456,
            "interface_id": 789,
            "ip_ranges": ["192.0.2.0/24"],
            "ipv4": {"vpc": "10.0.0.2"},
            "primary": True,
            "confirm": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["message"] == (
        "Configuration profile interface 789 updated on config 456 for instance 123"
    )
    assert payload["interface"]["id"] == 789
    assert payload["interface"]["primary"] is True
    mock_linode_client.update_instance_config_interface.assert_awaited_once_with(
        123,
        456,
        789,
        {
            "ip_ranges": ["192.0.2.0/24"],
            "ipv4": {"vpc": "10.0.0.2"},
            "primary": True,
        },
    )


@pytest.mark.asyncio
async def test_client_update_instance_config_interface_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="UpdateInstanceConfigInterface"):
            await client.update_instance_config_interface(
                123, 456, 789, {"primary": True}
            )
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_handle_linode_instance_config_interface_update_dry_run_skips_client(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_interface_update(
        {
            "linode_id": 123,
            "config_id": 456,
            "interface_id": 789,
            "primary": True,
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_config_interface_update"
    assert body["dry_run"] is True
    assert body["would_execute"]["method"] == "PUT"
    assert body["would_execute"]["path"] == (
        "/linode/instances/123/configs/456/interfaces/789"
    )
    assert body["would_execute"]["body"] == {"primary": True}
    assert body["side_effects"] == [
        "Interface 789 on configuration profile 456 for Linode 123 will be updated."
    ]
    mock_linode_client.update_instance_config_interface.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_update_config_interface_requires_confirm_true(
    confirm: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    arguments: dict[str, Any] = {
        "linode_id": 123,
        "config_id": 456,
        "interface_id": 789,
        "primary": True,
    }
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await handle_linode_instance_config_interface_update(
        arguments, sample_config
    )

    assert result[0].text == "Error: confirm must be true"
    mock_linode_client.update_instance_config_interface.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_instance_config_interface_update_requires_update_field(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_interface_update(
        {"linode_id": 123, "config_id": 456, "interface_id": 789, "confirm": True},
        sample_config,
    )

    assert result[0].text == "Error: at least one update field is required"
    mock_linode_client.update_instance_config_interface.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("field", "value", "message"),
    [
        ("ip_ranges", None, "ip_ranges must be an array of strings"),
        ("ip_ranges", "192.0.2.0/24", "ip_ranges must be an array of strings"),
        ("ip_ranges", ["192.0.2.0/24", 5], "ip_ranges must be an array of strings"),
        ("ipv4", None, "ipv4 must be an object"),
        ("ipv4", "10.0.0.2", "ipv4 must be an object"),
        ("primary", None, "primary must be a boolean"),
        ("primary", "yes", "primary must be a boolean"),
        ("primary", 1, "primary must be a boolean"),
    ],
)
async def test_handle_linode_instance_config_interface_update_rejects_bad_body_fields(
    field: str,
    value: Any,
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    arguments: dict[str, Any] = {
        "linode_id": 123,
        "config_id": 456,
        "interface_id": 789,
        "confirm": True,
        field: value,
    }

    result = await handle_linode_instance_config_interface_update(
        arguments,
        sample_config,
    )

    assert result[0].text == f"Error: {message}"
    mock_linode_client.update_instance_config_interface.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {
            "linode_id": "1/2",
            "config_id": 456,
            "interface_id": 789,
            "confirm": True,
            "primary": True,
        },
        {
            "linode_id": "1?x=2",
            "config_id": 456,
            "interface_id": 789,
            "confirm": True,
            "primary": True,
        },
        {
            "linode_id": "..",
            "config_id": 456,
            "interface_id": 789,
            "confirm": True,
            "primary": True,
        },
        {
            "linode_id": 0,
            "config_id": 456,
            "interface_id": 789,
            "confirm": True,
            "primary": True,
        },
        {
            "linode_id": 123,
            "config_id": "4/5",
            "interface_id": 789,
            "confirm": True,
            "primary": True,
        },
        {
            "linode_id": 123,
            "config_id": "4?x=5",
            "interface_id": 789,
            "confirm": True,
            "primary": True,
        },
        {
            "linode_id": 123,
            "config_id": "..",
            "interface_id": 789,
            "confirm": True,
            "primary": True,
        },
        {
            "linode_id": 123,
            "config_id": 0,
            "interface_id": 789,
            "confirm": True,
            "primary": True,
        },
        {
            "linode_id": 123,
            "config_id": 456,
            "interface_id": "7/8",
            "confirm": True,
            "primary": True,
        },
        {
            "linode_id": 123,
            "config_id": 456,
            "interface_id": "7?x=8",
            "confirm": True,
            "primary": True,
        },
        {
            "linode_id": 123,
            "config_id": 456,
            "interface_id": "..",
            "confirm": True,
            "primary": True,
        },
        {
            "linode_id": 123,
            "config_id": 456,
            "interface_id": 0,
            "confirm": True,
            "primary": True,
        },
    ],
)
async def test_handle_update_config_interface_rejects_bad_path_params(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_interface_update(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.update_instance_config_interface.assert_not_called()


def test_linode_instance_config_interface_update_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_config_interface_update"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_instance_config_interface_update"
    assert entry.handle_fn is handle_linode_instance_config_interface_update
    assert "linode_instance_config_interface_update" in FEATURE_TOOLS_LIST
