"""Tests for the Longview client update route."""

from __future__ import annotations

import json
from typing import Any
from unittest.mock import AsyncMock

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient
from linodemcp.profiles import Capability, Scope
from linodemcp.profiles.builtin import categories
from linodemcp.profiles.scope import required_scopes
from linodemcp.server import get_tool_registry
from linodemcp.tools.linode_longview import (
    create_linode_longview_client_get_tool,
    create_linode_longview_client_update_tool,
    handle_linode_longview_client_get,
    handle_linode_longview_client_update,
)
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
async def test_client_update_longview_client_sends_exact_path_and_body() -> None:
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json=_longview_client_json())

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.update_longview_client(123, label="updated-client")
    finally:
        await client.close()

    assert result["label"] == "updated-client"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "PUT"
    assert request.url.path == "/v4/longview/clients/123"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert json.loads(request.content) == {"label": "updated-client"}


@pytest.mark.asyncio
@pytest.mark.parametrize("client_id", ["1/2", "1?x=2", "..", 0, -1, True])
async def test_client_update_longview_client_rejects_invalid_client_id(
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
            await client.update_longview_client(client_id, label="updated-client")
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_update_longview_client_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="UpdateLongviewClient"):
            await client.update_longview_client(123, label="updated-client")
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_update_longview_client_does_not_replay_put() -> None:
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    network_error = NetworkError("UpdateLongviewClient", httpx.ConnectTimeout("boom"))
    mock_update = AsyncMock(side_effect=network_error)
    object.__setattr__(retryable.client, "update_longview_client", mock_update)

    try:
        with pytest.raises(NetworkError):
            await retryable.update_longview_client(123, label="updated-client")
    finally:
        await retryable.close()

    mock_update.assert_awaited_once_with(123, label="updated-client")


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
    assert tool.inputSchema["required"] == ["client_id", "label", "confirm"]
    properties = tool.inputSchema["properties"]
    assert properties["client_id"]["type"] == "integer"
    assert properties["label"]["type"] == "string"
    assert properties["confirm"]["type"] == "boolean"
    assert properties["dry_run"]["type"] == "boolean"
    for field in ("api_key", "apps", "created", "id", "install_code", "updated"):
        assert field not in properties


@pytest.mark.asyncio
async def test_handle_linode_longview_client_update_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.update_longview_client.return_value = _longview_client_json()

    result = await handle_linode_longview_client_update(
        {"client_id": 123, "label": "updated-client", "confirm": True},
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["message"] == "Longview client updated successfully"
    assert payload["longview_client"]["label"] == "updated-client"
    # The metadata LongviewClient element carries no install secret, so the
    # api_key and install_code the API returns are dropped from the output.
    assert "api_key" not in payload["longview_client"]
    assert "install_code" not in payload["longview_client"]
    mock_linode_client.update_longview_client.assert_awaited_once_with(
        123, label="updated-client"
    )


@pytest.mark.asyncio
async def test_handle_linode_longview_client_update_dry_run_skips_client(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_longview_client_update(
        {"client_id": 123, "label": "updated-client", "confirm": True, "dry_run": True},
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "PUT",
        "path": "/longview/clients/123",
        "body": {"label": "updated-client"},
    }
    assert len(payload["side_effects"]) == 1
    mock_linode_client.update_longview_client.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_handle_linode_longview_client_update_rejects_non_true_confirm(
    confirm_value: Any,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    arguments: dict[str, Any] = {"client_id": 123, "label": "updated-client"}
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    result = await handle_linode_longview_client_update(arguments, sample_config)

    assert result[0].text.startswith("Error: This updates a Longview client")
    mock_linode_client.update_longview_client.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"label": "updated-client", "confirm": True},
        {"client_id": "1/2", "label": "updated-client", "confirm": True},
        {"client_id": "1?x=2", "label": "updated-client", "confirm": True},
        {"client_id": "..", "label": "updated-client", "confirm": True},
        {"client_id": 0, "label": "updated-client", "confirm": True},
        {"client_id": True, "label": "updated-client", "confirm": True},
        {"client_id": 123, "confirm": True},
        {"client_id": 123, "label": "", "confirm": True},
        {"client_id": 123, "label": {"bad": object()}, "confirm": True},
    ],
)
async def test_handle_linode_longview_client_update_rejects_invalid_arguments(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_longview_client_update(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.update_longview_client.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_longview_client_update_reports_client_errors(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.update_longview_client.side_effect = NetworkError(
        "UpdateLongviewClient", httpx.ConnectTimeout("boom")
    )

    result = await handle_linode_longview_client_update(
        {"client_id": 123, "label": "updated-client", "confirm": True},
        sample_config,
    )

    assert result[0].text.startswith("Failed to update Longview client: ")
    mock_linode_client.update_longview_client.assert_awaited_once()


def test_linode_longview_client_update_registered() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_longview_client_update"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_longview_client_update"
    assert entry.handle_fn is handle_linode_longview_client_update


def test_linode_longview_client_update_in_version_features() -> None:
    assert "linode_longview_client_update" in FEATURE_TOOLS_LIST.split(",")


def test_linode_longview_client_update_profile_metadata() -> None:
    assert categories("linode_longview_client_update") == ["longview"]
    assert required_scopes("linode_longview_client_update", Capability.Write) == [
        Scope.LongviewReadWrite
    ]


def test_create_linode_longview_client_get_tool_schema() -> None:
    tool, capability = create_linode_longview_client_get_tool()

    assert tool.name == "linode_longview_client_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["client_id"]
    assert "client_id" in tool.inputSchema["properties"]


def test_linode_longview_client_get_registered() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_longview_client_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_longview_client_get"
    assert entry.handle_fn is handle_linode_longview_client_get


def test_linode_longview_client_get_in_version_features() -> None:
    assert "linode_longview_client_get" in FEATURE_TOOLS_LIST.split(",")


def test_linode_longview_client_get_profile_metadata() -> None:
    assert categories("linode_longview_client_get") == ["longview"]
    assert required_scopes("linode_longview_client_get", Capability.Read) == [
        Scope.LongviewReadOnly
    ]
