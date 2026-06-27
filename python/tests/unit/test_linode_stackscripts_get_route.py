"""Tests for the StackScripts get route."""

from __future__ import annotations

import json
from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient, StackScript
from linodemcp.profiles import Capability
from linodemcp.tools.linode_stackscripts import (
    create_linode_stackscript_get_tool,
    handle_linode_stackscript_get,
)


def _stackscript_payload(stackscript_id: int = 123) -> dict[str, Any]:
    return {
        "id": stackscript_id,
        "username": "testuser",
        "user_gravatar_id": "abc123",
        "label": "my-script",
        "description": "Test script",
        "images": ["linode/ubuntu22.04"],
        "deployments_total": 10,
        "deployments_active": 5,
        "is_public": False,
        "mine": True,
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
        "script": "#!/bin/bash",
        "user_defined_fields": [],
    }


def _stackscript(stackscript_id: int = 123) -> StackScript:
    data = _stackscript_payload(stackscript_id)
    return StackScript(**data)


@pytest.mark.asyncio
async def test_client_get_stackscript_sends_exact_route() -> None:
    """Low-level client sends GET /linode/stackscripts/{stackscriptId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = _stackscript_payload(123)

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_stackscript(123)

    assert result.id == 123
    assert result.label == "my-script"
    mock_request.assert_called_once_with("GET", "/linode/stackscripts/123")

    await client.close()


@pytest.mark.asyncio
async def test_client_get_stackscript_url_encodes_stackscript_id() -> None:
    """Low-level client URL-encodes StackScript IDs at the path boundary."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = _stackscript_payload(123)

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        await client.get_stackscript("123/../456?x=1")

    mock_request.assert_called_once_with(
        "GET", "/linode/stackscripts/123%2F..%2F456%3Fx%3D1"
    )

    await client.close()


@pytest.mark.asyncio
async def test_client_get_stackscript_wraps_http_error() -> None:
    """StackScript get wraps client HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError, match="GetStackScript"):
            await client.get_stackscript(123)

    await client.close()


@pytest.mark.asyncio
async def test_retryable_get_stackscript_delegates_with_retry() -> None:
    """Retryable client delegates read-only StackScript get through retry."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    stackscript = _stackscript()

    with patch.object(
        client.client, "get_stackscript", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = stackscript

        result = await client.get_stackscript(123)

    assert result is stackscript
    mock_get.assert_awaited_once_with(123)

    await client.close()


def test_create_linode_stackscript_get_tool_schema() -> None:
    """Tool schema exposes the documented StackScript ID path parameter."""
    tool, capability = create_linode_stackscript_get_tool()

    assert tool.name == "linode_stackscript_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["stackscript_id"]
    assert "stackscript_id" in tool.inputSchema["properties"]


@pytest.mark.asyncio
async def test_handle_linode_stackscript_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns StackScript details."""
    mock_linode_client.get_stackscript.return_value = _stackscript(123)

    result = await handle_linode_stackscript_get({"stackscript_id": 123}, sample_config)

    payload = json.loads(result[0].text)
    assert payload == {
        "username": "testuser",
        "user_gravatar_id": "abc123",
        "label": "my-script",
        "description": "Test script",
        "images": ["linode/ubuntu22.04"],
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
        "script": "#!/bin/bash",
        "user_defined_fields": [],
        "id": 123,
        "deployments_total": 10,
        "deployments_active": 5,
        "is_public": False,
        "mine": True,
    }
    mock_linode_client.get_stackscript.assert_awaited_once_with(123)


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"stackscript_id": ""},
        {"stackscript_id": 0},
        {"stackscript_id": -1},
        {"stackscript_id": True},
        {"stackscript_id": 1.2},
        {"stackscript_id": "123/456"},
        {"stackscript_id": "123?foo=bar"},
        {"stackscript_id": ".."},
    ],
)
async def test_handle_linode_stackscript_get_rejects_invalid_ids(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects invalid StackScript IDs before a client call."""
    result = await handle_linode_stackscript_get(arguments, sample_config)

    assert result[0].text == "Error: stackscript_id must be a positive integer"
    mock_linode_client.get_stackscript.assert_not_called()
