"""Tests for listing NodeBalancers assigned to a Linode instance."""

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
    create_linode_instance_nodebalancer_list_tool,
    handle_linode_instance_nodebalancer_list,
)
from linodemcp.version import FEATURE_TOOLS_LIST

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable


T = TypeVar("T")


@pytest.mark.asyncio
async def test_client_list_instance_nodebalancers_sends_exact_request() -> None:
    """Low-level client sends GET for the documented Linode NodeBalancers path."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"data": [{"id": 456}], "page": 1})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.list_instance_nodebalancers(123)
    finally:
        await client.close()

    assert result == {"data": [{"id": 456}], "page": 1}
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/linode/instances/123/nodebalancers"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert request.content == b""


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, True])
async def test_client_list_instance_nodebalancers_rejects_invalid_linode_id(
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
            await client.list_instance_nodebalancers(linode_id)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_list_instance_nodebalancers_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="ListInstanceNodeBalancers"):
            await client.list_instance_nodebalancers(123)
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_list_instance_nodebalancers_uses_retry() -> None:
    calls: list[int] = []

    class _RetryingClient(RetryableClient):
        async def _execute_with_retry(
            self, func: Callable[..., Awaitable[T]], *args: Any
        ) -> T:
            calls.append(1)
            return await func(*args)

    retryable = _RetryingClient("https://api.linode.com/v4", "test-token")
    list_mock = AsyncMock(return_value={"data": [{"id": 456}]})
    object.__setattr__(retryable.client, "list_instance_nodebalancers", list_mock)

    try:
        result = await retryable.list_instance_nodebalancers(123)
    finally:
        await retryable.close()

    assert result == {"data": [{"id": 456}]}
    assert calls == [1]
    list_mock.assert_awaited_once_with(123)


def test_create_linode_instance_nodebalancers_list_tool_schema() -> None:
    tool, capability = create_linode_instance_nodebalancer_list_tool()

    assert tool.name == "linode_instance_nodebalancer_list"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["linode_id"]
    linode_schema = tool.inputSchema["properties"]["linode_id"]
    assert linode_schema["type"] == "integer"
    assert linode_schema["minimum"] == 1


@pytest.mark.asyncio
async def test_handle_linode_instance_nodebalancers_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.list_instance_nodebalancers.return_value = {
        "data": [{"id": 456, "label": "nb-1"}],
        "page": 1,
    }

    result = await handle_linode_instance_nodebalancer_list(
        {"linode_id": 123}, sample_config
    )

    assert json.loads(result[0].text) == {
        "data": [{"id": 456, "label": "nb-1"}],
        "page": 1,
    }
    mock_linode_client.list_instance_nodebalancers.assert_awaited_once_with(123)


@pytest.mark.asyncio
@pytest.mark.parametrize("linode_id", ["1/2", "1?x=2", "..", 0, True])
async def test_handle_linode_instance_nodebalancers_list_rejects_invalid_linode_id(
    linode_id: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_instance_nodebalancer_list(
        {"linode_id": linode_id}, sample_config
    )

    assert result[0].text.startswith("Error: linode_id must be a positive integer")
    mock_linode_client.list_instance_nodebalancers.assert_not_called()


def test_linode_instance_nodebalancers_list_registered() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_instance_nodebalancer_list"]
    assert entry.capability is Capability.Read
    assert entry.handle_fn is handle_linode_instance_nodebalancer_list


def test_linode_instance_nodebalancers_list_in_version_features() -> None:
    assert "linode_instance_nodebalancer_list" in FEATURE_TOOLS_LIST.split(",")
