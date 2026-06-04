"""Tests for updating Linode instance configuration profiles."""

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
    create_linode_instance_config_update_tool,
    handle_linode_instance_config_update,
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
async def test_client_update_instance_config_sends_exact_method_path_and_body() -> None:
    """Low-level client sends PUT /linode/instances/{linodeId}/configs/{configId}."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": 456, "label": "rescue"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.update_instance_config(
            123,
            456,
            {
                "comments": "boot rescue kernel",
                "devices": {"sda": {"disk_id": 789}},
                "helpers": {"network": True},
                "interfaces": [{"purpose": "public"}],
                "kernel": "linode/latest-64bit",
                "label": "rescue",
                "memory_limit": 1024,
                "root_device": "/dev/sda",
                "run_level": "default",
                "virt_mode": "paravirt",
            },
        )
    finally:
        await client.close()

    assert result["label"] == "rescue"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "PUT"
    assert request.url.path == "/v4/linode/instances/123/configs/456"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert json.loads(request.content) == {
        "comments": "boot rescue kernel",
        "devices": {"sda": {"disk_id": 789}},
        "helpers": {"network": True},
        "interfaces": [{"purpose": "public"}],
        "kernel": "linode/latest-64bit",
        "label": "rescue",
        "memory_limit": 1024,
        "root_device": "/dev/sda",
        "run_level": "default",
        "virt_mode": "paravirt",
    }


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "linode_id",
    ["1/2", "1?x=2", "..", 0, True],
)
async def test_client_update_instance_config_rejects_invalid_linode_id_before_request(
    linode_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="linode_id must be a positive integer"):
            await client.update_instance_config(linode_id, 456, {})
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "config_id",
    ["4/5", "4?x=5", "..", 0, False],
)
async def test_client_update_instance_config_rejects_invalid_config_id_before_request(
    config_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="config_id must be a positive integer"):
            await client.update_instance_config(123, config_id, {})
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_retryable_client_update_instance_config_no_replay() -> None:
    """Mutating update delegates once and does not use generic replay retry."""
    retryable = _FailingRetryableClient()
    transient = httpx.ReadTimeout("timeout")
    mock_update = AsyncMock(side_effect=NetworkError("UpdateInstanceConfig", transient))
    cast("Any", retryable.client).update_instance_config = mock_update

    try:
        with pytest.raises(NetworkError):
            await retryable.update_instance_config(123, 456, {"label": "rescue"})
    finally:
        await retryable.close()

    assert retryable.retry_calls == 0
    mock_update.assert_awaited_once_with(123, 456, {"label": "rescue"})


def test_create_linode_instance_config_update_tool_schema() -> None:
    tool, capability = create_linode_instance_config_update_tool()

    assert tool.name == "linode_instance_config_update"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["linode_id", "config_id", "confirm"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["config_id"]["minimum"] == 1
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert "dry_run" in tool.inputSchema["properties"]
    assert tool.inputSchema["properties"]["dry_run"]["type"] == "boolean"
    assert "dry_run" not in tool.inputSchema["required"]
    assert {"required": ["label"]} in tool.inputSchema["anyOf"]
    assert "id" not in tool.inputSchema["properties"]
    assert "devices" in tool.inputSchema["properties"]
    assert "virt_mode" in tool.inputSchema["properties"]


@pytest.mark.asyncio
async def test_handle_linode_instance_config_update_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.update_instance_config.return_value = {
        "id": 456,
        "label": "rescue",
    }

    result = await handle_linode_instance_config_update(
        {
            "linode_id": 123,
            "config_id": 456,
            "label": "rescue",
            "devices": {"sda": {"disk_id": 789}},
            "confirm": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {"id": 456, "label": "rescue"}
    mock_linode_client.update_instance_config.assert_awaited_once_with(
        123, 456, {"devices": {"sda": {"disk_id": 789}}, "label": "rescue"}
    )


@pytest.mark.asyncio
async def test_client_update_instance_config_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="UpdateInstanceConfig"):
            await client.update_instance_config(123, 456, {"label": "rescue"})
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_handle_linode_instance_config_update_dry_run_skips_client(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_update(
        {
            "linode_id": 123,
            "config_id": 456,
            "label": "updated-config",
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    body = json.loads(result[0].text)
    assert body["tool"] == "linode_instance_config_update"
    assert body["dry_run"] is True
    assert body["would_execute"]["method"] == "PUT"
    assert body["would_execute"]["path"] == "/linode/instances/123/configs/456"
    assert body["would_execute"]["body"] == {"label": "updated-config"}
    mock_linode_client.update_instance_config.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_instance_config_update_requires_boolean_confirm_true(
    confirm: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    arguments: dict[str, Any] = {"linode_id": 123, "config_id": 456}
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await handle_linode_instance_config_update(arguments, sample_config)

    assert result[0].text == "Error: confirm must be true"
    mock_linode_client.update_instance_config.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_instance_config_update_requires_update_field(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_update(
        {"linode_id": 123, "config_id": 456, "confirm": True}, sample_config
    )

    assert result[0].text == "Error: at least one update field is required"
    mock_linode_client.update_instance_config.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"linode_id": "1/2", "config_id": 456, "confirm": True},
        {"linode_id": "1?x=2", "config_id": 456, "confirm": True},
        {"linode_id": "..", "config_id": 456, "confirm": True},
        {"linode_id": 0, "config_id": 456, "confirm": True},
        {"linode_id": 123, "config_id": "4/5", "confirm": True},
        {"linode_id": 123, "config_id": "4?x=5", "confirm": True},
        {"linode_id": 123, "config_id": "..", "confirm": True},
        {"linode_id": 123, "config_id": 0, "confirm": True},
    ],
)
async def test_handle_linode_instance_config_update_rejects_invalid_path_params(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_config_update(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.update_instance_config.assert_not_called()


def test_linode_instance_config_update_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_config_update"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_instance_config_update"
    assert entry.handle_fn is handle_linode_instance_config_update
    assert "linode_instance_config_update" in FEATURE_TOOLS_LIST
