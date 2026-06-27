"""Tests for the Managed Databases types route."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeVar, cast
from unittest.mock import AsyncMock

import httpx
import pytest

from linodemcp.linode import Client, RetryableClient
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry
from linodemcp.tools.linode_databases import (
    create_linode_database_type_get_tool,
    create_linode_database_type_list_tool,
    handle_linode_database_type_get,
    handle_linode_database_type_list,
)

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
async def test_client_list_database_types_sends_exact_path_and_query() -> None:
    """Low-level client sends GET /databases/types with documented pagination."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "data": [{"id": "g6-dedicated-2", "label": "Dedicated 4GB"}],
                "page": 2,
                "pages": 3,
                "results": 7,
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.list_database_types(page=2, page_size=50)
    finally:
        await client.close()

    assert result["data"][0]["id"] == "g6-dedicated-2"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/databases/types"
    assert request.url.query == b"page=2&page_size=50"
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("kwargs", "message"),
    [
        ({"page": 0}, "page must be an integer at least 1"),
        ({"page": "2"}, "page must be an integer at least 1"),
        ({"page": True}, "page must be an integer at least 1"),
        ({"page_size": 24}, "page_size must be an integer between 25 and 500"),
        ({"page_size": 501}, "page_size must be an integer between 25 and 500"),
        ({"page_size": "50"}, "page_size must be an integer between 25 and 500"),
        ({"page_size": False}, "page_size must be an integer between 25 and 500"),
    ],
)
async def test_client_list_database_types_validates_pagination_before_request(
    kwargs: dict[str, Any], message: str
) -> None:
    """Invalid pagination is rejected locally before an HTTP request."""
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={"data": []})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match=message):
            await client.list_database_types(**kwargs)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_get_database_type_sends_exact_path_and_query() -> None:
    """Low-level client sends GET /databases/types/{typeId}."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={"id": "g6-dedicated-2", "label": "Dedicated 4GB"},
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.get_database_type("g6-dedicated-2", page=2, page_size=50)
    finally:
        await client.close()

    assert result["id"] == "g6-dedicated-2"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/databases/types/g6-dedicated-2"
    assert request.url.query == b"page=2&page_size=50"
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("kwargs", "message", "exc_type"),
    [
        ({"type_id": ""}, "type_id is required", ValueError),
        ({"type_id": " g6-dedicated-2"}, "type_id is required", ValueError),
        ({"type_id": 123}, "type_id must be a string", TypeError),
        (
            {"type_id": "g6/dedicated-2"},
            "type_id must use letters, numbers, dots, underscores, and hyphens",
            ValueError,
        ),
        (
            {"type_id": "g6?dedicated-2"},
            "type_id must use letters, numbers, dots, underscores, and hyphens",
            ValueError,
        ),
        (
            {"type_id": "g6#dedicated-2"},
            "type_id must use letters, numbers, dots, underscores, and hyphens",
            ValueError,
        ),
        (
            {"type_id": ".."},
            "type_id must use letters, numbers, dots, underscores, and hyphens",
            ValueError,
        ),
        (
            {"type_id": "g6-dedicated-2", "page": 0},
            "page must be an integer at least 1",
            ValueError,
        ),
        (
            {"type_id": "g6-dedicated-2", "page_size": 501},
            "page_size must be an integer between 25 and 500",
            ValueError,
        ),
    ],
)
async def test_client_get_database_type_validates_inputs_before_request(
    kwargs: dict[str, Any], message: str, exc_type: type[Exception]
) -> None:
    """Invalid type and pagination inputs are rejected before an HTTP request."""
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={"id": "g6-dedicated-2"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(exc_type, match=message):
            await client.get_database_type(**kwargs)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_retryable_client_get_database_type_uses_read_retry() -> None:
    """Read-only database type get goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    mock_get = AsyncMock(return_value={"id": "g6-dedicated-2"})
    cast("Any", retryable.client).get_database_type = mock_get

    try:
        result = await retryable.get_database_type(
            "g6-dedicated-2", page=1, page_size=25
        )
    finally:
        await retryable.close()

    assert result["id"] == "g6-dedicated-2"
    assert len(retryable.calls) == 1
    mock_get.assert_awaited_once_with("g6-dedicated-2", page=1, page_size=25)


@pytest.mark.asyncio
async def test_retryable_client_list_database_types_uses_read_retry() -> None:
    """Read-only database types list goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    mock_list = AsyncMock(
        return_value={"data": [], "page": 1, "pages": 1, "results": 0}
    )
    cast("Any", retryable.client).list_database_types = mock_list

    try:
        result = await retryable.list_database_types(page=1, page_size=25)
    finally:
        await retryable.close()

    assert result["results"] == 0
    assert len(retryable.calls) == 1
    mock_list.assert_awaited_once_with(page=1, page_size=25)


def test_create_linode_database_type_get_tool_schema() -> None:
    """Tool schema exposes the documented type ID and pagination params."""
    tool, capability = create_linode_database_type_get_tool()

    assert tool.name == "linode_database_type_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["type_id"]
    assert "type_id" in tool.inputSchema["properties"]
    assert "page" in tool.inputSchema["properties"]
    assert "page_size" in tool.inputSchema["properties"]


def test_create_linode_databases_types_list_tool_schema() -> None:
    """Tool schema exposes the documented pagination params."""
    tool, capability = create_linode_database_type_list_tool()

    assert tool.name == "linode_database_type_list"
    assert capability is Capability.Read
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500


@pytest.mark.asyncio
async def test_handle_linode_database_type_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns database type details."""
    mock_linode_client.get_database_type.return_value = {
        "id": "g6-dedicated-2",
        "label": "Dedicated 4GB",
    }

    result = await handle_linode_database_type_get(
        {"type_id": "g6-dedicated-2", "page": 2, "page_size": 50}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload["id"] == "g6-dedicated-2"
    assert payload["label"] == "Dedicated 4GB"
    assert payload["engines"] == {"mysql": [], "postgresql": []}
    assert payload["deprecated"] is False
    mock_linode_client.get_database_type.assert_awaited_once_with(
        "g6-dedicated-2", page=2, page_size=50
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"type_id": ""},
        {"type_id": " g6-dedicated-2"},
        {"type_id": 123},
        {"type_id": "g6/dedicated-2"},
        {"type_id": "g6?dedicated-2"},
        {"type_id": "g6#dedicated-2"},
        {"type_id": ".."},
        {"type_id": "g6-dedicated-2", "page": 0},
        {"type_id": "g6-dedicated-2", "page_size": 501},
    ],
)
async def test_handle_linode_database_type_get_rejects_invalid_inputs(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects invalid path/pagination inputs before a client call."""
    result = await handle_linode_database_type_get(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.get_database_type.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_databases_types_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns database types and pagination metadata."""
    mock_linode_client.list_database_types.return_value = {
        "data": [{"id": "g6-dedicated-2", "label": "Dedicated 4GB"}],
        "page": 2,
        "pages": 3,
        "results": 7,
    }

    result = await handle_linode_database_type_list(
        {"page": 2, "page_size": 50}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Database types listed",
        "count": 1,
        "types": [{"id": "g6-dedicated-2", "label": "Dedicated 4GB"}],
        "page": 2,
        "pages": 3,
        "results": 7,
    }
    mock_linode_client.list_database_types.assert_awaited_once_with(
        page=2, page_size=50
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"page": 0},
        {"page": "2"},
        {"page": True},
        {"page_size": 24},
        {"page_size": 501},
        {"page_size": "50"},
        {"page_size": False},
    ],
)
async def test_handle_linode_databases_types_list_rejects_invalid_pagination(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects invalid pagination before creating a client call."""
    result = await handle_linode_database_type_list(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.list_database_types.assert_not_called()


def test_linode_databases_types_list_registered() -> None:
    """Dynamic registry exports the new tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    type_entry = entries["linode_database_type_get"]
    assert type_entry.capability is Capability.Read
    assert type_entry.tool.name == "linode_database_type_get"
    assert type_entry.handle_fn is handle_linode_database_type_get

    entry = entries["linode_database_type_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_database_type_list"
    assert entry.handle_fn is handle_linode_database_type_list
