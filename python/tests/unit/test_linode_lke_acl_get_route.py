"""Tests for the LKE control plane ACL get route unwrapping."""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import Client, NetworkError


async def test_client_get_lke_control_plane_acl_unwraps_acl_envelope() -> None:
    """Client unwraps the API's {"acl": {...}} envelope to the bare ACL."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "acl": {
            "enabled": True,
            "addresses": {"ipv4": ["10.0.0.0/8"], "ipv6": ["::1/128"]},
        },
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_lke_control_plane_acl(123)

    assert result == {
        "enabled": True,
        "addresses": {"ipv4": ["10.0.0.0/8"], "ipv6": ["::1/128"]},
    }
    mock_request.assert_awaited_once_with("GET", "/lke/clusters/123/control_plane_acl")

    await client.close()


async def test_client_get_lke_control_plane_acl_normalizes_empty_addresses() -> None:
    """Missing or null address lists become empty arrays, matching Go."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"acl": {"enabled": False}}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_lke_control_plane_acl(123)

    assert result == {
        "enabled": False,
        "addresses": {"ipv4": [], "ipv6": []},
    }

    await client.close()


async def test_client_get_lke_control_plane_acl_wraps_http_error() -> None:
    """Client wraps ACL get HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError, match="GetLKEControlPlaneACL"):
            await client.get_lke_control_plane_acl(123)

    await client.close()
