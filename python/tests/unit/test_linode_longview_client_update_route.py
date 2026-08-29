"""Tests for the Longview client update route."""

from __future__ import annotations

from typing import Any
from unittest.mock import AsyncMock

import httpx
import pytest

from linodemcp.gentools import (
    categories_for,
    create_linode_longview_client_get_tool,
    create_linode_longview_client_update_tool,
    handle_linode_longview_client_get,
    handle_linode_longview_client_update,
    scopes_for,
)
from linodemcp.linode import Client, NetworkError, RetryableClient
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry
from linodemcp.version import FEATURE_TOOLS_LIST


def _longview_client_json() -> dict[str, Any]:
    return {
        "id": 123,
        "label": "updated-client",
        "api_key": "test-api-key",
        "apps": {"apache": True, "mysql": True, "nginx": False},
        "created": "2026-01-01T00:00:00",
        "install_code": "install me",
        "updated": "2026-01-02T00:00:00",
    }


@pytest.mark.asyncio
async def test_client_get_longview_client_sends_exact_path_without_query_or_body() -> (
    None
):
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json=_longview_client_json())

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.get_longview_client(123)
    finally:
        await client.close()

    assert result["label"] == "updated-client"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "GET"
    assert request.url.path == "/v4/longview/clients/123"
    assert request.url.query == b""
    assert request.content == b""
    assert request.headers["Authorization"] == "Bearer test-token"


@pytest.mark.asyncio
@pytest.mark.parametrize("client_id", ["1/2", "1?x=2", "..", 0, -1, True])
async def test_client_get_longview_client_rejects_invalid_client_id(
    client_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(ValueError, match="client_id must be a positive integer"):
            await client.get_longview_client(client_id)
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_get_longview_client_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="GetLongviewClient"):
            await client.get_longview_client(123)
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_get_longview_client_retries_read() -> None:
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    mock_get = AsyncMock(return_value={"id": 123})
    object.__setattr__(retryable.client, "get_longview_client", mock_get)

    try:
        result = await retryable.get_longview_client(123)
    finally:
        await retryable.close()

    assert result == {"id": 123}
    mock_get.assert_awaited_once_with(123)


def test_create_linode_longview_client_update_tool_schema() -> None:
    tool, capability = create_linode_longview_client_update_tool()

    assert tool.name == "linode_longview_client_update"
    assert capability is Capability.Write
    assert tool.input_schema["required"] == ["client_id", "label", "confirm"]
    properties = tool.input_schema["properties"]
    assert properties["client_id"]["type"] == "integer"
    assert properties["label"]["type"] == "string"
    assert properties["confirm"]["type"] == "boolean"
    assert properties["dry_run"]["type"] == "boolean"
    for field in ("api_key", "apps", "created", "id", "install_code", "updated"):
        assert field not in properties


def test_linode_longview_client_update_registered() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_longview_client_update"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_longview_client_update"
    assert entry.handle_fn is handle_linode_longview_client_update


def test_linode_longview_client_update_in_version_features() -> None:
    assert "linode_longview_client_update" in FEATURE_TOOLS_LIST.split(",")


def test_linode_longview_client_update_profile_metadata() -> None:
    assert categories_for("linode_longview_client_update") == ["longview"]
    assert scopes_for("linode_longview_client_update") == ["longview:read_write"]


def test_create_linode_longview_client_get_tool_schema() -> None:
    tool, capability = create_linode_longview_client_get_tool()

    assert tool.name == "linode_longview_client_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["client_id"]
    assert "client_id" in tool.input_schema["properties"]


def test_linode_longview_client_get_registered() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_longview_client_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_longview_client_get"
    assert entry.handle_fn is handle_linode_longview_client_get


def test_linode_longview_client_get_in_version_features() -> None:
    assert "linode_longview_client_get" in FEATURE_TOOLS_LIST.split(",")


def test_linode_longview_client_get_profile_metadata() -> None:
    assert categories_for("linode_longview_client_get") == ["longview"]
    assert scopes_for("linode_longview_client_get") == ["longview:read_only"]
