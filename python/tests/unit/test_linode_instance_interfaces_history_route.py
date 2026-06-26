"""Tests for Linode instance network interface history route."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeVar, cast
from unittest.mock import AsyncMock

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry
from linodemcp.tools import (
    create_linode_instance_interface_history_list_tool as exported_create_tool,
)
from linodemcp.tools.linode_instances import (
    create_linode_instance_interface_history_list_tool,
    handle_linode_instance_interface_history_list,
)
from linodemcp.version import FEATURE_TOOLS_LIST

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable


T = TypeVar("T")


class _CapturingRetryableClient(RetryableClient):
    """RetryableClient test double that records retry callbacks."""

    def __init__(self) -> None:
        super().__init__("https://api.linode.com/v4", "test-token")
        self.calls: list[Callable[..., Awaitable[Any]]] = []

    async def _execute_with_retry(
        self, func: Callable[..., Awaitable[T]], *args: Any
    ) -> T:
        self.calls.append(func)
        return await func(*args)


@pytest.mark.asyncio
async def test_client_list_instance_interface_history_sends_exact_request() -> None:
    """Low-level client sends GET /linode/instances/{linodeId}/interfaces/history."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "data": [{"linode_id": 123, "action": "interface_create"}],
                "page": 1,
                "pages": 1,
                "results": 1,
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.list_instance_interface_history(123)
    finally:
        await client.close()

    assert result["results"] == 1
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/linode/instances/123/interfaces/history"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert request.content == b""


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, True])
async def test_client_list_instance_interface_history_rejects_invalid_linode_id(
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
            await client.list_instance_interface_history(linode_id)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_list_instance_interface_history_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="ListInstanceInterfaceHistory"):
            await client.list_instance_interface_history(123)
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_client_list_instance_interface_history_uses_read_retry() -> (
    None
):
    """Read-only interface history list goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    mock_list = AsyncMock(
        return_value={"data": [], "page": 1, "pages": 1, "results": 0}
    )
    cast("Any", retryable.client).list_instance_interface_history = mock_list

    try:
        result = await retryable.list_instance_interface_history(123)
    finally:
        await retryable.close()

    assert result["results"] == 0
    assert len(retryable.calls) == 1
    mock_list.assert_awaited_once_with(123, page=None, page_size=None)


def test_create_linode_instance_interfaces_history_list_tool_schema() -> None:
    tool, capability = create_linode_instance_interface_history_list_tool()

    assert tool.name == "linode_instance_interface_history_list"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["linode_id"]
    assert tool.inputSchema["properties"]["linode_id"]["minimum"] == 1


@pytest.mark.asyncio
async def test_handle_linode_instance_interfaces_history_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.list_instance_interface_history.return_value = {
        "data": [{"linode_id": 123, "action": "interface_create"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    result = await handle_linode_instance_interface_history_list(
        {"linode_id": 123}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload["results"] == 1
    assert payload["data"] == [{"linode_id": 123, "action": "interface_create"}]
    mock_linode_client.list_instance_interface_history.assert_awaited_once_with(
        123, page=None, page_size=None
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"linode_id": "1/2"},
        {"linode_id": "1?x=2"},
        {"linode_id": ".."},
        {"linode_id": 0},
        {"linode_id": True},
    ],
)
async def test_handle_linode_instance_interfaces_history_list_rejects_invalid_linode_id(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_interface_history_list(
        arguments, sample_config
    )

    assert result[0].text == "Error: linode_id must be a positive integer"
    mock_linode_client.list_instance_interface_history.assert_not_called()


def test_linode_instance_interfaces_history_list_registered_and_exported() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_interface_history_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_instance_interface_history_list"
    assert entry.handle_fn is handle_linode_instance_interface_history_list
    assert exported_create_tool is create_linode_instance_interface_history_list_tool
    assert "linode_instance_interface_history_list" in FEATURE_TOOLS_LIST
