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
    Image,
    InstanceType,
    NetworkError,
    Region,
    SSHKey,
    Volume,
)

pytestmark = pytest.mark.asyncio


def _ok_response(body: Any) -> MagicMock:
    """Build a mock httpx response whose json() returns body."""
    response = MagicMock()
    response.status_code = 200
    response.json.return_value = body
    return response


# --- Account / regions / types / volumes / images (typed reads) ------------


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


async def test_list_regions_parses_each_region() -> None:
    """list_regions GETs /regions and parses each entry into a Region."""
    client = Client("https://api.linode.com/v4", "test-token")
    body: dict[str, Any] = {
        "data": [{"id": "us-east", "country": "us", "capabilities": []}]
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.list_regions()

    assert len(result) == 1
    assert isinstance(result[0], Region)
    assert result[0].id == "us-east"
    mock_request.assert_awaited_once_with("GET", "/regions")
    await client.close()


async def test_list_regions_wraps_http_errors() -> None:
    """list_regions wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_regions()

    assert "ListRegions" in str(excinfo.value)
    await client.close()


async def test_list_types_parses_each_type() -> None:
    """list_types GETs /linode/types and parses each entry into InstanceType."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {
        "data": [
            {
                "id": "g6-nanode-1",
                "label": "Nanode 1GB",
                "memory": 1024,
                "vcpus": 1,
                "disk": 25600,
            }
        ]
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.list_types()

    assert len(result) == 1
    assert isinstance(result[0], InstanceType)
    assert result[0].id == "g6-nanode-1"
    mock_request.assert_awaited_once_with("GET", "/linode/types")
    await client.close()


async def test_list_types_wraps_http_errors() -> None:
    """list_types wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_types()

    assert "ListTypes" in str(excinfo.value)
    await client.close()


async def test_list_volumes_parses_each_volume() -> None:
    """list_volumes GETs /volumes and parses each entry into a Volume."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {
        "data": [
            {
                "id": 900,
                "label": "data-vol",
                "status": "active",
                "size": 20,
                "region": "us-east",
            }
        ]
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.list_volumes()

    assert len(result) == 1
    assert isinstance(result[0], Volume)
    assert result[0].id == 900
    mock_request.assert_awaited_once_with("GET", "/volumes")
    await client.close()


async def test_list_volumes_wraps_http_errors() -> None:
    """list_volumes wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_volumes()

    assert "ListVolumes" in str(excinfo.value)
    await client.close()


async def test_list_images_parses_each_image() -> None:
    """list_images GETs /images and parses each entry into an Image."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {
        "data": [
            {
                "id": "private/123",
                "label": "gold-master",
                "status": "available",
                "size": 1500,
            }
        ]
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.list_images()

    assert len(result) == 1
    assert isinstance(result[0], Image)
    assert result[0].id == "private/123"
    mock_request.assert_awaited_once_with("GET", "/images")
    await client.close()


async def test_list_images_wraps_http_errors() -> None:
    """list_images wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_images()

    assert "ListImages" in str(excinfo.value)
    await client.close()


async def test_delete_image_encodes_id_path() -> None:
    """delete_image DELETEs /images/<encoded id>; the slash is percent-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_image("private/123")

    mock_request.assert_awaited_once_with("DELETE", "/images/private%2F123")
    await client.close()


async def test_delete_image_wraps_http_errors() -> None:
    """delete_image wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_image("private/123")

    assert "DeleteImage" in str(excinfo.value)
    await client.close()


# --- Instance backups ------------------------------------------------------


async def test_list_instance_backups_returns_body() -> None:
    """list_instance_backups GETs the backups route and returns the raw body."""
    client = Client("https://api.linode.com/v4", "test-token")
    body: dict[str, Any] = {
        "automatic": [],
        "snapshot": {"current": None, "in_progress": None},
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.list_instance_backups(555)

    assert result == body
    mock_request.assert_awaited_once_with("GET", "/linode/instances/555/backups")
    await client.close()


async def test_list_instance_backups_wraps_http_errors() -> None:
    """list_instance_backups wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_instance_backups(555)

    assert "ListInstanceBackups" in str(excinfo.value)
    await client.close()


async def test_create_instance_backup_omits_label_when_none() -> None:
    """create_instance_backup POSTs an empty body when no label is given."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"id": 1})

        await client.create_instance_backup(555)

    mock_request.assert_awaited_once_with("POST", "/linode/instances/555/backups", {})
    await client.close()


async def test_create_instance_backup_includes_label_when_set() -> None:
    """create_instance_backup POSTs the label field when supplied."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"id": 1})

        await client.create_instance_backup(555, label="pre-upgrade")

    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/555/backups"
    assert sent == {"label": "pre-upgrade"}
    await client.close()


async def test_create_instance_backup_wraps_http_errors() -> None:
    """create_instance_backup wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_instance_backup(555)

    assert "CreateInstanceBackup" in str(excinfo.value)
    await client.close()


async def test_enable_instance_backups_posts_enable() -> None:
    """enable_instance_backups POSTs to the backups enable subroute."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.enable_instance_backups(555)

    mock_request.assert_awaited_once_with(
        "POST", "/linode/instances/555/backups/enable"
    )
    await client.close()


async def test_enable_instance_backups_wraps_http_errors() -> None:
    """enable_instance_backups wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.enable_instance_backups(555)

    assert "EnableInstanceBackups" in str(excinfo.value)
    await client.close()


async def test_cancel_instance_backups_posts_cancel() -> None:
    """cancel_instance_backups POSTs to the backups cancel subroute."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.cancel_instance_backups(555)

    mock_request.assert_awaited_once_with(
        "POST", "/linode/instances/555/backups/cancel"
    )
    await client.close()


async def test_cancel_instance_backups_wraps_http_errors() -> None:
    """cancel_instance_backups wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.cancel_instance_backups(555)

    assert "CancelInstanceBackups" in str(excinfo.value)
    await client.close()


# --- Instance disks --------------------------------------------------------


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


async def test_delete_instance_disk_sends_delete() -> None:
    """delete_instance_disk DELETEs the disk route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_instance_disk(555, 700)

    mock_request.assert_awaited_once_with("DELETE", "/linode/instances/555/disks/700")
    await client.close()


async def test_delete_instance_disk_wraps_http_errors() -> None:
    """delete_instance_disk wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_instance_disk(555, 700)

    assert "DeleteInstanceDisk" in str(excinfo.value)
    await client.close()


# --- Instance IPs + password reset -----------------------------------------


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


async def test_reset_instance_password_posts_root_pass() -> None:
    """reset_instance_password POSTs the root_pass body to the password route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.reset_instance_password(555, "NewSecret123")

    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/555/password"
    assert sent == {"root_pass": "NewSecret123"}
    await client.close()


async def test_reset_instance_password_wraps_http_errors() -> None:
    """reset_instance_password wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.reset_instance_password(555, "NewSecret123")

    assert "ResetInstancePassword" in str(excinfo.value)
    await client.close()


# --- Destructive delete / shutdown (multi-except handlers) -----------------


async def test_shutdown_instance_posts_shutdown() -> None:
    """shutdown_instance POSTs to the shutdown subroute."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.shutdown_instance(555)

    mock_request.assert_awaited_once_with("POST", "/linode/instances/555/shutdown")
    await client.close()


async def test_shutdown_instance_wraps_timeout() -> None:
    """shutdown_instance wraps a ConnectTimeout into NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ConnectTimeout("slow")

        with pytest.raises(NetworkError) as excinfo:
            await client.shutdown_instance(555)

    assert "ShutdownInstance" in str(excinfo.value)
    await client.close()


async def test_delete_instance_sends_delete() -> None:
    """delete_instance DELETEs the instance route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_instance(555)

    mock_request.assert_awaited_once_with("DELETE", "/linode/instances/555")
    await client.close()


async def test_delete_instance_wraps_timeout() -> None:
    """delete_instance wraps a ReadTimeout into NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("slow")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_instance(555)

    assert "DeleteInstance" in str(excinfo.value)
    await client.close()


async def test_delete_instance_wraps_http_status_error() -> None:
    """delete_instance wraps an HTTPStatusError into NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")
    request = httpx.Request("DELETE", "https://api.linode.com/v4/linode/instances/555")
    response = httpx.Response(404, request=request)

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPStatusError(
            "not found", request=request, response=response
        )

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_instance(555)

    assert "DeleteInstance" in str(excinfo.value)
    await client.close()


async def test_delete_firewall_sends_delete() -> None:
    """delete_firewall DELETEs the firewall route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_firewall(88)

    mock_request.assert_awaited_once_with("DELETE", "/networking/firewalls/88")
    await client.close()


async def test_delete_firewall_wraps_http_errors() -> None:
    """delete_firewall wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_firewall(88)

    assert "DeleteFirewall" in str(excinfo.value)
    await client.close()


async def test_delete_domain_sends_delete() -> None:
    """delete_domain DELETEs the domain route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_domain(4242)

    mock_request.assert_awaited_once_with("DELETE", "/domains/4242")
    await client.close()


async def test_delete_domain_wraps_http_errors() -> None:
    """delete_domain wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_domain(4242)

    assert "DeleteDomain" in str(excinfo.value)
    await client.close()


async def test_detach_volume_posts_detach() -> None:
    """detach_volume POSTs to the volume detach subroute."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.detach_volume(900)

    mock_request.assert_awaited_once_with("POST", "/volumes/900/detach")
    await client.close()


async def test_detach_volume_wraps_http_errors() -> None:
    """detach_volume wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.detach_volume(900)

    assert "DetachVolume" in str(excinfo.value)
    await client.close()


async def test_delete_volume_sends_delete() -> None:
    """delete_volume DELETEs the volume route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_volume(900)

    mock_request.assert_awaited_once_with("DELETE", "/volumes/900")
    await client.close()


async def test_delete_volume_wraps_http_errors() -> None:
    """delete_volume wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_volume(900)

    assert "DeleteVolume" in str(excinfo.value)
    await client.close()


# --- SSH keys --------------------------------------------------------------

_VALID_SSH_KEY = "ssh-ed25519 " + ("A" * 80) + " user@host"


async def test_create_ssh_key_posts_and_parses() -> None:
    """create_ssh_key validates, POSTs, and parses into an SSHKey."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {
        "id": 33,
        "label": "laptop",
        "ssh_key": _VALID_SSH_KEY,
        "created": "2026-01-01T00:00:00",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.create_ssh_key("laptop", _VALID_SSH_KEY)

    assert isinstance(result, SSHKey)
    assert result.id == 33
    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/profile/sshkeys"
    assert sent == {"label": "laptop", "ssh_key": _VALID_SSH_KEY}
    await client.close()


async def test_create_ssh_key_rejects_bad_key_before_request() -> None:
    """create_ssh_key raises ValueError for a malformed key, no request sent."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="SSH key"),
    ):
        await client.create_ssh_key("laptop", "not-a-key")

    mock_request.assert_not_awaited()
    await client.close()


async def test_create_ssh_key_wraps_http_errors() -> None:
    """create_ssh_key wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_ssh_key("laptop", _VALID_SSH_KEY)

    assert "CreateSSHKey" in str(excinfo.value)
    await client.close()


async def test_delete_ssh_key_sends_delete() -> None:
    """delete_ssh_key DELETEs the sshkeys route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_ssh_key(33)

    mock_request.assert_awaited_once_with("DELETE", "/profile/sshkeys/33")
    await client.close()


async def test_delete_ssh_key_wraps_http_errors() -> None:
    """delete_ssh_key wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_ssh_key(33)

    assert "DeleteSSHKey" in str(excinfo.value)
    await client.close()


# --- IPv6 ranges -----------------------------------------------------------


async def test_create_ipv6_range_omits_optional_fields() -> None:
    """create_ipv6_range POSTs only prefix_length when nothing else is set."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"range": "2600:3c00::/64"})

        await client.create_ipv6_range(64)

    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/networking/ipv6/ranges"
    assert sent == {"prefix_length": 64}
    await client.close()


async def test_create_ipv6_range_includes_linode_and_route() -> None:
    """create_ipv6_range sends linode_id and route_target when provided."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"range": "2600:3c00::/64"})

        await client.create_ipv6_range(64, linode_id=555, route_target="2600::1")

    sent = mock_request.await_args_list[0].args[2]
    assert sent == {
        "prefix_length": 64,
        "linode_id": 555,
        "route_target": "2600::1",
    }
    await client.close()


async def test_create_ipv6_range_wraps_http_errors() -> None:
    """create_ipv6_range wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_ipv6_range(64)

    assert "CreateIPv6Range" in str(excinfo.value)
    await client.close()


async def test_list_ipv6_ranges_no_params_omits_query() -> None:
    """list_ipv6_ranges GETs the bare route when no paging args are given."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {"data": [{"range": "2600:3c00::/64"}], "page": 1}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.list_ipv6_ranges()

    assert result == body
    mock_request.assert_awaited_once_with("GET", "/networking/ipv6/ranges")
    await client.close()


async def test_list_ipv6_ranges_builds_paging_query() -> None:
    """list_ipv6_ranges appends the page and page_size query params."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": []})

        await client.list_ipv6_ranges(page=2, page_size=50)

    method, endpoint = mock_request.await_args_list[0].args
    assert method == "GET"
    assert endpoint == "/networking/ipv6/ranges?page=2&page_size=50"
    await client.close()


async def test_list_ipv6_ranges_wraps_http_errors() -> None:
    """list_ipv6_ranges wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_ipv6_ranges()

    assert "ListIPv6Ranges" in str(excinfo.value)
    await client.close()


# --- NodeBalancer config create (validation + happy path) ------------------


async def test_create_nodebalancer_config_posts_fields() -> None:
    """create_nodebalancer_config POSTs the fields to the configs route."""
    client = Client("https://api.linode.com/v4", "test-token")
    fields = {"port": 80, "protocol": "http"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"id": 5, "port": 80})

        result = await client.create_nodebalancer_config(1234, fields)

    assert result == {"id": 5, "port": 80}
    mock_request.assert_awaited_once_with("POST", "/nodebalancers/1234/configs", fields)
    await client.close()


async def test_create_nodebalancer_config_rejects_bad_id() -> None:
    """create_nodebalancer_config rejects a non-positive id before any request."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="positive integer"),
    ):
        await client.create_nodebalancer_config(0, {"port": 80})

    mock_request.assert_not_awaited()
    await client.close()


async def test_create_nodebalancer_config_wraps_http_errors() -> None:
    """create_nodebalancer_config wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_nodebalancer_config(1234, {"port": 80})

    assert "CreateNodeBalancerConfig" in str(excinfo.value)
    await client.close()
