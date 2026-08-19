"""Low-level client coverage for reserved public IPv4 routes."""

from __future__ import annotations

from unittest.mock import AsyncMock, patch

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient


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
        ("get_reserved_ip", ("192.0.2.10",)),
    ],
)
async def test_reserved_ip_reads_use_retry_helper(
    method: str, args: tuple[object, ...]
) -> None:
    """The surviving read route passes through bounded retry execution."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    with patch.object(
        retryable, "_execute_with_retry", new_callable=AsyncMock
    ) as execute:
        execute.return_value = {"data": []}
        await getattr(retryable, method)(*args)
    execute.assert_awaited_once()
    await retryable.close()
