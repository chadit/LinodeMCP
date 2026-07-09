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


# --- Bucket + key + transfer reads -----------------------------------------


async def test_list_object_storage_buckets_unwraps_data() -> None:
    """list_object_storage_buckets GETs /object-storage/buckets, unwraps data."""
    client = Client("https://api.linode.com/v4", "test-token")
    buckets = [{"label": "web", "region": "us-east"}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": buckets})

        result = await client.list_object_storage_buckets()

    assert result == buckets
    mock_request.assert_awaited_once_with("GET", "/object-storage/buckets")
    await client.close()


async def test_list_object_storage_buckets_wraps_http_errors() -> None:
    """list_object_storage_buckets wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_object_storage_buckets()

    assert "ListObjectStorageBuckets" in str(excinfo.value)
    await client.close()


async def test_list_object_storage_types_unwraps_data() -> None:
    """list_object_storage_types GETs /object-storage/types, unwraps data."""
    client = Client("https://api.linode.com/v4", "test-token")
    types = [{"id": "objectstorage", "price": {"monthly": 5}}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": types})

        result = await client.list_object_storage_types()

    assert result == types
    mock_request.assert_awaited_once_with("GET", "/object-storage/types")
    await client.close()


async def test_list_object_storage_types_wraps_http_errors() -> None:
    """list_object_storage_types wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_object_storage_types()

    assert "ListObjectStorageTypes" in str(excinfo.value)
    await client.close()


async def test_list_object_storage_keys_unwraps_data() -> None:
    """list_object_storage_keys GETs /object-storage/keys, unwraps data."""
    client = Client("https://api.linode.com/v4", "test-token")
    keys = [{"id": 22, "label": "backup-key"}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": keys})

        result = await client.list_object_storage_keys()

    assert result == keys
    mock_request.assert_awaited_once_with("GET", "/object-storage/keys")
    await client.close()


async def test_list_object_storage_keys_wraps_http_errors() -> None:
    """list_object_storage_keys wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_object_storage_keys()

    assert "ListObjectStorageKeys" in str(excinfo.value)
    await client.close()


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


async def test_get_object_storage_transfer_returns_body() -> None:
    """get_object_storage_transfer GETs the transfer route and returns body."""
    client = Client("https://api.linode.com/v4", "test-token")
    transfer = {"used": 12345, "quota": 1000000}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(transfer)

        result = await client.get_object_storage_transfer()

    assert result == transfer
    mock_request.assert_awaited_once_with("GET", "/object-storage/transfer")
    await client.close()


async def test_get_object_storage_transfer_wraps_http_errors() -> None:
    """get_object_storage_transfer wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_object_storage_transfer()

    assert "GetObjectStorageTransfer" in str(excinfo.value)
    await client.close()


# --- Bucket + key deletes --------------------------------------------------


async def test_delete_object_storage_bucket_sends_delete() -> None:
    """delete_object_storage_bucket DELETEs the region/label bucket route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_object_storage_bucket("us-east", "web")

    mock_request.assert_awaited_once_with(
        "DELETE", "/object-storage/buckets/us-east/web"
    )
    await client.close()


async def test_delete_object_storage_bucket_wraps_http_errors() -> None:
    """delete_object_storage_bucket wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_object_storage_bucket("us-east", "web")

    assert "DeleteObjectStorageBucket" in str(excinfo.value)
    await client.close()


async def test_delete_object_storage_key_sends_delete() -> None:
    """delete_object_storage_key DELETEs the key route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_object_storage_key(22)

    mock_request.assert_awaited_once_with("DELETE", "/object-storage/keys/22")
    await client.close()


async def test_delete_object_storage_key_wraps_http_errors() -> None:
    """delete_object_storage_key wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_object_storage_key(22)

    assert "DeleteObjectStorageKey" in str(excinfo.value)
    await client.close()


# --- Object ACL ------------------------------------------------------------


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


# --- Bucket SSL ------------------------------------------------------------


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


async def test_delete_bucket_ssl_sends_delete() -> None:
    """delete_bucket_ssl DELETEs the bucket ssl route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_bucket_ssl("us-east", "web")

    mock_request.assert_awaited_once_with(
        "DELETE", "/object-storage/buckets/us-east/web/ssl"
    )
    await client.close()


async def test_delete_bucket_ssl_wraps_http_errors() -> None:
    """delete_bucket_ssl wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_bucket_ssl("us-east", "web")

    assert "DeleteBucketSSL" in str(excinfo.value)
    await client.close()
