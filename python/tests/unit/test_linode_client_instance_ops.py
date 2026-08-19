"""Behavioral tests for previously-untested instance-ops and top-level Client reads.

Groups together the whole untested method bodies for instance backups, disks,
IPs, the destructive delete/shutdown paths, account/regions/types/volumes/images
reads, SSH key create/delete, the IPv6 range create/list, and the NodeBalancer
config create validation. Each pair asserts the outgoing HTTP verb + endpoint,
the decoded return, and NetworkError wrapping on httpx failures. The delete and
shutdown paths catch several httpx subclasses; the timeout cases below exercise
the ConnectTimeout branch in addition to the base HTTPError branch.
"""

from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import (
    Account,
    Client,
    NetworkError,
)

pytestmark = pytest.mark.asyncio


def _ok_response(body: Any) -> MagicMock:
    """Build a mock httpx response whose json() returns body."""
    response = MagicMock()
    response.status_code = 200
    response.json.return_value = body
    return response


async def test_get_account_parses_account() -> None:
    """get_account GETs /account and parses into an Account dataclass."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {
        "first_name": "Ada",
        "last_name": "Lovelace",
        "email": "ada@example.com",
        "balance": 12.5,
        "capabilities": ["Linodes", "Object Storage"],
        "active_promotions": [
            {"description": "Free credit", "credit_remaining": "5.00"}
        ],
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.get_account()

    assert isinstance(result, Account)
    assert result.email == "ada@example.com"
    assert result.balance == 12.5
    assert len(result.active_promotions) == 1
    assert result.active_promotions[0].description == "Free credit"
    mock_request.assert_awaited_once_with("GET", "/account")
    await client.close()


async def test_get_account_wraps_http_errors() -> None:
    """get_account wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account()

    assert "GetAccount" in str(excinfo.value)
    await client.close()


async def test_list_instance_disks_unwraps_data() -> None:
    """list_instance_disks GETs the disks route and unwraps the data list."""
    client = Client("https://api.linode.com/v4", "test-token")
    disks = [{"id": 700, "label": "boot", "size": 25600}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": disks})

        result = await client.list_instance_disks(555)

    assert result == disks
    mock_request.assert_awaited_once_with("GET", "/linode/instances/555/disks")
    await client.close()


async def test_list_instance_disks_wraps_http_errors() -> None:
    """list_instance_disks wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_instance_disks(555)

    assert "ListInstanceDisks" in str(excinfo.value)
    await client.close()


async def test_get_instance_disk_returns_body() -> None:
    """get_instance_disk GETs the disk route and returns the body."""
    client = Client("https://api.linode.com/v4", "test-token")
    disk = {"id": 700, "label": "boot", "size": 25600}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(disk)

        result = await client.get_instance_disk(555, 700)

    assert result == disk
    mock_request.assert_awaited_once_with("GET", "/linode/instances/555/disks/700")
    await client.close()


async def test_get_instance_disk_wraps_http_errors() -> None:
    """get_instance_disk wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_instance_disk(555, 700)

    assert "GetInstanceDisk" in str(excinfo.value)
    await client.close()


async def test_list_instance_ips_returns_body() -> None:
    """list_instance_ips GETs the ips route and returns the raw body."""
    client = Client("https://api.linode.com/v4", "test-token")
    body: dict[str, Any] = {"ipv4": {"public": []}, "ipv6": {"slaac": None}}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.list_instance_ips(555)

    assert result == body
    mock_request.assert_awaited_once_with("GET", "/linode/instances/555/ips")
    await client.close()


async def test_list_instance_ips_wraps_http_errors() -> None:
    """list_instance_ips wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_instance_ips(555)

    assert "ListInstanceIPs" in str(excinfo.value)
    await client.close()


_VALID_SSH_KEY = "ssh-ed25519 " + ("A" * 80) + " user@host"
