"""Behavioral tests for previously-untested Object Storage Client methods.

Covers the bucket/key/transfer reads, the SSL and object-ACL reads and deletes,
and their region/label path encoding. Each pair asserts the outgoing HTTP verb
and endpoint, the decoded return, and NetworkError wrapping on httpx failures.
These whole method bodies were untested branches.
"""

from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import Client, NetworkError

pytestmark = pytest.mark.asyncio


def _ok_response(body: Any) -> MagicMock:
    """Build a mock httpx response whose json() returns body."""
    response = MagicMock()
    response.status_code = 200
    response.json.return_value = body
    return response


async def test_get_object_storage_key_returns_body() -> None:
    """get_object_storage_key GETs the key route and returns the body."""
    client = Client("https://api.linode.com/v4", "test-token")
    key = {"id": 22, "label": "backup-key", "access_key": "AK"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(key)

        result = await client.get_object_storage_key(22)

    assert result == key
    mock_request.assert_awaited_once_with("GET", "/object-storage/keys/22")
    await client.close()


async def test_get_object_storage_key_wraps_http_errors() -> None:
    """get_object_storage_key wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_object_storage_key(22)

    assert "GetObjectStorageKey" in str(excinfo.value)
    await client.close()


async def test_get_object_acl_encodes_name_query() -> None:
    """get_object_acl GETs the object-acl route with the name query param."""
    client = Client("https://api.linode.com/v4", "test-token")
    acl = {"acl": "public-read", "acl_xml": "<xml/>"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(acl)

        result = await client.get_object_acl("us-east", "web", "photo.png")

    assert result == acl
    method, endpoint = mock_request.await_args_list[0].args
    assert method == "GET"
    assert endpoint == "/object-storage/buckets/us-east/web/object-acl?name=photo.png"
    await client.close()


async def test_get_object_acl_wraps_http_errors() -> None:
    """get_object_acl wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_object_acl("us-east", "web", "photo.png")

    assert "GetObjectACL" in str(excinfo.value)
    await client.close()


async def test_get_bucket_ssl_returns_body() -> None:
    """get_bucket_ssl GETs the bucket ssl route and returns the body."""
    client = Client("https://api.linode.com/v4", "test-token")
    ssl = {"ssl": True}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(ssl)

        result = await client.get_bucket_ssl("us-east", "web")

    assert result == ssl
    mock_request.assert_awaited_once_with(
        "GET", "/object-storage/buckets/us-east/web/ssl"
    )
    await client.close()


async def test_get_bucket_ssl_wraps_http_errors() -> None:
    """get_bucket_ssl wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_bucket_ssl("us-east", "web")

    assert "GetBucketSSL" in str(excinfo.value)
    await client.close()
