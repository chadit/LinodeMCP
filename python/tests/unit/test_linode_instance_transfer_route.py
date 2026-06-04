"""Tests for getting monthly transfer stats for a Linode instance."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeVar
from unittest.mock import AsyncMock

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry
from linodemcp.tools.linode_instances import (
    create_linode_instance_transfer_month_get_tool,
    handle_linode_instance_transfer_month_get,
)
from linodemcp.version import FEATURE_TOOLS_LIST

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable


T = TypeVar("T")


@pytest.mark.asyncio
async def test_client_get_instance_transfer_sends_exact_request() -> None:
    """Low-level client sends GET for the documented transfer stats path."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"in": 1.25, "out": 2.5, "total": 3.75})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.get_instance_transfer_by_year_month(123, 2024, 5)
    finally:
        await client.close()

    assert result == {"in": 1.25, "out": 2.5, "total": 3.75}
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/linode/instances/123/transfer/2024/5"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert request.content == b""


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("linode_id", "year", "month", "message"),
    [
        ("1/2", 2024, 5, "linode_id must be a positive integer"),
        ("1?x=2", 2024, 5, "linode_id must be a positive integer"),
        ("..", 2024, 5, "linode_id must be a positive integer"),
        (0, 2024, 5, "linode_id must be a positive integer"),
        (True, 2024, 5, "linode_id must be a positive integer"),
        (123, "2024/2025", 5, "year must be an integer between 1970 and 9999"),
        (123, "2024?x=1", 5, "year must be an integer between 1970 and 9999"),
        (123, "..", 5, "year must be an integer between 1970 and 9999"),
        (123, 0, 5, "year must be an integer between 1970 and 9999"),
        (123, 1, 5, "year must be an integer between 1970 and 9999"),
        (123, 999, 5, "year must be an integer between 1970 and 9999"),
        (123, 10000, 5, "year must be an integer between 1970 and 9999"),
        (123, True, 5, "year must be an integer between 1970 and 9999"),
        (123, 2024, "5/6", "month must be an integer between 1 and 12"),
        (123, 2024, "5?x=6", "month must be an integer between 1 and 12"),
        (123, 2024, "..", "month must be an integer between 1 and 12"),
        (123, 2024, 0, "month must be an integer between 1 and 12"),
        (123, 2024, 13, "month must be an integer between 1 and 12"),
        (123, 2024, True, "month must be an integer between 1 and 12"),
    ],
)
async def test_client_get_instance_transfer_rejects_invalid_path_params(
    linode_id: Any, year: Any, month: Any, message: str
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
            await client.get_instance_transfer_by_year_month(linode_id, year, month)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_get_instance_transfer_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="GetInstanceTransferByYearMonth"):
            await client.get_instance_transfer_by_year_month(123, 2024, 5)
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_get_instance_transfer_uses_retry() -> None:
    calls: list[int] = []

    class _RetryingClient(RetryableClient):
        async def _execute_with_retry(
            self, func: Callable[..., Awaitable[T]], *args: Any
        ) -> T:
            calls.append(1)
            return await func(*args)

    retryable = _RetryingClient("https://api.linode.com/v4", "test-token")
    transfer_mock = AsyncMock(return_value={"total": 3.75})
    object.__setattr__(
        retryable.client, "get_instance_transfer_by_year_month", transfer_mock
    )

    try:
        result = await retryable.get_instance_transfer_by_year_month(123, 2024, 5)
    finally:
        await retryable.close()

    assert result == {"total": 3.75}
    assert calls == [1]
    transfer_mock.assert_awaited_once_with(123, 2024, 5)


def test_create_linode_instance_transfer_month_get_tool_schema() -> None:
    tool, capability = create_linode_instance_transfer_month_get_tool()

    assert tool.name == "linode_instance_transfer_month_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["linode_id", "year", "month"]
    linode_schema = tool.inputSchema["properties"]["linode_id"]
    assert linode_schema["type"] == "integer"
    assert linode_schema["minimum"] == 1
    year_schema = tool.inputSchema["properties"]["year"]
    assert year_schema["type"] == "integer"
    assert year_schema["minimum"] == 1970
    assert year_schema["maximum"] == 9999
    month_schema = tool.inputSchema["properties"]["month"]
    assert month_schema["type"] == "integer"
    assert month_schema["minimum"] == 1
    assert month_schema["maximum"] == 12


@pytest.mark.asyncio
async def test_handle_linode_instance_transfer_month_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.get_instance_transfer_by_year_month.return_value = {
        "in": 1.25,
        "out": 2.5,
        "total": 3.75,
    }

    result = await handle_linode_instance_transfer_month_get(
        {"linode_id": 123, "year": 2024, "month": 5}, sample_config
    )

    assert json.loads(result[0].text) == {"in": 1.25, "out": 2.5, "total": 3.75}
    mock_linode_client.get_instance_transfer_by_year_month.assert_awaited_once_with(
        123, 2024, 5
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"linode_id": "1/2", "year": 2024, "month": 5}, "linode_id"),
        ({"linode_id": "1?x=2", "year": 2024, "month": 5}, "linode_id"),
        ({"linode_id": "..", "year": 2024, "month": 5}, "linode_id"),
        ({"linode_id": 0, "year": 2024, "month": 5}, "linode_id"),
        ({"linode_id": True, "year": 2024, "month": 5}, "linode_id"),
        ({"linode_id": 123, "year": "2024/2025", "month": 5}, "year"),
        ({"linode_id": 123, "year": "2024?x=1", "month": 5}, "year"),
        ({"linode_id": 123, "year": "..", "month": 5}, "year"),
        ({"linode_id": 123, "year": 0, "month": 5}, "year"),
        ({"linode_id": 123, "year": 1, "month": 5}, "year"),
        ({"linode_id": 123, "year": 999, "month": 5}, "year"),
        ({"linode_id": 123, "year": 10000, "month": 5}, "year"),
        ({"linode_id": 123, "year": True, "month": 5}, "year"),
        ({"linode_id": 123, "year": 2024, "month": "5/6"}, "month"),
        ({"linode_id": 123, "year": 2024, "month": "5?x=6"}, "month"),
        ({"linode_id": 123, "year": 2024, "month": ".."}, "month"),
        ({"linode_id": 123, "year": 2024, "month": 0}, "month"),
        ({"linode_id": 123, "year": 2024, "month": 13}, "month"),
        ({"linode_id": 123, "year": 2024, "month": True}, "month"),
    ],
)
async def test_handle_linode_instance_transfer_month_get_rejects_invalid_path_params(
    arguments: dict[str, Any],
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    result = await handle_linode_instance_transfer_month_get(arguments, sample_config)

    assert result[0].text.startswith(f"Error: {message}")
    mock_linode_client.get_instance_transfer_by_year_month.assert_not_called()


def test_linode_instance_transfer_month_get_registered() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_transfer_month_get"]
    assert entry.capability is Capability.Read
    assert entry.handle_fn is handle_linode_instance_transfer_month_get


def test_linode_instance_transfer_month_get_in_version_features() -> None:
    assert "linode_instance_transfer_month_get" in FEATURE_TOOLS_LIST.split(",")
