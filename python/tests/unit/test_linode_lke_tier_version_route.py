"""Tests for the LKE tier version get route."""

from __future__ import annotations

import json
from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient
from linodemcp.profiles import Capability
from linodemcp.server import Server, get_tool_registry
from linodemcp.tools.linode_lke import (
    create_linode_lke_tier_version_get_tool,
    handle_linode_lke_tier_version_get,
)
from linodemcp.version import get_version_info


async def test_client_get_lke_tier_version_sends_exact_route() -> None:
    """Low-level client sends the documented tier version route."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": "1.31", "tier": "standard"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_lke_tier_version("standard", "1.31")

    assert result == {"id": "1.31", "tier": "standard"}
    mock_request.assert_awaited_once_with("GET", "/lke/tiers/standard/versions/1.31")

    await client.close()


async def test_client_get_lke_tier_version_url_encodes_path_params() -> None:
    """Low-level client URL-encodes tier and version path params."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": "1/31?x=1"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        await client.get_lke_tier_version("standard/../x", "1/31?x=1")

    mock_request.assert_awaited_once_with(
        "GET", "/lke/tiers/standard%2F..%2Fx/versions/1%2F31%3Fx%3D1"
    )

    await client.close()


async def test_client_get_lke_tier_version_wraps_http_error() -> None:
    """Client wraps tier version HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError, match="GetLKETierVersion"):
            await client.get_lke_tier_version("standard", "1.31")

    await client.close()


async def test_retryable_get_lke_tier_version_delegates_with_retry() -> None:
    """Retryable client delegates read-only tier version get through retry."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    mock_get = AsyncMock(return_value={"id": "1.31"})
    object.__setattr__(client.client, "get_lke_tier_version", mock_get)

    try:
        result = await client.get_lke_tier_version("standard", "1.31")
    finally:
        await client.close()

    assert result == {"id": "1.31"}
    mock_get.assert_awaited_once_with("standard", "1.31")


def test_linode_lke_tier_version_get_tool_schema() -> None:
    """Tool schema exposes tier and version path params."""
    tool, capability = create_linode_lke_tier_version_get_tool()

    assert tool.name == "linode_lke_tier_version_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["tier", "version"]
    assert tool.inputSchema["properties"]["tier"]["type"] == "string"
    assert tool.inputSchema["properties"]["version"]["type"] == "string"


async def test_handle_linode_lke_tier_version_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns LKE tier version details."""
    mock_linode_client.get_lke_tier_version.return_value = {
        "id": "1.31",
        "tier": "standard",
    }

    result = await handle_linode_lke_tier_version_get(
        {"tier": "standard", "version": "1.31"}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {"id": "1.31", "tier": "standard"}
    mock_linode_client.get_lke_tier_version.assert_awaited_once_with("standard", "1.31")


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({}, "tier"),
        ({"tier": "", "version": "1.31"}, "tier"),
        ({"tier": "standard", "version": ""}, "version"),
        ({"tier": "standard/x", "version": "1.31"}, "tier"),
        ({"tier": "standard?x=1", "version": "1.31"}, "tier"),
        ({"tier": "..", "version": "1.31"}, "tier"),
        ({"tier": "standard", "version": "1/31"}, "version"),
        ({"tier": "standard", "version": "1.31?x=1"}, "version"),
        ({"tier": "standard", "version": ".."}, "version"),
        ({"tier": 123, "version": "1.31"}, "tier"),
        ({"tier": "standard", "version": 1.31}, "version"),
    ],
)
async def test_handle_linode_lke_tier_version_get_rejects_invalid_path_params(
    arguments: dict[str, Any],
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Handler rejects malformed path params before client calls."""
    result = await handle_linode_lke_tier_version_get(arguments, sample_config)

    assert message in result[0].text
    mock_linode_client.get_lke_tier_version.assert_not_called()


def test_linode_lke_tier_version_get_exported_registered_and_versioned(
    sample_config: Any,
) -> None:
    """Tier version get is exported, registered, and listed in features."""
    registry_names = {entry.name for entry in get_tool_registry()}
    assert "linode_lke_tier_version_get" in registry_names

    srv = Server(sample_config)
    assert "linode_lke_tier_version_get" in srv.registered_tool_names
    assert "linode_lke_tier_version_get" in get_version_info().features["tools"]
