"""Tests for the image share groups route."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeVar, cast
from unittest.mock import AsyncMock

import httpx
import pytest

from linodemcp.linode import Client, RetryableClient
from linodemcp.profiles import Capability, Scope, required_scopes
from linodemcp.server import get_tool_registry
from linodemcp.tools.linode_images import (
    create_linode_images_sharegroups_list_tool,
    create_linode_images_sharegroups_tokens_list_tool,
    handle_linode_images_sharegroups_list,
    handle_linode_images_sharegroups_tokens_list,
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
async def test_client_list_image_sharegroups_sends_exact_path_and_query() -> None:
    """Low-level client sends GET /images/sharegroups with pagination."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "data": [{"id": "share-1", "label": "shared images"}],
                "page": 2,
                "pages": 3,
                "results": 7,
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.list_image_sharegroups(page=2, page_size=50)
    finally:
        await client.close()

    assert result["data"][0]["id"] == "share-1"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/images/sharegroups"
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
async def test_client_list_image_sharegroups_validates_pagination_before_request(
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
            await client.list_image_sharegroups(**kwargs)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_retryable_client_list_image_sharegroups_uses_read_retry() -> None:
    """Read-only image share groups list goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    mock_list = AsyncMock(
        return_value={"data": [], "page": 1, "pages": 1, "results": 0}
    )
    cast("Any", retryable.client).list_image_sharegroups = mock_list

    try:
        result = await retryable.list_image_sharegroups(page=1, page_size=25)
    finally:
        await retryable.close()

    assert result["results"] == 0
    assert len(retryable.calls) == 1
    mock_list.assert_awaited_once_with(page=1, page_size=25)


def test_create_linode_images_sharegroups_list_tool_schema() -> None:
    """Tool schema exposes the documented pagination params."""
    tool, capability = create_linode_images_sharegroups_list_tool()

    assert tool.name == "linode_images_sharegroups_list"
    assert capability is Capability.Read
    assert tool.inputSchema["properties"]["page"]["minimum"] == 1
    assert tool.inputSchema["properties"]["page_size"]["minimum"] == 25
    assert tool.inputSchema["properties"]["page_size"]["maximum"] == 500


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns image share groups and pagination metadata."""
    mock_linode_client.list_image_sharegroups.return_value = {
        "data": [{"id": "share-1", "label": "shared images"}],
        "page": 2,
        "pages": 3,
        "results": 7,
    }

    result = await handle_linode_images_sharegroups_list(
        {"page": 2, "page_size": 50}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share groups listed",
        "count": 1,
        "sharegroups": [{"id": "share-1", "label": "shared images"}],
        "page": 2,
        "pages": 3,
        "results": 7,
    }
    mock_linode_client.list_image_sharegroups.assert_awaited_once_with(
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
async def test_handle_linode_images_sharegroups_list_rejects_invalid_pagination(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects invalid pagination before creating a client call."""
    result = await handle_linode_images_sharegroups_list(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.list_image_sharegroups.assert_not_called()


def test_linode_images_sharegroups_list_registered() -> None:
    """Dynamic registry exports the new tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroups_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_images_sharegroups_list"
    assert entry.handle_fn is handle_linode_images_sharegroups_list


def test_linode_images_sharegroups_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = required_scopes("linode_images_sharegroups_list", Capability.Read)

    assert scopes == [Scope.ImagesReadOnly]


@pytest.mark.asyncio
async def test_client_list_image_sharegroup_tokens_sends_exact_path() -> None:
    """Low-level client sends GET /images/sharegroups/tokens with no query/body."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(
            200,
            json={
                "data": [
                    {
                        "id": "sharegroup-record-1",
                        "created": "2026-01-01T00:00:00",
                    }
                ],
                "page": 1,
                "pages": 1,
                "results": 1,
            },
        )

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.list_image_sharegroup_tokens()
    finally:
        await client.close()

    assert result["data"][0]["id"] == "sharegroup-record-1"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/images/sharegroups/tokens"
    assert request.url.query == b""
    assert await request.aread() == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
async def test_retryable_client_list_image_sharegroup_tokens_uses_read_retry() -> None:
    """Read-only image share group tokens list goes through the retry wrapper."""
    retryable = _CapturingRetryableClient()
    mock_list = AsyncMock(
        return_value={"data": [], "page": 1, "pages": 1, "results": 0}
    )
    cast("Any", retryable.client).list_image_sharegroup_tokens = mock_list

    try:
        result = await retryable.list_image_sharegroup_tokens()
    finally:
        await retryable.close()

    assert result["results"] == 0
    assert len(retryable.calls) == 1
    mock_list.assert_awaited_once_with()


def test_create_linode_images_sharegroups_tokens_list_tool_schema() -> None:
    """Tool schema exposes only the documented environment argument."""
    tool, capability = create_linode_images_sharegroups_tokens_list_tool()

    assert tool.name == "linode_images_sharegroups_tokens_list"
    assert capability is Capability.Read
    assert set(tool.inputSchema["properties"]) == {"environment"}
    assert "required" not in tool.inputSchema


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_tokens_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns image share group tokens and pagination metadata."""
    mock_linode_client.list_image_sharegroup_tokens.return_value = {
        "data": [
            {
                "id": "sharegroup-record-1",
                "created": "2026-01-01T00:00:00",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    result = await handle_linode_images_sharegroups_tokens_list({}, sample_config)

    payload = json.loads(result[0].text)
    assert payload == {
        "message": "Image share group tokens listed",
        "count": 1,
        "tokens": [
            {
                "id": "sharegroup-record-1",
                "created": "2026-01-01T00:00:00",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    mock_linode_client.list_image_sharegroup_tokens.assert_awaited_once_with()


def test_linode_images_sharegroups_tokens_list_registered() -> None:
    """Dynamic registry exports the token list tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_images_sharegroups_tokens_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_images_sharegroups_tokens_list"
    assert entry.handle_fn is handle_linode_images_sharegroups_tokens_list


def test_linode_images_sharegroups_tokens_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = required_scopes("linode_images_sharegroups_tokens_list", Capability.Read)

    assert scopes == [Scope.ImagesReadOnly]


def test_linode_images_sharegroups_tokens_list_in_version_features() -> None:
    """Version metadata advertises the token list tool."""
    assert "linode_images_sharegroups_tokens_list" in FEATURE_TOOLS_LIST.split(",")
