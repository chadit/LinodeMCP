"""Low-level client coverage for reserved public IPv4 routes."""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient


def _response(payload: object) -> MagicMock:
    response = MagicMock()
    response.status_code = 200
    response.json.return_value = payload
    return response


async def test_reserved_ip_client_uses_exact_collection_routes() -> None:
    """Reserve, list, and pricing calls use exact methods, paths, query, and body."""
    client = Client("https://api.linode.com/v4", "test-token")
    reserved = {"address": "192.0.2.10", "reserved": True}
    page = {"data": [reserved], "page": 2, "pages": 2, "results": 1}
    pricing = {"data": [{"id": "reserved-ipv4"}], "page": 1, "pages": 1}

    with patch.object(client, "make_request", new_callable=AsyncMock) as request:
        request.side_effect = [_response(reserved), _response(page), _response(pricing)]

        assert await client.create_reserved_ip("us-east", ["web", "prod"]) == reserved
        assert await client.list_reserved_ips(page=2, page_size=50) == page
        assert await client.list_reserved_ip_types() == pricing

    assert request.await_args_list[0].args == (
        "POST",
        "/networking/reserved/ips",
        {"region": "us-east", "tags": ["web", "prod"]},
    )
    assert request.await_args_list[1].args == (
        "GET",
        "/networking/reserved/ips?page=2&page_size=50",
    )
    assert request.await_args_list[2].args == (
        "GET",
        "/networking/reserved/ips/types",
    )
    await client.close()


async def test_reserved_ip_create_omits_optional_tags() -> None:
    """The reserve body omits tags when the caller does not provide them."""
    client = Client("https://api.linode.com/v4", "test-token")
    with patch.object(client, "make_request", new_callable=AsyncMock) as request:
        request.return_value = _response({"address": "192.0.2.10"})
        await client.create_reserved_ip("us-east")
    request.assert_awaited_once_with(
        "POST", "/networking/reserved/ips", {"region": "us-east"}
    )
    await client.close()


async def test_reserved_ip_client_uses_exact_address_routes() -> None:
    """Get, update, and delete use the exact reserved-IP address route."""
    client = Client("https://api.linode.com/v4", "test-token")
    reserved = {"address": "192.0.2.10", "tags": ["prod"]}
    with patch.object(client, "make_request", new_callable=AsyncMock) as request:
        request.side_effect = [_response(reserved), _response(reserved), _response({})]

        assert await client.get_reserved_ip("192.0.2.10") == reserved
        assert await client.update_reserved_ip("192.0.2.10", ["prod"]) == reserved
        await client.delete_reserved_ip("192.0.2.10")

    assert request.await_args_list[0].args == (
        "GET",
        "/networking/reserved/ips/192.0.2.10",
    )
    assert request.await_args_list[1].args == (
        "PUT",
        "/networking/reserved/ips/192.0.2.10",
        {"tags": ["prod"]},
    )
    assert request.await_args_list[2].args == (
        "DELETE",
        "/networking/reserved/ips/192.0.2.10",
    )
    await client.close()


@pytest.mark.parametrize(
    "method", ["get_reserved_ip", "update_reserved_ip", "delete_reserved_ip"]
)
async def test_reserved_ip_client_fully_encodes_address_path(method: str) -> None:
    """Every address method quotes separators and query characters at the boundary."""
    client = Client("https://api.linode.com/v4", "test-token")
    malicious = "192.0.2.10/../../other?x=1"
    with patch.object(client, "make_request", new_callable=AsyncMock) as request:
        request.return_value = _response({})
        if method == "update_reserved_ip":
            await client.update_reserved_ip(malicious, [])
        else:
            await getattr(client, method)(malicious)

    assert request.await_args is not None
    assert request.await_args.args[1] == (
        "/networking/reserved/ips/192.0.2.10%2F..%2F..%2Fother%3Fx%3D1"
    )
    await client.close()


async def test_reserved_ip_client_maps_http_errors() -> None:
    """HTTP failures include the reserved-IP operation name."""
    client = Client("https://api.linode.com/v4", "test-token")
    with patch.object(client, "make_request", new_callable=AsyncMock) as request:
        request.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError, match="GetReservedIP"):
            await client.get_reserved_ip("192.0.2.10")
    await client.close()


@pytest.mark.parametrize(
    ("method", "args"),
    [
        ("create_reserved_ip", ("us-east", ["prod"])),
        ("update_reserved_ip", ("192.0.2.10", ["prod"])),
        ("delete_reserved_ip", ("192.0.2.10",)),
    ],
)
async def test_reserved_ip_mutations_are_not_replayed(
    method: str, args: tuple[object, ...]
) -> None:
    """POST, PUT, and DELETE delegate once after a transient error."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    with (
        patch.object(
            retryable.client,
            method,
            new=AsyncMock(side_effect=httpx.HTTPError("transient")),
        ) as mutation,
        pytest.raises(httpx.HTTPError, match="transient"),
    ):
        await getattr(retryable, method)(*args)
    mutation.assert_awaited_once_with(*args)
    await retryable.close()


@pytest.mark.parametrize(
    ("method", "args"),
    [
        ("list_reserved_ips", ()),
        ("get_reserved_ip", ("192.0.2.10",)),
        ("list_reserved_ip_types", ()),
    ],
)
async def test_reserved_ip_reads_use_retry_helper(
    method: str, args: tuple[object, ...]
) -> None:
    """All three read routes pass through bounded retry execution."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    with patch.object(
        retryable, "_execute_with_retry", new_callable=AsyncMock
    ) as execute:
        execute.return_value = {"data": []}
        await getattr(retryable, method)(*args)
    execute.assert_awaited_once()
    await retryable.close()
